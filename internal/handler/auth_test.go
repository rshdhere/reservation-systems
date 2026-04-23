// 06
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rshdhere/bookmyShow/internal/auth"
	"github.com/rshdhere/bookmyShow/internal/handler"
	"github.com/rshdhere/bookmyShow/internal/model"
	"github.com/rshdhere/bookmyShow/internal/store"
)

type mockTokenService struct {
	issueTokenFn      func(userID uint) (string, error)
	hashPasswordFn    func(password string) (string, error)
	comparePasswordFn func(hash, password string) error
}

func (m *mockTokenService) IssueToken(id uint) (string, error) { return m.issueTokenFn(id) }
func (m *mockTokenService) HashPassword(p string) (string, error) {
	return m.hashPasswordFn(p)
}
func (m *mockTokenService) ComparePassword(h, p string) error { return m.comparePasswordFn(h, p) }

type mockAuthStore struct {
	createUserFn     func(ctx context.Context, name, email, hash string) (model.User, error)
	getUserByEmailFn func(ctx context.Context, email string) (model.User, bool, error)
	getUserByIDFn    func(ctx context.Context, id uint) (model.User, bool, error)
}

func (m *mockAuthStore) CreateUser(ctx context.Context, name, email, hash string) (model.User, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, name, email, hash)
	}
	return model.User{}, nil
}

func (m *mockAuthStore) GetUserByEmail(ctx context.Context, email string) (model.User, bool, error) {
	if m.getUserByEmailFn != nil {
		return m.getUserByEmailFn(ctx, email)
	}
	return model.User{}, false, nil
}

func (m *mockAuthStore) GetUserByID(ctx context.Context, id uint) (model.User, bool, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return model.User{}, false, nil
}

func newJSONRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

var fixedUser = model.User{
	ID:        1,
	Name:      "Alice",
	Email:     "alice@example.com",
	CreatedAt: time.Now(),
	UpdatedAt: time.Now(),
}

var defaultTokenSvc = &mockTokenService{
	hashPasswordFn: func(p string) (string, error) { return "hashed-" + p, nil },
	issueTokenFn:   func(id uint) (string, error) { return "test-token", nil },
}

func TestHandleSignup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       any
		tokenSvc   handler.TokenService
		authStore  handler.AuthStore
		wantStatus int
	}{
		{
			name:     "success",
			body:     map[string]string{"name": "Alice", "email": "alice@example.com", "password": "secret123"},
			tokenSvc: defaultTokenSvc,
			authStore: &mockAuthStore{
				createUserFn: func(_ context.Context, n, e, h string) (model.User, error) {
					return fixedUser, nil
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing name",
			body:       map[string]string{"name": "", "email": "alice@example.com", "password": "secret123"},
			tokenSvc:   defaultTokenSvc,
			authStore:  &mockAuthStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid email",
			body:       map[string]string{"name": "Alice", "email": "not-an-email", "password": "secret123"},
			tokenSvc:   defaultTokenSvc,
			authStore:  &mockAuthStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password too short",
			body:       map[string]string{"name": "Alice", "email": "alice@example.com", "password": "short"},
			tokenSvc:   defaultTokenSvc,
			authStore:  &mockAuthStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "email already exists",
			body:     map[string]string{"name": "Alice", "email": "alice@example.com", "password": "secret123"},
			tokenSvc: defaultTokenSvc,
			authStore: &mockAuthStore{
				createUserFn: func(_ context.Context, n, e, h string) (model.User, error) {
					return model.User{}, store.ErrEmailExists
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "malformed json",
			body:       nil,
			tokenSvc:   defaultTokenSvc,
			authStore:  &mockAuthStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var req *http.Request
			if tt.body != nil {
				req = newJSONRequest(t, http.MethodPost, "/signup", tt.body)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/signup", bytes.NewBufferString("not-json"))
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			handler.HandleSignup(tt.tokenSvc, tt.authStore).ServeHTTP(w, req)
			assertStatus(t, w.Code, tt.wantStatus)
		})
	}
}

func TestHandleSignup_ResponseBody(t *testing.T) {
	t.Parallel()

	svc := &mockTokenService{
		hashPasswordFn: func(p string) (string, error) { return "hashed", nil },
		issueTokenFn:   func(id uint) (string, error) { return "my-token", nil },
	}
	st := &mockAuthStore{
		createUserFn: func(_ context.Context, n, e, h string) (model.User, error) {
			return fixedUser, nil
		},
	}

	req := newJSONRequest(
		t,
		http.MethodPost,
		"/signup",
		map[string]string{"name": "Alice", "email": "alice@example.com", "password": "secret123"},
	)
	w := httptest.NewRecorder()
	handler.HandleSignup(svc, st).ServeHTTP(w, req)

	assertStatus(t, w.Code, http.StatusCreated)

	var resp struct {
		Token string     `json:"token"`
		User  model.User `json:"user"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token != "my-token" {
		t.Errorf("token: got %q, want %q", resp.Token, "my-token")
	}
	if resp.User.Email != fixedUser.Email {
		t.Errorf("email: got %q, want %q", resp.User.Email, fixedUser.Email)
	}
	if resp.User.PasswordHash != "" {
		t.Error("password hash must not be exposed in response")
	}
}

func TestHandleLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       any
		tokenSvc   handler.TokenService
		authStore  handler.AuthStore
		wantStatus int
	}{
		{
			name: "success",
			body: map[string]string{"email": "alice@example.com", "password": "secret123"},
			tokenSvc: &mockTokenService{
				comparePasswordFn: func(h, p string) error { return nil },
				issueTokenFn:      func(id uint) (string, error) { return "token", nil },
			},
			authStore: &mockAuthStore{
				getUserByEmailFn: func(_ context.Context, email string) (model.User, bool, error) {
					return fixedUser, true, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:     "user not found",
			body:     map[string]string{"email": "nobody@example.com", "password": "secret123"},
			tokenSvc: &mockTokenService{},
			authStore: &mockAuthStore{
				getUserByEmailFn: func(_ context.Context, email string) (model.User, bool, error) {
					return model.User{}, false, nil
				},
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong password",
			body: map[string]string{"email": "alice@example.com", "password": "wrongpass"},
			tokenSvc: &mockTokenService{
				comparePasswordFn: func(h, p string) error { return auth.ErrInvalidPassword },
			},
			authStore: &mockAuthStore{
				getUserByEmailFn: func(_ context.Context, email string) (model.User, bool, error) {
					return fixedUser, true, nil
				},
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid email",
			body:       map[string]string{"email": "bad-email", "password": "secret123"},
			tokenSvc:   &mockTokenService{},
			authStore:  &mockAuthStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:     "store error",
			body:     map[string]string{"email": "alice@example.com", "password": "secret123"},
			tokenSvc: &mockTokenService{},
			authStore: &mockAuthStore{
				getUserByEmailFn: func(_ context.Context, email string) (model.User, bool, error) {
					return model.User{}, false, errors.New("db error")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := newJSONRequest(t, http.MethodPost, "/login", tt.body)
			w := httptest.NewRecorder()
			handler.HandleLogin(tt.tokenSvc, tt.authStore).ServeHTTP(w, req)
			assertStatus(t, w.Code, tt.wantStatus)
		})
	}
}

func TestHandleLogout(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()
	handler.HandleLogout().ServeHTTP(w, req)
	assertStatus(t, w.Code, http.StatusNoContent)
}
