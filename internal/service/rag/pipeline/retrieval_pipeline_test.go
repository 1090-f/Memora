package pipeline

import (
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/schema"
)

func TestReciprocalRankFusionBoostsExactKeywordMatch(t *testing.T) {
	keyword := &schema.Document{ID: "exact", MetaData: map[string]any{
		einoadapter.MetaChunkID: "exact", einoadapter.MetaKeywordRank: 1,
		einoadapter.MetaKeywordMatchLevel: string(contracts.KeywordMatchExact),
	}}
	vector := &schema.Document{ID: "vector", MetaData: map[string]any{
		einoadapter.MetaChunkID: "vector", einoadapter.MetaVectorRank: 1,
	}}
	got := reciprocalRankFusion([]*schema.Document{keyword}, []*schema.Document{vector}, 60)
	if len(got) != 2 || got[0].ID != "exact" {
		t.Fatalf("unexpected RRF order: %#v", got)
	}
}

func TestEffectiveResultCountRejectsWeakOnlyKeywordResults(t *testing.T) {
	weak := &schema.Document{MetaData: map[string]any{
		einoadapter.MetaKeywordMatchLevel: string(contracts.KeywordMatchWeak),
	}}
	strong := &schema.Document{MetaData: map[string]any{
		einoadapter.MetaKeywordMatchLevel: string(contracts.KeywordMatchStrong),
	}}
	vectorQualified := &schema.Document{MetaData: map[string]any{
		einoadapter.MetaVectorScore: 0.62,
	}}
	if got := effectiveResultCount([]*schema.Document{weak}, contracts.RetrievalKeyword, 0.3); got != 0 {
		t.Fatalf("weak-only effective count = %d", got)
	}
	if got := effectiveResultCount([]*schema.Document{weak, strong}, contracts.RetrievalKeyword, 0.3); got != 1 {
		t.Fatalf("effective count = %d", got)
	}
	// 未配置阈值时保持纯计数，弱结果仍计入（向后兼容）。
	if got := effectiveResultCount([]*schema.Document{weak}, contracts.RetrievalHybrid, 0); got != 1 {
		t.Fatalf("hybrid without threshold effective count = %d", got)
	}
	// 配置阈值后：弱关键词召回且无向量分数不计入，带合格向量分数的结果计入。
	if got := effectiveResultCount([]*schema.Document{weak}, contracts.RetrievalHybrid, 0.3); got != 0 {
		t.Fatalf("hybrid weak-only effective count = %d", got)
	}
	if got := effectiveResultCount([]*schema.Document{weak, vectorQualified}, contracts.RetrievalHybrid, 0.3); got != 1 {
		t.Fatalf("hybrid weak+vector effective count = %d", got)
	}
	if got := effectiveResultCount([]*schema.Document{strong, vectorQualified}, contracts.RetrievalHybrid, 0.3); got != 2 {
		t.Fatalf("hybrid strong+vector effective count = %d", got)
	}
	// 向量模式候选已在召回层过滤，数量即有效数量。
	if got := effectiveResultCount([]*schema.Document{weak, vectorQualified}, contracts.RetrievalVector, 0.3); got != 2 {
		t.Fatalf("vector effective count = %d", got)
	}
}
