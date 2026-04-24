// 10
package handler

import (
	"context"
	"errors"
	"net/http"
	"net/mail"
	"strings"

	"github.com/rshdhere/bookmyShow/internal/auth"
	"github.com/rshdhere/bookmyShow/internal/model"
	"github.com/rshdhere/bookmyShow/internal/store"
)

type TokenService interface {
	IssueToken(userID uint) (string, error)
	HashPassword(password string) (string, error)
	ComparePassword(hash, password string) error
}

type AuthStore interface {
	CreateUser(ctx context.Context, name, email, passwordHash string) (model.User, error)
	GetUserByEmail(ctx context.Context, email string) (model.User, bool, error)
	GetUserByID(ctx context.Context, id uint) (model.User, bool, error)
}

func HandleSignup(tokenSvc TokenService, userStore AuthStore) http.Handler {
	type request struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		Token string     `json:"token"`
		User  model.User `json:"user"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		email, err := normalizeEmail(req.Email)
		if err != nil {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		if len(req.Password) < 8 {
			http.Error(w, "password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		hash, err := tokenSvc.HashPassword(req.Password)
		if err != nil {
			http.Error(w, "failed to secure password", http.StatusInternalServerError)
			return
		}

		user, err := userStore.CreateUser(r.Context(), name, email, hash)
		if err != nil {
			if errors.Is(err, store.ErrEmailExists) {
				http.Error(w, "email already in use", http.StatusConflict)
				return
			}
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}

		token, err := tokenSvc.IssueToken(user.ID)
		if err != nil {
			http.Error(w, "failed to issue token", http.StatusInternalServerError)
			return
		}

		_ = encode(w, r, http.StatusCreated, response{Token: token, User: user})
	})
}

func HandleLogin(tokenSvc TokenService, userStore AuthStore) http.Handler {
	type request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		Token string     `json:"token"`
		User  model.User `json:"user"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := decode[request](r)
		if err != nil {
			http.Error(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		email, err := normalizeEmail(req.Email)
		if err != nil {
			http.Error(w, "invalid email", http.StatusBadRequest)
			return
		}

		user, ok, err := userStore.GetUserByEmail(r.Context(), email)
		if err != nil {
			http.Error(w, "failed to load user", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := tokenSvc.ComparePassword(user.PasswordHash, req.Password); err != nil {
			if errors.Is(err, auth.ErrInvalidPassword) {
				http.Error(w, "invalid credentials", http.StatusUnauthorized)
				return
			}
			http.Error(w, "failed to verify credentials", http.StatusInternalServerError)
			return
		}

		token, err := tokenSvc.IssueToken(user.ID)
		if err != nil {
			http.Error(w, "failed to issue token", http.StatusInternalServerError)
			return
		}

		_ = encode(w, r, http.StatusOK, response{Token: token, User: user})
	})
}

func HandleMe(userStore AuthStore) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthenticated", http.StatusUnauthorized)
			return
		}

		user, ok, err := userStore.GetUserByID(r.Context(), id)
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

func HandleLogout() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", errors.New("email is required")
	}
	parsed, err := mail.ParseAddress(normalized)
	if err != nil {
		return "", errors.New("email is invalid")
	}
	return parsed.Address, nil
}
