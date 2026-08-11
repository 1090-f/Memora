package service

import (
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
)

func TestDocumentIndexMode(t *testing.T) {
	activeVersion := 1
	embeddingModelID := "embedding-model"

	tests := []struct {
		name string
		doc  *entity.Document
		want string
	}{
		{name: "nil document", want: string(contracts.DocumentIndexNone)},
		{name: "no active index", doc: &entity.Document{}, want: string(contracts.DocumentIndexNone)},
		{name: "keyword index", doc: &entity.Document{ActiveIndexVersion: &activeVersion}, want: string(contracts.DocumentIndexKeyword)},
		{name: "hybrid index", doc: &entity.Document{ActiveIndexVersion: &activeVersion, EmbeddingModelID: &embeddingModelID}, want: string(contracts.DocumentIndexHybrid)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := documentIndexMode(tt.doc); got != tt.want {
				t.Fatalf("documentIndexMode() = %q, want %q", got, tt.want)
			}
		})
	}
}
