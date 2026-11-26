// Package fixture holds a small hand-written card dataset for local work and
// tests. It is not accurate card data: some numbers, texts, and printings are
// made up to cover the special cases (X power, gigantic, evil twin, token
// creature, reference card, houseless card, anomaly printing, reprints,
// Enhance, keywords, and name aliases).
package fixture

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/sets"
)

//go:embed testdata/cards.json
var cardsJSON string

// Dataset returns the hand-written dataset. Its sets are the sets of
// sets.All that the cards use.
func Dataset() (*card.Dataset, error) {
	var cards []card.Card
	if err := json.NewDecoder(strings.NewReader(cardsJSON)).Decode(&cards); err != nil {
		return nil, fmt.Errorf("read the fixture: %w", err)
	}

	used := map[string]bool{}

	for _, c := range cards {
		for _, p := range c.Printings {
			used[p.Set] = true
		}
	}

	ds := &card.Dataset{Version: "fixture", Cards: cards}

	for _, s := range sets.All {
		if used[s.Slug] {
			ds.Sets = append(ds.Sets, s)
		}
	}

	return ds, nil
}
