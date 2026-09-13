// Package web serves the application UI over HTTP on top of the centralized
// SQLite database. It never accepts a database path: every handler reads from
// the single shared *database.DB handed to Serve.
package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"sarrast/internal/database"
)

// DefaultAddr is the fixed address the server listens on.
const DefaultAddr = "127.0.0.1:8080"

// Options configures the server.
type Options struct {
	// Addr is the listen address; DefaultAddr when empty.
	Addr string
	// Log receives server logs; slog.Default() when nil.
	Log *slog.Logger
}

// server wires the handlers to a single shared database.
type server struct {
	db  *database.DB
	log *slog.Logger
}

// Serve starts an HTTP server on Addr backed by db and blocks until the
// process exits. It returns false on a fatal listen error.
func Serve(db *database.DB, opts Options) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := Listen(ctx, db, opts); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(stderr, "web server:", err)
		return false
	}
	return true
}

// Listen runs the server until ctx is cancelled.
func Listen(ctx context.Context, db *database.DB, opts Options) error {
	if db == nil {
		return fmt.Errorf("web: nil database")
	}
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}
	addr := opts.Addr
	if addr == "" {
		addr = DefaultAddr
	}

	s, err := newServer(db, log)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpServer.ListenAndServe() }()
	log.Info("web server listening", "addr", addr, "database", database.StoragePath)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		return err
	}
}
