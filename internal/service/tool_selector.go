package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	agenttools "github.com/1090-f/Memora/internal/agent/tools"
	"github.com/1090-f/Memora/internal/contracts"
)

var externalURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)

type ToolSelector struct {
	executor *agenttools.Executor
}

func NewToolSelector(executor *agenttools.Executor) *ToolSelector {
	return &ToolSelector{executor: executor}
}

func (s *ToolSelector) catalog(request contracts.AgentRunRequest) []contracts.ToolSpec {
	if len(request.Context.AvailableTools) > 0 {
		return agenttools.CatalogSpecs(request.Context.AvailableTools)
	}
	if s == nil || s.executor == nil {
		return nil
	}
	return agenttools.CatalogSpecs(s.executor.Specs())
}

func (s *ToolSelector) PreparePlan(plan *contracts.Plan, request contracts.AgentRunRequest) error {
	if plan == nil || len(plan.Steps) == 0 {
		return fmt.Errorf("plan has no steps")
	}
	catalog := s.catalog(request)
	requiredCapability, external := requiredExternalCapability(request.Context.Query)
	if external {
		if !request.Context.NetworkEnabled {
			return fmt.Errorf("external information requires network tools, but network access is disabled")
		}
		index := findExternalToolStep(plan.Steps)
		if index < 0 {
			index = 0
		}
		step := &plan.Steps[index]
		step.Kind = contracts.PlanStepKindTool
		step.ToolPolicy = contracts.ToolPolicyRequired
		if !containsString(step.RequiredCapabilities, requiredCapability) {
			step.RequiredCapabilities = append(step.RequiredCapabilities, requiredCapability)
		}
		ensureExternalArguments(step, request.Context.Query, requiredCapability)
	}

	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Kind == "" {
			if step.ToolName != "" || len(step.RequiredCapabilities) > 0 || step.ToolPolicy == contracts.ToolPolicyRequired {
				step.Kind = contracts.PlanStepKindTool
			} else {
				step.Kind = contracts.PlanStepKindReasoning
			}
		}
		if step.Kind == contracts.PlanStepKindReasoning {
			if step.ToolPolicy == contracts.ToolPolicyRequired {
				return fmt.Errorf("step %d requires a tool but is marked as reasoning", step.StepNumber)
			}
			if step.ToolName != "" {
				return fmt.Errorf("step %d is reasoning but declares tool %q", step.StepNumber, step.ToolName)
			}
			continue
		}
		if step.Kind != contracts.PlanStepKindTool {
			return fmt.Errorf("step %d has unsupported kind %q", step.StepNumber, step.Kind)
		}
		spec, err := selectToolSpec(*step, catalog, request)
		if err != nil {
			return fmt.Errorf("select tool for step %d: %w", step.StepNumber, err)
		}
		step.ToolName = spec.Name
		step.ToolPolicy = contracts.ToolPolicyRequired
	}
	return nil
}

func selectToolSpec(step contracts.PlanStep, catalog []contracts.ToolSpec, request contracts.AgentRunRequest) (contracts.ToolSpec, error) {
	candidates := make([]contracts.ToolSpec, 0)
	for _, spec := range catalog {
		if !spec.Enabled || !spec.ReadOnly {
			continue
		}
		if spec.NetworkRequired && !request.Context.NetworkEnabled {
			continue
		}
		if len(request.Context.AllowedTools) > 0 && !containsString(request.Context.AllowedTools, spec.Name) {
			continue
		}
		candidates = append(candidates, spec)
	}
	if step.ToolName != "" {
		var matches []contracts.ToolSpec
		for _, spec := range candidates {
			if step.ToolName == spec.Name || step.ToolName == spec.Alias || step.ToolName == agenttools.ShortToolName(spec.Name) {
				matches = append(matches, spec)
			}
		}
		if len(matches) == 1 && specSatisfiesCapabilities(matches[0], step.RequiredCapabilities) {
			return matches[0], nil
		}
		if len(matches) > 1 {
			return contracts.ToolSpec{}, fmt.Errorf("tool alias %q is ambiguous", step.ToolName)
		}
	}
	for _, capability := range step.RequiredCapabilities {
		for _, spec := range candidates {
			if agenttools.HasCapability(spec, capability) {
				return spec, nil
			}
		}
	}
	if step.ToolName != "" {
		return contracts.ToolSpec{}, fmt.Errorf("tool %q is unavailable", step.ToolName)
	}
	return contracts.ToolSpec{}, fmt.Errorf("no available tool provides capabilities %v", step.RequiredCapabilities)
}

func specSatisfiesCapabilities(spec contracts.ToolSpec, required []string) bool {
	for _, capability := range required {
		if !agenttools.HasCapability(spec, capability) {
			return false
		}
	}
	return true
}
func requiredExternalCapability(query string) (string, bool) {
	lower := strings.ToLower(query)
	if externalURLPattern.MatchString(query) {
		return agenttools.CapabilityWebFetch, true
	}
	keywords := []string{"\u641c\u7d22", "\u67e5\u8be2\u7f51\u9875", "\u8054\u7f51", "search the web", "web search", "internet search"}
	for _, keyword := range keywords {
		if strings.Contains(lower, strings.ToLower(keyword)) {
			return agenttools.CapabilityWebSearch, true
		}
	}
	return "", false
}

func findExternalToolStep(steps []contracts.PlanStep) int {
	for i, step := range steps {
		for _, capability := range step.RequiredCapabilities {
			if capability == agenttools.CapabilityWebFetch || capability == agenttools.CapabilityWebSearch {
				return i
			}
		}
		text := strings.ToLower(step.Title + " " + step.Description)
		keywords := []string{"\u8bbf\u95ee", "\u7f51\u9875", "url", "\u641c\u7d22", "\u6293\u53d6", "\u83b7\u53d6", "fetch", "search", "browse"}
		for _, keyword := range keywords {
			if strings.Contains(text, keyword) {
				return i
			}
		}
	}
	return -1
}

func ensureExternalArguments(step *contracts.PlanStep, query, capability string) {
	if step.Arguments == nil {
		step.Arguments = make(map[string]any)
	}
	if capability == agenttools.CapabilityWebFetch {
		if _, exists := step.Arguments["url"]; !exists {
			if value := extractExternalURL(query); value != "" {
				step.Arguments["url"] = value
			}
		}
		return
	}
	if capability == agenttools.CapabilityWebSearch {
		if _, exists := step.Arguments["query"]; !exists {
			step.Arguments["query"] = query
		}
	}
}

func extractExternalURL(query string) string {
	return strings.TrimRight(externalURLPattern.FindString(query), ".,;:!?)]}")
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func requestRequiresExternalEvidence(request contracts.AgentRunRequest) bool {
	_, required := requiredExternalCapability(request.Context.Query)
	return required
}

func planHasSuccessfulExternalEvidence(plan *contracts.Plan, catalog []contracts.ToolSpec) bool {
	specs := make(map[string]contracts.ToolSpec, len(catalog))
	for _, spec := range agenttools.CatalogSpecs(catalog) {
		specs[spec.Name] = spec
	}
	for _, step := range plan.Steps {
		if step.Status != contracts.PlanStepStatusCompleted || step.ToolName == "" || step.Output == "" {
			continue
		}
		spec, exists := specs[step.ToolName]
		if !exists || !spec.NetworkRequired {
			continue
		}
		var result contracts.ToolResult
		if json.Unmarshal([]byte(step.Output), &result) == nil && result.Success {
			return true
		}
	}
	return false
}
