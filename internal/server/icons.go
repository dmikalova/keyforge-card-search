package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

// IconOrigin is where keyforge-data publishes the icons.
const IconOrigin = "https://storage.googleapis.com/mklv-keyforge-cards-public/images/icons/"

// iconPathRe is the form of an icon path: a group and a file name.
var iconPathRe = regexp.MustCompile(`^(house|rarity|set|stat|type)/[a-z0-9]+\.(svg|webp)$`)

// maxIcon is the largest icon that the proxy reads.
const maxIcon = 1 << 20

// iconTTL is the time that the proxy keeps an icon. The bucket gives the
// icons a cache time of one day.
const iconTTL = 24 * time.Hour

// iconProxy serves the published icons from the site's own origin. The page
// draws an SVG icon as a CSS mask, and a browser loads a mask image from
// another origin only with CORS headers, which the bucket does not send.
type iconProxy struct {
	origin string
	client *http.Client

	mu    sync.Mutex
	cache map[string]cachedIcon
}

type cachedIcon struct {
	body    []byte
	ctype   string
	fetched time.Time
	// missing is true when the origin has no icon at the path.
	missing bool
}

func newIconProxy(origin string) *iconProxy {
	return &iconProxy{
		origin: origin,
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  map[string]cachedIcon{},
	}
}

func (p *iconProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if !iconPathRe.MatchString(path) {
		http.NotFound(w, r)

		return
	}

	ic, err := p.get(r.Context(), path)
	if err != nil {
		http.Error(w, "icon unavailable", http.StatusBadGateway)

		return
	}

	if ic.missing {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", ic.ctype)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(ic.body)
}

func (p *iconProxy) get(ctx context.Context, path string) (cachedIcon, error) {
	p.mu.Lock()
	ic, ok := p.cache[path]
	p.mu.Unlock()

	if ok && time.Since(ic.fetched) < iconTTL {
		return ic, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.origin+path, nil)
	if err != nil {
		return cachedIcon{}, err
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return cachedIcon{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	ic = cachedIcon{fetched: time.Now()}

	switch resp.StatusCode {
	case http.StatusOK:
		if ic.body, err = io.ReadAll(io.LimitReader(resp.Body, maxIcon)); err != nil {
			return cachedIcon{}, err
		}

		ic.ctype = resp.Header.Get("Content-Type")
	case http.StatusNotFound, http.StatusForbidden:
		ic.missing = true
	default:
		return cachedIcon{}, fmt.Errorf("icon origin: %s", resp.Status)
	}

	p.mu.Lock()
	p.cache[path] = ic
	p.mu.Unlock()

	return ic, nil
}
