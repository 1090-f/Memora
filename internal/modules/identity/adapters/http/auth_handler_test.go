package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	identityhttp "github.com/1090-f/Memora/internal/modules/identity/adapters/http"
	"github.com/1090-f/Memora/internal/modules/identity/application"
	"github.com/1090-f/Memora/internal/modules/identity/domain"
	"github.com/1090-f/Memora/internal/platform/httpx"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLoginReturnsDocumentedResponse(t *testing.T) {
	router := newIdentityRouter(t)

	response := request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"account": "admin@example.com", "password": "correct",
	}, "")

	require.Equal(t, http.StatusOK, response.Code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	require.Equal(t, "OK", envelope["code"])
	data := envelope["data"].(map[string]any)
	require.NotEmpty(t, data["access_token"])
	require.Equal(t, "Bearer", data["token_type"])
	require.Equal(t, float64(7200), data["expires_in"])
	user := data["user"].(map[string]any)
	require.Equal(t, map[string]any{
		"id": "user-1", "username": "admin", "nickname": "admin",
		"email": "admin@example.com", "avatar_url": nil,
	}, user)
}

func TestLoginDoesNotRevealWhetherAccountExists(t *testing.T) {
	router := newIdentityRouter(t)

	wrongPassword := request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"account": "admin@example.com", "password": "wrong",
	}, "")
	unknownAccount := request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"account": "missing@example.com", "password": "correct",
	}, "")

	require.Equal(t, http.StatusUnauthorized, wrongPassword.Code)
	require.Equal(t, http.StatusUnauthorized, unknownAccount.Code)
	require.Contains(t, wrongPassword.Body.String(), `"code":"UNAUTHORIZED"`)
	require.Contains(t, unknownAccount.Body.String(), `"code":"UNAUTHORIZED"`)
}

func TestLogoutImmediatelyInvalidatesToken(t *testing.T) {
	router := newIdentityRouter(t)
	login := request(t, router, http.MethodPost, "/api/v1/auth/login", map[string]string{
		"account": "admin@example.com", "password": "correct",
	}, "")
	var envelope struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &envelope))

	me := request(t, router, http.MethodGet, "/api/v1/users/me", nil, envelope.Data.AccessToken)
	require.Equal(t, http.StatusOK, me.Code)
	require.Contains(t, me.Body.String(), `"bio":""`)

	logout := request(t, router, http.MethodPost, "/api/v1/auth/logout", nil, envelope.Data.AccessToken)
	require.Equal(t, http.StatusOK, logout.Code)

	meAfterLogout := request(t, router, http.MethodGet, "/api/v1/users/me", nil, envelope.Data.AccessToken)
	require.Equal(t, http.StatusUnauthorized, meAfterLogout.Code)
}

func newIdentityRouter(t *testing.T) *gin.Engine {
	t.Helper()
	hash, err := application.HashPassword("correct")
	require.NoError(t, err)
	user := &domain.User{ID: "user-1", Username: "admin", Email: "admin@example.com", PasswordHash: hash}
	service, err := application.NewAuthService(&handlerUserRepository{user: user}, &handlerBlacklist{revoked: make(map[string]bool)}, "test-secret-with-enough-entropy", 2*time.Hour)
	require.NoError(t, err)
	handler := identityhttp.NewAuthHandler(service)
	middleware := identityhttp.NewAuthMiddleware(service)
	router := gin.New()
	router.Use(httpx.RequestID())
	identityhttp.RegisterRoutes(router, handler, middleware.AuthRequired())
	return router
}

func request(t *testing.T, router http.Handler, method, target string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		require.NoError(t, err)
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(encoded))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

type handlerUserRepository struct{ user *domain.User }

func (r *handlerUserRepository) FindActiveByAccount(_ context.Context, account string) (*domain.User, error) {
	if account != r.user.Username && account != r.user.Email {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

func (r *handlerUserRepository) FindActiveByID(_ context.Context, id string) (*domain.User, error) {
	if id != r.user.ID {
		return nil, domain.ErrUserNotFound
	}
	copy := *r.user
	return &copy, nil
}

type handlerBlacklist struct{ revoked map[string]bool }

func (b *handlerBlacklist) Revoke(_ context.Context, tokenID string, _ time.Duration) error {
	b.revoked[tokenID] = true
	return nil
}

func (b *handlerBlacklist) IsRevoked(_ context.Context, tokenID string) (bool, error) {
	return b.revoked[tokenID], nil
}
