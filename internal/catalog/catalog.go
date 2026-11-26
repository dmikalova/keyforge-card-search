// Package catalog holds the card data that the site searches.
//
// The dataset of keyforge-data does not exist yet, so the site starts with a
// small hand-written fixture. The fixture is not accurate card data: some
// numbers, texts, and printings are made up to cover the special cases
// (X power, gigantic, evil twin, token creature, reference card, houseless
// card, anomaly printing, reprints, Enhance, keywords, and name aliases).
// When the dataset exists, a second Catalog reads it and the fixture moves
// to the tests.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
)

// ImageBaseURL is the public bucket folder of the card images. A printing's
// Image is a path under it, for example "grimreminders/239.webp".
const ImageBaseURL = "https://storage.googleapis.com/mklv-keyforge-cards-public/images/cards/"

// Catalog gives the cards that the site searches.
type Catalog interface {
	// Cards returns every card. The caller must not change the cards.
	Cards() []card.Card
}

// Static is a Catalog that holds its cards in memory.
type Static struct {
	cards []card.Card
}

// Cards returns every card.
func (s *Static) Cards() []card.Card {
	return s.cards
}

// Load reads a JSON array of cards.
func Load(r io.Reader) (*Static, error) {
	var cards []card.Card
	if err := json.NewDecoder(r).Decode(&cards); err != nil {
		return nil, fmt.Errorf("read the cards: %w", err)
	}

	return &Static{cards: cards}, nil
}

//go:embed testdata/cards.json
var fixture string

// Fixture returns the embedded hand-written cards.
func Fixture() (*Static, error) {
	return Load(strings.NewReader(fixture))
}

// ImageURL returns the absolute URL of an image path. It returns an empty
// string for an empty path.
func ImageURL(path string) string {
	if path == "" {
		return ""
	}

	return ImageBaseURL + path
}
