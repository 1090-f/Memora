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
	if got := effectiveResultCount([]*schema.Document{weak}, contracts.RetrievalKeyword); got != 0 {
		t.Fatalf("weak-only effective count = %d", got)
	}
	if got := effectiveResultCount([]*schema.Document{weak, strong}, contracts.RetrievalKeyword); got != 1 {
		t.Fatalf("effective count = %d", got)
	}
	if got := effectiveResultCount([]*schema.Document{weak}, contracts.RetrievalHybrid); got != 1 {
		t.Fatalf("hybrid effective count = %d", got)
	}
}
