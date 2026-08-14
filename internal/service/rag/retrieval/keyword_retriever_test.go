package retrieval

import (
	"context"
	"reflect"
	"testing"

	"github.com/1090-f/Memora/internal/contracts"
	"github.com/1090-f/Memora/internal/repository"
	"github.com/1090-f/Memora/internal/service/rag/einoadapter"
	"github.com/cloudwego/eino/components/retriever"
)

type fakeKeywordRepository struct {
	search func(repository.KeywordSearchParams) []*repository.KeywordHit
	calls  []repository.KeywordSearchParams
}

func (f *fakeKeywordRepository) Search(_ context.Context, params repository.KeywordSearchParams) ([]*repository.KeywordHit, error) {
	f.calls = append(f.calls, params)
	return f.search(params), nil
}

func newKeywordRetrieverForTest(t *testing.T, repo repository.KeywordSearchRepository) *ParadeDBKeywordRetriever {
	t.Helper()
	r, err := NewParadeDBKeywordRetriever(repo)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func retrieveForTest(t *testing.T, r *ParadeDBKeywordRetriever, query string) []map[string]any {
	t.Helper()
	docs, err := r.Retrieve(context.Background(), query,
		retriever.WithTopK(8),
		WithKeywordScope(KeywordRetrieverOptions{UserID: "u1", KnowledgeBaseID: "kb1"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	metas := make([]map[string]any, len(docs))
	for i, doc := range docs {
		metas[i] = doc.MetaData
	}
	return metas
}

func TestKeywordRetrieverStopsAtExactMatch(t *testing.T) {
	repo := &fakeKeywordRepository{search: func(params repository.KeywordSearchParams) []*repository.KeywordHit {
		if params.Mode != repository.KeywordSearchExact {
			t.Fatal("exact match should stop before broader recall")
		}
		if params.Query != "胡智敏" {
			t.Fatalf("query = %q", params.Query)
		}
		return []*repository.KeywordHit{{ChunkID: "c1", DocumentID: "d1", Content: "姓名：胡智敏", Score: 0.8, IndexVersion: 1}}
	}}
	metas := retrieveForTest(t, newKeywordRetrieverForTest(t, repo), "  胡智敏 ")
	if len(metas) != 1 {
		t.Fatalf("results = %d", len(metas))
	}
	if got := einoadapter.GetMetaString(metas[0], einoadapter.MetaKeywordMatchLevel); got != string(contracts.KeywordMatchExact) {
		t.Fatalf("match level = %q", got)
	}
	if got := einoadapter.GetMetaStrings(metas[0], einoadapter.MetaKeywordMatchedTerms); !reflect.DeepEqual(got, []string{"胡智敏"}) {
		t.Fatalf("matched terms = %#v", got)
	}
}

func TestKeywordRetrieverUsesParadeDBConjunctionAsStrong(t *testing.T) {
	repo := &fakeKeywordRepository{search: func(params repository.KeywordSearchParams) []*repository.KeywordHit {
		if params.Mode == repository.KeywordSearchExact {
			return nil
		}
		if params.Mode != repository.KeywordSearchAll {
			t.Fatal("strong result should come from conjunction recall")
		}
		return []*repository.KeywordHit{{
			ChunkID: "strong", DocumentID: "d1", Content: "胡智负责开发，智敏负责检索", Score: 0.4, IndexVersion: 1,
		}}
	}}
	metas := retrieveForTest(t, newKeywordRetrieverForTest(t, repo), "胡智敏")
	if len(metas) != 1 {
		t.Fatalf("results = %d", len(metas))
	}
	if got := einoadapter.GetMetaString(metas[0], einoadapter.MetaKeywordMatchLevel); got != string(contracts.KeywordMatchStrong) {
		t.Fatalf("match level = %q", got)
	}
	if got := einadapterCoverage(metas[0]); got != 1 {
		t.Fatalf("coverage = %v", got)
	}
	if len(repo.calls) != 2 {
		t.Fatalf("search calls = %d", len(repo.calls))
	}
}

func TestKeywordRetrieverUsesLimitedWeakFallback(t *testing.T) {
	repo := &fakeKeywordRepository{search: func(params repository.KeywordSearchParams) []*repository.KeywordHit {
		if params.Mode != repository.KeywordSearchAny {
			return nil
		}
		return []*repository.KeywordHit{
			{ChunkID: "w1", DocumentID: "d1", Content: "胡智负责开发", Score: 0.4, IndexVersion: 1},
			{ChunkID: "w2", DocumentID: "d2", Content: "智敏负责检索", Score: 0.3, IndexVersion: 1},
			{ChunkID: "w3", DocumentID: "d3", Content: "胡智项目", Score: 0.2, IndexVersion: 1},
			{ChunkID: "w4", DocumentID: "d4", Content: "智敏项目", Score: 0.1, IndexVersion: 1},
		}
	}}
	metas := retrieveForTest(t, newKeywordRetrieverForTest(t, repo), "胡智敏")
	if len(metas) != keywordWeakResultLimit {
		t.Fatalf("results = %d", len(metas))
	}
	if got := einoadapter.GetMetaString(metas[0], einoadapter.MetaKeywordMatchLevel); got != string(contracts.KeywordMatchWeak) {
		t.Fatalf("match level = %q", got)
	}
	if !einoadapter.GetMetaBool(metas[0], einoadapter.MetaKeywordLowConfidence) {
		t.Fatal("weak fallback must be low confidence")
	}
}

func einadapterCoverage(meta map[string]any) float64 {
	return einoadapter.GetMetaFloat(meta, einoadapter.MetaKeywordCoverage)
}
