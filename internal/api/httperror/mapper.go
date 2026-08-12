// Package httperror 将稳定应用错误码映射为 HTTP 响应语义。
package httperror

import (
	"net/http"

	"github.com/1090-f/Memora/internal/contracts"
)

var statusByCode = map[contracts.ErrorCode]int{
	contracts.ErrInvalidArgument:        http.StatusBadRequest,
	contracts.ErrInvalidState:           http.StatusConflict,
	contracts.ErrUnsupportedFileType:    http.StatusUnsupportedMediaType,
	contracts.ErrWriteMCPToolForbidden:  http.StatusForbidden,
	contracts.ErrUnauthorized:           http.StatusUnauthorized,
	contracts.ErrForbidden:              http.StatusForbidden,
	contracts.ErrNetworkDisabled:        http.StatusForbidden,
	contracts.ErrResourceNotFound:       http.StatusNotFound,
	contracts.ErrDuplicateResource:      http.StatusConflict,
	contracts.ErrIndexVersionConflict:   http.StatusConflict,
	contracts.ErrPayloadTooLarge:        http.StatusRequestEntityTooLarge,
	contracts.ErrKnowledgeInsufficient:  http.StatusUnprocessableEntity,
	contracts.ErrRateLimited:            http.StatusTooManyRequests,
	contracts.ErrServiceUnavailable:     http.StatusServiceUnavailable,
	contracts.ErrInternal:               http.StatusInternalServerError,
	contracts.ErrModelCallFailed:        http.StatusBadGateway,
	contracts.ErrMCPCallFailed:          http.StatusBadGateway,
	contracts.ErrMCPConnectionFailed:    http.StatusBadGateway,
	contracts.ErrUpstreamTimeout:        http.StatusGatewayTimeout,
	contracts.ErrPreviewNotReady:        http.StatusConflict,
	contracts.ErrPreviewRenderTimeout:   http.StatusGatewayTimeout,
	contracts.ErrPreviewRenderFailed:    http.StatusUnprocessableEntity,
	contracts.ErrPreviewUnsupported:     http.StatusUnsupportedMediaType,
	contracts.ErrPreviewArtifactMissing: http.StatusNotFound,
	contracts.ErrPreviewArtifactCorrupt: http.StatusUnprocessableEntity,
	contracts.ErrPreviewTableTooLarge:   http.StatusRequestEntityTooLarge,
}

var messageByCode = map[contracts.ErrorCode]string{
	contracts.CodeOK:                    "成功",
	contracts.ErrInvalidArgument:        "参数无效",
	contracts.ErrInvalidState:           "资源状态无效",
	contracts.ErrUnsupportedFileType:    "不支持的文件类型",
	contracts.ErrWriteMCPToolForbidden:  "禁止执行写入型 MCP 工具",
	contracts.ErrUnauthorized:           "未授权",
	contracts.ErrForbidden:              "禁止访问",
	contracts.ErrNetworkDisabled:        "网络访问已禁用",
	contracts.ErrResourceNotFound:       "资源不存在",
	contracts.ErrDuplicateResource:      "资源重复",
	contracts.ErrIndexVersionConflict:   "索引版本冲突",
	contracts.ErrPayloadTooLarge:        "请求内容过大",
	contracts.ErrKnowledgeInsufficient:  "知识不足",
	contracts.ErrRateLimited:            "请求过于频繁",
	contracts.ErrServiceUnavailable:     "服务暂不可用",
	contracts.ErrInternal:               "服务器内部错误",
	contracts.ErrModelCallFailed:        "模型调用失败",
	contracts.ErrMCPCallFailed:          "MCP 工具调用失败",
	contracts.ErrMCPConnectionFailed:    "MCP 服务连接失败",
	contracts.ErrUpstreamTimeout:        "上游服务超时",
	contracts.ErrPreviewNotReady:        "预览尚未生成完成",
	contracts.ErrPreviewRenderTimeout:   "预览生成超时",
	contracts.ErrPreviewRenderFailed:    "预览生成失败",
	contracts.ErrPreviewUnsupported:     "不支持该预览类型",
	contracts.ErrPreviewArtifactMissing: "预览产物不存在",
	contracts.ErrPreviewArtifactCorrupt: "预览产物已损坏",
	contracts.ErrPreviewTableTooLarge:   "表格过大，无法结构化预览",
}

// Status 返回错误码对应的 HTTP 状态码，未知错误码按内部错误处理。
func Status(code contracts.ErrorCode) int {
	if status, ok := statusByCode[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Message 返回适合对外展示的稳定消息，未知错误码不泄漏内部信息。
func Message(code contracts.ErrorCode) string {
	if message, ok := messageByCode[code]; ok {
		return message
	}
	return messageByCode[contracts.ErrInternal]
}
