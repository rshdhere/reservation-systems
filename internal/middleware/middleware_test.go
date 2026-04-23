// 08
package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rshdhere/bookmyShow/internal/middleware"
)

type mockTokenProvider struct {
	fn func(ctx context.Context, token string) (string, error)
}

func (m *mockTokenProvider) SubjectForToken(ctx context.Context, token string) (string, error) {
	return m.fn(ctx, token)
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	validProvider := &mockTokenProvider{
		fn: func(_ context.Context, token string) (string, error) {
			if token == "valid-token" {
				return "42", nil
			}
			return "", errors.New("invalid token")
		},
	}

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{
			name:       "valid token",
			authHeader: "Bearer valid-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "missing header",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong scheme",
			authHeader: "Basic valid-token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "bearer without token",
			authHeader: "Bearer ",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "just bearer",
			authHeader: "Bearer",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			middleware.Auth(validProvider)(okHandler).ServeHTTP(w, req)
			assertStatus(t, w.Code, tt.wantStatus)
		})
	}
}

func TestAuthMiddleware_SetsSubjectInContext(t *testing.T) {
	t.Parallel()

	provider := &mockTokenProvider{
		fn: func(_ context.Context, token string) (string, error) {
			return "user-99", nil
		},
	}

	var gotSubject string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subj, ok := middleware.SubjectFromContext(r.Context())
		if !ok {
			t.Error("expected subject in context, got none")
		}
		gotSubject = subj
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer any-token")
	w := httptest.NewRecorder()
	middleware.Auth(provider)(next).ServeHTTP(w, req)

	assertStatus(t, w.Code, http.StatusOK)
	if gotSubject != "user-99" {
		t.Errorf("subject: got %q, want %q", gotSubject, "user-99")
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.SecurityHeaders(okHandler).ServeHTTP(w, req)

	headers := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"X-XSS-Protection":       "1; mode=block",
	}
	for header, want := range headers {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("header %s: got %q, want %q", header, got, want)
		}
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()

	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	middleware.Recovery(panicking).ServeHTTP(w, req)

	assertStatus(t, w.Code, http.StatusInternalServerError)
}

func TestChain(t *testing.T) {
	t.Parallel()

	var order []string

	makeMiddleware := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	middleware.Chain(
		okHandler,
		makeMiddleware("first"),
		makeMiddleware("second"),
		makeMiddleware("third"),
	).ServeHTTP(w, req)

	want := []string{"first", "second", "third"}
	for i, got := range order {
		if got != want[i] {
			t.Errorf("order[%d]: got %q, want %q", i, got, want[i])
		}
	}
}

func TestLoggingMiddleware(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest(http.MethodGet, "/some-path", nil)
	w := httptest.NewRecorder()
	middleware.Logging(okHandler).ServeHTTP(w, req)
	assertStatus(t, w.Code, http.StatusOK)
}

func TestSubjectFromContext_Empty(t *testing.T) {
	t.Parallel()
	_, ok := middleware.SubjectFromContext(context.Background())
	if ok {
		t.Error("expected no subject in empty context")
	}
}
