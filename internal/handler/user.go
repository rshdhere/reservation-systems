// 15
package handler

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/rshdhere/bookmyShow/internal/model"
)

type UserStore interface {
	GetUserByID(ctx context.Context, id uint) (model.User, bool, error)
	DeleteUser(ctx context.Context, id uint) (bool, error)
}

func HandleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "hello from http")
	})
}

func HandleHealthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = encode(w, r, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func HandleGetUser(userStore UserStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}

		user, ok, err := userStore.GetUserByID(r.Context(), uint(id))
		if err != nil {
			http.Error(w, "failed to load user", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		_ = encode(w, r, http.StatusOK, user)
	})
}

func HandleDeleteUser(userStore UserStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "invalid user id", http.StatusBadRequest)
			return
		}

		ok, err := userStore.DeleteUser(r.Context(), uint(id))
		if err != nil {
			http.Error(w, "failed to delete user", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
