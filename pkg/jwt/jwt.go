package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken 表示令牌无效的错误
var ErrInvalidToken = errors.New("令牌无效")

// Manager 是JWT令牌管理器，负责令牌的生成和解析
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager 创建JWT管理器实例，secret为签名密钥，ttl为令牌有效期
func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if secret == "" || ttl <= 0 {
		return nil, errors.New("JWT 配置无效")
	}
	return &Manager{secret: []byte(secret), ttl: ttl}, nil
}

// Generate 生成JWT令牌，返回签名后的令牌字符串、有效期秒数和可能的错误
func (m *Manager) Generate(userID, username, tokenID string, now time.Time) (string, int64, error) {
	expiresAt := now.Add(m.ttl)
	claims := Claims{Username: username, RegisteredClaims: jwtv5.RegisteredClaims{
		Subject: userID, ID: tokenID, IssuedAt: jwtv5.NewNumericDate(now), ExpiresAt: jwtv5.NewNumericDate(expiresAt),
	}}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, int64(m.ttl / time.Second), err
}

// Parse 解析并验证JWT令牌，返回解析后的Claims或错误
func (m *Manager) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwtv5.ParseWithClaims(tokenString, claims, func(token *jwtv5.Token) (any, error) {
		if token.Method != jwtv5.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwtv5.WithValidMethods([]string{jwtv5.SigningMethodHS256.Alg()}), jwtv5.WithExpirationRequired(), jwtv5.WithIssuedAt())
	if err != nil || !token.Valid || claims.Subject == "" || claims.ID == "" || claims.ExpiresAt == nil {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
