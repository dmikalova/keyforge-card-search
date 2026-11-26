package web

import (
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
