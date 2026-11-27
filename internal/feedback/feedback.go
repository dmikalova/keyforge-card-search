// Package feedback sends the feedback of the site's feedback form to the
// maintenance Discord channel.
//
// Cloud Run mounts the secrets as one JSON object at /secrets/config.json
// (the cloud-run-app module in infrastructure). DISCORD_WEBHOOK in that
// object is the webhook of the channel. Without the file, the form is off:
// the page hides the button and the route answers 503.
package feedback

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// DefaultSecretsPath is where the cloud-run-app module mounts the secrets.
const DefaultSecretsPath = "/secrets/config.json"

// The limits of one message.
const (
	// MaxMessage is the longest message in characters. Discord takes 2000
	// characters, and the post adds a heading and the page.
	MaxMessage = 1500
	// maxBody bounds the request body.
	maxBody = 8 << 10
	// maxPage is the longest page URL that the post keeps.
	maxPage = 300
)

// The rate limit: each client IP can send perWindow messages in window.
const (
	perWindow = 5
	window    = 10 * time.Minute
)

// ReadWebhook returns DISCORD_WEBHOOK from the secrets file, or "" if the
// file does not exist.
func ReadWebhook(path string) (string, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("open the secrets directory of %s: %w", path, err)
	}

	defer func() { _ = root.Close() }()

	raw, err := root.ReadFile(filepath.Base(path))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}

	if err != nil {
		return "", fmt.Errorf("read the secrets file %s: %w", path, err)
	}

	var s struct {
		Webhook string `json:"DISCORD_WEBHOOK"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("parse the secrets file %s: %w", path, err)
	}

	return strings.TrimSpace(s.Webhook), nil
}

// Handler is the POST route of the feedback form.
type Handler struct {
	webhook string
	client  *http.Client
	log     *slog.Logger
	now     func() time.Time

	mu   sync.Mutex
	sent map[string][]time.Time
}

// New returns the handler that posts to the webhook. An empty webhook turns
// the form off.
func New(webhook string, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}

	return &Handler{
		webhook: webhook,
		client:  &http.Client{Timeout: 10 * time.Second},
		log:     log,
		now:     time.Now,
		sent:    map[string][]time.Time{},
	}
}

// Enabled reports whether the form can send feedback.
func (h *Handler) Enabled() bool { return h != nil && h.webhook != "" }

// request is the body that the form sends.
type request struct {
	Message string `json:"message"`
	Page    string `json:"page"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled() {
		http.Error(w, "feedback is off", http.StatusServiceUnavailable)
		return
	}

	var req request
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	msg := strings.TrimSpace(req.Message)
	if msg == "" || utf8.RuneCountInString(msg) > MaxMessage {
		http.Error(w, fmt.Sprintf("write 1 to %d characters", MaxMessage), http.StatusBadRequest)
		return
	}

	if !h.allow(clientIP(r)) {
		http.Error(w, "too much feedback: try again later", http.StatusTooManyRequests)
		return
	}

	if err := h.post(r.Context(), msg, page(req.Page)); err != nil {
		h.log.Error("send feedback", "error", err)
		http.Error(w, "the feedback did not send", http.StatusBadGateway)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// allow records a message from the IP and reports whether the IP is under
// the rate limit.
func (h *Handler) allow(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := h.now()

	var recent []time.Time

	for _, t := range h.sent[ip] {
		if now.Sub(t) < window {
			recent = append(recent, t)
		}
	}

	if len(recent) >= perWindow {
		h.sent[ip] = recent
		return false
	}

	h.sent[ip] = append(recent, now)

	return true
}

// post sends the message to Discord. The message is in a quote, and Discord
// parses no mentions, so the text cannot ping anyone.
func (h *Handler) post(ctx context.Context, msg, pageURL string) error {
	content := "**search.keyforge.cards feedback**\n> " +
		strings.ReplaceAll(msg, "\n", "\n> ")
	if pageURL != "" {
		content += "\nPage: <" + pageURL + ">"
	}

	body, err := json.Marshal(map[string]any{
		"content":          content,
		"allowed_mentions": map[string]any{"parse": []string{}},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.webhook, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("discord: %s", resp.Status)
	}

	return nil
}

// page keeps a page URL of the site, or "".
func page(u string) string {
	u = strings.TrimSpace(u)
	if !strings.HasPrefix(u, "https://search.keyforge.cards/") &&
		!strings.HasPrefix(u, "http://localhost") {
		return ""
	}

	if len(u) > maxPage {
		u = u[:maxPage]
	}

	return u
}

// clientIP returns the client IP: the first address of X-Forwarded-For,
// which Cloud Run sets, or the remote address.
func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		ip, _, _ := strings.Cut(f, ",")
		return strings.TrimSpace(ip)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}
