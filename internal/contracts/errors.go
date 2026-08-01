package contracts

type ErrorCode string

const (
	ErrInvalidArgument       ErrorCode = "INVALID_ARGUMENT"
	ErrInvalidState          ErrorCode = "INVALID_STATE"
	ErrUnsupportedFileType   ErrorCode = "UNSUPPORTED_FILE_TYPE"
	ErrWriteMCPToolForbidden ErrorCode = "WRITE_MCP_TOOL_FORBIDDEN"
	ErrUnauthorized          ErrorCode = "UNAUTHORIZED"
	ErrForbidden             ErrorCode = "FORBIDDEN"
	ErrNetworkDisabled       ErrorCode = "NETWORK_DISABLED"
	ErrResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	ErrDuplicateResource     ErrorCode = "DUPLICATE_RESOURCE"
	ErrIndexVersionConflict  ErrorCode = "INDEX_VERSION_CONFLICT"
	ErrPayloadTooLarge       ErrorCode = "PAYLOAD_TOO_LARGE"
	ErrKnowledgeInsufficient ErrorCode = "KNOWLEDGE_INSUFFICIENT"
	ErrRateLimited           ErrorCode = "RATE_LIMITED"
	ErrInternal              ErrorCode = "INTERNAL_ERROR"
	ErrModelCallFailed       ErrorCode = "MODEL_CALL_FAILED"
	ErrMCPCallFailed         ErrorCode = "MCP_CALL_FAILED"
	ErrUpstreamTimeout       ErrorCode = "UPSTREAM_TIMEOUT"
)
