package web

import (
	"strings"
	"testing"

	"github.com/dmikalova/keyforge-data/card"
)

func TestCardMeta(t *testing.T) {
	c := &card.Card{
		ID:    "troll-brobnar",
		Title: "Troll",
		House: "brobnar",
		Type:  "creature",
		Power: &card.Power{N: 8},
		Text:  "Reap: Troll heals 3 damage.\n{tide} Raise the tide.",
		Printings: []card.Printing{
			{
				Set:    "callofthearchons",
				Number: "048",
				Image:  "callofthearchons/048.webp",
				Flavor: "Don’t feed it.",
			},
		},
	}
	v := NewCardView(
		c,
		0,
		func(s string) string { return s },
		func(string) *card.Card { return nil },
	)
	m := CardMeta(v, c)

	want := "Brobnar · Creature · Power 8 · Armor 0\nReap: Troll heals 3 damage.\ntide Raise the tide.\nDon’t feed it."
	if m.Title != "Troll" || m.Description != want ||
		!strings.HasSuffix(m.Image, "callofthearchons/048.webp") {
		t.Errorf("CardMeta = %+v", m)
	}

	if m.Path != "/card/troll-brobnar" {
		t.Errorf("path = %q", m.Path)
	}
}
