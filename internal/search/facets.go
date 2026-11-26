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

	for _, it := range items {
		houses[houseBucket(it.Card)]++
		types[it.Card.Type]++
		rarities[it.Printing.Rarity]++
		setCounts[it.Set.Code]++

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
		if f, ok := bucketFlags[k]; ok && field == "h" {
			term = "is:" + f
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

// The house facet buckets of the cards without a house. A bucket selects its
// cards with a flag, not with a house term.
const (
	bucketHouseless = "houseless"
	bucketReference = "reference"
)

var bucketFlags = map[string]string{
	bucketHouseless: FlagHouseless,
	bucketReference: FlagReference,
}

// houseBucket returns the house facet bucket of a card: its house, or the
// houseless or reference bucket, or HouseNone.
func houseBucket(c *card.Card) string {
	switch {
	case c.House != "":
		return c.House
	case c.HasFlag(FlagReference):
		return bucketReference
	case c.HasFlag(FlagHouseless):
		return bucketHouseless
	default:
		return HouseNone
	}
}

// bucketRank puts the houses first, then the houseless, reference, and
// no-house buckets.
func bucketRank(b string) int {
	switch b {
	case bucketHouseless:
		return 1
	case bucketReference:
		return 2
	case HouseNone:
		return 3
	default:
		return 0
	}
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
	HouseNone: "No house", bucketHouseless: "Houseless", bucketReference: "Reference",
}

// HouseLabel returns the printed name of a house slug.
func HouseLabel(slug string) string {
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

// fuzzyOffTerm is the directive term that turns fuzzy name matching off.
const fuzzyOffTerm = DirFuzzy + ":" + FuzzyOff

// FuzzyIsOff reports whether the query has the term "fuzzy:off" at the top
// level. It does not parse the query, so it works on a query with an error.
func FuzzyIsOff(q string) bool {
	off := false

	for w := range strings.FieldsSeq(q) {
		switch strings.ToLower(w) {
		case fuzzyOffTerm:
			off = true
		case DirFuzzy + ":on":
			off = false
		}
	}

	return off
}

// SetFuzzy returns the query with fuzzy name matching on or off: it removes
// the fuzzy directives and adds "fuzzy:off" for off.
func SetFuzzy(q string, on bool) string {
	var keep []string

	for w := range strings.FieldsSeq(q) {
		if l := strings.ToLower(w); l != fuzzyOffTerm && l != DirFuzzy+":on" {
			keep = append(keep, w)
		}
	}

	out := strings.Join(keep, " ")
	if !on {
		out = AddTerm(out, fuzzyOffTerm)
	}

	return out
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
