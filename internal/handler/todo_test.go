package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rshdhere/bookmyShow/internal/handler"
	"github.com/rshdhere/bookmyShow/internal/middleware"
	"github.com/rshdhere/bookmyShow/internal/model"
)

type mockTodoStore struct {
	createTodoFn  func(ctx context.Context, userID uint, title string) (model.Todo, error)
	getTodoByIDFn func(ctx context.Context, id, userID uint) (model.Todo, bool, error)
}

func (m *mockTodoStore) CreateTodo(ctx context.Context, userID uint, title string) (model.Todo, error) {
	if m.createTodoFn != nil {
		return m.createTodoFn(ctx, userID, title)
	}
	return model.Todo{}, nil
}

func (m *mockTodoStore) GetTodoByID(ctx context.Context, id, userID uint) (model.Todo, bool, error) {
	if m.getTodoByIDFn != nil {
		return m.getTodoByIDFn(ctx, id, userID)
	}
	return model.Todo{}, false, nil
}

type fixedSubjectProvider struct {
	subject string
	err     error
}

func (f *fixedSubjectProvider) SubjectForToken(_ context.Context, _ string) (string, error) {
	return f.subject, f.err
}

func withAuth(next http.Handler, subject string) http.Handler {
	return middleware.Auth(&fixedSubjectProvider{subject: subject})(next)
}

func TestHandleCreateTodo(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockTodoStore{
			createTodoFn: func(_ context.Context, userID uint, title string) (model.Todo, error) {
				if userID != 42 {
					t.Fatalf("userID: got %d, want %d", userID, 42)
				}
				if title != "Buy ticket" {
					t.Fatalf("title: got %q, want %q", title, "Buy ticket")
				}
				return model.Todo{
					ID:        1,
					UserID:    userID,
					Title:     title,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}, nil
			},
		}

		req := newJSONRequest(t, http.MethodPost, "/todos", map[string]string{"title": " Buy ticket "})
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleCreateTodo(store), "42").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusCreated)

		var todo model.Todo
		if err := json.NewDecoder(w.Body).Decode(&todo); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if todo.ID != 1 {
			t.Errorf("id: got %d, want %d", todo.ID, 1)
		}
		if todo.UserID != 42 {
			t.Errorf("user_id: got %d, want %d", todo.UserID, 42)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		t.Parallel()
		req := newJSONRequest(t, http.MethodPost, "/todos", map[string]string{"title": "Buy ticket"})
		w := httptest.NewRecorder()
		handler.HandleCreateTodo(&mockTodoStore{}).ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusUnauthorized)
	})

	t.Run("invalid payload", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodPost, "/todos", strings.NewReader("{"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleCreateTodo(&mockTodoStore{}), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest)
	})

	t.Run("missing title", func(t *testing.T) {
		t.Parallel()
		req := newJSONRequest(t, http.MethodPost, "/todos", map[string]string{"title": "  "})
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleCreateTodo(&mockTodoStore{}), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest)
	})

	t.Run("store error", func(t *testing.T) {
		t.Parallel()
		store := &mockTodoStore{
			createTodoFn: func(_ context.Context, userID uint, title string) (model.Todo, error) {
				return model.Todo{}, errors.New("db error")
			},
		}
		req := newJSONRequest(t, http.MethodPost, "/todos", map[string]string{"title": "Buy ticket"})
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleCreateTodo(store), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusInternalServerError)
	})
}

func TestHandleGetTodo(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockTodoStore{
			getTodoByIDFn: func(_ context.Context, id, userID uint) (model.Todo, bool, error) {
				if id != 9 {
					t.Fatalf("id: got %d, want %d", id, 9)
				}
				if userID != 7 {
					t.Fatalf("userID: got %d, want %d", userID, 7)
				}
				return model.Todo{ID: 9, UserID: 7, Title: "Watch movie"}, true, nil
			},
		}

		req := httptest.NewRequest(http.MethodGet, "/todos/9", nil)
		req.SetPathValue("id", "9")
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleGetTodo(store), "7").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusOK)

		var todo model.Todo
		if err := json.NewDecoder(w.Body).Decode(&todo); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if todo.Title != "Watch movie" {
			t.Errorf("title: got %q, want %q", todo.Title, "Watch movie")
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/todos/1", nil)
		req.SetPathValue("id", "1")
		w := httptest.NewRecorder()
		handler.HandleGetTodo(&mockTodoStore{}).ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusUnauthorized)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()
		req := httptest.NewRequest(http.MethodGet, "/todos/abc", nil)
		req.SetPathValue("id", "abc")
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleGetTodo(&mockTodoStore{}), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusBadRequest)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		store := &mockTodoStore{
			getTodoByIDFn: func(_ context.Context, id, userID uint) (model.Todo, bool, error) {
				return model.Todo{}, false, nil
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/todos/99", nil)
		req.SetPathValue("id", "99")
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleGetTodo(store), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusNotFound)
	})

	t.Run("store error", func(t *testing.T) {
		t.Parallel()
		store := &mockTodoStore{
			getTodoByIDFn: func(_ context.Context, id, userID uint) (model.Todo, bool, error) {
				return model.Todo{}, false, errors.New("db error")
			},
		}
		req := httptest.NewRequest(http.MethodGet, "/todos/1", nil)
		req.SetPathValue("id", "1")
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		withAuth(handler.HandleGetTodo(store), "1").ServeHTTP(w, req)
		assertStatus(t, w.Code, http.StatusInternalServerError)
	})
}
