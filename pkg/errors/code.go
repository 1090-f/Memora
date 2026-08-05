package errors

type Code string

const (
	CodeOK               Code = "OK"
	CodeInvalidArgument  Code = "INVALID_ARGUMENT"
	CodeUnauthorized     Code = "UNAUTHORIZED"
	CodeForbidden        Code = "FORBIDDEN"
	CodeNotFound         Code = "RESOURCE_NOT_FOUND"
	CodeConflict         Code = "DUPLICATE_RESOURCE"
	CodePayloadTooLarge  Code = "PAYLOAD_TOO_LARGE"
	CodeMCPImportFailed  Code = "MCP_IMPORT_FAILED"
	CodeMCPConnFailed    Code = "MCP_CONNECTION_FAILED"
	CodeMCPDiscoveryFail Code = "MCP_DISCOVERY_FAILED"
	CodeMCPCallFailed    Code = "MCP_CALL_FAILED"
	CodeInternal         Code = "INTERNAL_ERROR"
)

var messages = map[Code]string{
	CodeOK: "success", CodeInvalidArgument: "invalid argument", CodeUnauthorized: "unauthorized",
	CodeForbidden: "forbidden", CodeNotFound: "resource not found", CodeConflict: "duplicate resource",
	CodePayloadTooLarge: "payload too large", CodeMCPImportFailed: "mcp import failed",
	CodeMCPConnFailed: "mcp connection failed", CodeMCPDiscoveryFail: "mcp discovery failed",
	CodeMCPCallFailed: "mcp call failed", CodeInternal: "internal server error",
}

func Message(code Code) string {
	if message, ok := messages[code]; ok {
		return message
	}
	return messages[CodeInternal]
}
