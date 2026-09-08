// Package evaluation 提供与具体检索后端解耦的离线金标评估。
package evaluation

import (
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

// GoldCase 是一条问题及其可接受来源。StartByte/EndByte 均为 Canonical UTF-8 byte offset；
// 二者都为 0 时只校验文档命中。
type GoldCase struct {
	ID                 string           `json:"id"`
	Question           string           `json:"question"`
	RelevantSources    []RelevantSource `json:"relevant_sources"`
	ExpectedAnswerable bool             `json:"expected_answerable"`
	ReferenceAnswer    string           `json:"reference_answer,omitempty"`
	Tags               []string         `json:"tags,omitempty"`
	Difficulty         string           `json:"difficulty,omitempty"`
}

type RelevantSource struct {
	DocumentID    string `json:"document_id,omitempty"`
	DocumentTitle string `json:"document_title,omitempty"`
	SourceText    string `json:"source_text,omitempty"`
	StartByte     int    `json:"start_byte,omitempty"`
	EndByte       int    `json:"end_byte,omitempty"`
	// Relevance 是 1~3 的分级相关性；0 表示兼容旧金标并按 1 处理。
	Relevance           int    `json:"relevance,omitempty"`
	DocumentVersion     string `json:"document_version,omitempty"`
	DocumentFingerprint string `json:"document_fingerprint,omitempty"`
}

// RankedHit 是任意检索器输出到评估器的统一投影。
type RankedHit struct {
	ChunkID            string                 `json:"chunk_id"`
	DocumentID         string                 `json:"document_id"`
	DocumentTitle      string                 `json:"document_title,omitempty"`
	Content            string                 `json:"content,omitempty"`
	Score              float64                `json:"score"`
	KeywordScore       *float64               `json:"keyword_score,omitempty"`
	VectorScore        *float64               `json:"vector_score,omitempty"`
	RerankerScore      *float64               `json:"reranker_score,omitempty"`
	KeywordRank        *int                   `json:"keyword_rank,omitempty"`
	VectorRank         *int                   `json:"vector_rank,omitempty"`
	RRFRank            *int                   `json:"rrf_rank,omitempty"`
	FinalRank          *int                   `json:"final_rank,omitempty"`
	KeywordRecallStage string                 `json:"keyword_recall_stage,omitempty"`
	LowConfidence      bool                   `json:"low_confidence,omitempty"`
	TokenCount         int                    `json:"token_count"`
	SourceSpans        []canonical.SourceSpan `json:"source_spans,omitempty"`
}

type Report struct {
	CaseCount                        int             `json:"case_count"`
	AnswerableCaseCount              int             `json:"answerable_case_count"`
	UnanswerableCaseCount            int             `json:"unanswerable_case_count"`
	RecallAtK                        map[int]float64 `json:"recall_at_k"`
	HitRateAtK                       map[int]float64 `json:"hit_rate_at_k"`
	PrecisionAtK                     map[int]float64 `json:"precision_at_k"`
	MRR                              float64         `json:"mrr"`
	NDCGAtK                          map[int]float64 `json:"ndcg_at_k"`
	SourceSpanHitRate                float64         `json:"source_span_hit_rate"`
	SourceTextHitRate                float64         `json:"source_text_hit_rate"`
	UnanswerableFalsePositiveRate    float64         `json:"unanswerable_false_positive_rate"`
	AcceptedContextFalsePositiveRate float64         `json:"accepted_context_false_positive_rate"`
	MedianChunkTokens                int             `json:"median_chunk_tokens"`
	P95ChunkTokens                   int             `json:"p95_chunk_tokens"`
	Cases                            []CaseReport    `json:"cases"`
}

// CaseReport 保留逐题诊断数据，使总体回归可以追溯到具体失败样例。
type CaseReport struct {
	CaseID                string          `json:"case_id"`
	Answerable            bool            `json:"answerable"`
	RetrievedCount        int             `json:"retrieved_count"`
	HitAtK                map[int]bool    `json:"hit_at_k,omitempty"`
	RecallAtK             map[int]float64 `json:"recall_at_k,omitempty"`
	PrecisionAtK          map[int]float64 `json:"precision_at_k,omitempty"`
	NDCGAtK               map[int]float64 `json:"ndcg_at_k,omitempty"`
	ReciprocalRank        float64         `json:"reciprocal_rank,omitempty"`
	FirstRelevantRank     int             `json:"first_relevant_rank,omitempty"`
	SourceSpanHit         *bool           `json:"source_span_hit,omitempty"`
	SourceTextHit         *bool           `json:"source_text_hit,omitempty"`
	AcceptedFalsePositive bool            `json:"accepted_false_positive,omitempty"`
}

type PairedComparison struct {
	RecallDeltaAtK                    map[int]float64 `json:"recall_delta_at_k"`
	HitRateDeltaAtK                   map[int]float64 `json:"hit_rate_delta_at_k"`
	PrecisionDeltaAtK                 map[int]float64 `json:"precision_delta_at_k"`
	MRRDelta                          float64         `json:"mrr_delta"`
	NDCGDeltaAtK                      map[int]float64 `json:"ndcg_delta_at_k"`
	SpanHitDelta                      float64         `json:"source_span_hit_delta"`
	SourceTextHitDelta                float64         `json:"source_text_hit_delta"`
	AcceptedContextFalsePositiveDelta float64         `json:"accepted_context_false_positive_delta"`
}

// Evaluate 计算 macro Recall@K、MRR、nDCG@K、精确 SourceSpan 命中率及候选 Chunk Token 分布。
func Evaluate(cases []GoldCase, ranked map[string][]RankedHit, ks []int) Report {
	ks = normalizeKs(ks)
	report := Report{
		CaseCount: len(cases), RecallAtK: map[int]float64{}, HitRateAtK: map[int]float64{},
		PrecisionAtK: map[int]float64{}, NDCGAtK: map[int]float64{}, Cases: make([]CaseReport, 0, len(cases)),
	}
	if len(cases) == 0 {
		return report
	}
	maxK := ks[len(ks)-1]
	spanCases, spanHits := 0, 0
	textCases, textHits := 0, 0
	falsePositives := 0
	var tokenCounts []int
	for _, gold := range cases {
		hits := ranked[gold.ID]
		answerable := gold.ExpectedAnswerable || len(gold.RelevantSources) > 0
		caseReport := CaseReport{CaseID: gold.ID, Answerable: answerable, RetrievedCount: len(hits)}
		if !answerable {
			report.UnanswerableCaseCount++
			if len(hits) > 0 {
				falsePositives++
				caseReport.AcceptedFalsePositive = true
			}
			report.Cases = append(report.Cases, caseReport)
			continue
		}
		report.AnswerableCaseCount++
		caseReport.HitAtK = map[int]bool{}
		caseReport.RecallAtK = map[int]float64{}
		caseReport.PrecisionAtK = map[int]float64{}
		caseReport.NDCGAtK = map[int]float64{}
		for _, k := range ks {
			recall := recallAt(hits, gold.RelevantSources, k)
			precision := precisionAt(hits, gold.RelevantSources, k)
			ndcg := ndcgAt(hits, gold.RelevantSources, k)
			hit := recall > 0
			caseReport.HitAtK[k], caseReport.RecallAtK[k] = hit, recall
			caseReport.PrecisionAtK[k], caseReport.NDCGAtK[k] = precision, ndcg
			report.RecallAtK[k] += recall
			report.PrecisionAtK[k] += precision
			report.NDCGAtK[k] += ndcg
			if hit {
				report.HitRateAtK[k]++
			}
		}
		caseReport.ReciprocalRank = reciprocalRank(hits, gold.RelevantSources)
		caseReport.FirstRelevantRank = firstRelevantRank(hits, gold.RelevantSources)
		report.MRR += caseReport.ReciprocalRank
		if hasPreciseSource(gold.RelevantSources) {
			spanCases++
			spanHit := anyPreciseHit(hits, gold.RelevantSources, maxK)
			caseReport.SourceSpanHit = &spanHit
			if spanHit {
				spanHits++
			}
		}
		if hasSourceText(gold.RelevantSources) {
			textCases++
			textHit := anySourceTextHit(hits, gold.RelevantSources, maxK)
			caseReport.SourceTextHit = &textHit
			if textHit {
				textHits++
			}
		}
		for i := 0; i < len(hits) && i < maxK; i++ {
			if hits[i].TokenCount > 0 {
				tokenCounts = append(tokenCounts, hits[i].TokenCount)
			}
		}
		report.Cases = append(report.Cases, caseReport)
	}
	for _, k := range ks {
		if report.AnswerableCaseCount > 0 {
			report.RecallAtK[k] /= float64(report.AnswerableCaseCount)
			report.HitRateAtK[k] /= float64(report.AnswerableCaseCount)
			report.PrecisionAtK[k] /= float64(report.AnswerableCaseCount)
			report.NDCGAtK[k] /= float64(report.AnswerableCaseCount)
		}
	}
	if report.AnswerableCaseCount > 0 {
		report.MRR /= float64(report.AnswerableCaseCount)
	}
	if spanCases > 0 {
		report.SourceSpanHitRate = float64(spanHits) / float64(spanCases)
	}
	if textCases > 0 {
		report.SourceTextHitRate = float64(textHits) / float64(textCases)
	}
	if report.UnanswerableCaseCount > 0 {
		report.UnanswerableFalsePositiveRate = float64(falsePositives) / float64(report.UnanswerableCaseCount)
		report.AcceptedContextFalsePositiveRate = report.UnanswerableFalsePositiveRate
	}
	sort.Ints(tokenCounts)
	report.MedianChunkTokens = percentile(tokenCounts, .5)
	report.P95ChunkTokens = percentile(tokenCounts, .95)
	return report
}

func Compare(baseline, candidate Report) PairedComparison {
	out := PairedComparison{
		RecallDeltaAtK: map[int]float64{}, HitRateDeltaAtK: map[int]float64{},
		PrecisionDeltaAtK: map[int]float64{}, NDCGDeltaAtK: map[int]float64{},
		MRRDelta:                          candidate.MRR - baseline.MRR,
		SpanHitDelta:                      candidate.SourceSpanHitRate - baseline.SourceSpanHitRate,
		SourceTextHitDelta:                candidate.SourceTextHitRate - baseline.SourceTextHitRate,
		AcceptedContextFalsePositiveDelta: candidate.AcceptedContextFalsePositiveRate - baseline.AcceptedContextFalsePositiveRate,
	}
	for k, value := range candidate.RecallAtK {
		out.RecallDeltaAtK[k] = value - baseline.RecallAtK[k]
	}
	for k, value := range candidate.NDCGAtK {
		out.NDCGDeltaAtK[k] = value - baseline.NDCGAtK[k]
	}
	for k, value := range candidate.HitRateAtK {
		out.HitRateDeltaAtK[k] = value - baseline.HitRateAtK[k]
	}
	for k, value := range candidate.PrecisionAtK {
		out.PrecisionDeltaAtK[k] = value - baseline.PrecisionAtK[k]
	}
	return out
}

func precisionAt(hits []RankedHit, relevant []RelevantSource, k int) float64 {
	if k <= 0 {
		return 0
	}
	relevantHits := 0
	for i := 0; i < len(hits) && i < k; i++ {
		for _, source := range relevant {
			if hitMatches(hits[i], source) {
				relevantHits++
				break
			}
		}
	}
	return float64(relevantHits) / float64(k)
}

func firstRelevantRank(hits []RankedHit, relevant []RelevantSource) int {
	for i, hit := range hits {
		for _, source := range relevant {
			if hitMatches(hit, source) {
				return i + 1
			}
		}
	}
	return 0
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
		gain := 0
		for index, source := range relevant {
			if !found[index] && hitMatches(hits[i], source) {
				gain = relevanceGrade(source)
				found[index] = true
				break
			}
		}
		if gain > 0 {
			dcg += (math.Pow(2, float64(gain)) - 1) / math.Log2(float64(i+2))
		}
	}
	grades := make([]int, 0, len(relevant))
	for _, source := range relevant {
		grades = append(grades, relevanceGrade(source))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(grades)))
	ideal := 0.0
	for i := 0; i < len(grades) && i < k; i++ {
		ideal += (math.Pow(2, float64(grades[i])) - 1) / math.Log2(float64(i+2))
	}
	return dcg / ideal
}

func relevanceGrade(source RelevantSource) int {
	if source.Relevance <= 0 {
		return 1
	}
	return source.Relevance
}

func hitMatches(hit RankedHit, relevant RelevantSource) bool {
	if relevant.DocumentID != "" && hit.DocumentID != relevant.DocumentID {
		return false
	}
	if relevant.DocumentID == "" && !sameDocumentTitle(hit.DocumentTitle, relevant.DocumentTitle) {
		return false
	}
	if relevant.SourceText != "" {
		return strings.Contains(hit.Content, relevant.SourceText)
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

func sameDocumentTitle(actual, expected string) bool {
	normalize := func(value string) string {
		value = strings.TrimSpace(strings.ToLower(filepath.Base(value)))
		return strings.TrimSuffix(value, ".md")
	}
	return normalize(actual) != "" && normalize(actual) == normalize(expected)
}

func hasSourceText(sources []RelevantSource) bool {
	for _, source := range sources {
		if source.SourceText != "" {
			return true
		}
	}
	return false
}

func anySourceTextHit(hits []RankedHit, relevant []RelevantSource, k int) bool {
	for i := 0; i < len(hits) && i < k; i++ {
		for _, source := range relevant {
			if source.SourceText != "" && hitMatches(hits[i], source) {
				return true
			}
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
