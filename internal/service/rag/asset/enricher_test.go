package asset

import (
	"context"
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/parser"
)

func TestNoopEnricherKeepsAssetsUnchanged(t *testing.T) {
	doc := &parser.ParsedDocument{
		Assets: []parser.Asset{
			{ID: "asset-000001", Caption: "图 1", Omitted: true},
		},
	}
	if err := NewNoopEnricher().Enrich(context.Background(), doc); err != nil {
		t.Fatalf("NoopEnricher 不应出错: %v", err)
	}
	if len(doc.Assets) != 1 || doc.Assets[0].Caption != "图 1" {
		t.Error("NoopEnricher 不应修改资产")
	}
}
