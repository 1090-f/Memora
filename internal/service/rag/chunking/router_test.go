package chunking

import (
	"testing"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

func TestStrategyRouter(t *testing.T) {
	router := NewStrategyRouter(DefaultRouterConfig())
	tests := []struct {
		name      string
		profile   canonical.DocumentProfile
		requested string
		want      string
	}{
		{name: "fixed", requested: StrategyParagraph, want: StrategyParagraph},
		{name: "xlsx", requested: StrategyAuto, profile: canonical.DocumentProfile{SourceFormat: "xlsx"}, want: StrategyStructured},
		{name: "heading", requested: StrategyAuto, profile: canonical.DocumentProfile{HasReliableHeadingPath: true, HeadingCount: 3, HeadingDepth: 2, HeadingCoverage: .8}, want: StrategyStructured},
		{name: "paragraph", requested: StrategyAuto, profile: canonical.DocumentProfile{ParagraphCount: 5, AverageParagraphTokens: 80}, want: StrategyParagraph},
		{name: "fallback", requested: StrategyAuto, profile: canonical.DocumentProfile{ParagraphCount: 1}, want: StrategyRecursive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := router.Route(tt.profile, tt.requested)
			if err != nil {
				t.Fatal(err)
			}
			if got.Strategy != tt.want || got.Version != RouterVersion || len(got.Reasons) == 0 {
				t.Fatalf("decision = %+v, want %s", got, tt.want)
			}
		})
	}
}

func TestStrategyRouterMarksDocumentOverride(t *testing.T) {
	decision, err := NewStrategyRouter(DefaultRouterConfig()).RouteWithOverride(
		canonical.DocumentProfile{ParagraphCount: 10}, StrategyStructured, StrategyParagraph,
	)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Strategy != StrategyParagraph || !decision.ManualOverride || len(decision.Reasons) != 1 {
		t.Fatalf("manual override decision = %+v", decision)
	}
}

func TestStrategyRouterRejectsUnknownOverride(t *testing.T) {
	_, err := NewStrategyRouter(DefaultRouterConfig()).Route(canonical.DocumentProfile{}, "semantic")
	if err == nil {
		t.Fatal("expected unsupported strategy error")
	}
}
