package application_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/1090-f/Memora/internal/modules/identity/application"
	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/stretchr/testify/require"
)

func TestLoginRejectsUnknownAccountAndWrongPasswordIdentically(t *testing.T) {
	service := newAuthService(t, &domain.User{ID: "user-1", Username: "admin", Email: "admin@example.com"})

	_, wrongPassword := service.Login(context.Background(), "admin@example.com", "wrong")
	_, unknownAccount := service.Login(context.Background(), "missing@example.com", "correct")

	require.ErrorIs(t, wrongPassword, domain.ErrInvalidCredentials)
	require.ErrorIs(t, unknownAccount, domain.ErrInvalidCredentials)
}

func TestLoginReturnsBearerTokenWithRequiredClaims(t *testing.T) {
	service := newAuthService(t, &domain.User{ID: "user-1", Username: "admin", Email: "admin@example.com"})

	result, err := service.Login(context.Background(), "admin@example.com", "correct")

	require.NoError(t, err)
	require.Equal(t, "Bearer", result.TokenType)
	require.Equal(t, int64(7200), result.ExpiresIn)
	require.Equal(t, "user-1", result.User.ID)
	claims := decodeClaims(t, result.AccessToken)
	require.Equal(t, "user-1", claims["sub"])
	require.NotEmpty(t, claims["jti"])
	require.NotZero(t, claims["iat"])
	require.NotZero(t, claims["exp"])
}

func TestLogoutBlacklistsTokenForItsRemainingLifetime(t *testing.T) {
	blacklist := &fakeBlacklist{revoked: make(map[string]bool)}
	service := newAuthServiceWithBlacklist(t, blacklist)
	expiresAt := time.Now().Add(30 * time.Minute)

	err := service.Logout(context.Background(), "token-1", expiresAt)

	require.NoError(t, err)
	require.Equal(t, "token-1", blacklist.revokedTokenID)
	require.InDelta(t, (30 * time.Minute).Seconds(), blacklist.revokedTTL.Seconds(), 2)
}

func TestAuthenticateRejectsBlacklistedToken(t *testing.T) {
	blacklist := &fakeBlacklist{revoked: make(map[string]bool)}
	service := newAuthServiceWithBlacklist(t, blacklist)
	result, err := service.Login(context.Background(), "admin@example.com", "correct")
	require.NoError(t, err)
	claims := decodeClaims(t, result.AccessToken)
	blacklist.revoked[stringClaim(t, claims, "jti")] = true

	_, _, err = service.Authenticate(context.Background(), result.AccessToken)

	require.ErrorIs(t, err, domain.ErrUnauthorized)
}

func newAuthService(t *testing.T, user *domain.User) *application.AuthService {
	t.Helper()
	hash, err := application.HashPassword("correct")
	require.NoError(t, err)
	user.PasswordHash = hash
	service, err := application.NewAuthService(&fakeUserRepository{user: user}, &fakeBlacklist{}, "test-secret-with-enough-entropy", 2*time.Hour)
	require.NoError(t, err)
	return service
}

func newAuthServiceWithBlacklist(t *testing.T, blacklist *fakeBlacklist) *application.AuthService {
	t.Helper()
	hash, err := application.HashPassword("correct")
	require.NoError(t, err)
	user := &domain.User{ID: "user-1", Username: "admin", Email: "admin@example.com", PasswordHash: hash}
	service, err := application.NewAuthService(&fakeUserRepository{user: user}, blacklist, "test-secret-with-enough-entropy", 2*time.Hour)
	require.NoError(t, err)
	return service
}

type fakeUserRepository struct {
	user *domain.User
}

func (r *fakeUserRepository) FindActiveByAccount(_ context.Context, account string) (*domain.User, error) {
	if r.user == nil || (account != r.user.Username && account != r.user.Email) {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

func (r *fakeUserRepository) FindActiveByID(_ context.Context, id string) (*domain.User, error) {
	if r.user == nil || id != r.user.ID {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

type fakeBlacklist struct {
	revoked        map[string]bool
	revokedTokenID string
	revokedTTL     time.Duration
}

func (b *fakeBlacklist) Revoke(_ context.Context, tokenID string, ttl time.Duration) error {
	if b.revoked == nil {
		b.revoked = make(map[string]bool)
	}
	b.revoked[tokenID] = true
	b.revokedTokenID = tokenID
	b.revokedTTL = ttl
	return nil
}

func (b *fakeBlacklist) IsRevoked(_ context.Context, tokenID string) (bool, error) {
	return b.revoked[tokenID], nil
}

func decodeClaims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims
}

func stringClaim(t *testing.T, claims map[string]any, name string) string {
	t.Helper()
	value, ok := claims[name].(string)
	require.True(t, ok)
	return value
}
