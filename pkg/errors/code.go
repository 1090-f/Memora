package errors

// Code 表示业务错误码类型
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
	CodeOK:               "成功",
	CodeInvalidArgument:  "参数无效",
	CodeUnauthorized:     "未授权",
	CodeForbidden:        "禁止访问",
	CodeNotFound:         "资源不存在",
	CodeConflict:         "资源重复",
	CodeInternal:         "服务器内部错误",
	CodePayloadTooLarge:  "传输数据包过大",
	CodeMCPImportFailed:  "MCP 导入加载失败",
	CodeMCPConnFailed:    "MCP 服务连接失败",
	CodeMCPDiscoveryFail: "MCP 服务发现失败",
	CodeMCPCallFailed:    "调用 MCP 方法执行失败",
}

// Message 根据错误码返回对应的默认错误消息，未知错误码返回内部错误消息
func Message(code Code) string {
	if message, ok := messages[code]; ok {
		return message
	}
	return messages[CodeInternal]
}
