package chunking

import "testing"

func TestResolveEmbeddingTokenizerOpenAIEmbedding3(t *testing.T) {
	resolution, err := ResolveEmbeddingTokenizer("openai", "text-embedding-3-small")
	if err != nil {
		t.Fatal(err)
	}
	if !resolution.Exact || resolution.Encoding != "cl100k_base" || resolution.Tokenizer.Name() != cl100kTokenizerName {
		t.Fatalf("unexpected resolution: %+v", resolution)
	}
	count, err := resolution.Tokenizer.Count("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("cl100k_base count = %d, want 2", count)
	}
}

func TestResolveEmbeddingTokenizerUnknownModelFallsBack(t *testing.T) {
	resolution, err := ResolveEmbeddingTokenizer("openai-compatible", "custom-embedding")
	if err != nil {
		t.Fatal(err)
	}
	if resolution.Exact || resolution.Tokenizer.Name() != "heuristic-v1" {
		t.Fatalf("unexpected fallback: %+v", resolution)
	}
}

func TestModelTokenizerSplitRespectsModelBudget(t *testing.T) {
	resolution, err := ResolveEmbeddingTokenizer("openai", "text-embedding-3-large")
	if err != nil {
		t.Fatal(err)
	}
	pieces, err := resolution.Tokenizer.Split("hello world. this is a tokenizer-aligned split test.", 4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) < 2 {
		t.Fatalf("expected multiple pieces, got %q", pieces)
	}
	for i, piece := range pieces {
		count, err := resolution.Tokenizer.Count(piece)
		if err != nil {
			t.Fatal(err)
		}
		if count > 4 {
			t.Fatalf("piece %d exceeds budget: %d %q", i, count, piece)
		}
	}
}
