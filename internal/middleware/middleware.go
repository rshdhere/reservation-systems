// 12
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type TokenProvider interface {
	SubjectForToken(ctx context.Context, token string) (string, error)
}

type contextKey struct{}

func SubjectFromContext(ctx context.Context) (string, bool) {
	subject, ok := ctx.Value(contextKey{}).(string)
	if !ok || strings.TrimSpace(subject) == "" {
		return "", false
	}
	return subject, true
}

func Auth(tp TokenProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || strings.TrimSpace(parts[1]) == "" {
				http.Error(w, "invalid authorization header", http.StatusUnauthorized)
				return
			}

			subject, err := tp.SubjectForToken(r.Context(), parts[1])
			if err != nil || strings.TrimSpace(subject) == "" {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func Chain(next http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	chained := next
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		chained = middlewares[i](chained)
	}
	return chained
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("| REQUEST",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		next.ServeHTTP(w, r)
	})
}

func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("| PANIC RECOVERED", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
