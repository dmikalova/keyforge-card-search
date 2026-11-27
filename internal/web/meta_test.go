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
		nil,
		func(s string) string { return s },
		func(string) *card.Card { return nil },
	)
	m := CardMeta(v, c)

	want := "Brobnar · Creature · P 8\nReap: Troll heals 3 damage.\ntide Raise the tide."
	if m.Title != "Troll" || m.Description != want ||
		!strings.HasSuffix(m.Image, "callofthearchons/048.webp") {
		t.Errorf("CardMeta = %+v", m)
	}

	if m.Path != "/card/troll-brobnar" {
		t.Errorf("path = %q", m.Path)
	}
}

// TestCardNotes checks that the card view and the link preview show the
// notes of a card.
func TestCardNotes(t *testing.T) {
	c := &card.Card{
		ID: "plaguerat-dis", Title: "Plague Rat", House: "dis", Type: "creature",
		Power: &card.Power{N: 1},
		Notes: []card.Note{{Kind: "copies", Text: "Always 3+ copies"}},
		Printings: []card.Printing{
			{Set: "callofthearchons", Number: "089", Image: "callofthearchons/089.webp"},
		},
	}
	v := NewCardView(c, 0, nil, func(s string) string { return s },
		func(string) *card.Card { return nil })

	if len(v.Notes) != 1 || v.Notes[0] != "Always 3+ copies" {
		t.Errorf("notes = %v", v.Notes)
	}

	if m := CardMeta(v, c); !strings.HasSuffix(m.Description, "\nAlways 3+ copies") {
		t.Errorf("description = %q", m.Description)
	}
}

// TestCardMetaStats checks the stats of the link preview: X power, armor
// above 0, and no stats on a card without power.
func TestCardMetaStats(t *testing.T) {
	for _, c := range []struct {
		card card.Card
		want string
	}{
		{card.Card{House: "sanctum", Type: "creature", Power: &card.Power{N: 5}, Armor: 1}, "Sanctum · Creature · P 5 · A 1"},
		{card.Card{House: "logos", Type: "creature", Power: &card.Power{X: true}}, "Logos · Creature · P X"},
		{card.Card{House: "dis", Type: "action"}, "Dis · Action"},
	} {
		c.card.Printings = []card.Printing{{Set: "callofthearchons", Number: "001"}}
		v := NewCardView(&c.card, 0, nil, func(s string) string { return s },
			func(string) *card.Card { return nil })

		if got := CardMeta(v, &c.card).Description; got != c.want {
			t.Errorf("description = %q, want %q", got, c.want)
		}
	}
}
