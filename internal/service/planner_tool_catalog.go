package service

import (
	"fmt"
	"strings"

	"github.com/1090-f/Memora/internal/contracts"
)

func appendPlannerToolCatalog(builder *strings.Builder, specs []contracts.ToolSpec) {
	if len(specs) == 0 {
		return
	}
	builder.WriteString("## Tool capability catalog\n")
	builder.WriteString("Use the alias in tool_name. The server resolves it to registry_name.\n")
	for _, spec := range specs {
		fmt.Fprintf(builder, "- alias=%s; registry_name=%s; capabilities=%s; description=%s\n",
			spec.Alias, spec.Name, strings.Join(spec.Capabilities, ","), spec.Description)
		if len(spec.InputSchema) > 0 {
			fmt.Fprintf(builder, "  input_schema=%s\n", string(spec.InputSchema))
		}
	}
	builder.WriteString("\n")
}

func appendPlanStepContract(builder *strings.Builder) {
	builder.WriteString("Every step MUST include these fields:\n")
	builder.WriteString("- kind: tool or reasoning\n")
	builder.WriteString("- tool_policy: required for tool steps; forbidden for reasoning steps\n")
	builder.WriteString("- required_capabilities: semantic capabilities such as web.fetch or web.search\n")
	builder.WriteString("- tool_name: a catalog alias for tool steps, otherwise an empty string\n")
	builder.WriteString("A URL-reading or web-search step MUST be kind=tool and tool_policy=required.\n")
	builder.WriteString("A reasoning step may only analyze outputs from its depends_on steps.\n\n")
}
