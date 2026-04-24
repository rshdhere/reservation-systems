// 15
package server

import (
	"net/http"

	"github.com/rshdhere/bookmyShow/internal/handler"
	"github.com/rshdhere/bookmyShow/internal/middleware"
	"github.com/rshdhere/bookmyShow/internal/store"
)

func addRoutes(
	mux *http.ServeMux,
	authSvc AuthService,
	userStore store.Store,
) {
	authMiddleware := middleware.Auth(authSvc)

	apiV1 := http.NewServeMux()

	authMux := http.NewServeMux()
	authMux.Handle("POST /signup", handler.HandleSignup(authSvc, userStore))
	authMux.Handle("POST /login", handler.HandleLogin(authSvc, userStore))
	authMux.Handle("POST /logout", handler.HandleLogout())
	authMux.Handle("GET /me", authMiddleware(handler.HandleMe(userStore)))

	apiV1.Handle("/auth/", http.StripPrefix("/auth", authMux))
	apiV1.Handle("GET /users/{id}", authMiddleware(handler.HandleGetUser(userStore)))
	apiV1.Handle("DELETE /users/{id}", authMiddleware(handler.HandleDeleteUser(userStore)))
	apiV1.Handle("POST /todos", authMiddleware(handler.HandleCreateTodo(userStore)))
	apiV1.Handle("GET /todos/{id}", authMiddleware(handler.HandleGetTodo(userStore)))

	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))
	mux.Handle("/healthz", handler.HandleHealthz())
	mux.Handle("/", handler.HandleRoot())
}
