// Package evaluation 提供与具体检索后端解耦的离线金标评估。
package evaluation

import (
	"math"
	"sort"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

// GoldCase 是一条问题及其可接受来源。StartByte/EndByte 均为 Canonical UTF-8 byte offset；
// 二者都为 0 时只校验文档命中。
type GoldCase struct {
	ID                 string           `json:"id"`
	Question           string           `json:"question"`
	RelevantSources    []RelevantSource `json:"relevant_sources"`
	ExpectedAnswerable bool             `json:"expected_answerable"`
}

type RelevantSource struct {
	DocumentID string `json:"document_id"`
	StartByte  int    `json:"start_byte,omitempty"`
	EndByte    int    `json:"end_byte,omitempty"`
}

// RankedHit 是任意检索器输出到评估器的统一投影。
type RankedHit struct {
	ChunkID     string                 `json:"chunk_id"`
	DocumentID  string                 `json:"document_id"`
	Score       float64                `json:"score"`
	TokenCount  int                    `json:"token_count"`
	SourceSpans []canonical.SourceSpan `json:"source_spans,omitempty"`
}

type Report struct {
	CaseCount                     int             `json:"case_count"`
	AnswerableCaseCount           int             `json:"answerable_case_count"`
	UnanswerableCaseCount         int             `json:"unanswerable_case_count"`
	RecallAtK                     map[int]float64 `json:"recall_at_k"`
	MRR                           float64         `json:"mrr"`
	NDCGAtK                       map[int]float64 `json:"ndcg_at_k"`
	SourceSpanHitRate             float64         `json:"source_span_hit_rate"`
	UnanswerableFalsePositiveRate float64         `json:"unanswerable_false_positive_rate"`
	MedianChunkTokens             int             `json:"median_chunk_tokens"`
	P95ChunkTokens                int             `json:"p95_chunk_tokens"`
}

type PairedComparison struct {
	RecallDeltaAtK map[int]float64 `json:"recall_delta_at_k"`
	MRRDelta       float64         `json:"mrr_delta"`
	NDCGDeltaAtK   map[int]float64 `json:"ndcg_delta_at_k"`
	SpanHitDelta   float64         `json:"source_span_hit_delta"`
}

// Evaluate 计算 macro Recall@K、MRR、nDCG@K、精确 SourceSpan 命中率及候选 Chunk Token 分布。
func Evaluate(cases []GoldCase, ranked map[string][]RankedHit, ks []int) Report {
	ks = normalizeKs(ks)
	report := Report{CaseCount: len(cases), RecallAtK: map[int]float64{}, NDCGAtK: map[int]float64{}}
	if len(cases) == 0 {
		return report
	}
	maxK := ks[len(ks)-1]
	spanCases, spanHits := 0, 0
	falsePositives := 0
	var tokenCounts []int
	for _, gold := range cases {
		hits := ranked[gold.ID]
		answerable := gold.ExpectedAnswerable || len(gold.RelevantSources) > 0
		if !answerable {
			report.UnanswerableCaseCount++
			if len(hits) > 0 {
				falsePositives++
			}
			continue
		}
		report.AnswerableCaseCount++
		for _, k := range ks {
			report.RecallAtK[k] += recallAt(hits, gold.RelevantSources, k)
			report.NDCGAtK[k] += ndcgAt(hits, gold.RelevantSources, k)
		}
		report.MRR += reciprocalRank(hits, gold.RelevantSources)
		if hasPreciseSource(gold.RelevantSources) {
			spanCases++
			if anyPreciseHit(hits, gold.RelevantSources, maxK) {
				spanHits++
			}
		}
		for i := 0; i < len(hits) && i < maxK; i++ {
			if hits[i].TokenCount > 0 {
				tokenCounts = append(tokenCounts, hits[i].TokenCount)
			}
		}
	}
	for _, k := range ks {
		if report.AnswerableCaseCount > 0 {
			report.RecallAtK[k] /= float64(report.AnswerableCaseCount)
			report.NDCGAtK[k] /= float64(report.AnswerableCaseCount)
		}
	}
	if report.AnswerableCaseCount > 0 {
		report.MRR /= float64(report.AnswerableCaseCount)
	}
	if spanCases > 0 {
		report.SourceSpanHitRate = float64(spanHits) / float64(spanCases)
	}
	if report.UnanswerableCaseCount > 0 {
		report.UnanswerableFalsePositiveRate = float64(falsePositives) / float64(report.UnanswerableCaseCount)
	}
	sort.Ints(tokenCounts)
	report.MedianChunkTokens = percentile(tokenCounts, .5)
	report.P95ChunkTokens = percentile(tokenCounts, .95)
	return report
}

func Compare(baseline, candidate Report) PairedComparison {
	out := PairedComparison{
		RecallDeltaAtK: map[int]float64{}, NDCGDeltaAtK: map[int]float64{},
		MRRDelta:     candidate.MRR - baseline.MRR,
		SpanHitDelta: candidate.SourceSpanHitRate - baseline.SourceSpanHitRate,
	}
	for k, value := range candidate.RecallAtK {
		out.RecallDeltaAtK[k] = value - baseline.RecallAtK[k]
	}
	for k, value := range candidate.NDCGAtK {
		out.NDCGDeltaAtK[k] = value - baseline.NDCGAtK[k]
	}
	return out
}

func recallAt(hits []RankedHit, relevant []RelevantSource, k int) float64 {
	if len(relevant) == 0 {
		return 0
	}
	found := make(map[int]bool)
	for i := 0; i < len(hits) && i < k; i++ {
		for index, source := range relevant {
			if hitMatches(hits[i], source) {
				found[index] = true
			}
		}
	}
	return float64(len(found)) / float64(len(relevant))
}

func reciprocalRank(hits []RankedHit, relevant []RelevantSource) float64 {
	for i, hit := range hits {
		for _, source := range relevant {
			if hitMatches(hit, source) {
				return 1 / float64(i+1)
			}
		}
	}
	return 0
}

func ndcgAt(hits []RankedHit, relevant []RelevantSource, k int) float64 {
	if len(relevant) == 0 || k <= 0 {
		return 0
	}
	dcg := 0.0
	found := make(map[int]bool)
	for i := 0; i < len(hits) && i < k; i++ {
		matched := false
		for index, source := range relevant {
			if !found[index] && hitMatches(hits[i], source) {
				matched = true
				found[index] = true
				break
			}
		}
		if matched {
			dcg += 1 / math.Log2(float64(i+2))
		}
	}
	ideal := 0.0
	for i := 0; i < len(relevant) && i < k; i++ {
		ideal += 1 / math.Log2(float64(i+2))
	}
	return dcg / ideal
}

func hitMatches(hit RankedHit, relevant RelevantSource) bool {
	if hit.DocumentID != relevant.DocumentID {
		return false
	}
	if relevant.StartByte == 0 && relevant.EndByte == 0 {
		return true
	}
	for _, span := range hit.SourceSpans {
		if span.StartByte < relevant.EndByte && relevant.StartByte < span.EndByte {
			return true
		}
	}
	return false
}

func hasPreciseSource(sources []RelevantSource) bool {
	for _, source := range sources {
		if source.EndByte > source.StartByte {
			return true
		}
	}
	return false
}

func anyPreciseHit(hits []RankedHit, relevant []RelevantSource, k int) bool {
	for i := 0; i < len(hits) && i < k; i++ {
		for _, source := range relevant {
			if source.EndByte > source.StartByte && hitMatches(hits[i], source) {
				return true
			}
		}
	}
	return false
}

func normalizeKs(ks []int) []int {
	seen := make(map[int]bool)
	out := make([]int, 0, len(ks))
	for _, k := range ks {
		if k > 0 && !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	if len(out) == 0 {
		out = []int{5, 10, 20}
	}
	sort.Ints(out)
	return out
}

func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	index := int(math.Ceil(float64(len(sorted))*p)) - 1
	if index < 0 {
		index = 0
	}
	return sorted[index]
}
