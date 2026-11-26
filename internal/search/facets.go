package search

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/derive"
	"github.com/dmikalova/keyforge-data/sets"
)

// Count is one value of a facet and the number of results with it.
type Count struct {
	// Value is the query value, for example "brobnar" or "GR".
	Value string
	// Label is the text to show, for example "Brobnar" or "GR".
	Label string
	N     int
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
	// Keywords, Abilities, and Traits count the cards that have each value,
	// most first.
	Keywords  []Count
	Abilities []Count
	Traits    []Count
}

func (e *Engine) count(items []Item) Facets {
	houses := map[string]int{}
	types := map[string]int{}
	rarities := map[string]int{}
	setCounts := map[string]int{}
	power := map[string]int{}
	armor := map[string]int{}
	keywords := map[string]int{}
	abilities := map[string]int{}
	traits := map[string]int{}

	// A set term matches a card that has a printing in the set, so the set
	// facet counts each card one time in each set that it is in. The count
	// is then the number of results of the query with the set term.
	counted := map[*card.Card]bool{}

	for _, it := range items {
		if !counted[it.Card] {
			counted[it.Card] = true

			for _, code := range e.setCodes(it.Card) {
				setCounts[code]++
			}
		}

		houses[houseBucket(it.Card)]++
		types[it.Card.Type]++
		rarities[it.Printing.Rarity]++

		for k := range it.Card.Keywords {
			keywords[k]++
		}

		for _, a := range it.Card.Abilities {
			abilities[a]++
		}

		for _, t := range it.Card.Traits {
			traits[t]++
		}

		if p := it.Card.Power; p != nil {
			power[p.String()]++
			armor[strconv.Itoa(it.Card.Armor)]++
		}
	}

	f := Facets{
		Houses: counts(houses, "h", HouseLabel, func(a, b string) int {
			return cmp.Or(cmp.Compare(bucketRank(a), bucketRank(b)), cmp.Compare(a, b))
		}),
		Types: counts(types, "t", strings.ToLower, cmp.Compare[string]),
		Rarities: counts(rarities, "r", func(s string) string { return s }, func(a, b string) int {
			return cmp.Or(cmp.Compare(rarityRank(a), rarityRank(b)), cmp.Compare(a, b))
		}),
		Sets: counts(setCounts, "s", func(s string) string { return s }, func(a, b string) int {
			return cmp.Or(cmp.Compare(e.codeIndex(a), e.codeIndex(b)), cmp.Compare(a, b))
		}),
		Power:     counts(power, "p", func(s string) string { return s }, compareNumbers),
		Armor:     counts(armor, "ar", func(s string) string { return s }, compareNumbers),
		Keywords:  counts(keywords, "k", keywordLabel, byCount(keywords)),
		Abilities: counts(abilities, "ab", AbilityLabel, abilityOrder),
		Traits:    counts(traits, "tr", traitLabel, byCount(traits)),
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
		if g, ok := strings.CutPrefix(k, groupBucket); ok && field == "h" {
			term = FieldGroup + ":" + g
		}

		out[i] = Count{Value: k, Label: label(k), N: m[k], Term: term}
	}

	return out
}

// compareNumbers orders numbers numerically, with other values (X) last.
func compareNumbers(a, b string) int {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)

	return cmp.Or(compareBool(ea != nil, eb != nil), cmp.Compare(na, nb), cmp.Compare(a, b))
}

// groupBucket is the prefix of the house facet bucket of a card without a
// house: "~anomaly" for the group anomaly. The bucket selects its cards
// with a group term, not with a house term.
const groupBucket = "~"

// houseBucket returns the house facet bucket of a card: its house, or the
// bucket of its group (see GroupOf).
func houseBucket(c *card.Card) string {
	if c.House != "" {
		return c.House
	}

	return groupBucket + GroupOf(c)
}

// bucketRank puts the houses first, then the groups of the cards without a
// house in the order of card.HouselessGroups.
func bucketRank(b string) int {
	g, ok := strings.CutPrefix(b, groupBucket)
	if !ok {
		return 0
	}

	i := slices.Index(card.HouselessGroups, g)
	if i < 0 {
		return len(card.HouselessGroups) + 1
	}

	return 1 + i
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

// houseLabels are the printed house names.
var houseLabels = map[string]string{
	"brobnar": "Brobnar", "dis": "Dis", "ekwidon": "Ekwidon", "elders": "Elders",
	"geistoid": "Geistoid", "ironyxrebels": "Ironyx Rebels", "keyraken": "Keyraken",
	"logos": "Logos", "mars": "Mars", "ouboros": "Ouboros", "redemption": "Redemption",
	"sanctum": "Sanctum", "saurian": "Saurian", "shadows": "Shadows", "skyborn": "Skyborn",
	"staralliance": "Star Alliance", "unfathomable": "Unfathomable", "untamed": "Untamed",
	HouseNone: "No house",
}

// groupLabels are the names of the groups of the cards without a house.
var groupLabels = map[string]string{
	"anomaly": "Anomaly", "multihouse": "Multi-house", "skybeast": "Skybeast",
	"revenant": "Revenant", "mercenary": "Mercenary", "houseless": "Houseless",
	"archonpower": "Archon power", "prophecy": "Prophecy", "thetide": "The Tide",
	"reference": "Reference",
}

// GroupLabel returns the name of a group of cards without a house.
func GroupLabel(g string) string {
	if l, ok := groupLabels[g]; ok {
		return l
	}

	return g
}

// HouseLabel returns the printed name of a house slug, or the name of a
// group bucket of the house facet.
func HouseLabel(slug string) string {
	if g, ok := strings.CutPrefix(slug, groupBucket); ok {
		return GroupLabel(g)
	}

	if l, ok := houseLabels[slug]; ok {
		return l
	}

	return slug
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

// AbilityLabels are the printed names of the ability triggers.
var AbilityLabels = map[string]string{
	"action": "Action", "beforefight": "Before Fight", "destroyed": "Destroyed",
	"fight": "Fight", "leavesplay": "Leaves Play", "omni": "Omni", "play": "Play",
	"reap": "Reap", "scrap": "Scrap",
}

// AbilityLabel returns the printed name of an ability trigger.
func AbilityLabel(a string) string {
	if l, ok := AbilityLabels[a]; ok {
		return l
	}

	return capitalize(a)
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

// abilityOrder orders the abilities as people read them: Play, Fight, Reap,
// and then the other triggers (derive.Abilities).
func abilityOrder(a, b string) int {
	rank := func(s string) int {
		if i := slices.Index(derive.Abilities, s); i >= 0 {
			return i
		}

		return len(derive.Abilities)
	}

	return cmp.Or(cmp.Compare(rank(a), rank(b)), cmp.Compare(a, b))
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
