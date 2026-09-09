package service

import (
	"context"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/model/entity"
	"github.com/1090-f/Memora/internal/repository"
)

type tokenizerTestKnowledgeBases struct {
	repository.KnowledgeBaseRepository
}

func (tokenizerTestKnowledgeBases) FindByID(context.Context, string, string) (*entity.KnowledgeBase, error) {
	return &entity.KnowledgeBase{EmbeddingModelID: "model-config-1"}, nil
}

type tokenizerTestModels struct {
	repository.AIModelConfigRepository
}

func (tokenizerTestModels) FindByIDForUserAndType(context.Context, string, string, string) (*entity.AIModelConfig, error) {
	return &entity.AIModelConfig{ID: "model-config-1", Provider: "openai", Name: "text-embedding-3-small"}, nil
}

type tokenizerTestEmbeddingModel struct{}

func (tokenizerTestEmbeddingModel) Embed(_ context.Context, texts []string) ([][]float32, error) {
	return make([][]float32, len(texts)), nil
}

func (tokenizerTestEmbeddingModel) Dimension() int { return 3 }

type tokenizerTestModelFactory struct{ contracts.ModelFactory }

func (tokenizerTestModelFactory) GetEmbeddingModel(context.Context, contracts.ID) (contracts.EmbeddingModel, error) {
	return tokenizerTestEmbeddingModel{}, nil
}

func TestDocumentEmbeddingResolverResolvesModelTokenizer(t *testing.T) {
	resolver, err := NewDocumentEmbeddingResolver(tokenizerTestKnowledgeBases{}, tokenizerTestModels{}, tokenizerTestModelFactory{})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.Resolve(context.Background(), "user-1", "kb-1")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ModelID != "model-config-1" || resolved.Provider != "openai" || resolved.ModelName != "text-embedding-3-small" {
		t.Fatalf("unexpected model resolution: %+v", resolved)
	}
	if resolved.Embedder == nil || resolved.Tokenizer == nil || !resolved.TokenizerExact {
		t.Fatalf("incomplete embedding resolution: %+v", resolved)
	}
	if resolved.Tokenizer.Name() != "tiktoken-cl100k_base-v1" {
		t.Fatalf("tokenizer = %q", resolved.Tokenizer.Name())
	}
}
