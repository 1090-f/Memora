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
	if report.HitRateAtK[1] != .5 || report.HitRateAtK[2] != 1 || report.PrecisionAtK[2] != .5 {
		t.Fatalf("HitRate/Precision 异常: hit=%+v precision=%+v", report.HitRateAtK, report.PrecisionAtK)
	}
	if math.Abs(report.MRR-.75) > 1e-9 {
		t.Fatalf("MRR = %f, want .75", report.MRR)
	}
	if report.SourceSpanHitRate != 1 || report.MedianChunkTokens != 200 || report.P95ChunkTokens != 300 {
		t.Fatalf("来源/Token 指标异常: %+v", report)
	}
	if len(report.Cases) != 2 || report.Cases[0].FirstRelevantRank != 2 || report.Cases[0].SourceSpanHit == nil || !*report.Cases[0].SourceSpanHit {
		t.Fatalf("逐题诊断异常: %+v", report.Cases)
	}
}

func TestCompareReports(t *testing.T) {
	baseline := Report{RecallAtK: map[int]float64{5: .7}, HitRateAtK: map[int]float64{5: .8}, PrecisionAtK: map[int]float64{5: .3}, NDCGAtK: map[int]float64{5: .5}, MRR: .4, SourceSpanHitRate: .6, AcceptedContextFalsePositiveRate: .2}
	candidate := Report{RecallAtK: map[int]float64{5: .8}, HitRateAtK: map[int]float64{5: .9}, PrecisionAtK: map[int]float64{5: .4}, NDCGAtK: map[int]float64{5: .6}, MRR: .5, SourceSpanHitRate: .75, AcceptedContextFalsePositiveRate: .1}
	delta := Compare(baseline, candidate)
	if math.Abs(delta.RecallDeltaAtK[5]-.1) > 1e-9 || math.Abs(delta.SpanHitDelta-.15) > 1e-9 {
		t.Fatalf("paired comparison 异常: %+v", delta)
	}
	if math.Abs(delta.HitRateDeltaAtK[5]-.1) > 1e-9 || math.Abs(delta.PrecisionDeltaAtK[5]-.1) > 1e-9 || math.Abs(delta.AcceptedContextFalsePositiveDelta+.1) > 1e-9 {
		t.Fatalf("新增 paired comparison 异常: %+v", delta)
	}
}

func TestEvaluateUnanswerableFalsePositiveRate(t *testing.T) {
	cases := []GoldCase{{ID: "empty-1"}, {ID: "empty-2"}}
	ranked := map[string][]RankedHit{"empty-1": {{DocumentID: "noise"}}}
	report := Evaluate(cases, ranked, []int{5})
	if report.UnanswerableCaseCount != 2 || report.UnanswerableFalsePositiveRate != .5 {
		t.Fatalf("不可回答问题指标异常: %+v", report)
	}
	if report.AcceptedContextFalsePositiveRate != .5 || !report.Cases[0].AcceptedFalsePositive {
		t.Fatalf("最终接受上下文误召指标异常: %+v", report)
	}
}

func TestNDCGUsesGradedRelevance(t *testing.T) {
	cases := []GoldCase{{ID: "q1", ExpectedAnswerable: true, RelevantSources: []RelevantSource{
		{DocumentID: "high", Relevance: 3}, {DocumentID: "low", Relevance: 1},
	}}}
	ideal := Evaluate(cases, map[string][]RankedHit{"q1": {{DocumentID: "high"}, {DocumentID: "low"}}}, []int{2})
	reversed := Evaluate(cases, map[string][]RankedHit{"q1": {{DocumentID: "low"}, {DocumentID: "high"}}}, []int{2})
	if ideal.NDCGAtK[2] != 1 || reversed.NDCGAtK[2] <= 0 || reversed.NDCGAtK[2] >= 1 {
		t.Fatalf("分级 nDCG 异常: ideal=%f reversed=%f", ideal.NDCGAtK[2], reversed.NDCGAtK[2])
	}
}

func TestEvaluatePortableTitleAndSourceTextGold(t *testing.T) {
	cases := []GoldCase{
		{ID: "q1", ExpectedAnswerable: true, RelevantSources: []RelevantSource{
			{DocumentTitle: "查询.md", SourceText: "零值默认会被忽略", Relevance: 3},
		}},
	}
	ranked := map[string][]RankedHit{"q1": {{DocumentTitle: "查询", Content: "结构体条件中的零值默认会被忽略。"}}}
	report := Evaluate(cases, ranked, []int{1})
	if report.HitRateAtK[1] != 1 || report.SourceTextHitRate != 1 || report.Cases[0].SourceTextHit == nil || !*report.Cases[0].SourceTextHit {
		t.Fatalf("标题/证据原文金标命中异常: %+v", report)
	}
}
