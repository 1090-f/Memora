package response

// UserResponse 表示用户信息的响应结构。
type UserResponse struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Nickname  string  `json:"nickname"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Bio       *string `json:"bio,omitempty"`
}

// LoginResponse 表示登录成功的响应，包含访问令牌和用户信息。
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int64        `json:"expires_in"`
	User        UserResponse `json:"user"`
}
