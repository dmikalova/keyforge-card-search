package web

import (
	"slices"
	"testing"
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
		{"Each creature gains, “Reap: Draw a card.”", nil},
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
	if i := chipIcon("h:dis"); i.URL != iconBase+"house/dis.webp" || i.Mask {
		t.Errorf("h:dis icon = %+v", i)
	}

	if i := chipIcon("-type:creature"); i.URL != iconBase+"type/creature.svg" || !i.Mask {
		t.Errorf("-type:creature icon = %+v", i)
	}

	if i := chipIcon("h:dis,logos"); i.URL != "" {
		t.Errorf("a list chip has no icon: %+v", i)
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
