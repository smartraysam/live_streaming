package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/smartraysam/livestream-service/pkg/api"
	"github.com/smartraysam/livestream-service/pkg/laravel"
)

type contextKey string

const userContextKey contextKey = "auth_user"

type cachedUser struct {
	user      *laravel.UserInfo
	expiresAt time.Time
}

// AuthMiddleware validates bearer tokens against Laravel and caches user records for 60 seconds.
type AuthMiddleware struct {
	client  laravel.Client
	enabled bool
	cache   sync.Map
}

func NewAuth(client laravel.Client, enabled bool) *AuthMiddleware {
	return &AuthMiddleware{client: client, enabled: enabled}
}

func (m *AuthMiddleware) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.enabled {
			// Auth bypass mode for local/dev smoke tests.
			// You can override identity with X-User-ID or ?user_id=...
			userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
			if userID == "" {
				userID = strings.TrimSpace(r.URL.Query().Get("user_id"))
			}
			if userID == "" {
				userID = "anonymous"
			}

			role := strings.TrimSpace(r.Header.Get("X-User-Role"))
			if role == "" {
				role = strings.TrimSpace(r.URL.Query().Get("role"))
			}
			if role == "" {
				role = "viewer"
			}

			username := strings.TrimSpace(r.Header.Get("X-Username"))
			if username == "" {
				username = strings.TrimSpace(r.URL.Query().Get("username"))
			}
			if username == "" {
				username = userID
			}

			user := &laravel.UserInfo{UserID: userID, Role: role, Username: username}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
			return
		}

		// Check Authorization header first; fall back to ?token= query param
		// (WebSocket browser clients cannot set headers, so they pass token in URL).
		token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer"))
		if token == "" {
			token = strings.TrimSpace(r.URL.Query().Get("token"))
		}
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
