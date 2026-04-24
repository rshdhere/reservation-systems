// 13
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rshdhere/bookmyShow/internal/auth"
	"github.com/rshdhere/bookmyShow/internal/store"
)

func Run(
	ctx context.Context,
	getenv func(string) string,
	stderr io.Writer,
) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	port := getenv("PORT")
	if port == "" {
		port = "8080"
	}

	userStore, err := store.NewPostgresStore(getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("init store: %w", err)
	}
	defer func() {
		_ = userStore.Close()
	}()

	tokenTTL, err := parseTokenTTL(getenv("JWT_TTL"))
	if err != nil {
		return fmt.Errorf("parse jwt ttl: %w", err)
	}

	authSvc, err := auth.NewService(auth.Config{
		Secret:   getenv("JWT_SECRET"),
		Issuer:   getenv("JWT_ISSUER"),
		TokenTTL: tokenTTL,
	})
	if err != nil {
		return fmt.Errorf("init auth: %w", err)
	}

	srv := NewServer(authSvc, userStore)

	httpServer := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      srv,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  10 * time.Second,
	}

	go func() {
		_, _ = fmt.Fprintf(stderr, "| SERVER LIVE on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			_, _ = fmt.Fprintf(stderr, "| SERVER FAILED: %s\n", err)
		}
	}()

	<-ctx.Done()
	_, _ = fmt.Fprintf(stderr, "| SERVER SHUTTING DOWN\n")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			_, _ = fmt.Fprintf(stderr, "| SERVER FORCED TO SHUTDOWN: %s\n", err)
		}
	}()

	wg.Wait()
	_, _ = fmt.Fprintf(stderr, "| SERVER STOPPED\n")
	return nil
}

func parseTokenTTL(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid jwt ttl %q: %w", value, err)
	}
	if d <= 0 {
		return 0, errors.New("jwt ttl must be greater than zero")
	}
	return d, nil
}
