package request

type LoginRequest struct {
	Account  string `json:"account" binding:"required,max=255"`
	Password string `json:"password" binding:"required,max=1024"`
}
