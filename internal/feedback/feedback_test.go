package feedback

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadWebhook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if got, err := ReadWebhook(path); err != nil || got != "" {
		t.Errorf("missing file = %q, %v", got, err)
	}

	if err := os.WriteFile(
		path,
		[]byte(`{"DISCORD_WEBHOOK":" https://discord/x "}`),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	if got, err := ReadWebhook(path); err != nil || got != "https://discord/x" {
		t.Errorf("webhook = %q, %v", got, err)
	}
}

func TestHandlerPostsToDiscord(t *testing.T) {
	var got map[string]any

	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	h := New(discord.URL, nil)
	body := `{"message":"Hi @everyone\nline two","page":"https://search.keyforge.cards/?q=h:dis"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}

	content, _ := got["content"].(string)
	if !strings.Contains(content, "> Hi @everyone\n> line two") ||
		!strings.Contains(content, "Page: <https://search.keyforge.cards/?q=h:dis>") {
		t.Errorf("content = %q", content)
	}

	if m, _ := got["allowed_mentions"].(map[string]any); m == nil {
		t.Errorf("no allowed_mentions: %v", got)
	}
}

func TestHandlerChecksTheMessageAndTheRate(t *testing.T) {
	discord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer discord.Close()

	h := New(discord.URL, nil)
	now := time.Unix(0, 0)
	h.now = func() time.Time { return now }

	send := func(msg string) int {
		body, _ := json.Marshal(map[string]string{"message": msg})
		req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(body))
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		return rec.Code
	}

	if code := send("  "); code != http.StatusBadRequest {
		t.Errorf("empty message = %d", code)
	}

	if code := send(strings.Repeat("a", MaxMessage+1)); code != http.StatusBadRequest {
		t.Errorf("long message = %d", code)
	}

	for i := range perWindow {
		if code := send("ok"); code != http.StatusNoContent {
			t.Fatalf("message %d = %d", i, code)
		}
	}

	if code := send("one more"); code != http.StatusTooManyRequests {
		t.Errorf("over the limit = %d", code)
	}

	now = now.Add(window)
	if code := send("later"); code != http.StatusNoContent {
		t.Errorf("after the window = %d", code)
	}
}

func TestHandlerOff(t *testing.T) {
	h := New("", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(`{}`)))

	if h.Enabled() || rec.Code != http.StatusServiceUnavailable {
		t.Errorf("enabled %t, status %d", h.Enabled(), rec.Code)
	}
}
