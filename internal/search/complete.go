package search

import (
	"slices"
	"strings"

	"github.com/dmikalova/keyforge-data/derive"
	"github.com/dmikalova/keyforge-data/query"
)

// CompleteField is a field that the search bar can complete.
type CompleteField struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	// Desc says in a few words what the field matches.
	Desc string `json:"desc,omitempty"`
	// Values are the values that the bar suggests after "name:". A sort
	// value is a comma list, so the bar suggests each key after a comma.
	Values []string `json:"values,omitempty"`
	// List is true when the value is a comma list, so the bar completes
	// the part after the last comma.
	List bool `json:"list,omitempty"`
	// Num is true for a field that takes a number after it: power>=3,
	// power:2..4. The bar then suggests the comparison operators.
	Num bool `json:"num,omitempty"`
	// NumKeys are the keys of a keyed field that take a number:
	// keyword:assault>=2. The bar suggests the operators after them.
	NumKeys []string `json:"numkeys,omitempty"`
	// Labels are the names to show next to values, by value: the full set
	// name next to "wc", "Token Creature" next to "tokencreature". The bar
	// also matches the typed text against them.
	Labels map[string]string `json:"labels,omitempty"`
}

// Completions are the field names and the values that the search bar
// completes in the browser. They come from the dataset, so a new house or
// trait needs no code change.
type Completions struct {
	Version string          `json:"version"`
	Fields  []CompleteField `json:"fields"`
}

// Completions returns the completions of the dataset.
func (e *Engine) Completions() Completions {
	traits := []string{}
	for _, en := range e.cards {
		for _, t := range en.Card.Traits {
			traits = appendNew(traits, t)
		}
	}

	slices.Sort(traits)

	out := Completions{Version: e.version}

	for _, f := range e.reg.Fields() {
		cf := CompleteField{Name: f.Name, Alias: f.Alias, Desc: fieldDocs[f.Name]}
		cf.Num = f.Kind == query.KindNumber || f.Name == "enhance"
		cf.NumKeys = numericKeys(f)

		switch {
		case f.Name == "trait":
			cf.Values, cf.List = traits, true
		case f.Name == FieldArtist:
			// A name has spaces, so the value is in quotes. The label is the
			// name with its diacritics, and the bar matches the typed text
			// against it folded: "fab" finds Fábio Perez.
			cf.Values, cf.Labels = e.artistValues()
		case f.Name == DirSort:
			cf.Values, cf.List = sortKeys, true
		case f.Name == "set" || f.Name == "first":
			cf.Labels = map[string]string{}

			for _, s := range e.sets {
				code := strings.ToLower(s.Code)
				cf.Values = append(cf.Values, code)
				cf.Labels[code] = s.Name
			}

			cf.List = true
		case f.Kind == query.KindEnum || f.Kind == query.KindFlag || f.Kind == query.KindKeyed:
			for _, v := range f.Values {
				n := query.Normalize(v)
				cf.Values = append(cf.Values, n)

				if label := valueLabel(f.Name, v); label != n {
					if cf.Labels == nil {
						cf.Labels = map[string]string{}
					}

					cf.Labels[n] = label
				}
			}

			cf.List = true
		case f.Kind == query.KindDirective:
			cf.Values = f.Values
		}

		out.Fields = append(out.Fields, cf)
	}

	return out
}

// fieldDocs are the short descriptions that the search bar shows next to
// each field.
var fieldDocs = map[string]string{
	"name":            "Card name",
	"text":            "Card text",
	"flavor":          "Flavor text",
	"house":           "House",
	FieldGroup:        "Houseless group: anomaly, prophecy",
	"set":             "Printed in the set",
	"first":           "First printed in the set",
	"type":            "Card type",
	"rarity":          "Rarity",
	"keyword":         "Keyword, with a number: keyword:assault>=2",
	"ability":         "Ability trigger: play, fight, reap",
	"trait":           "Trait",
	FieldArtist:       "Artist, or none: artist:none",
	query.NumberField: "Collector number",
	"power":           "Power, or x",
	"armor":           "Armor",
	"aember":          "Bonus æmber",
	"enhance":         "Enhance icons: enhance:damage>=1",
	query.FlagField:   "Flag: anomaly, reprint, houseless",
	DirSort:           "Sort keys, comma list, -key for descending",
	DirOrder:          "Order: asc or desc",
	DirUnique:         "unique:prints shows each printing",
	DirSeed:           "Seed of sort:random",
}

// numericKeys returns the keys of a keyed field that take a number: the
// keywords with a value (assault, hazardous, splash-attack), and every
// enhance icon, because enhance counts icons.
func numericKeys(f query.FieldInfo) []string {
	switch f.Name {
	case "keyword":
		var out []string
		for _, k := range derive.Keywords {
			if k.Valued {
				out = append(out, query.Normalize(k.Name))
			}
		}

		return out
	case "enhance":
		out := make([]string, len(f.Values))
		for i, v := range f.Values {
			out[i] = query.Normalize(v)
		}

		return out
	}

	return nil
}

// valueLabel returns the name to show for an enum value: the type with a
// capital letter, the rarity and the other values as the dataset has them.
func valueLabel(field, v string) string {
	if field == "type" {
		return TypeLabel(v)
	}

	if field == "house" {
		return HouseLabel(v)
	}

	return v
}

// artistValues returns the artist values for the search bar, quoted, and
// their names. The first value is "none".
func (e *Engine) artistValues() ([]string, map[string]string) {
	var names []string

	for _, en := range e.cards {
		for _, a := range cardArtists(en.Card) {
			names = appendNew(names, a)
		}
	}

	slices.SortFunc(names, func(a, b string) int {
		return strings.Compare(query.Fold(a), query.Fold(b))
	})

	values := []string{ArtistNone}
	labels := map[string]string{ArtistNone: "No artist yet"}

	for _, n := range names {
		v := `"` + strings.ReplaceAll(n, `"`, "") + `"`
		values = append(values, v)
		labels[v] = n
	}

	return values, labels
}
