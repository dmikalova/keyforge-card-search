// Package catalog gives the server the search engine of the current card
// dataset.
//
// The site does not embed the dataset. Loader reads the dataset that
// keyforge-data publishes to the public bucket, and reads it again when it
// changes, so a new card or set needs no deploy. Static serves one dataset,
// for local work with the fixture package and for tests.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dmikalova/keyforge-data/card"

	"github.com/dmikalova/gallery.keyforge.cards/internal/search"
)

// Source gives the engine of the current dataset.
type Source interface {
	// Engine returns the current engine. It is never nil.
	Engine() *search.Engine
	// Missing reports whether the dataset is not published yet.
	Missing() bool
}

// Static is a Source with one engine.
type Static struct {
	engine *search.Engine
}

// NewStatic returns a Source of the dataset.
func NewStatic(ds *card.Dataset) (*Static, error) {
	e, err := search.New(ds)
	if err != nil {
		return nil, err
	}

	return &Static{engine: e}, nil
}

// Engine returns the engine.
func (s *Static) Engine() *search.Engine { return s.engine }

// Missing returns false.
func (*Static) Missing() bool { return false }

// ImageURL returns the absolute URL of an image path. It returns an empty
// string for an empty path.
func ImageURL(path string) string {
	if path == "" {
		return ""
	}

	return card.ImageBase + path
}

// DefaultInterval is the time between two checks of the dataset.
const DefaultInterval = 5 * time.Minute

// maxDataset is the largest dataset that the loader reads.
const maxDataset = 256 << 20

// Loader is a Source that reads the dataset from a URL and reads it again
// when it changes. On an error it keeps the last good dataset.
type Loader struct {
	URL      string
	HTTP     *http.Client
	Log      *slog.Logger
	Interval time.Duration

	engine  atomic.Pointer[search.Engine]
	missing atomic.Bool
	mu      sync.Mutex
	// version is the version of the loaded dataset.
	version string
}

// NewLoader returns a Loader of the URL with an empty engine. Call Refresh
// or Run to read the dataset.
func NewLoader(url string, log *slog.Logger) (*Loader, error) {
	if log == nil {
		log = slog.Default()
	}

	empty, err := search.New(&card.Dataset{})
	if err != nil {
		return nil, err
	}

	l := &Loader{
		URL:      url,
		HTTP:     &http.Client{Timeout: time.Minute},
		Log:      log,
		Interval: DefaultInterval,
	}
	l.engine.Store(empty)

	return l, nil
}

// Engine returns the engine of the last good dataset.
func (l *Loader) Engine() *search.Engine {
	return l.engine.Load()
}

// Missing reports whether the URL gave 404 and no dataset is loaded.
func (l *Loader) Missing() bool {
	return l.missing.Load()
}

// errNotFound is the error of a dataset URL that gives 404.
var errNotFound = errors.New("catalog: the dataset is not published")

// Refresh reads the pointer file (card.LatestPath) at the loader URL, and
// then the version file that the pointer names when the version changed. On
// an error it keeps the last good dataset.
func (l *Loader) Refresh(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var latest card.Latest
	if err := l.getJSON(ctx, l.URL, maxPointer, &latest); err != nil {
		return fmt.Errorf("the dataset pointer: %w", err)
	}

	if latest.Version == "" || latest.URL == "" {
		return errors.New("catalog: the dataset pointer has no version")
	}

	if latest.Version == l.version {
		return nil
	}

	var ds card.Dataset
	if err := l.getJSON(ctx, latest.URL, maxDataset, &ds); err != nil {
		return fmt.Errorf("the dataset %s: %w", latest.Version, err)
	}

	return l.store(&ds)
}

// maxPointer is the largest pointer file that the loader reads.
const maxPointer = 64 << 10

// getJSON reads a JSON file of at most limit bytes into v. A 404 before the
// first good dataset marks the dataset as missing.
func (l *Loader) getJSON(ctx context.Context, url string, limit int64, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := l.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer l.close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		if l.version == "" {
			l.missing.Store(true)
		}

		return errNotFound
	default:
		return errors.New(resp.Status)
	}

	return json.NewDecoder(io.LimitReader(resp.Body, limit)).Decode(v)
}

// store indexes a dataset and makes it the current one.
func (l *Loader) store(ds *card.Dataset) error {
	e, err := search.New(ds)
	if err != nil {
		return fmt.Errorf("index the dataset: %w", err)
	}

	l.engine.Store(e)
	l.missing.Store(false)
	l.version = ds.Version
	l.Log.Info(
		"dataset loaded",
		"version",
		ds.Version,
		"cards",
		len(ds.Cards),
		"sets",
		len(ds.Sets),
	)

	return nil
}

func (l *Loader) close(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		l.Log.Warn("close the dataset response", "error", err)
	}
}

// Run reads the dataset now and then at each interval, until the context
// ends. It logs the errors.
func (l *Loader) Run(ctx context.Context) {
	t := time.NewTicker(l.Interval)
	defer t.Stop()

	for {
		if err := l.Refresh(ctx); err != nil && ctx.Err() == nil {
			l.Log.Warn("refresh the dataset", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
