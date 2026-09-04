package evaluation

import (
	"math"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

func TestEvaluateRetrievalMetrics(t *testing.T) {
	cases := []GoldCase{
		{ID: "q1", Question: "问题一", ExpectedAnswerable: true, RelevantSources: []RelevantSource{{DocumentID: "d1", StartByte: 10, EndByte: 20}}},
		{ID: "q2", Question: "问题二", ExpectedAnswerable: true, RelevantSources: []RelevantSource{{DocumentID: "d2"}}},
	}
	ranked := map[string][]RankedHit{
		"q1": {
			{DocumentID: "noise", TokenCount: 100},
			{DocumentID: "d1", TokenCount: 200, SourceSpans: []canonical.SourceSpan{{StartByte: 15, EndByte: 25}}},
		},
		"q2": {{DocumentID: "d2", TokenCount: 300}},
	}
	report := Evaluate(cases, ranked, []int{1, 2})
	if report.RecallAtK[1] != .5 || report.RecallAtK[2] != 1 {
		t.Fatalf("Recall 异常: %+v", report.RecallAtK)
	}
	if math.Abs(report.MRR-.75) > 1e-9 {
		t.Fatalf("MRR = %f, want .75", report.MRR)
	}
	if report.SourceSpanHitRate != 1 || report.MedianChunkTokens != 200 || report.P95ChunkTokens != 300 {
		t.Fatalf("来源/Token 指标异常: %+v", report)
	}
}

func TestCompareReports(t *testing.T) {
	baseline := Report{RecallAtK: map[int]float64{5: .7}, NDCGAtK: map[int]float64{5: .5}, MRR: .4, SourceSpanHitRate: .6}
	candidate := Report{RecallAtK: map[int]float64{5: .8}, NDCGAtK: map[int]float64{5: .6}, MRR: .5, SourceSpanHitRate: .75}
	delta := Compare(baseline, candidate)
	if math.Abs(delta.RecallDeltaAtK[5]-.1) > 1e-9 || math.Abs(delta.SpanHitDelta-.15) > 1e-9 {
		t.Fatalf("paired comparison 异常: %+v", delta)
	}
}

func TestEvaluateUnanswerableFalsePositiveRate(t *testing.T) {
	cases := []GoldCase{{ID: "empty-1"}, {ID: "empty-2"}}
	ranked := map[string][]RankedHit{"empty-1": {{DocumentID: "noise"}}}
	report := Evaluate(cases, ranked, []int{5})
	if report.UnanswerableCaseCount != 2 || report.UnanswerableFalsePositiveRate != .5 {
		t.Fatalf("不可回答问题指标异常: %+v", report)
	}
}
