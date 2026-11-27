package search

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/sets"
)

// Count is one value of a facet and the number of results with it.
type Count struct {
	// Value is the query value, for example "brobnar" or "GR".
	Value string
	// Label is the text to show, for example "Brobnar" or "GR".
	Label string
	N     int
	// Field is the full name of the facet field, for example "house".
	Field string
	// Term is the query term that selects the value, for example "h:brobnar".
	Term string
	// Key names the icon of the value: the house, type, or set slug, or the
	// rarity. It is empty for a value without an icon.
	Key string
}

// Facets count the results by field.
type Facets struct {
	Houses   []Count
	Types    []Count
	Rarities []Count
	Sets     []Count
	// Power and Armor are histograms in numeric order. Power X is last.
	Power []Count
	Armor []Count
	// Aember counts the cards by bonus aember, in numeric order.
	Aember []Count
	// Keywords, Abilities, and Traits count the cards that have each value,
	// most first.
	Keywords  []Count
	Abilities []Count
	Traits    []Count
	// Flags count the cards that have each "is:" flag, most first.
	Flags []Count
	// Artists count the cards that have a printing by each artist, most
	// first. It is empty while the dataset has no artists.
	Artists []Count
}

// tally holds the counts of the facets while count reads the results.
type tally struct {
	houses, types, rarities, sets, power, armor, aember map[string]int
	keywords, abilities, traits, artists, flags         map[string]int
}

func newTally() *tally {
	return &tally{
		houses: map[string]int{}, types: map[string]int{}, rarities: map[string]int{},
		sets: map[string]int{}, power: map[string]int{}, armor: map[string]int{},
		aember: map[string]int{}, keywords: map[string]int{}, abilities: map[string]int{},
		traits: map[string]int{}, artists: map[string]int{}, flags: map[string]int{},
	}
}

// card counts the printing values of a card one time: each set, artist, and
// flag that one of its printings has, and each rarity if rarities is true.
func (t *tally) card(e *Engine, c *card.Card, rarities bool) {
	for _, code := range e.setCodes(c) {
		t.sets[code]++
	}

	for _, a := range cardArtists(c) {
		t.artists[a]++
	}

	if rarities {
		for _, r := range cardRarities(c) {
			t.rarities[r]++
		}
	}

	// A flag term matches a card if one of its printings has the flag.
	for _, fl := range e.flagTests {
		if fl.Test(e.entryOf[c]) {
			t.flags[fl.Name]++
		}
	}
}

// item counts the card values of one result.
func (t *tally) item(it Item) {
	c := it.Card
	t.houses[houseBucket(c)]++
	t.types[c.Type]++

	for k := range c.Keywords {
		t.keywords[k]++
	}

	for _, a := range c.Abilities {
		t.abilities[a]++
	}

	for _, tr := range c.Traits {
		t.traits[tr]++
	}

	t.aember[strconv.Itoa(c.Aember)]++

	if p := c.Power; p != nil {
		t.power[p.String()]++
		t.armor[strconv.Itoa(c.Armor)]++
	}
}

// count counts the results by facet value. A count is the number of
// results that a click on the value finds. A printing term (set, rarity,
// artist, a printing flag) matches a card if one of its printings matches,
// so the printing facets count each card one time for each value that one
// of its printings has. With unique:prints each result is a printing, so
// the rarity facet counts each result by its own printing.
func (e *Engine) count(items []Item, prints bool) Facets {
	t := newTally()
	counted := map[*card.Card]bool{}

	for _, it := range items {
		if !counted[it.Card] {
			counted[it.Card] = true
			t.card(e, it.Card, !prints)
		}

		if prints {
			t.rarities[it.Printing.Rarity]++
		}

		t.item(it)
	}

	f := Facets{
		Houses: counts(t.houses, "h", e.labels.House, func(a, b string) int {
			return cmp.Or(cmp.Compare(bucketRank(a), bucketRank(b)), cmp.Compare(a, b))
		}),
		Types: counts(t.types, "t", e.labels.Type, cmp.Compare[string]),
		Rarities: counts(
			t.rarities,
			"r",
			func(s string) string { return s },
			func(a, b string) int {
				return cmp.Or(cmp.Compare(rarityRank(a), rarityRank(b)), cmp.Compare(a, b))
			},
		),
		Sets: counts(t.sets, "s", e.setLabel, func(a, b string) int {
			return cmp.Or(cmp.Compare(e.codeIndex(a), e.codeIndex(b)), cmp.Compare(a, b))
		}),
		Power:     counts(t.power, "p", func(s string) string { return s }, compareNumbers),
		Armor:     counts(t.armor, "ar", func(s string) string { return s }, compareNumbers),
		Aember:    counts(t.aember, "a", func(s string) string { return s }, compareNumbers),
		Keywords:  counts(t.keywords, "k", keywordLabel, byCount(t.keywords)),
		Abilities: counts(t.abilities, "ab", e.labels.Ability, byCount(t.abilities)),
		Traits:    counts(t.traits, "tr", traitLabel, byCount(t.traits)),
		Flags:     counts(t.flags, "is", e.labels.Flag, byCount(t.flags)),
		Artists: counts(
			t.artists,
			"art",
			func(s string) string { return s },
			byCount(t.artists),
		),
	}

	for _, cs := range [][]Count{f.Houses, f.Types, f.Rarities} {
		for i := range cs {
			cs[i].Key = cs[i].Value
		}
	}

	for i := range f.Sets {
		if j := e.codeIndex(f.Sets[i].Value); j < len(e.sets) {
			f.Sets[i].Key = e.sets[j].Slug
		}
	}

	return f
}

func counts(
	m map[string]int,
	field string,
	label func(string) string,
	order func(a, b string) int,
) []Count {
	keys := slices.SortedFunc(maps.Keys(m), order)
	out := make([]Count, len(keys))

	for i, k := range keys {
		term := field + ":" + quote(k)

		out[i] = Count{Value: k, Label: label(k), N: m[k], Term: term, Field: facetFields[field]}
	}

	return out
}

// compareNumbers orders numbers numerically, with other values (X) last.
func compareNumbers(a, b string) int {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)

	return cmp.Or(compareBool(ea != nil, eb != nil), cmp.Compare(na, nb), cmp.Compare(a, b))
}

// houseBucket returns the house facet bucket of a card: its house, or
// HouseNone for every card without a house (anomalies,
// houseless kinds, and reference cards). The facet shows that bucket as
// "Other", and `house:none` selects the same cards. The group field and
// is:houseless or is:reference select one part of it.
func houseBucket(c *card.Card) string {
	if c.House != "" {
		return c.House
	}

	return HouseNone
}

// bucketRank puts the houses first and the bucket Other last.
func bucketRank(b string) int {
	if b == HouseNone {
		return 1
	}

	return 0
}

// codeIndex returns the position of the set with the code, or the end.
func (e *Engine) codeIndex(code string) int {
	i := slices.IndexFunc(e.sets, func(s sets.Set) bool { return s.Code == code })
	if i < 0 {
		return len(e.sets)
	}

	return i
}

// quote puts a value in quotes if it has a space or a quote.
func quote(v string) string {
	if strings.ContainsAny(v, " \"()") {
		return `"` + strings.ReplaceAll(v, `"`, "") + `"`
	}

	return v
}

// AddTerm returns the query with a term added at the end.
func AddTerm(q, term string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return term
	}

	return q + " " + term
}

// WithSeed returns the query with a "seed:" directive added.
func WithSeed(q string, seed uint64) string {
	return AddTerm(q, DirSeed+":"+strconv.FormatUint(seed, 10))
}

// byCount orders the values by count, most first, then by name.
func byCount(m map[string]int) func(a, b string) int {
	return func(a, b string) int {
		return cmp.Or(cmp.Compare(m[b], m[a]), cmp.Compare(a, b))
	}
}

func keywordLabel(k string) string {
	return capitalize(strings.ReplaceAll(k, "-", " "))
}

func traitLabel(t string) string {
	return capitalize(t)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}

	return strings.ToUpper(s[:1]) + s[1:]
}

// setCodes returns the codes of the sets that a card has a printing in, one
// time each.
func (e *Engine) setCodes(c *card.Card) []string {
	var out []string

	for _, p := range c.Printings {
		code := p.Set
		if s, ok := e.SetOf(p.Set); ok {
			code = s.Code
		}

		if !slices.Contains(out, code) {
			out = append(out, code)
		}
	}

	return out
}

// setLabel returns the full name of the set with the code, or the code.
func (e *Engine) setLabel(code string) string {
	if j := e.codeIndex(code); j < len(e.sets) {
		return e.sets[j].Name
	}

	return code
}

// facetFields maps the alias of each facet field to its full name.
var facetFields = map[string]string{
	"h": "house", "t": "type", "r": "rarity", "s": "set", "p": "power", "ar": "armor",
	"a": "aember", "k": "keyword", "ab": "ability", "tr": "trait", "art": "artist",
	"is": "is",
}

// cardArtists returns the artists of a card's printings, one time each,
// without the empty artist.
func cardArtists(c *card.Card) []string {
	var out []string

	for _, p := range c.Printings {
		if p.Artist != "" && !slices.Contains(out, p.Artist) {
			out = append(out, p.Artist)
		}
	}

	return out
}

// cardRarities returns the rarities of a card's printings, one time each.
func cardRarities(c *card.Card) []string {
	var out []string

	for _, p := range c.Printings {
		if p.Rarity != "" && !slices.Contains(out, p.Rarity) {
			out = append(out, p.Rarity)
		}
	}

	return out
}
