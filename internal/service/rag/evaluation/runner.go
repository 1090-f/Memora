package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

const DatasetSchemaVersion = "retrieval-gold-v1"

// GoldDataset 是可落盘、可版本控制的离线检索金标格式。
type GoldDataset struct {
	SchemaVersion string     `json:"schema_version"`
	Name          string     `json:"name"`
	Cases         []GoldCase `json:"cases"`
}

// RunConfig 描述一次真实检索基线运行；TopK 小于最大 K 时会自动提升。
type RunConfig struct {
	UserID          contracts.ID
	KnowledgeBaseID contracts.ID
	Mode            contracts.RetrievalMode
	DocumentIDs     []contracts.ID
	TopK            int
	Ks              []int
	SearchConfig    contracts.SearchConfig
}

type RunResult struct {
	Report Report                 `json:"report"`
	Ranked map[string][]RankedHit `json:"ranked"`
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
	if cfg.SearchConfig.KeywordTopK == 0 && cfg.SearchConfig.VectorTopK == 0 && cfg.SearchConfig.RRFTopK == 0 {
		cfg.SearchConfig = contracts.DefaultSearchConfig()
	}

	ranked := make(map[string][]RankedHit, len(dataset.Cases))
	for _, gold := range dataset.Cases {
		result, err := r.retrieval.Retrieve(ctx, contracts.RetrievalRequest{
			UserID: cfg.UserID, KnowledgeBaseID: cfg.KnowledgeBaseID,
			Query: gold.Question, Mode: cfg.Mode, DocumentIDs: append([]contracts.ID(nil), cfg.DocumentIDs...),
			TopK: cfg.TopK, Config: cfg.SearchConfig,
		})
		if err != nil {
			return RunResult{}, fmt.Errorf("评估用例 %s 检索失败: %w", gold.ID, err)
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
				TokenCount: tokenCount, SourceSpans: decodeSourceSpans(item.SourceLocation),
			})
		}
		ranked[gold.ID] = hits
	}
	return RunResult{Report: Evaluate(dataset.Cases, ranked, ks), Ranked: ranked}, nil
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
			if strings.TrimSpace(source.DocumentID) == "" {
				return fmt.Errorf("金标 case %s relevant_sources[%d] 缺少 document_id", gold.ID, sourceIndex)
			}
			documentLevel := source.StartByte == 0 && source.EndByte == 0
			if source.StartByte < 0 || source.EndByte < 0 ||
				(!documentLevel && source.EndByte <= source.StartByte) {
				return fmt.Errorf("金标 case %s relevant_sources[%d] byte 区间非法", gold.ID, sourceIndex)
			}
		}
	}
	return nil
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
