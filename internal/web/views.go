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

	"github.com/dmikalova/gallery.keyforge.cards/internal/catalog"
	"github.com/dmikalova/gallery.keyforge.cards/internal/search"
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
	Name string
	// Icon is the icon next to the heading, or the zero Icon.
	Icon   Icon
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
	// Active is true if a top-level filter of the query selects the value,
	// in any spelling.
	Active bool
	// Excluded is true if a top-level negated filter excludes the value.
	Excluded bool
	// Percent is the bar width: the count as a part of the largest count.
	Percent int
	// Icon is the icon of the value, or the zero Icon.
	Icon Icon
}

// Results is the part of the search page that a new query replaces: the
// count, the facets, and the first tiles, or the parse error.
type Results struct {
	Err   *ErrorView
	Count int
	// Total is the number of results before limit:N.
	Total  int
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
	// Meta is the link preview of the page, or nil for the default one.
	Meta *PageMeta
	// Feedback is true when the feedback form can send.
	Feedback bool
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
	filters := e.Filters(q)

	excluded := filters.Excluded()

	group := func(name, field string, counts []search.Count, long bool) FacetGroup {
		// An excluded value has no results, so it gets a row with a count
		// of 0. The row shows the exclusion, and a click removes it.
		for _, x := range excluded {
			if x.Field == field && !slices.ContainsFunc(counts, func(c search.Count) bool {
				return strings.EqualFold(c.Value, x.Value)
			}) {
				counts = append(counts, e.ZeroCount(x.Field, x.Value))
			}
		}

		g := FacetGroup{Name: name, Icon: StatIcon(name)}
		top := 0

		for _, c := range counts {
			top = max(top, c.N)
		}

		for i, c := range counts {
			state, without := filters.State(c.Field, c.Value)
			l := FacetLink{
				Label:    c.Label,
				N:        c.N,
				Term:     c.Term,
				Active:   state == search.StateActive,
				Excluded: state == search.StateExcluded,
				URL:      SearchURL(e.Toggle(q, c.Term)),
				Percent:  c.N * 100 / max(top, 1),
				Icon:     facetIcon(name, c.Key, c.Label),
			}
			if state != search.StateNone {
				l.URL = SearchURL(without)
			}

			if long && i >= facetTop && !l.Active && !l.Excluded {
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
		group("House", "house", f.Houses, false),
		group("Type", "type", f.Types, false),
		group("Rarity", "rarity", f.Rarities, false),
		group("Set", "set", f.Sets, false),
		group("Power", "power", f.Power, false),
		group("Armor", "armor", f.Armor, false),
		group("Æmber", "aember", f.Aember, false),
		group("Keyword", "keyword", f.Keywords, false),
		group("Ability", "ability", f.Abilities, false),
		group("Flags", "is", f.Flags, false),
		group("Trait", "trait", f.Traits, true),
		group("Artist", "artist", f.Artists, true),
	} {
		if len(g.Counts) > 0 {
			out = append(out, g)
		}
	}

	return out
}

// CardView is the zoomed card: the image and every field of the card.
type CardView struct {
	ID    string
	Title string
	// House, Type, and the other values are filters: a click adds the
	// value's term to the query (Field.Term).
	House  Field
	Type   Field
	Traits []Field
	// Stats are the power, armor, and aember lines that the card has.
	Stats     []Field
	Keywords  []Field
	Abilities []Field
	Enhance   []Field
	Flags     []Field
	// Notes are short facts about how the card appears in decks (card.Note),
	// for example "Always 3+ copies".
	Notes []string
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

// Field is one labeled value. In the card view, a click on a value with a
// term toggles that term in the query, and State says how the query uses it.
type Field struct {
	Label, Value string
	Icon         Icon
	Term         string
	State        search.FilterState
}

// Active reports whether the query filters on the value.
func (f Field) Active() bool { return f.State == search.StateActive }

// Excluded reports whether the query excludes the value.
func (f Field) Excluded() bool { return f.State == search.StateExcluded }

// PrintView is one printing in the card view.
type PrintView struct {
	Index int
	// Label is the set code and the number: "WoE 193". Name is the full set
	// name and the number, for the tooltip.
	Label   string
	Name    string
	SetIcon Icon
	Image   string
	// Flavor is the flavor text, one line in each element, with the names
	// of other cards as links. An attribution ("—Name") is its own line.
	Flavor [][]TextPart
	// Artist is the artist of the printing as a filter, or an empty value.
	Artist Field
	// Set is the printing's set as a filter. The set chip switches the
	// shown printing, and a funnel button next to it adds the term.
	Set Field
	// Text is the card text of the printing, one line in each element.
	Text [][]TextPart
	// HouseIcon is the house of a printing in another house than the card
	// (Redemption), or the zero Icon.
	HouseIcon Icon
	// Changed reports whether the printing has its own text: Master Vault
	// changed the text of another printing (card.Printing.Text).
	Changed bool
	// ArtFrom is the set code of the printing whose image a maverick
	// printing shows (card.Printing.Maverick), or "".
	ArtFrom string
	// Diff is the plain text of the printing with the words that differ
	// from the card text marked, for the side-by-side view of all sets.
	Diff [][]TextPart
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

// SetNames gives the code and the full name of a set slug, for example "WoE"
// and "Winds of Exchange".
type SetNames func(slug string) (code, name string)

// NewCardView makes the card view. setNames gives the code and the full name
// of a set, and find gives the other cards of the mentions and the threads.
// The set chips show the code: the card view is narrow.
func NewCardView(
	c *card.Card, shown int, labels search.Labels, setNames SetNames, find Lookup,
) CardView {
	v := CardView{
		ID:    c.ID,
		Title: c.Title,
		House: Field{
			Label: "House", Value: labels.House(search.HouseOf(c)),
			Icon: HouseIcon(c.House, labels.House(c.House)),
			Term: search.FieldTerm("house", search.HouseOf(c)),
		},
		Type: Field{
			Label: "Type", Value: labels.Type(c.Type),
			Icon: TypeIcon(c.Type, labels.Type(c.Type)),
			Term: search.FieldTerm("type", strings.ToLower(c.Type)),
		},
		Keywords: keywordFields(c.Keywords),
		Enhance:  enhanceFields(c.Enhance),
		Print:    shown,
	}

	for _, t := range c.Traits {
		v.Traits = append(v.Traits, Field{Value: capitalize(t), Term: search.FieldTerm("trait", t)})
	}

	for _, a := range c.Abilities {
		v.Abilities = append(v.Abilities, Field{
			Value: labels.Ability(a), Term: search.FieldTerm("ability", a),
		})
	}

	for _, f := range c.Flags {
		v.Flags = append(v.Flags, Field{Value: labels.Flag(f), Term: "is:" + f})
	}

	for _, n := range c.Notes {
		v.Notes = append(v.Notes, n.Text)
	}

	if c.Power != nil {
		power := "power=" + strconv.Itoa(c.Power.N)
		if c.Power.X {
			power = "power:x"
		}

		v.Stats = append(
			v.Stats,
			Field{Label: "Power", Value: c.Power.String(), Icon: StatIcon("Power"), Term: power},
			Field{
				Label: "Armor", Value: strconv.Itoa(c.Armor), Icon: StatIcon("Armor"),
				Term: "armor=" + strconv.Itoa(c.Armor),
			},
		)
	}

	if c.Aember > 0 {
		v.Stats = append(v.Stats, Field{
			Label: "Æmber", Value: strconv.Itoa(c.Aember), Icon: StatIcon("Æmber"),
			Term: "aember=" + strconv.Itoa(c.Aember),
		})
	}

	v.Text = textLines(c.Text, c)

	for i, p := range c.Printings {
		code, name := setNames(p.Set)
		v.Sets = append(v.Sets, PrintView{
			Index:   i,
			Label:   code + " " + p.Number,
			Name:    name + " " + p.Number,
			SetIcon: SetIcon(p.Set, name),
			Image:   catalog.ImageURL(p.Image),
			Flavor:  flavorLines(p.Flavor, c),
			Text:    v.Text,
			Changed: p.Text != "",
			Diff:    DiffParts(search.PlainText(c.Text), search.PlainText(c.Text)),
			Set: Field{
				Value: name, Icon: SetIcon(p.Set, name),
				Term: search.FieldTerm("set", p.Set),
			},
		})

		if p.Text != "" {
			v.Sets[i].Text = textLines(p.Text, c)
			v.Sets[i].Diff = DiffParts(search.PlainText(p.Text), search.PlainText(c.Text))
		}

		if p.House != "" {
			v.Sets[i].HouseIcon = HouseIcon(p.House, labels.House(p.House))
			v.Sets[i].Name += " · " + labels.House(p.House)
		}

		if from := artFrom(c, p); from != "" {
			v.Sets[i].ArtFrom, _ = setNames(from)
			v.Sets[i].Name += " · Maverick: art from " + v.Sets[i].ArtFrom
		}

		if p.Artist != "" {
			v.Sets[i].Artist = Field{Value: p.Artist, Term: ArtistTerm(p.Artist)}
		}

		r := printRarity(p)
		if !slices.ContainsFunc(
			v.Rarities,
			func(f Field) bool { return f.Value == rarityLabel(r) },
		) {
			v.Rarities = append(v.Rarities, Field{
				Label: "Rarity", Value: rarityLabel(r), Icon: RarityIcon(r),
				Term: search.FieldTerm("rarity", strings.ToLower(p.Rarity)),
			})
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

// keywordFields returns the keywords as filters. A keyword with a number,
// such as assault 3, selects that number: keyword:assault=3.
func keywordFields(m map[string]int) []Field {
	var out []Field

	for _, k := range slices.Sorted(maps.Keys(m)) {
		f := Field{Value: capitalize(strings.ReplaceAll(k, "-", " ")), Term: "keyword:" + k}
		if m[k] > 0 {
			f.Value += " " + strconv.Itoa(m[k])
			f.Term += "=" + strconv.Itoa(m[k])
		}

		out = append(out, f)
	}

	return out
}

// enhanceFields returns the enhance icons as filters: enhance:aember=2.
func enhanceFields(m map[string]int) []Field {
	var out []Field

	for _, k := range slices.Sorted(maps.Keys(m)) {
		out = append(out, Field{
			Value: capitalize(k) + " ×" + strconv.Itoa(m[k]),
			Term:  "enhance:" + k + "=" + strconv.Itoa(m[k]),
		})
	}

	return out
}

// Mark sets the state of each value from the query's filters.
func (v *CardView) Mark(f *search.Filters) {
	mark := func(fs []Field) {
		for i := range fs {
			if fs[i].Term != "" {
				fs[i].State = f.TermState(fs[i].Term)
			}
		}
	}

	v.House.State = f.TermState(v.House.Term)
	v.Type.State = f.TermState(v.Type.Term)

	for _, fs := range [][]Field{v.Traits, v.Stats, v.Keywords, v.Abilities, v.Enhance, v.Flags, v.Rarities} {
		mark(fs)
	}

	for i := range v.Sets {
		v.Sets[i].Set.State = f.TermState(v.Sets[i].Set.Term)
		if v.Sets[i].Artist.Term != "" {
			v.Sets[i].Artist.State = f.TermState(v.Sets[i].Artist.Term)
		}
	}
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
	{
		"artist", "art", "art:fabio",
		"The artist of a printing. Accents do not matter: art:fabio finds Fábio Perez. " +
			"artist:none gives the printings without an artist.",
	},
}

// PageTitle returns the page title of a query.
func PageTitle(q string) string {
	if q == "" {
		return "KeyForge card gallery"
	}

	return q + " · KeyForge card gallery"
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
	switch {
	case c.Active:
		return "Remove the filter " + c.Label
	case c.Excluded:
		return "Remove the exclusion of " + c.Label
	}

	return "Add " + c.Term
}

func itoa(n int) string {
	return strconv.Itoa(n)
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
		if !p.Anomaly && !p.Maverick && p.House == "" && p.Image != "" {
			return catalog.ImageURL(p.Image)
		}
	}

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

// commas writes a count with thousands separators: 1670 becomes "1,670".
func commas(n int) string {
	s := strconv.Itoa(n)
	if n < 0 {
		return "-" + commas(-n)
	}
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// ArtistTerm returns the query term for an artist: artist:"Fábio Perez".
func ArtistTerm(name string) string {
	return search.FieldArtist + `:"` + strings.ReplaceAll(name, `"`, "") + `"`
}

// filterAction is the Datastar action of a card view filter button. The
// script builds the URL: the term (negated for an Alt-click or a long
// press), the open card, and its shown printing.
const filterAction = "@get(window.kfsearch.filterURL(el, evt))"

// filterTitle is the tooltip of a card view filter button.
func filterTitle(f Field) string {
	switch f.State {
	case search.StateActive, search.StateExcluded:
		return "Remove " + f.Term
	case search.StateNone:
	}

	return "Add " + f.Term + " (Alt-click or long press to exclude)"
}

// PageMeta is the link preview of a page: the OpenGraph and Twitter tags.
type PageMeta struct {
	Title, Description string
	// Image is the absolute URL of the preview image, or empty.
	Image string
	// Path is the page's path, for og:url.
	Path string
}

// CardMeta returns the link preview of a card: the name, the card image of
// the shown printing, and readable text: house, type, and stats, then the
// card text. Power shows as "P 5" (or "P X"), and armor as "A 1" only when
// it is above 0. The preview has no flavor text.
func CardMeta(v CardView, c *card.Card) PageMeta {
	head := []string{v.House.Value, v.Type.Value}
	if c.Power != nil {
		head = append(head, "P "+c.Power.String())
	}

	if c.Armor > 0 {
		head = append(head, "A "+strconv.Itoa(c.Armor))
	}

	lines := []string{strings.Join(head, " · ")}

	for line := range strings.SplitSeq(search.PlainText(c.Text), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}

	if notes := strings.Join(v.Notes, " · "); notes != "" && len(notes) <= maxMetaNotes {
		lines = append(lines, notes)
	}

	return PageMeta{
		Title:       c.Title,
		Description: strings.Join(lines, "\n"),
		Image:       v.Current().Image,
		Path:        CardPageURL(c.ID),
	}
}

// maxMetaNotes is the longest notes line that a link preview shows. Longer
// notes stay in the card view.
const maxMetaNotes = 120

// CardPageURL returns the path of a card's own page, the link to share: a
// link preview shows the card.
func CardPageURL(id string) string {
	return "/card/" + url.PathEscape(id)
}

// textLines returns card text as lines of text parts: the triggers bold,
// the names of other cards as links, and the icon tokens as icons.
func textLines(text string, c *card.Card) [][]TextPart {
	var out [][]TextPart

	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, IconParts(LinkParts(TextParts(line), c.Mentions, c.ID)))
		}
	}

	return out
}

// artFrom returns the set of the printing whose image a maverick printing
// shows: Master Vault has only maverick images of it, so keyforge-data
// gives it the image of the card's newest normal printing. It returns "" for
// a printing that is not a maverick or that keeps its own image.
func artFrom(c *card.Card, p card.Printing) string {
	if !p.Maverick || p.Image == "" {
		return ""
	}

	for _, o := range slices.Backward(c.Printings) {
		if !o.Maverick && o.Image == p.Image {
			return o.Set
		}
	}

	return ""
}
