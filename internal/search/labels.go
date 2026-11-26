package search

// Labels are the names for people of the data values, by kind and value. They
// come from the dataset (card.Dataset.Labels): "houses", "groups", "types",
// and "abilities". A value without a label shows with a capital letter.
type Labels map[string]map[string]string

// The label kinds of the dataset.
const (
	labelHouses    = "houses"
	labelGroups    = "groups"
	labelTypes     = "types"
	labelAbilities = "abilities"
)

// houseNoneLabel is the facet label of the cards without a house. It is a
// choice of this site, not data.
const houseNoneLabel = "Other"

func (l Labels) get(kind, v string) string {
	if s, ok := l[kind][v]; ok && s != "" {
		return s
	}

	return capitalize(v)
}

// House returns the printed name of a house slug. HouseNone is "Other".
func (l Labels) House(slug string) string {
	if slug == HouseNone {
		return houseNoneLabel
	}

	return l.get(labelHouses, slug)
}

// Type returns the name of a card type: "action" is "Action".
func (l Labels) Type(t string) string { return l.get(labelTypes, t) }

// Ability returns the printed name of an ability trigger: "beforefight" is
// "Before Fight".
func (l Labels) Ability(a string) string { return l.get(labelAbilities, a) }

// Group returns the name of a group of the cards without a house: "thetide"
// is "The Tide".
func (l Labels) Group(g string) string { return l.get(labelGroups, g) }

// Labels returns the labels of the dataset.
func (e *Engine) Labels() Labels { return e.labels }
