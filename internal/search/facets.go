package search

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"
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
}

func count(items []Item) Facets {
	houses := map[string]int{}
	types := map[string]int{}
	rarities := map[string]int{}
	setCounts := map[string]int{}
	power := map[string]int{}
	armor := map[string]int{}

	for _, it := range items {
		houses[HouseOf(it.Card)]++
		types[it.Card.Type]++
		rarities[it.Printing.Rarity]++
		setCounts[it.Printing.Set]++

		if p := it.Card.Power; p != nil {
			power[p.String()]++
			armor[strconv.Itoa(it.Card.Armor)]++
		}
	}

	return Facets{
		Houses: counts(houses, "h", HouseLabel, func(a, b string) int {
			return cmp.Or(compareBool(a == HouseNone, b == HouseNone), cmp.Compare(a, b))
		}),
		Types: counts(types, "t", strings.ToLower, cmp.Compare[string]),
		Rarities: counts(rarities, "r", func(s string) string { return s }, func(a, b string) int {
			return cmp.Or(cmp.Compare(rarityRank(a), rarityRank(b)), cmp.Compare(a, b))
		}),
		Sets: counts(setCounts, "s", setCode, func(a, b string) int {
			return cmp.Compare(setIndex(a), setIndex(b))
		}),
		Power: counts(power, "p", func(s string) string { return s }, compareNumbers),
		Armor: counts(armor, "ar", func(s string) string { return s }, compareNumbers),
	}
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
		value := k
		if field == "s" {
			value = setCode(k)
		}

		out[i] = Count{Value: value, Label: label(k), N: m[k], Term: field + ":" + quote(value)}
	}

	return out
}

// compareNumbers orders numbers numerically, with other values (X) last.
func compareNumbers(a, b string) int {
	na, ea := strconv.Atoi(a)
	nb, eb := strconv.Atoi(b)

	return cmp.Or(compareBool(ea != nil, eb != nil), cmp.Compare(na, nb), cmp.Compare(a, b))
}

func setCode(slug string) string {
	if s, ok := SetOf(slug); ok {
		return s.Code
	}

	return slug
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
