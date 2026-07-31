package contracts

// ErrorCode is a stable, machine-readable application error identifier.
type ErrorCode string

const (
	InvalidArgument       ErrorCode = "INVALID_ARGUMENT"
	InvalidState          ErrorCode = "INVALID_STATE"
	UnsupportedFileType   ErrorCode = "UNSUPPORTED_FILE_TYPE"
	WriteMCPToolForbidden ErrorCode = "WRITE_MCP_TOOL_FORBIDDEN"
	Unauthorized          ErrorCode = "UNAUTHORIZED"
	Forbidden             ErrorCode = "FORBIDDEN"
	NetworkDisabled       ErrorCode = "NETWORK_DISABLED"
	ResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	DuplicateResource     ErrorCode = "DUPLICATE_RESOURCE"
	IndexVersionConflict  ErrorCode = "INDEX_VERSION_CONFLICT"
	PayloadTooLarge       ErrorCode = "PAYLOAD_TOO_LARGE"
	KnowledgeInsufficient ErrorCode = "KNOWLEDGE_INSUFFICIENT"
	RateLimited           ErrorCode = "RATE_LIMITED"
	InternalError         ErrorCode = "INTERNAL_ERROR"
	ModelCallFailed       ErrorCode = "MODEL_CALL_FAILED"
	MCPCallFailed         ErrorCode = "MCP_CALL_FAILED"
	UpstreamTimeout       ErrorCode = "UPSTREAM_TIMEOUT"
)

// AppError describes a failure that can be safely returned by the public API.
// Cause is retained for logging and must never be serialized to clients.
type AppError struct {
	Code    ErrorCode
	Message string
	Details any
	Cause   error
}

// Error implements error.
func (e AppError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}
