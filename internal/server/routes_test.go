// 09
package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rshdhere/bookmyShow/internal/auth"
	"github.com/rshdhere/bookmyShow/internal/model"
	"github.com/rshdhere/bookmyShow/internal/server"
	"github.com/rshdhere/bookmyShow/internal/store"
)

type mockStore struct {
	mu     sync.RWMutex
	users  map[uint]model.User
	nextID uint
}

func newMockStore() *mockStore {
	return &mockStore{
		users:  make(map[uint]model.User),
		nextID: 1,
	}
}

func (m *mockStore) CreateUser(_ context.Context, name, email, passwordHash string) (model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, u := range m.users {
		if u.Email == email {
			return model.User{}, store.ErrEmailExists
		}
	}

	user := model.User{
		ID:           m.nextID,
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	m.users[m.nextID] = user
	m.nextID++
	return user, nil
}

func (m *mockStore) GetUserByEmail(_ context.Context, email string) (model.User, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.Email == email {
			return u, true, nil
		}
	}
	return model.User{}, false, nil
}

func (m *mockStore) GetUserByID(_ context.Context, id uint) (model.User, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[id]
	return u, ok, nil
}

func (m *mockStore) DeleteUser(_ context.Context, id uint) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[id]; !ok {
		return false, nil
	}
	delete(m.users, id)
	return true, nil
}

func (m *mockStore) Close() error { return nil }

func newTestServer(t *testing.T) (*httptest.Server, *auth.Service, *mockStore) {
	t.Helper()

	authSvc, err := auth.NewService(auth.Config{
		Secret:   "test-secret-that-is-at-least-32-chars-long",
		Issuer:   "test",
		TokenTTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}

	ms := newMockStore()
	ts := httptest.NewServer(server.NewServer(authSvc, ms))
	t.Cleanup(ts.Close)
	return ts, authSvc, ms
}

func doRequest(t *testing.T, ts *httptest.Server, method, path, token, body string) *http.Response {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequest(method, ts.URL+path, bodyReader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func signup(t *testing.T, ts *httptest.Server, name, email, password string) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"email":%q,"password":%q}`, name, email, password)
	return doRequest(t, ts, http.MethodPost, "/api/v1/auth/signup", "", body)
}

func loginGetToken(t *testing.T, ts *httptest.Server, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", "", body)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return result.Token
}

func assertStatus(t *testing.T, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("status: got %d, want %d", got, want)
	}
}

func TestRoutes_Healthz(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)
	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertStatus(t, resp.StatusCode, http.StatusOK)
}

func TestRoutes_Root(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)
	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	assertStatus(t, resp.StatusCode, http.StatusOK)
}

func TestRoutes_Signup(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)

	t.Run("success", func(t *testing.T) {
		resp := signup(t, ts, "Alice", "alice@example.com", "secret123")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusCreated)
	})

	t.Run("duplicate email", func(t *testing.T) {
		signup(t, ts, "Bob", "bob@example.com", "secret123")
		resp := signup(t, ts, "Bob2", "bob@example.com", "secret123")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusConflict)
	})

	t.Run("invalid email", func(t *testing.T) {
		resp := signup(t, ts, "Charlie", "not-an-email", "secret123")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusBadRequest)
	})

	t.Run("short password", func(t *testing.T) {
		resp := signup(t, ts, "Dave", "dave@example.com", "short")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusBadRequest)
	})
}

func TestRoutes_Login(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)

	resp := signup(t, ts, "Alice", "alice@example.com", "secret123")
	resp.Body.Close()

	t.Run("success", func(t *testing.T) {
		body := `{"email":"alice@example.com","password":"secret123"}`
		resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", "", body)
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusOK)

		var result struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if result.Token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		body := `{"email":"alice@example.com","password":"wrongpass"}`
		resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", "", body)
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})

	t.Run("unknown email", func(t *testing.T) {
		body := `{"email":"nobody@example.com","password":"secret123"}`
		resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", "", body)
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})
}

func TestRoutes_Logout(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/logout", "", "")
	defer resp.Body.Close()
	assertStatus(t, resp.StatusCode, http.StatusNoContent)
}

func TestRoutes_Me(t *testing.T) {
	t.Parallel()
	ts, _, _ := newTestServer(t)

	signup(t, ts, "Alice", "alice@example.com", "secret123").Body.Close()
	token := loginGetToken(t, ts, "alice@example.com", "secret123")

	t.Run("authenticated", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/auth/me", token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusOK)

		var user model.User
		if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if user.Email != "alice@example.com" {
			t.Errorf("email: got %q, want %q", user.Email, "alice@example.com")
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/auth/me", "", "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})

	t.Run("invalid token", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/auth/me", "bad.token.here", "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})
}

func TestRoutes_GetUser(t *testing.T) {
	t.Parallel()
	ts, _, ms := newTestServer(t)

	signup(t, ts, "Alice", "alice@example.com", "secret123").Body.Close()
	token := loginGetToken(t, ts, "alice@example.com", "secret123")

	ms.mu.RLock()
	var userID uint
	for id := range ms.users {
		userID = id
		break
	}
	ms.mu.RUnlock()

	t.Run("found", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", userID), token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusOK)
	})

	t.Run("not found", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/users/9999", token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusNotFound)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, fmt.Sprintf("/api/v1/users/%d", userID), "", "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})

	t.Run("invalid id", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/users/abc", token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusBadRequest)
	})
}

func TestRoutes_DeleteUser(t *testing.T) {
	t.Parallel()
	ts, _, ms := newTestServer(t)

	signup(t, ts, "Alice", "alice@example.com", "secret123").Body.Close()
	token := loginGetToken(t, ts, "alice@example.com", "secret123")

	ms.mu.RLock()
	var userID uint
	for id := range ms.users {
		userID = id
		break
	}
	ms.mu.RUnlock()

	t.Run("unauthenticated", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", userID), "", "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusUnauthorized)
	})

	t.Run("not found", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodDelete, "/api/v1/users/9999", token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusNotFound)
	})

	t.Run("success", func(t *testing.T) {
		resp := doRequest(t, ts, http.MethodDelete, fmt.Sprintf("/api/v1/users/%d", userID), token, "")
		defer resp.Body.Close()
		assertStatus(t, resp.StatusCode, http.StatusNoContent)
	})
}
