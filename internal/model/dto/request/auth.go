package request

// LoginRequest 表示用户登录请求，包含账号和密码字段。
type LoginRequest struct {
	Account  string `json:"account" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=1024"`
}
