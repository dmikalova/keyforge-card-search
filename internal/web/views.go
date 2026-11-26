package web

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/dmikalova/keyforge-data/card"

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
	// ID is the card ID, and Print is the index of the shown printing in the
	// card's printings. A click opens the card view with them.
	ID    string
	Print int
	// Index is the place of the tile in the results, from 0. The page keeps
	// the index of the top tile in the URL, so a reload comes back to it.
	Index int
}

// ErrorView is a query parse error.
type ErrorView struct {
	Msg     string
	Suggest string
	// Before, At, and After are the query split at the error.
	Before, At, After string
}

// FacetGroup is one facet with the links that add or remove its terms.
type FacetGroup struct {
	Name   string
	Counts []FacetLink
	// More are the values after the first ones. The page shows them in a
	// closed list.
	More []FacetLink
}

// FacetLink is one facet value.
type FacetLink struct {
	Label string
	N     int
	// URL is the page of the query with the term toggled: added, or removed
	// if the query has it.
	URL string
	// Term is the query term of the value.
	Term string
	// Active is true if the query has the term at the top level.
	Active bool
	// Percent is the bar width: the count as a part of the largest count.
	Percent int
}

// Chip is one top-level term of the query in the search bar.
type Chip struct {
	Text  string
	Index int
	// Negated is true for a term with a minus sign.
	Negated bool
}

// Results is the part of the search page that a new query replaces: the
// count, the facets, and the first tiles, or the parse error.
type Results struct {
	Err    *ErrorView
	Count  int
	Facets []FacetGroup
	Tiles  []Tile
	// Next is the URL of the next batch, or empty.
	Next string
}

// PageView is the search page.
type PageView struct {
	// Query is the whole query: the chips and the draft.
	Query string
	// Chips are the committed terms. Draft is the text in the input after
	// the chips. A query with a parse error has no chips: it is all draft.
	Chips []Chip
	Draft string
	Results
	// Missing is true when the card dataset is not published yet.
	Missing bool
}

// Signals returns the Datastar signals of the page: the committed query and
// the draft.
func (v PageView) Signals() string {
	return SignalsJSON(strings.Join(chipTexts(v.Chips), " "), v.Draft)
}

// SignalsJSON returns the Datastar signals object.
func SignalsJSON(q, draft string) string {
	b, err := json.Marshal(map[string]string{"q": q, "draft": draft})
	if err != nil {
		return "{}"
	}

	return string(b)
}

func chipTexts(chips []Chip) []string {
	out := make([]string, len(chips))
	for i, c := range chips {
		out[i] = c.Text
	}

	return out
}

// NewChips makes the chips of the terms.
func NewChips(terms []string) []Chip {
	out := make([]Chip, len(terms))
	for i, t := range terms {
		out[i] = Chip{Text: t, Index: i, Negated: strings.HasPrefix(t, "-")}
	}

	return out
}

// NewTiles makes the tiles of the items. The first item has the index
// start.
func NewTiles(items []search.Item, start int) []Tile {
	out := make([]Tile, len(items))

	for i, it := range items {
		out[i] = Tile{
			ID:      it.Card.ID,
			Print:   printIndex(it.Card, it.Printing),
			Title:   it.Card.Title,
			Image:   catalog.ImageURL(it.Printing.Image),
			Caption: it.Set.Code + " " + it.Printing.Number,
			Anomaly: it.Printing.Anomaly,
			Index:   start + i,
		}
	}

	return out
}

func printIndex(c *card.Card, p *card.Printing) int {
	for i := range c.Printings {
		if &c.Printings[i] == p {
			return i
		}
	}

	return 0
}

// SearchURL returns the page URL of a query.
func SearchURL(q string) string {
	if q == "" {
		return "/"
	}

	return "/?q=" + url.QueryEscape(q)
}

// JSONURL returns the export URL of a query.
func JSONURL(q string) string {
	return "/api/cards?q=" + url.QueryEscape(q)
}

// BatchURL returns the URL of the batch at the offset, or empty if the
// offset is past the end.
func BatchURL(q string, offset, total int) string {
	if offset >= total {
		return ""
	}

	return "/batch?q=" + url.QueryEscape(q) + "&offset=" + strconv.Itoa(offset)
}

// facetTop is the number of values that a long facet shows before "more".
const facetTop = 12

// NewFacets makes the facet groups of a result. Each link toggles its term
// in the query.
func NewFacets(e *search.Engine, q string, f search.Facets) []FacetGroup {
	chips, _ := e.Chips(q)

	group := func(name string, counts []search.Count, long bool) FacetGroup {
		g := FacetGroup{Name: name}
		top := 0

		for _, c := range counts {
			top = max(top, c.N)
		}

		for i, c := range counts {
			l := FacetLink{
				Label:   c.Label,
				N:       c.N,
				Term:    c.Term,
				Active:  e.HasChip(chips, c.Term),
				URL:     SearchURL(e.Toggle(q, c.Term)),
				Percent: c.N * 100 / max(top, 1),
			}
			if long && i >= facetTop && !l.Active {
				g.More = append(g.More, l)
			} else {
				g.Counts = append(g.Counts, l)
			}
		}

		return g
	}

	var out []FacetGroup

	for _, g := range []FacetGroup{
		group("House", f.Houses, false),
		group("Type", f.Types, false),
		group("Rarity", f.Rarities, false),
		group("Set", f.Sets, false),
		group("Power", f.Power, false),
		group("Armor", f.Armor, false),
		group("Keyword", f.Keywords, false),
		group("Ability", f.Abilities, false),
		group("Trait", f.Traits, true),
	} {
		if len(g.Counts) > 0 {
			out = append(out, g)
		}
	}

	return out
}

// CardView is the zoomed card: the image and every field of the card.
type CardView struct {
	ID     string
	Title  string
	House  string
	Type   string
	Traits string
	// Stats are the power, armor, and aember lines that the card has.
	Stats     []Field
	Keywords  string
	Abilities string
	Enhance   string
	Flags     string
	Text      []string
	Printings []PrintView
	// Print is the index of the shown printing.
	Print int
}

// Field is one labeled value.
type Field struct {
	Label, Value string
}

// PrintView is one printing in the card view.
type PrintView struct {
	Index   int
	Label   string
	Rarity  string
	Anomaly bool
	Image   string
	Flavor  string
	MVIDs   string
}

// Current returns the shown printing.
func (v CardView) Current() PrintView {
	if v.Print >= 0 && v.Print < len(v.Printings) {
		return v.Printings[v.Print]
	}

	return PrintView{}
}

// NewCardView makes the card view. setName gives the set code of a slug.
func NewCardView(c *card.Card, shown int, setName func(string) string) CardView {
	v := CardView{
		ID:        c.ID,
		Title:     c.Title,
		House:     search.HouseLabel(search.HouseOf(c)),
		Type:      capitalize(c.Type),
		Traits:    strings.Join(mapStrings(c.Traits, capitalize), " • "),
		Keywords:  keywordList(c.Keywords),
		Abilities: strings.Join(mapStrings(c.Abilities, search.AbilityLabel), ", "),
		Enhance:   countList(c.Enhance),
		Flags:     strings.Join(c.Flags, ", "),
		Print:     shown,
	}

	if c.Power != nil {
		v.Stats = append(
			v.Stats,
			Field{"Power", c.Power.String()},
			Field{"Armor", strconv.Itoa(c.Armor)},
		)
	}

	if c.Aember > 0 {
		v.Stats = append(v.Stats, Field{"Æmber", strconv.Itoa(c.Aember)})
	}

	for line := range strings.SplitSeq(c.Text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			v.Text = append(v.Text, line)
		}
	}

	for i, p := range c.Printings {
		v.Printings = append(v.Printings, PrintView{
			Index:   i,
			Label:   setName(p.Set) + " " + p.Number,
			Rarity:  p.Rarity,
			Anomaly: p.Anomaly,
			Image:   catalog.ImageURL(p.Image),
			Flavor:  p.Flavor,
			MVIDs:   strings.Join(p.MVIDs, " "),
		})
	}

	return v
}

func mapStrings(in []string, f func(string) string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = f(s)
	}

	return out
}

func keywordList(m map[string]int) string {
	keys := slices.Sorted(maps.Keys(m))
	parts := make([]string, len(keys))

	for i, k := range keys {
		parts[i] = capitalize(strings.ReplaceAll(k, "-", " "))
		if m[k] > 0 {
			parts[i] += " " + strconv.Itoa(m[k])
		}
	}

	return strings.Join(parts, ", ")
}

func countList(m map[string]int) string {
	keys := slices.Sorted(maps.Keys(m))
	parts := make([]string, len(keys))

	for i, k := range keys {
		parts[i] = capitalize(k) + " ×" + strconv.Itoa(m[k])
	}

	return strings.Join(parts, ", ")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
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
	{
		"ability", "ab", "ab:reap,fight",
		"An ability trigger: play, reap, fight, action, omni, destroyed, beforefight, " +
			"leavesplay, scrap. After Fight counts as fight, and After Reap as reap.",
	},
	{"trait", "tr", "tr:giant", ""},
	{"number", "#", "#001", "The collector number. #S01 works too."},
	{"power", "p", "p:x", "p:x gives the cards with X power."},
	{"armor", "ar", "ar>=1", ""},
	{"aember", "a", "a>=2", "The bonus aember."},
	{"enhance", "en", "en:capture>=1", "The Enhance icons, in total or by icon."},
}

// PageTitle returns the page title of a query.
func PageTitle(q string) string {
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

// action returns a Datastar expression that sends an operation to the
// results endpoint.
func action(op string, params ...string) string {
	var u strings.Builder
	u.WriteString("/results?op=" + op)
	for i := 0; i+1 < len(params); i += 2 {
		u.WriteString("&" + params[i] + "=" + url.QueryEscape(params[i+1]))
	}

	return "@get('" + u.String() + "')"
}

// MaxRestore is the most tiles that a page renders to come back to a place
// in the results.
const MaxRestore = 1200

// FirstCount returns the number of tiles that the page renders first: one
// batch, or the batches up to the tile at the index at and one more batch.
// The extra batch lets the page scroll the tile to the top. The count is at
// most MaxRestore and at most the total.
func FirstCount(at, total int) int {
	n := BatchSize
	if at > 0 {
		n = min((at/BatchSize+2)*BatchSize, MaxRestore)
	}

	return min(n, total)
}

// chipQuery returns the committed query of the chips.
func chipQuery(chips []Chip) string {
	return strings.Join(chipTexts(chips), " ")
}

func placeholder(chips []Chip) string {
	if len(chips) > 0 {
		return "Add a term"
	}

	return "Search cards: h:brobnar p>=5 or k:elusive"
}

func toggleTitle(c FacetLink) string {
	if c.Active {
		return "Remove " + c.Term
	}

	return "Add " + c.Term
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
