package contracts

// ErrorCode 表示用于跨层错误传播的标准化错误码。
type ErrorCode string

// 系统预定义的所有错误码常量。
const (
	// ErrInvalidArgument 表示请求包含无效参数。
	ErrInvalidArgument       ErrorCode = "INVALID_ARGUMENT"
	// ErrInvalidState 表示资源处于无效状态，无法执行请求的操作。
	ErrInvalidState          ErrorCode = "INVALID_STATE"
	// ErrUnsupportedFileType 表示上传的文件类型不支持。
	ErrUnsupportedFileType   ErrorCode = "UNSUPPORTED_FILE_TYPE"
	// ErrWriteMCPToolForbidden 表示不允许通过 MCP 工具执行写操作。
	ErrWriteMCPToolForbidden ErrorCode = "WRITE_MCP_TOOL_FORBIDDEN"
	// ErrUnauthorized 表示请求缺少有效的认证凭据。
	ErrUnauthorized          ErrorCode = "UNAUTHORIZED"
	// ErrForbidden 表示已认证用户没有权限执行请求的操作。
	ErrForbidden             ErrorCode = "FORBIDDEN"
	// ErrNetworkDisabled 表示当前操作已禁用网络访问。
	ErrNetworkDisabled       ErrorCode = "NETWORK_DISABLED"
	// ErrResourceNotFound 表示请求的资源不存在。
	ErrResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"
	// ErrDuplicateResource 表示已存在具有相同唯一约束的资源。
	ErrDuplicateResource     ErrorCode = "DUPLICATE_RESOURCE"
	// ErrIndexVersionConflict 表示更新过程中发生索引版本冲突。
	ErrIndexVersionConflict  ErrorCode = "INDEX_VERSION_CONFLICT"
	// ErrPayloadTooLarge 表示请求负载超过允许的大小限制。
	ErrPayloadTooLarge       ErrorCode = "PAYLOAD_TOO_LARGE"
	// ErrKnowledgeInsufficient 表示知识库信息不足以回答问题。
	ErrKnowledgeInsufficient ErrorCode = "KNOWLEDGE_INSUFFICIENT"
	// ErrRateLimited 表示请求已被限流。
	ErrRateLimited           ErrorCode = "RATE_LIMITED"
	// ErrInternal 表示发生了意外的内部服务器错误。
	ErrInternal              ErrorCode = "INTERNAL_ERROR"
	// ErrModelCallFailed 表示 AI 模型调用失败。
	ErrModelCallFailed       ErrorCode = "MODEL_CALL_FAILED"
	// ErrMCPCallFailed 表示 MCP 工具调用失败。
	ErrMCPCallFailed         ErrorCode = "MCP_CALL_FAILED"
	// ErrUpstreamTimeout 表示上游服务请求超时。
	ErrUpstreamTimeout       ErrorCode = "UPSTREAM_TIMEOUT"
)
