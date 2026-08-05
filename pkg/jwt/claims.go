package jwt

import jwtv5 "github.com/golang-jwt/jwt/v5"

// Claims 定义JWT令牌的载荷结构，包含用户名和标准注册声明
type Claims struct {
	Username string `json:"username"`
	jwtv5.RegisteredClaims
}
