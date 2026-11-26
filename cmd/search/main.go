// Command search serves the KeyForge card search at search.keyforge.cards.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
	"github.com/dmikalova/keyforge-card-search/internal/search"
	"github.com/dmikalova/keyforge-card-search/internal/server"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 20 * time.Second
	defaultPort       = "8080"
)

func main() {
	if err := run(); err != nil {
		slog.Error("search stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// The keyforge-data dataset does not exist yet. The fixture stands in.
	cards, err := catalog.Fixture()
	if err != nil {
		return err
	}

	engine, err := search.New(cards.Cards())
	if err != nil {
		return fmt.Errorf("make the search engine: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(engine, log).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)

	go func() {
		log.Info("listening", "port", port, "cards", len(cards.Cards()))

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}

	return nil
}
