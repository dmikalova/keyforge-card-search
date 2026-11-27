package web

import (
	"strings"
	"testing"

	"github.com/dmikalova/keyforge-data/card"
)

func TestFirstCount(t *testing.T) {
	cases := []struct{ at, total, want int }{
		{0, 1000, BatchSize},
		{0, 10, 10},
		{59, 1000, 120},
		{60, 1000, 180},
		{240, 1000, 360},
		{240, 250, 250},
		{1150, 100_000, 1260},
		// At the end of a long list, the page renders up to the last tile.
		{1398, 1402, 1402},
		{5990, 100_000, MaxRestore},
		{-5, 1000, BatchSize},
		{1_000_000, 100_000, MaxRestore},
	}
	for _, c := range cases {
		if got := FirstCount(c.at, c.total); got != c.want {
			t.Errorf("FirstCount(%d, %d) = %d, want %d", c.at, c.total, got, c.want)
		}
	}
}

func TestFlavorLinesKeepTheAttribution(t *testing.T) {
	lines := flavorLines("“Onward!”\n—Squire Alys", &card.Card{})
	if len(lines) != 2 || lines[1][0].Text != "—Squire Alys" {
		t.Errorf("lines = %+v, want the attribution on its own line", lines)
	}
}

func TestIconPartsShowTheTide(t *testing.T) {
	parts := IconParts([]TextPart{{Text: "{tide} Reap: Raise the tide."}})
	if len(parts) != 2 || parts[0].Icon.Label != "Tide" ||
		parts[1].Text != " Reap: Raise the tide." {
		t.Errorf("parts = %+v", parts)
	}
}

func TestCommas(t *testing.T) {
	for n, want := range map[int]string{0: "0", 33: "33", 999: "999", 1670: "1,670", 1234567: "1,234,567"} {
		if got := commas(n); got != want {
			t.Errorf("commas(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestCardViewNamesFlagsAndSets checks the card view of a token creature: the
// flags have their names, the type icon is the creature icon, the rarity
// icon is the token creature icon, and the set chip shows the set code.
func TestCardViewNamesFlagsAndSets(t *testing.T) {
	c := &card.Card{
		ID: "grumpus-brobnar", Title: "Grumpus", House: "brobnar", Type: "creature",
		Flags: []string{"reference", "tokencreature"},
		Printings: []card.Printing{
			{Set: "windsofexchange", Number: "T01", Rarity: "Token Creature"},
		},
	}
	v := NewCardView(c, 0, nil,
		func(string) (string, string) { return "WoE", "Winds of Exchange" },
		func(string) *card.Card { return nil })

	var flags []string
	for _, f := range v.Flags {
		flags = append(flags, f.Value)
	}
	if len(flags) != 2 || flags[0] != "Reference" || flags[1] != "Token Creature" {
		t.Errorf("flags = %v", flags)
	}
	if v.Type.Icon.URL != iconBase+"type/creature.svg" {
		t.Errorf("type icon = %q, want the creature icon", v.Type.Icon.URL)
	}
	if v.Rarities[0].Icon.URL != iconBase+"type/tokencreature.svg" {
		t.Errorf("rarity icon = %q, want the token creature icon", v.Rarities[0].Icon.URL)
	}
	if p := v.Sets[0]; p.Label != "WoE T01" || p.Name != "Winds of Exchange T01" ||
		p.Set.Value != "Winds of Exchange" {
		t.Errorf("set chip = %q / %q / %q", p.Label, p.Name, p.Set.Value)
	}
}

// TestThumbSkipsMaverickPrintings checks that a related-card thumbnail is
// the newest printing that is not a maverick.
func TestThumbSkipsMaverickPrintings(t *testing.T) {
	c := &card.Card{Printings: []card.Printing{
		{Set: "grimreminders", Number: "155", Image: "grimreminders/155.webp"},
		{Set: "menagerie", Number: "233", Image: "menagerie/233.webp", Maverick: true},
	}}
	if got := thumb(c); !strings.HasSuffix(got, "grimreminders/155.webp") {
		t.Errorf("thumb = %q, want the Grim Reminders image", got)
	}
}

// TestArtFrom checks the set that a maverick printing borrows its art from.
func TestArtFrom(t *testing.T) {
	c := &card.Card{Printings: []card.Printing{
		{Set: "grimreminders", Image: "grimreminders/155.webp"},
		{Set: "menagerie", Image: "grimreminders/155.webp", Maverick: true},
		{Set: "vaultmasters2024", Image: "vaultmasters2024/229.webp", Maverick: true},
	}}
	if got := artFrom(c, c.Printings[1]); got != "grimreminders" {
		t.Errorf("artFrom(menagerie) = %q, want grimreminders", got)
	}
	if got := artFrom(c, c.Printings[0]); got != "" {
		t.Errorf("artFrom(a normal printing) = %q, want none", got)
	}
	// A maverick printing that keeps its own image borrows nothing.
	if got := artFrom(c, c.Printings[2]); got != "" {
		t.Errorf("artFrom(own image) = %q, want none", got)
	}
}
