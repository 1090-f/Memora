package errors

// Code 表示业务错误码类型
type Code string

const (
	CodeOK              Code = "OK"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeUnauthorized    Code = "UNAUTHORIZED"
	CodeForbidden       Code = "FORBIDDEN"
	CodeNotFound        Code = "RESOURCE_NOT_FOUND"
	CodeConflict        Code = "DUPLICATE_RESOURCE"
	CodeInternal        Code = "INTERNAL_ERROR"
)

var messages = map[Code]string{
	CodeOK: "成功", CodeInvalidArgument: "参数无效", CodeUnauthorized: "未授权",
	CodeForbidden: "禁止访问", CodeNotFound: "资源不存在", CodeConflict: "资源重复",
	CodeInternal: "服务器内部错误",
}

// Message 根据错误码返回对应的默认错误消息，未知错误码返回内部错误消息
func Message(code Code) string {
	if message, ok := messages[code]; ok {
		return message
	}
	return messages[CodeInternal]
}
