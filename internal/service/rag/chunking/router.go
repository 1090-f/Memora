package chunking

import (
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/service/rag/canonical"
)

const (
	StrategyAuto       = "auto"
	StrategyStructured = "structured"
	StrategyParagraph  = "paragraph"
	StrategyRecursive  = "recursive_fallback"
	RouterVersion      = "deterministic-router-v1"
	ParagraphVersion   = "paragraph-v1"
	RecursiveVersion   = "recursive-v1"
)

// RouterConfig contains deterministic and versionable routing thresholds.
type RouterConfig struct {
	MinHeadingCount           int     `json:"min_heading_count"`
	MinHeadingDepth           int     `json:"min_heading_depth"`
	MinHeadingCoverage        float64 `json:"min_heading_coverage"`
	MinParagraphCount         int     `json:"min_paragraph_count"`
	MaxAverageParagraphTokens float64 `json:"max_average_paragraph_tokens"`
}

// DefaultRouterConfig returns conservative, deterministic rules used by the
// default auto strategy.
func DefaultRouterConfig() RouterConfig {
	return RouterConfig{
		MinHeadingCount: 2, MinHeadingDepth: 2, MinHeadingCoverage: 0.25,
		MinParagraphCount: 3, MaxAverageParagraphTokens: 600,
	}
}

// ChunkDecision records the chosen strategy and the reasons required for observability.
type ChunkDecision struct {
	Strategy       string         `json:"strategy"`
	Version        string         `json:"version"`
	Features       map[string]any `json:"features"`
	Reasons        []string       `json:"reasons"`
	ManualOverride bool           `json:"manual_override"`
}

// StrategyRouter selects a deterministic canonical chunking strategy.
type StrategyRouter struct{ cfg RouterConfig }

func NewStrategyRouter(cfg RouterConfig) *StrategyRouter { return &StrategyRouter{cfg: cfg} }

// RouteWithOverride 优先应用文档/知识库级显式覆盖；未提供覆盖时使用 configured。
func (r *StrategyRouter) RouteWithOverride(profile canonical.DocumentProfile, configured, override string) (ChunkDecision, error) {
	override = strings.TrimSpace(strings.ToLower(override))
	if override == "" {
		return r.Route(profile, configured)
	}
	if !validStrategy(override) {
		return ChunkDecision{}, fmt.Errorf("不支持的分块策略覆盖 %q", override)
	}
	decision, err := r.Route(profile, override)
	if err != nil {
		return ChunkDecision{}, err
	}
	decision.ManualOverride = true
	decision.Reasons = []string{"document or knowledge-base strategy override"}
	return decision, nil
}

// Route applies an explicit strategy or evaluates canonical profile features.
func (r *StrategyRouter) Route(profile canonical.DocumentProfile, requested string) (ChunkDecision, error) {
	requested = strings.TrimSpace(strings.ToLower(requested))
	if requested == "" {
		requested = StrategyStructured
	}
	features := map[string]any{
		"source_format": profile.SourceFormat, "heading_count": profile.HeadingCount,
		"heading_depth": profile.HeadingDepth, "heading_coverage": profile.HeadingCoverage,
		"paragraph_count":          profile.ParagraphCount,
		"average_paragraph_tokens": profile.AverageParagraphTokens,
	}
	if requested != StrategyAuto {
		if !validStrategy(requested) {
			return ChunkDecision{}, fmt.Errorf("不支持的分块策略 %q", requested)
		}
		return ChunkDecision{
			Strategy: requested, Version: RouterVersion, Features: features,
			Reasons: []string{"configured fixed strategy"}, ManualOverride: false,
		}, nil
	}
	if strings.EqualFold(profile.SourceFormat, "xlsx") {
		return ChunkDecision{Strategy: StrategyStructured, Version: RouterVersion, Features: features, Reasons: []string{"xlsx preserves table boundaries"}}, nil
	}
	if profile.HasReliableHeadingPath && profile.HeadingCount >= r.cfg.MinHeadingCount &&
		profile.HeadingDepth >= r.cfg.MinHeadingDepth && profile.HeadingCoverage >= r.cfg.MinHeadingCoverage {
		return ChunkDecision{Strategy: StrategyStructured, Version: RouterVersion, Features: features, Reasons: []string{"reliable heading tree"}}, nil
	}
	if profile.ParagraphCount >= r.cfg.MinParagraphCount &&
		(profile.AverageParagraphTokens == 0 || profile.AverageParagraphTokens <= r.cfg.MaxAverageParagraphTokens) {
		return ChunkDecision{Strategy: StrategyParagraph, Version: RouterVersion, Features: features, Reasons: []string{"reliable paragraph boundaries"}}, nil
	}
	return ChunkDecision{Strategy: StrategyRecursive, Version: RouterVersion, Features: features, Reasons: []string{"weak document structure"}}, nil
}

func validStrategy(value string) bool {
	switch value {
	case StrategyStructured, StrategyParagraph, StrategyRecursive:
		return true
	default:
		return false
	}
}
