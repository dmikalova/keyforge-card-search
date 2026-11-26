package web

import (
	"net/url"
	"strconv"

	"github.com/a-h/templ"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
	"github.com/dmikalova/keyforge-card-search/internal/search"
)

// BatchSize is the number of results in one batch (ADR 0002).
const BatchSize = 60

// Tile is one result in the grid.
type Tile struct {
	Title string
	// Image is the absolute image URL. It can be empty.
	Image string
	// Caption is the set code and the number, for example "GR 239".
	Caption string
	// Anomaly is true for an anomaly printing. The tile shows the anomaly
	// rarity mark.
	Anomaly bool
}

// ErrorView is a query parse error.
type ErrorView struct {
	Msg     string
	Suggest string
	// Before, At, and After are the query split at the error.
	Before, At, After string
}

// FacetGroup is one facet with the links that add its terms.
type FacetGroup struct {
	Name   string
	Counts []FacetLink
	// Histogram is true for power and armor.
	Histogram bool
}

// FacetLink is one facet value.
type FacetLink struct {
	Label string
	N     int
	URL   string
	// Percent is the bar width of a histogram value.
	Percent int
}

// PageView is the search page.
type PageView struct {
	Query  string
	Err    *ErrorView
	Count  int
	Facets []FacetGroup
	Tiles  []Tile
	// Next is the URL of the next batch, or empty.
	Next    string
	JSONURL string
	Houses  []FacetLink
	// Missing is true when the card dataset is not published yet.
	Missing bool
	// Fuzzy is true when fuzzy name matching is on.
	Fuzzy bool
}

// NewTiles makes the tiles of the items.
func NewTiles(items []search.Item) []Tile {
	out := make([]Tile, len(items))

	for i, it := range items {
		out[i] = Tile{
			Title:   it.Card.Title,
			Image:   catalog.ImageURL(it.Printing.Image),
			Caption: it.Set.Code + " " + it.Printing.Number,
			Anomaly: it.Printing.Anomaly,
		}
	}

	return out
}

// SearchURL returns the page URL of a query.
func SearchURL(q string) string {
	if q == "" {
		return "/"
	}

	return "/?q=" + url.QueryEscape(q)
}

// BatchURL returns the URL of the batch at the offset, or empty if the
// offset is past the end.
func BatchURL(q string, offset, total int) string {
	if offset >= total {
		return ""
	}

	return "/batch?q=" + url.QueryEscape(q) + "&offset=" + strconv.Itoa(offset)
}

// NewFacets makes the facet groups of a result.
func NewFacets(q string, f search.Facets) []FacetGroup {
	group := func(name string, counts []search.Count, histogram bool) FacetGroup {
		g := FacetGroup{Name: name, Histogram: histogram}
		top := 0

		for _, c := range counts {
			top = max(top, c.N)
		}

		for _, c := range counts {
			g.Counts = append(g.Counts, FacetLink{
				Label:   c.Label,
				N:       c.N,
				URL:     SearchURL(search.AddTerm(q, c.Term)),
				Percent: c.N * 100 / max(top, 1),
			})
		}

		return g
	}

	var out []FacetGroup

	for _, g := range []FacetGroup{
		group("House", f.Houses, false),
		group("Type", f.Types, false),
		group("Rarity", f.Rarities, false),
		group("Set", f.Sets, false),
		group("Power", f.Power, true),
		group("Armor", f.Armor, true),
	} {
		if len(g.Counts) > 0 {
			out = append(out, g)
		}
	}

	return out
}

// builderHouses are the houses of the builder stub, in printed order.
var builderHouses = []string{
	"brobnar", "dis", "ekwidon", "geistoid", "keyraken", "logos", "mars", "redemption",
	"sanctum", "saurian", "shadows", "skyborn", "staralliance", "unfathomable", "untamed",
}

// NewHouseLinks makes the builder's house links.
func NewHouseLinks(q string) []FacetLink {
	out := make([]FacetLink, len(builderHouses))
	for i, h := range builderHouses {
		out[i] = FacetLink{Label: search.HouseLabel(h), URL: SearchURL(search.AddTerm(q, "h:"+h))}
	}

	return out
}

// syntaxField is one row of the field table on the syntax page.
type syntaxField struct {
	Name, Alias, Example, Notes string
}

var syntaxFields = []syntaxField{
	{"name", "n", "n:anthony", "Fuzzy: the letters in order. Also matches the aliases."},
	{"text", "x", `x:"steal 2"`, "The card text."},
	{"flavor", "fl", "fl:smash", "The flavor text."},
	{"house", "h", "h:staralliance", "h:none gives the cards without a house."},
	{"set", "s", "s:gr", "The set code, slug, or name."},
	{"first", "", "first:gr", "The set of the card's first printing."},
	{"type", "t", "t:artifact", ""},
	{"rarity", "r", "r:rare", ""},
	{"keyword", "k", "k:assault>=2", "A keyword, with an optional number."},
	{"trait", "tr", "tr:giant", ""},
	{"number", "#", "#001", "The collector number. #S01 works too."},
	{"power", "p", "p:x", "p:x gives the cards with X power."},
	{"armor", "ar", "ar>=1", ""},
	{"aember", "a", "a>=2", "The bonus aember."},
	{"enhance", "en", "en:capture>=1", "The Enhance icons, in total or by icon."},
}

func pageTitle(q string) string {
	if q == "" {
		return "KeyForge card search"
	}

	return q + " · KeyForge card search"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}

	return many
}

func barStyle(percent int) templ.SafeCSS {
	return templ.SafeCSS("width:" + strconv.Itoa(percent) + "%")
}

// FuzzyOff reports whether the query turns fuzzy name matching off.
func FuzzyOff(q string) bool {
	return search.FuzzyIsOff(q)
}

// QueryEscape escapes a query for a URL.
func QueryEscape(q string) string {
	return url.QueryEscape(q)
}
