// 07
package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rshdhere/bookmyShow/internal/handler"
	"github.com/rshdhere/bookmyShow/internal/model"
)

type mockUserStore struct {
	getUserByIDFn func(ctx context.Context, id uint) (model.User, bool, error)
	deleteUserFn  func(ctx context.Context, id uint) (bool, error)
}

func (m *mockUserStore) GetUserByID(ctx context.Context, id uint) (model.User, bool, error) {
	if m.getUserByIDFn != nil {
		return m.getUserByIDFn(ctx, id)
	}
	return model.User{}, false, nil
}

func (m *mockUserStore) DeleteUser(ctx context.Context, id uint) (bool, error) {
	if m.deleteUserFn != nil {
		return m.deleteUserFn(ctx, id)
	}
	return false, nil
}

func TestHandleGetUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathID     string
		userStore  handler.UserStore
		wantStatus int
	}{
		{
			name:   "success",
			pathID: "1",
			userStore: &mockUserStore{
				getUserByIDFn: func(_ context.Context, id uint) (model.User, bool, error) {
					return fixedUser, true, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:   "not found",
			pathID: "999",
			userStore: &mockUserStore{
				getUserByIDFn: func(_ context.Context, id uint) (model.User, bool, error) {
					return model.User{}, false, nil
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			pathID:     "abc",
			userStore:  &mockUserStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "store error",
			pathID: "1",
			userStore: &mockUserStore{
				getUserByIDFn: func(_ context.Context, id uint) (model.User, bool, error) {
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
			req := httptest.NewRequest(http.MethodGet, "/users/"+tt.pathID, nil)
			req.SetPathValue("id", tt.pathID)
			w := httptest.NewRecorder()
			handler.HandleGetUser(tt.userStore).ServeHTTP(w, req)
			assertStatus(t, w.Code, tt.wantStatus)
		})
	}
}

func TestHandleDeleteUser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		pathID     string
		userStore  handler.UserStore
		wantStatus int
	}{
		{
			name:   "success",
			pathID: "1",
			userStore: &mockUserStore{
				deleteUserFn: func(_ context.Context, id uint) (bool, error) {
					return true, nil
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:   "not found",
			pathID: "999",
			userStore: &mockUserStore{
				deleteUserFn: func(_ context.Context, id uint) (bool, error) {
					return false, nil
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			pathID:     "xyz",
			userStore:  &mockUserStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "store error",
			pathID: "1",
			userStore: &mockUserStore{
				deleteUserFn: func(_ context.Context, id uint) (bool, error) {
					return false, errors.New("db error")
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodDelete, "/users/"+tt.pathID, nil)
			req.SetPathValue("id", tt.pathID)
			w := httptest.NewRecorder()
			handler.HandleDeleteUser(tt.userStore).ServeHTTP(w, req)
			assertStatus(t, w.Code, tt.wantStatus)
		})
	}
}

func TestHandleRoot(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.HandleRoot().ServeHTTP(w, req)
	assertStatus(t, w.Code, http.StatusOK)
}

func TestHandleHealthz(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.HandleHealthz().ServeHTTP(w, req)
	assertStatus(t, w.Code, http.StatusOK)
}
