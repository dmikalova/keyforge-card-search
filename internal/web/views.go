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
	// Icon is the icon of the value, or the zero Icon.
	Icon Icon
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
	// Query is the query text in the search bar.
	Query string
	Results
	// Missing is true when the card dataset is not published yet.
	Missing bool
	// Complete is the JSON of the search bar completions.
	Complete string
	// Card is the card view that the page opens, from the URL (card=ID),
	// or nil.
	Card *CardView
}

// Signals returns the Datastar signals of the page: the query text.
func (v PageView) Signals() string {
	return SignalsJSON(v.Query)
}

// SignalsJSON returns the Datastar signals object of the search bar.
func SignalsJSON(q string) string {
	b, err := json.Marshal(map[string]any{"q": q})
	if err != nil {
		return "{}"
	}

	return string(b)
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
	terms, _ := e.Terms(q)

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
				Active:  e.HasTerm(terms, c.Term),
				URL:     SearchURL(e.Toggle(q, c.Term)),
				Percent: c.N * 100 / max(top, 1),
				Icon:    facetIcon(name, c.Key, c.Label),
			}
			if long && i >= facetTop && !l.Active {
				g.More = append(g.More, l)
			} else {
				g.Counts = append(g.Counts, l)
			}
		}

		blankIcons(g.Counts)
		blankIcons(g.More)

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
	ID        string
	Title     string
	House     string
	HouseIcon Icon
	Type      string
	TypeIcon  Icon
	Traits    string
	// Stats are the power, armor, and aember lines that the card has.
	Stats     []Field
	Keywords  string
	Abilities string
	Enhance   string
	Flags     string
	// Rarities are the distinct rarities of the printings, oldest first.
	Rarities []Field
	// Text is the card text, one line in each element, with the trigger
	// lists bold and the names of other cards as links.
	Text [][]TextPart
	// Sets are the printings: the set and the number.
	Sets []PrintView
	// Related are the threads of related cards.
	Related []RelatedView
	// Print is the index of the shown printing.
	Print int
}

// Field is one labeled value.
type Field struct {
	Label, Value string
	Icon         Icon
}

// PrintView is one printing in the card view.
type PrintView struct {
	Index   int
	Label   string
	SetIcon Icon
	Image   string
	// Flavor is the flavor text, one line in each element, with the names
	// of other cards as links. An attribution ("—Name") is its own line.
	Flavor [][]TextPart
}

// RelatedView is one thread of related cards in the card view.
type RelatedView struct {
	Label string
	Cards []RelatedCard
}

// RelatedCard is a card in a thread. A click opens it in the card view.
type RelatedCard struct {
	ID, Title string
	// Image is the thumbnail URL. It can be empty.
	Image string
}

// Lookup finds a card by ID for the card view. It returns nil for an
// unknown ID.
type Lookup func(id string) *card.Card

// Current returns the shown printing.
func (v CardView) Current() PrintView {
	if v.Print >= 0 && v.Print < len(v.Sets) {
		return v.Sets[v.Print]
	}

	return PrintView{}
}

// NewCardView makes the card view. setName gives the set code of a slug,
// and find gives the other cards of the mentions and the threads.
func NewCardView(c *card.Card, shown int, setName func(string) string, find Lookup) CardView {
	v := CardView{
		ID:        c.ID,
		Title:     c.Title,
		House:     search.HouseLabel(search.HouseOf(c)),
		HouseIcon: HouseIcon(c.House, search.HouseLabel(c.House)),
		Type:      capitalize(c.Type),
		TypeIcon:  TypeIcon(cardTypeKey(c), capitalize(c.Type)),
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
			Field{"Power", c.Power.String(), StatIcon("Power")},
			Field{"Armor", strconv.Itoa(c.Armor), StatIcon("Armor")},
		)
	}

	if c.Aember > 0 {
		v.Stats = append(v.Stats, Field{"Æmber", strconv.Itoa(c.Aember), StatIcon("Æmber")})
	}

	for line := range strings.SplitSeq(c.Text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			v.Text = append(v.Text, IconParts(LinkParts(TextParts(line), c.Mentions, c.ID)))
		}
	}

	for i, p := range c.Printings {
		v.Sets = append(v.Sets, PrintView{
			Index:   i,
			Label:   setName(p.Set) + " " + p.Number,
			SetIcon: SetIcon(p.Set, setName(p.Set)),
			Image:   catalog.ImageURL(p.Image),
			Flavor:  flavorLines(p.Flavor, c),
		})

		r := printRarity(p)
		if !slices.ContainsFunc(
			v.Rarities,
			func(f Field) bool { return f.Value == rarityLabel(r) },
		) {
			v.Rarities = append(
				v.Rarities,
				Field{Label: "Rarity", Value: rarityLabel(r), Icon: RarityIcon(r)},
			)
		}
	}

	for _, rel := range c.Related {
		rv := RelatedView{Label: rel.Label}

		for _, id := range rel.Cards {
			if o := find(id); o != nil && o.ID != c.ID {
				rv.Cards = append(rv.Cards, RelatedCard{ID: o.ID, Title: o.Title, Image: thumb(o)})
			}
		}

		if len(rv.Cards) > 0 {
			v.Related = append(v.Related, rv)
		}
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
	{
		"name",
		"n",
		"n:anthony",
		"Part of the name. Spaces and punctuation do not matter. Also matches the aliases.",
	},
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
const MaxRestore = 6000

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

// placeholder is the hint in the empty search bar.
const placeholder = "Search cards: h:brobnar p>=5 or k:elusive"

func toggleTitle(c FacetLink) string {
	if c.Active {
		return "Remove " + c.Term
	}

	return "Add " + c.Term
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

// cardTypeKey returns the type icon name of a card: a token creature has
// its own icon.
func cardTypeKey(c *card.Card) string {
	if c.HasFlag(search.FlagTokenCreature) {
		return "tokencreature"
	}

	return c.Type
}

// printRarity returns the rarity icon name of a printing. An anomaly has the
// anomaly icon.
func printRarity(p card.Printing) string {
	if p.Anomaly {
		return "anomaly"
	}

	return p.Rarity
}

// blankIcons gives a blank icon to each value without an icon, if some
// values have one, so the labels line up.
func blankIcons(links []FacetLink) {
	if !slices.ContainsFunc(links, func(l FacetLink) bool { return l.Icon.URL != "" }) {
		return
	}

	for i := range links {
		if links[i].Icon.URL == "" {
			links[i].Icon.Blank = true
		}
	}
}

// rarityLabel returns the name of a rarity icon key: "anomaly" becomes
// "Anomaly".
func rarityLabel(r string) string {
	return capitalize(r)
}

// thumb returns the image URL of a card's default printing: the newest
// printing that is not an anomaly, or the newest printing.
func thumb(c *card.Card) string {
	for _, p := range slices.Backward(c.Printings) {
		if !p.Anomaly && p.Image != "" {
			return catalog.ImageURL(p.Image)
		}
	}

	for _, p := range slices.Backward(c.Printings) {
		if p.Image != "" {
			return catalog.ImageURL(p.Image)
		}
	}

	return ""
}

// CardURL returns the page URL that opens the card view of a card.
func CardURL(id string) string {
	return "/?card=" + url.QueryEscape(id)
}

// CompleteJSON returns the completions as JSON for the page.
func CompleteJSON(c search.Completions) string {
	b, err := json.Marshal(c)
	if err != nil {
		return "{}"
	}

	return string(b)
}

// flavorLines splits flavor text into lines, with the names of other cards
// as links.
func flavorLines(flavor string, c *card.Card) [][]TextPart {
	var out [][]TextPart

	for line := range strings.SplitSeq(flavor, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, LinkParts([]TextPart{{Text: line}}, c.Mentions, c.ID))
		}
	}

	return out
}
