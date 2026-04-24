// 14
package server

import (
	"net/http"

	"github.com/rshdhere/bookmyShow/internal/handler"
	"github.com/rshdhere/bookmyShow/internal/middleware"
	"github.com/rshdhere/bookmyShow/internal/store"
)

type AuthService interface {
	handler.TokenService
	middleware.TokenProvider
}

func NewServer(authSvc AuthService, userStore store.Store) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, authSvc, userStore)

	var h http.Handler = mux
	h = middleware.Logging(h)
	h = middleware.SecurityHeaders(h)
	h = middleware.Recovery(h)
	return h
}
