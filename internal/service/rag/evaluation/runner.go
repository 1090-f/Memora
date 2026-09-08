package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service/rag/canonical"
	"github.com/google/uuid"
)

const DatasetSchemaVersion = "retrieval-gold-v1"

// GoldDataset 是可落盘、可版本控制的离线检索金标格式。
type GoldDataset struct {
	SchemaVersion string            `json:"schema_version"`
	Name          string            `json:"name"`
	Version       string            `json:"version,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Cases         []GoldCase        `json:"cases"`
}

// RunConfig 描述一次真实检索基线运行；TopK 小于最大 K 时会自动提升。
type RunConfig struct {
	UserID               contracts.ID
	KnowledgeBaseID      contracts.ID
	Mode                 contracts.RetrievalMode
	DocumentIDs          []contracts.ID
	TopK                 int
	Ks                   []int
	SearchConfig         contracts.SearchConfig
	OverrideSearchConfig bool
	ExperimentName       string
	GitCommit            string
	Labels               map[string]string
}

type RunResult struct {
	Experiment ExperimentMetadata     `json:"experiment"`
	Report     Report                 `json:"report"`
	Ranked     map[string][]RankedHit `json:"ranked"`
	Comparison *PairedComparison      `json:"comparison,omitempty"`
}

// ExperimentMetadata 固化一次评估的身份、数据集版本和实际生效配置。
type ExperimentMetadata struct {
	ID                   string                  `json:"id"`
	Name                 string                  `json:"name,omitempty"`
	DatasetName          string                  `json:"dataset_name"`
	DatasetVersion       string                  `json:"dataset_version,omitempty"`
	DatasetSchemaVersion string                  `json:"dataset_schema_version"`
	StartedAt            time.Time               `json:"started_at"`
	CompletedAt          time.Time               `json:"completed_at"`
	DurationMS           int64                   `json:"duration_ms"`
	UserID               contracts.ID            `json:"user_id"`
	KnowledgeBaseID      contracts.ID            `json:"knowledge_base_id"`
	Mode                 contracts.RetrievalMode `json:"mode"`
	TopK                 int                     `json:"top_k"`
	Ks                   []int                   `json:"ks"`
	SearchConfig         contracts.SearchConfig  `json:"search_config"`
	GitCommit            string                  `json:"git_commit,omitempty"`
	Labels               map[string]string       `json:"labels,omitempty"`
}

type TokenCounter interface {
	Count(text string) (int, error)
}

// Runner 通过生产 RetrievalService 执行真实 keyword/vector/hybrid 检索，
// 再将结果适配为后端无关的 RankedHit 交给 Evaluate。
type Runner struct {
	retrieval contracts.RetrievalService
	tokens    TokenCounter
}

func NewRunner(retrieval contracts.RetrievalService, tokens TokenCounter) *Runner {
	return &Runner{retrieval: retrieval, tokens: tokens}
}

func (r *Runner) Run(ctx context.Context, dataset GoldDataset, cfg RunConfig) (RunResult, error) {
	startedAt := time.Now().UTC()
	if r == nil || r.retrieval == nil {
		return RunResult{}, fmt.Errorf("离线评估缺少 RetrievalService")
	}
	if err := ValidateDataset(dataset); err != nil {
		return RunResult{}, err
	}
	if cfg.UserID == "" || cfg.KnowledgeBaseID == "" {
		return RunResult{}, fmt.Errorf("离线评估必须指定 user_id 与 knowledge_base_id")
	}
	ks := normalizeKs(cfg.Ks)
	if cfg.TopK < ks[len(ks)-1] {
		cfg.TopK = ks[len(ks)-1]
	}
	if cfg.Mode == "" {
		cfg.Mode = contracts.RetrievalHybrid
	}
	if cfg.OverrideSearchConfig && cfg.SearchConfig.KeywordTopK == 0 && cfg.SearchConfig.VectorTopK == 0 && cfg.SearchConfig.RRFTopK == 0 {
		cfg.SearchConfig = contracts.DefaultSearchConfig()
	}

	ranked := make(map[string][]RankedHit, len(dataset.Cases))
	effectiveSearchConfig := cfg.SearchConfig
	for _, gold := range dataset.Cases {
		var configOverride *contracts.SearchConfig
		if cfg.OverrideSearchConfig {
			value := cfg.SearchConfig
			configOverride = &value
		}
		result, err := r.retrieval.Retrieve(ctx, contracts.RetrievalRequest{
			UserID: cfg.UserID, KnowledgeBaseID: cfg.KnowledgeBaseID,
			Query: gold.Question, Mode: cfg.Mode, DocumentIDs: append([]contracts.ID(nil), cfg.DocumentIDs...),
			TopK: cfg.TopK, Config: cfg.SearchConfig, ConfigOverride: configOverride,
		})
		if err != nil {
			return RunResult{}, fmt.Errorf("评估用例 %s 检索失败: %w", gold.ID, err)
		}
		effectiveSearchConfig = result.SearchConfig
		if cfg.OverrideSearchConfig {
			effectiveSearchConfig = cfg.SearchConfig
		}
		hits := make([]RankedHit, 0, len(result.Items))
		for _, item := range result.Items {
			tokenCount := 0
			if r.tokens != nil {
				tokenCount, err = r.tokens.Count(item.Content)
				if err != nil {
					return RunResult{}, fmt.Errorf("评估用例 %s 统计 Chunk %s token 失败: %w", gold.ID, item.ChunkID, err)
				}
			}
			hits = append(hits, RankedHit{
				ChunkID: string(item.ChunkID), DocumentID: string(item.DocumentID), Score: item.Score,
				DocumentTitle: item.DocumentTitle, Content: item.Content,
				KeywordScore: item.KeywordScore, VectorScore: item.VectorScore, RerankerScore: item.RerankerScore,
				KeywordRank: item.KeywordRank, VectorRank: item.VectorRank, RRFRank: item.RRFRank, FinalRank: item.FinalRank,
				KeywordRecallStage: item.KeywordRecallStage, LowConfidence: item.LowConfidence,
				TokenCount: tokenCount, SourceSpans: decodeSourceSpans(item.SourceLocation),
			})
		}
		ranked[gold.ID] = hits
	}
	completedAt := time.Now().UTC()
	return RunResult{
		Experiment: ExperimentMetadata{
			ID: uuid.NewString(), Name: cfg.ExperimentName, DatasetName: dataset.Name,
			DatasetVersion: dataset.Version, DatasetSchemaVersion: dataset.SchemaVersion,
			StartedAt: startedAt, CompletedAt: completedAt, DurationMS: completedAt.Sub(startedAt).Milliseconds(),
			UserID: cfg.UserID, KnowledgeBaseID: cfg.KnowledgeBaseID, Mode: cfg.Mode,
			TopK: cfg.TopK, Ks: append([]int(nil), ks...), SearchConfig: effectiveSearchConfig,
			GitCommit: cfg.GitCommit, Labels: cloneStringMap(cfg.Labels),
		},
		Report: Evaluate(dataset.Cases, ranked, ks), Ranked: ranked,
	}, nil
}

// LoadDataset 读取严格的 JSON 金标文件；未知字段直接报错，避免拼写错误静默污染基线。
func LoadDataset(reader io.Reader) (GoldDataset, error) {
	if reader == nil {
		return GoldDataset{}, fmt.Errorf("金标数据 reader 不能为空")
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var dataset GoldDataset
	if err := decoder.Decode(&dataset); err != nil {
		return GoldDataset{}, fmt.Errorf("解析金标数据失败: %w", err)
	}
	if err := ValidateDataset(dataset); err != nil {
		return GoldDataset{}, err
	}
	return dataset, nil
}

func ValidateDataset(dataset GoldDataset) error {
	if dataset.SchemaVersion != DatasetSchemaVersion {
		return fmt.Errorf("不支持的金标 schema_version %q", dataset.SchemaVersion)
	}
	if strings.TrimSpace(dataset.Name) == "" {
		return fmt.Errorf("金标数据 name 不能为空")
	}
	if len(dataset.Cases) == 0 {
		return fmt.Errorf("金标数据至少需要一个 case")
	}
	seen := make(map[string]bool, len(dataset.Cases))
	for index, gold := range dataset.Cases {
		if strings.TrimSpace(gold.ID) == "" || strings.TrimSpace(gold.Question) == "" {
			return fmt.Errorf("金标 case[%d] 的 id/question 不能为空", index)
		}
		if seen[gold.ID] {
			return fmt.Errorf("金标 case id %q 重复", gold.ID)
		}
		seen[gold.ID] = true
		for sourceIndex, source := range gold.RelevantSources {
			if strings.TrimSpace(source.DocumentID) == "" && strings.TrimSpace(source.DocumentTitle) == "" {
				return fmt.Errorf("金标 case %s relevant_sources[%d] 必须提供 document_id 或 document_title", gold.ID, sourceIndex)
			}
			documentLevel := source.StartByte == 0 && source.EndByte == 0
			if source.StartByte < 0 || source.EndByte < 0 ||
				(!documentLevel && source.EndByte <= source.StartByte) {
				return fmt.Errorf("金标 case %s relevant_sources[%d] byte 区间非法", gold.ID, sourceIndex)
			}
			if source.Relevance < 0 || source.Relevance > 3 {
				return fmt.Errorf("金标 case %s relevant_sources[%d] relevance 必须为 0~3", gold.ID, sourceIndex)
			}
		}
		if gold.ExpectedAnswerable && len(gold.RelevantSources) == 0 {
			return fmt.Errorf("金标 case %s 标记为可回答但没有 relevant_sources", gold.ID)
		}
	}
	return nil
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]string, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func decodeSourceSpans(location map[string]any) []canonical.SourceSpan {
	if len(location) == 0 {
		return nil
	}
	value, ok := location["source_spans"]
	if !ok || value == nil {
		return nil
	}
	if spans, ok := value.([]canonical.SourceSpan); ok {
		return append([]canonical.SourceSpan(nil), spans...)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var spans []canonical.SourceSpan
	if err := json.Unmarshal(data, &spans); err != nil {
		return nil
	}
	return spans
}
