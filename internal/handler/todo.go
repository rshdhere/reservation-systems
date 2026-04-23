package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/rshdhere/bookmyShow/internal/middleware"
	"github.com/rshdhere/bookmyShow/internal/model"
)

type TodoStore interface {
	CreateTodo(ctx context.Context, userID uint, title string) (model.Todo, error)
	GetTodoByID(ctx context.Context, id, userID uint) (model.Todo, bool, error)
}

func userIDFromContext(ctx context.Context) (uint, bool) {
	subject, ok := middleware.SubjectFromContext(ctx)
	if !ok {
		return 0, false
	}

	id, err := strconv.ParseUint(subject, 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}

	return uint(id), true
}

func HandleCreateTodo(todoStore TodoStore) http.Handler {
	type request struct {
		Title string `json:"title"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}

		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		title := strings.TrimSpace(req.Title)
		if title == "" {
			http.Error(w, "title is required", http.StatusBadRequest)
			return
		}

		todo, err := todoStore.CreateTodo(r.Context(), userID, title)
		if err != nil {
			http.Error(w, "failed to create todo", http.StatusInternalServerError)
			return
		}

		_ = encode(w, r, http.StatusCreated, todo)
	})
}

func HandleGetTodo(todoStore TodoStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}

		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid todo id", http.StatusBadRequest)
			return
		}

		todo, found, err := todoStore.GetTodoByID(r.Context(), uint(id), userID)
		if err != nil {
			http.Error(w, "failed to load todo", http.StatusInternalServerError)
			return
		}
		if !found {
			http.Error(w, "todo not found", http.StatusNotFound)
			return
		}

		_ = encode(w, r, http.StatusOK, todo)
	})
}
