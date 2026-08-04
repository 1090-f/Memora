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
	CodeOK: "success", CodeInvalidArgument: "invalid argument", CodeUnauthorized: "unauthorized",
	CodeForbidden: "forbidden", CodeNotFound: "resource not found", CodeConflict: "duplicate resource",
	CodeInternal: "internal server error",
}

// Message 根据错误码返回对应的默认错误消息，未知错误码返回内部错误消息
func Message(code Code) string {
	if message, ok := messages[code]; ok {
		return message
	}
	return messages[CodeInternal]
}
