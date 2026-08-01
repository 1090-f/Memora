package jwt

import jwtv5 "github.com/golang-jwt/jwt/v5"

type Claims struct {
	Username string `json:"username"`
	jwtv5.RegisteredClaims
}
