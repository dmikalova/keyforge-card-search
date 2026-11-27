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
	"github.com/dmikalova/keyforge-card-search/internal/feedback"
	"github.com/dmikalova/keyforge-card-search/internal/fixture"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	src, err := source(ctx, log)
	if err != nil {
		return err
	}

	// The feedback form posts to the Discord webhook of the mounted secrets.
	// Without the secrets file, as in local work, the form is off.
	webhook, err := feedback.ReadWebhook(feedback.DefaultSecretsPath)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(src, log).WithFeedback(feedback.New(webhook, log)).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errs := make(chan error, 1)

	go func() {
		log.Info("listening", "port", port, "cards", src.Engine().Len())

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

// source returns the dataset source. DATASET_URL is the URL of the pointer
// file data/latest.json. The loader checks the pointer and reads the version
// file that it names when the version changes (ADR 0007 in keyforge-data).
// Without DATASET_URL, it uses the embedded fixture, for local work.
func source(ctx context.Context, log *slog.Logger) (catalog.Source, error) {
	url := os.Getenv("DATASET_URL")
	if url == "" {
		log.Info("DATASET_URL is not set: using the fixture")

		ds, err := fixture.Dataset()
		if err != nil {
			return nil, err
		}

		src, err := catalog.NewStatic(ds)
		if err != nil {
			return nil, fmt.Errorf("make the search engine: %w", err)
		}

		return src, nil
	}

	l, err := catalog.NewLoader(url, log)
	if err != nil {
		return nil, err
	}

	// A failed first read is not fatal: the site starts empty and the
	// loader tries again at each interval.
	if err := l.Refresh(ctx); err != nil {
		log.Warn("read the dataset", "url", url, "error", err)
	}

	go l.Run(ctx)

	return l, nil
}
