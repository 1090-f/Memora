package jwt

import (
	"errors"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Manager struct {
	secret []byte
	ttl    time.Duration
}

func NewManager(secret string, ttl time.Duration) (*Manager, error) {
	if secret == "" || ttl <= 0 {
		return nil, errors.New("invalid JWT configuration")
	}
	return &Manager{secret: []byte(secret), ttl: ttl}, nil
}

func (m *Manager) Generate(userID, username, tokenID string, now time.Time) (string, int64, error) {
	expiresAt := now.Add(m.ttl)
	claims := Claims{Username: username, RegisteredClaims: jwtv5.RegisteredClaims{
		Subject: userID, ID: tokenID, IssuedAt: jwtv5.NewNumericDate(now), ExpiresAt: jwtv5.NewNumericDate(expiresAt),
	}}
	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	return signed, int64(m.ttl / time.Second), err
}

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
