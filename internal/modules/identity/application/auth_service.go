// Package application implements identity use cases.
package application

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/1090-f/Memora/internal/modules/identity/ports"
	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

type LoginResult struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
	User        domain.User
}

type TokenClaims struct {
	Subject   string `json:"sub"`
	TokenID   string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type AuthService struct {
	users      ports.UserRepository
	blacklist  ports.TokenBlacklist
	secret     []byte
	accessTTL  time.Duration
	dummyHash  string
	now        func() time.Time
	newTokenID func() (string, error)
}

func NewAuthService(users ports.UserRepository, blacklist ports.TokenBlacklist, secret string, accessTTL time.Duration) (*AuthService, error) {
	if users == nil || blacklist == nil || strings.TrimSpace(secret) == "" || accessTTL <= 0 {
		return nil, errors.New("invalid authentication configuration")
	}
	dummyHash, err := HashPassword("memora-invalid-credential-placeholder")
	if err != nil {
		return nil, fmt.Errorf("build credential placeholder: %w", err)
	}
	return &AuthService{
		users:      users,
		blacklist:  blacklist,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		dummyHash:  dummyHash,
		now:        time.Now,
		newTokenID: randomTokenID,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, account, password string) (LoginResult, error) {
	user, err := s.users.FindActiveByAccount(ctx, strings.TrimSpace(account))
	if err != nil {
		_ = VerifyPassword(password, s.dummyHash)
		return LoginResult{}, domain.ErrInvalidCredentials
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return LoginResult{}, domain.ErrInvalidCredentials
	}

	now := s.now().UTC()
	tokenID, err := s.newTokenID()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate token ID: %w", err)
	}
	claims := TokenClaims{
		Subject:   user.ID,
		TokenID:   tokenID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.accessTTL).Unix(),
	}
	token, err := s.sign(claims)
	if err != nil {
		return LoginResult{}, err
	}
	return LoginResult{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.accessTTL / time.Second),
		User:        *user,
	}, nil
}

func (s *AuthService) Logout(ctx context.Context, tokenID string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	if err := s.blacklist.Revoke(ctx, tokenID, ttl); err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	return nil
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (*domain.User, TokenClaims, error) {
	claims, err := s.parse(token)
	if err != nil {
		return nil, TokenClaims{}, domain.ErrUnauthorized
	}
	revoked, err := s.blacklist.IsRevoked(ctx, claims.TokenID)
	if err != nil {
		return nil, TokenClaims{}, fmt.Errorf("check token revocation: %w", err)
	}
	if revoked {
		return nil, TokenClaims{}, domain.ErrUnauthorized
	}
	user, err := s.users.FindActiveByID(ctx, claims.Subject)
	if err != nil {
		return nil, TokenClaims{}, domain.ErrUnauthorized
	}
	return user, claims, nil
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	if memory == 0 || iterations == 0 || parallelism == 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func (s *AuthService) sign(claims TokenClaims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal JWT claims: %w", err)
	}
	unsigned := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *AuthService) parse(token string) (TokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return TokenClaims{}, domain.ErrUnauthorized
	}

	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	var claims TokenClaims
	if json.Unmarshal(payload, &claims) != nil || claims.Subject == "" || claims.TokenID == "" || claims.IssuedAt == 0 || claims.ExpiresAt == 0 {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	if s.now().Unix() >= claims.ExpiresAt {
		return TokenClaims{}, domain.ErrUnauthorized
	}
	return claims, nil
}

func randomTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
