package server

import (
	"net/url"
	"strings"
	"testing"
)

// liveGet sends a search bar operation with the bar's signals.
func liveGet(t *testing.T, target, q, draft string) string {
	t.Helper()

	sig := `{"q":` + jsonString(q) + `,"draft":` + jsonString(draft) + `}`
	sep := "?"
	if strings.Contains(target, "?") {
		sep = "&"
	}

	return get(t, handler(t), target+sep+"datastar="+url.QueryEscape(sig)).Body.String()
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func TestLiveOperations(t *testing.T) {
	cases := []struct {
		name, target, q, draft string
		want                   []string
		not                    []string
	}{
		{
			name:   "typing updates the results and keeps the signals",
			target: "/results?op=type",
			q:      "house:brobnar",
			draft:  "p>=5",
			want: []string{
				`id="results"`,
				`id="chips"`,
				`id="dl"`,
				`kfsearch.update`,
				`power%3E%3D5`,
			},
			not: []string{"datastar-patch-signals"},
		},
		{
			name: "typing a bad term only shows the error line", target: "/results?op=type",
			q: "", draft: "h:",
			want: []string{`id="qerror"`},
			not:  []string{`id="results"`},
		},
		{
			name:   "enter makes the draft into chips",
			target: "/results?op=commit",
			q:      "",
			draft:  "h:brobnar   p>=5",
			want: []string{
				"datastar-patch-signals",
				`"q":"house:brobnar power\u003e=5"`,
				`"draft":""`,
			},
		},
		{
			name:   "a facet click removes a term the query has",
			target: "/results?op=toggle&term=h%3Abrobnar",
			q:      "house:brobnar power>=5",
			draft:  "",
			want:   []string{`"q":"power\u003e=5"`},
		},
		{
			name: "a facet click adds a term", target: "/results?op=toggle&term=k%3Aelusive",
			q: "house:brobnar", draft: "",
			want: []string{`"q":"house:brobnar keyword:elusive"`},
		},
		{
			name: "the × removes a chip", target: "/results?op=remove&i=0",
			q: "house:brobnar power>=5", draft: "",
			want: []string{`"q":"power\u003e=5"`},
		},
		{
			name: "a chip click moves it to the draft", target: "/results?op=edit&i=1",
			q: "house:brobnar power>=5", draft: "",
			want: []string{`"q":"house:brobnar"`, `"draft":"power\u003e=5"`},
		},
		{
			name: "backspace in an empty draft edits the last chip", target: "/results?op=pop",
			q: "house:brobnar power>=5", draft: "",
			want: []string{`"draft":"power\u003e=5"`},
		},
		{
			name: "the fuzzy box writes fuzzy:off", target: "/results?op=fuzzy&on=off",
			q: "n:troll", draft: "",
			want: []string{`fuzzy:off`, `"fuzzy":false`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			body := liveGet(t, c.target, c.q, c.draft)
			for _, w := range c.want {
				if !strings.Contains(body, w) {
					t.Errorf("no %q in\n%s", w, body)
				}
			}
			for _, n := range c.not {
				if strings.Contains(body, n) {
					t.Errorf("unwanted %q in\n%s", n, body)
				}
			}
		})
	}
}

func TestCardView(t *testing.T) {
	body := get(t, handler(t), "/card?id=troll-brobnar&p=0").Body.String()
	for _, want := range []string{`id="zoom"`, "Troll", "Printings", "Master Vault IDs", "kfsearch.opened"} {
		if !strings.Contains(body, want) {
			t.Errorf("card view has no %q", want)
		}
	}

	random := liveGet(t, "/card?random=1", "house:brobnar", "")
	if !strings.Contains(random, "Brobnar") {
		t.Errorf("a random card of h:brobnar is not Brobnar:\n%s", random)
	}

	if w := get(t, handler(t), "/card?id=nope"); w.Code != 404 {
		t.Errorf("unknown card: %d", w.Code)
	}
}
