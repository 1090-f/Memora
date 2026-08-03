package tools

import (
	"context"
	"encoding/json"

	"github.com/1090-f/Memora/internal/contracts"
)

type Tool interface {
	Spec() contracts.ToolSpec
	Run(ctx context.Context, toolContext contracts.ToolContext, arguments json.RawMessage) (contracts.ToolResult, error)
}
