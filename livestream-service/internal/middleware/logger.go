package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	s.status = status
	s.ResponseWriter.WriteHeader(status)
}

// Logger emits request logs with method, path, status, latency and user id when available.
func Logger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			uid := "anonymous"
			if u, ok := UserFromContext(r.Context()); ok {
				uid = u.UserID
			}

			log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rec.status).
				Dur("latency", time.Since(start)).
				Str("user_id", uid).
				Msg("request completed")
		})
	}
}
