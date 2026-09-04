package evaluation

import (
	"context"
	"strings"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
)

type retrievalStub struct {
	requests []contracts.RetrievalRequest
	result   contracts.RetrievalResult
}

func (s *retrievalStub) Retrieve(_ context.Context, request contracts.RetrievalRequest) (contracts.RetrievalResult, error) {
	s.requests = append(s.requests, request)
	return s.result, nil
}

type runeCounter struct{}

func (runeCounter) Count(text string) (int, error) { return len([]rune(text)), nil }

func TestRunnerUsesProductionRetrievalContract(t *testing.T) {
	retrieval := &retrievalStub{
		result: contracts.RetrievalResult{
			Items: []contracts.RetrievalItem{
				{
					ChunkID: "c1", DocumentID: "d1", Content: "命中正文", Score: .9,
					SourceLocation: map[string]any{"source_spans": []any{map[string]any{
						"start_byte": float64(10), "end_byte": float64(20),
						"sources": []any{map[string]any{"block_id": "b1", "page": float64(1)}},
					}}},
				},
			},
		},
	}
	runner := NewRunner(retrieval, runeCounter{})
	dataset := GoldDataset{SchemaVersion: DatasetSchemaVersion, Name: "最小基线", Cases: []GoldCase{{
		ID: "q1", Question: "正文是什么？", ExpectedAnswerable: true,
		RelevantSources: []RelevantSource{{DocumentID: "d1", StartByte: 12, EndByte: 18}},
	}}}
	result, err := runner.Run(context.Background(), dataset, RunConfig{
		UserID: "u1", KnowledgeBaseID: "kb1", Mode: contracts.RetrievalKeyword, Ks: []int{1, 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(retrieval.requests) != 1 || retrieval.requests[0].TopK != 5 || retrieval.requests[0].Query != "正文是什么？" {
		t.Fatalf("生产检索请求错误: %+v", retrieval.requests)
	}
	if result.Report.RecallAtK[1] != 1 || result.Report.SourceSpanHitRate != 1 {
		t.Fatalf("真实检索适配后指标异常: %+v", result.Report)
	}
	if result.Ranked["q1"][0].TokenCount != 4 || len(result.Ranked["q1"][0].SourceSpans) != 1 {
		t.Fatalf("RankedHit 适配异常: %+v", result.Ranked)
	}
}

func TestLoadDatasetRejectsUnknownFields(t *testing.T) {
	_, err := LoadDataset(strings.NewReader(`{
  "schema_version":"retrieval-gold-v1",
  "name":"示例",
  "cases":[{"id":"q1","question":"问题","relevant_sources":[],"expected_answerable":false,"typo":true}]
}`))
	if err == nil {
		t.Fatal("未知字段应被拒绝")
	}
}

func TestValidateDatasetRejectsInvalidByteRange(t *testing.T) {
	err := ValidateDataset(GoldDataset{SchemaVersion: DatasetSchemaVersion, Name: "错误样例", Cases: []GoldCase{{
		ID: "q1", Question: "问题", RelevantSources: []RelevantSource{{DocumentID: "d1", StartByte: 20, EndByte: 10}},
	}}})
	if err == nil {
		t.Fatal("非法 byte 区间应被拒绝")
	}
}
