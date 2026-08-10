package react

import "github.com/1090-f/Memora/internal/contracts"

// State 保存单次 Worker Run 的有限执行状态，不持久化完整思维链。
type State struct {
	AgentContext contracts.AgentContext
	Config       contracts.AgentConfig
	Messages     []contracts.ChatMessage
	ReactRound   int
	ToolCalls    int
	Usage        contracts.TokenUsage
	FinalResult  string
	Citations    []contracts.Citation
	Completed    bool
}

func defaults(config contracts.AgentConfig) contracts.AgentConfig {
	value := contracts.DefaultAgentConfig()
	if config.MaxReactRounds > 0 {
		value.MaxReactRounds = config.MaxReactRounds
	}
	if config.MaxToolCalls > 0 {
		value.MaxToolCalls = config.MaxToolCalls
	}
	if config.MaxToolResultBytes > 0 {
		value.MaxToolResultBytes = config.MaxToolResultBytes
	}
	if config.MaxRunSeconds > 0 {
		value.MaxRunSeconds = config.MaxRunSeconds
	}
	if config.MaxDocumentReadTokens > 0 {
		value.MaxDocumentReadTokens = config.MaxDocumentReadTokens
	}
	return value
}
