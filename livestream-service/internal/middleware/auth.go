package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yourorg/livestream-service/pkg/api"
	"github.com/yourorg/livestream-service/pkg/laravel"
)

type contextKey string

const userContextKey contextKey = "auth_user"

type cachedUser struct {
	user      *laravel.UserInfo
	expiresAt time.Time
}

// AuthMiddleware validates bearer tokens against Laravel and caches user records for 60 seconds.
type AuthMiddleware struct {
	client laravel.Client
	cache  sync.Map
}

func NewAuth(client laravel.Client) *AuthMiddleware {
	return &AuthMiddleware{client: client}
}

func (m *AuthMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_bearer_token")
			return
		}
		user, err := m.lookup(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (m *AuthMiddleware) lookup(ctx context.Context, token string) (*laravel.UserInfo, error) {
	if val, ok := m.cache.Load(token); ok {
		entry := val.(cachedUser)
		if time.Now().Before(entry.expiresAt) {
			return entry.user, nil
		}
		m.cache.Delete(token)
	}
	user, err := m.client.VerifyToken(ctx, token)
	if err != nil {
		return nil, err
	}
	m.cache.Store(token, cachedUser{user: user, expiresAt: time.Now().Add(60 * time.Second)})
	return user, nil
}

func UserFromContext(ctx context.Context) (*laravel.UserInfo, bool) {
	u, ok := ctx.Value(userContextKey).(*laravel.UserInfo)
	return u, ok
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
}
