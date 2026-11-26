package search

import (
	"slices"
	"strings"

	"github.com/dmikalova/keyforge-data/query"
)

// CompleteField is a field that the search bar can complete.
type CompleteField struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	// Values are the values that the bar suggests after "name:". A sort
	// value is a comma list, so the bar suggests each key after a comma.
	Values []string `json:"values,omitempty"`
	// List is true when the value is a comma list, so the bar completes
	// the part after the last comma.
	List bool `json:"list,omitempty"`
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
		cf := CompleteField{Name: f.Name, Alias: f.Alias}

		switch {
		case f.Name == "trait":
			cf.Values, cf.List = traits, true
		case f.Name == DirSort:
			cf.Values, cf.List = sortKeys, true
		case f.Name == "set" || f.Name == "first":
			for _, s := range e.sets {
				cf.Values = append(cf.Values, strings.ToLower(s.Code))
			}

			cf.List = true
		case f.Kind == query.KindEnum || f.Kind == query.KindFlag || f.Kind == query.KindKeyed:
			for _, v := range f.Values {
				cf.Values = append(cf.Values, query.Normalize(v))
			}

			cf.List = true
		case f.Kind == query.KindDirective:
			cf.Values = f.Values
		}

		out.Fields = append(out.Fields, cf)
	}

	return out
}
