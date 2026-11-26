package web

import (
	"slices"
	"strings"
	"testing"

	"github.com/dmikalova/keyforge-data/card"
)

func TestTextParts(t *testing.T) {
	bold := func(parts []TextPart) []string {
		var out []string
		for _, p := range parts {
			if p.Bold {
				out = append(out, p.Text)
			}
		}

		return out
	}

	cases := []struct {
		line string
		want []string
	}{
		{"Play: Gain 1 Æmber.", []string{"Play:"}},
		{"Fight/Reap: Draw a card.", []string{"Fight/Reap:"}},
		{"Play/After Fight/After Reap: Steal 1 Æmber.", []string{"Play/After Fight/After Reap:"}},
		{"Elusive. Before Fight: Deal 2 damage.", []string{"Before Fight:"}},
		{"Each creature gains, “Reap: Draw a card.”", []string{"Reap:"}},
		{"This creature gains, \"Fight/Reap: Archive it.\"", []string{"Fight/Reap:"}},
		{"It gains elusive. Reap something: no", nil},
		{"Destroy a creature. Destroyed: Purge it.", []string{"Destroyed:"}},
	}
	for _, c := range cases {
		parts := TextParts(c.line)
		if got := bold(parts); !slices.Equal(got, c.want) {
			t.Errorf("TextParts(%q) bold = %q, want %q", c.line, got, c.want)
		}

		var all string
		for _, p := range parts {
			all += p.Text
		}

		if all != c.line {
			t.Errorf("TextParts(%q) joins to %q", c.line, all)
		}
	}
}

func TestIcons(t *testing.T) {
	if i := HouseIcon("dis", "Dis"); i.URL != iconBase+"house/dis.webp" || i.Mask {
		t.Errorf("dis icon = %+v", i)
	}

	if i := TypeIcon("creature", "Creature"); i.URL != iconBase+"type/creature.svg" || !i.Mask {
		t.Errorf("creature icon = %+v", i)
	}

	if i := HouseIcon("elders", "Elders"); i.URL != "" {
		t.Errorf("elders has no icon yet: %+v", i)
	}

	if i := RarityIcon("Variant"); i.URL != "" {
		t.Errorf("variant has no icon: %+v", i)
	}

	if i := SetIcon("moremutation", "MoMu"); i.URL != "" {
		t.Errorf("more mutation has no icon: %+v", i)
	}

	if i := StatIcon("Æmber"); i.Mask || i.URL == "" {
		t.Errorf("aember icon = %+v", i)
	}
}

func TestLinkParts(t *testing.T) {
	ms := []card.Mention{
		{ID: "tt", Title: "Timetraveller"},
		{ID: "hf", Title: "Help from Future Self"},
		{ID: "self", Title: "Me"},
	}

	parts := LinkParts(
		[]TextPart{
			{Text: "Play: ", Bold: true},
			{Text: "Search for Help from Future Self or Timetraveller. Me."},
		},
		ms,
		"self",
	)

	var links, all []string
	for _, p := range parts {
		all = append(all, p.Text)
		if p.Link != "" {
			links = append(links, p.Link+"="+p.Text)
		}
	}

	if want := []string{
		"hf=Help from Future Self",
		"tt=Timetraveller",
	}; !slices.Equal(
		links,
		want,
	) {
		t.Errorf("links = %q, want %q", links, want)
	}

	if got := strings.Join(
		all,
		"",
	); got != "Play: Search for Help from Future Self or Timetraveller. Me." {
		t.Errorf("parts join to %q", got)
	}

	if !parts[0].Bold || parts[0].Link != "" {
		t.Errorf("the bold run changed: %+v", parts[0])
	}
}
