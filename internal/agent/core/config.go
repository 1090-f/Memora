package core

import "github.com/1090-f/Memora/internal/contracts"

// withDefaults 使用系统安全上限补充用户配置中的零值或越限字段。
// 所有 Agent 执行器共享此函数，统一缺省配置的行为。
func withDefaults(cfg contracts.AgentConfig) contracts.AgentConfig {
	defaults := contracts.DefaultAgentConfig()
	if cfg.MaxReactRounds <= 0 || cfg.MaxReactRounds > maxReactRoundsLimit {
		cfg.MaxReactRounds = defaults.MaxReactRounds
	}
	if cfg.MaxPlanSteps <= 0 || cfg.MaxPlanSteps > maxPlanStepsLimit {
		cfg.MaxPlanSteps = defaults.MaxPlanSteps
	}
	if cfg.MaxReplans < 0 || cfg.MaxReplans > maxReplansLimit {
		cfg.MaxReplans = defaults.MaxReplans
	}
	if cfg.ReviewerRuns <= 0 {
		cfg.ReviewerRuns = defaults.ReviewerRuns
	}
	if cfg.MaxToolCalls <= 0 || cfg.MaxToolCalls > maxToolCallsLimit {
		cfg.MaxToolCalls = defaults.MaxToolCalls
	}
	if cfg.MaxDocumentReadTokens <= 0 || cfg.MaxDocumentReadTokens > maxDocumentReadTokensLimit {
		cfg.MaxDocumentReadTokens = defaults.MaxDocumentReadTokens
	}
	if cfg.MaxToolResultBytes <= 0 || cfg.MaxToolResultBytes > maxToolResultBytesLimit {
		cfg.MaxToolResultBytes = defaults.MaxToolResultBytes
	}
	if cfg.MaxRunSeconds <= 0 || cfg.MaxRunSeconds > maxRunSecondsLimit {
		cfg.MaxRunSeconds = defaults.MaxRunSeconds
	}
	if cfg.MemoryTopK <= 0 || cfg.MemoryTopK > maxMemoryTopKLimit {
		cfg.MemoryTopK = defaults.MemoryTopK
	}
	return cfg
}
