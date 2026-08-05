package contracts

// ErrorCode 是稳定的、机器可读的错误码，用于跨层传递与对外暴露。
type ErrorCode string

// 系统预定义的所有错误码常量。
const (
	ErrInvalidArgument       ErrorCode = "INVALID_ARGUMENT"         // 参数无效
	ErrInvalidState          ErrorCode = "INVALID_STATE"            // 当前状态非法
	ErrUnsupportedFileType   ErrorCode = "UNSUPPORTED_FILE_TYPE"    // 不支持的文件类型
	ErrWriteMCPToolForbidden ErrorCode = "WRITE_MCP_TOOL_FORBIDDEN" // 写入类 MCP 工具被禁用
	ErrUnauthorized          ErrorCode = "UNAUTHORIZED"             // 未认证
	ErrForbidden             ErrorCode = "FORBIDDEN"                // 无权限（已认证但被拒绝）
	ErrNetworkDisabled       ErrorCode = "NETWORK_DISABLED"         // 网络功能被禁用
	ErrResourceNotFound      ErrorCode = "RESOURCE_NOT_FOUND"       // 资源不存在
	ErrDuplicateResource     ErrorCode = "DUPLICATE_RESOURCE"       // 资源重复
	ErrIndexVersionConflict  ErrorCode = "INDEX_VERSION_CONFLICT"   // 索引版本冲突
	ErrPayloadTooLarge       ErrorCode = "PAYLOAD_TOO_LARGE"        // 请求体过大
	ErrKnowledgeInsufficient ErrorCode = "KNOWLEDGE_INSUFFICIENT"   // 知识库证据不足
	ErrRateLimited           ErrorCode = "RATE_LIMITED"             // 触发限流
	ErrInternal              ErrorCode = "INTERNAL_ERROR"           // 内部错误
	ErrModelCallFailed       ErrorCode = "MODEL_CALL_FAILED"        // 模型调用失败
	ErrMCPCallFailed         ErrorCode = "MCP_CALL_FAILED"          // MCP 工具调用失败
	ErrMCPImportFailed       ErrorCode = "MCP_IMPORT_FAILED"        // MCP Server 导入失败
	ErrMCPConnectionFailed   ErrorCode = "MCP_CONNECTION_FAILED"    // MCP 连接失败
	ErrMCPDiscoveryFailed    ErrorCode = "MCP_DISCOVERY_FAILED"     // MCP 工具发现失败
	ErrUpstreamTimeout       ErrorCode = "UPSTREAM_TIMEOUT"         // 上游调用超时
)
