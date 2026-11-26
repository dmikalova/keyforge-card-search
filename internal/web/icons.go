package web

import (
	"regexp"
	"slices"
	"strings"

	"github.com/a-h/templ"
	"github.com/dmikalova/keyforge-data/card"
)

// iconBase is the icon proxy of the site (server.IconOrigin). The page draws
// an SVG icon as a CSS mask, and a mask image must come from the same origin.
const iconBase = "/icons/"

// Icon is one icon image. An SVG icon draws with currentColor, so the page
// shows it as a CSS mask in the text color. A webp icon has its own colors.
type Icon struct {
	URL   string
	Mask  bool
	Label string
	// Blank is an empty space the size of an icon. It lines up a value
	// without an icon with the values that have one.
	Blank bool
}

// MaskStyle is the style that sets the mask image of a mask icon. The URL
// has no quotes: templ escapes quotes in a style attribute, and icon paths
// have only letters, digits, and slashes.
func (i Icon) MaskStyle() templ.SafeCSS {
	return templ.SafeCSS("--ico:url(" + i.URL + ")")
}

// The icons that keyforge-data publishes. A value without an icon shows its
// text only.
var (
	houseIcons = setOf(
		"brobnar", "dis", "ekwidon", "geistoid", "ironyxrebels", "keyraken", "logos", "mars",
		"ouboros", "redemption", "sanctum", "saurian", "shadows", "skyborn", "staralliance",
		"unfathomable", "untamed",
	)
	rarityIcons = setOf("common", "uncommon", "rare", "special", "anomaly")
	setIcons    = setOf(
		"aemberskies", "ageofascension", "anomalyexpansion", "callofthearchons", "crucibleclash",
		"darktidings", "discovery", "draconianmeasures", "grimreminders", "martiancivilwar",
		"massmutation", "menagerie", "propheticvisions", "tokensofchange", "unchained2022",
		"vaultmasters2023", "vaultmasters2024", "vaultmasters2025", "vaultmasters2026",
		"windsofexchange", "worldscollide",
	)
	typeIcons = setOf("action", "artifact", "creature", "upgrade", "tokencreature")
)

func setOf(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}

	return m
}

// HouseIcon returns the icon of a house slug, or the zero Icon.
func HouseIcon(slug, label string) Icon {
	if !houseIcons[slug] {
		return Icon{}
	}

	return Icon{URL: iconBase + "house/" + slug + ".webp", Label: label}
}

// RarityIcon returns the icon of a rarity, or the zero Icon.
func RarityIcon(rarity string) Icon {
	r := strings.ToLower(rarity)
	if !rarityIcons[r] {
		return Icon{}
	}

	return Icon{URL: iconBase + "rarity/" + r + ".svg", Mask: true, Label: rarity}
}

// SetIcon returns the icon of a set slug, or the zero Icon.
func SetIcon(slug, label string) Icon {
	if !setIcons[slug] {
		return Icon{}
	}

	return Icon{URL: iconBase + "set/" + slug + ".svg", Mask: true, Label: label}
}

// TypeIcon returns the icon of a card type, or the zero Icon.
func TypeIcon(typ, label string) Icon {
	t := strings.ReplaceAll(strings.ToLower(typ), " ", "")
	if !typeIcons[t] {
		return Icon{}
	}

	return Icon{URL: iconBase + "type/" + t + ".svg", Mask: true, Label: label}
}

// StatIcon returns the icon of power, armor, or aember.
func StatIcon(stat string) Icon {
	switch stat {
	case "Power":
		return Icon{URL: iconBase + "stat/power.svg", Mask: true, Label: stat}
	case "Armor":
		return Icon{URL: iconBase + "stat/armor.svg", Mask: true, Label: stat}
	case "Æmber":
		return Icon{URL: iconBase + "stat/aember.webp", Label: stat}
	}

	return Icon{}
}

// facetIcon returns the icon of a facet value.
func facetIcon(group, key, label string) Icon {
	switch group {
	case "House":
		return HouseIcon(key, label)
	case "Type":
		return TypeIcon(key, label)
	case "Rarity":
		return RarityIcon(key)
	case "Set":
		return SetIcon(key, label)
	}

	return Icon{}
}

// chipTermRe reads a chip with one value: "h:dis", "-type:creature".
var chipTermRe = regexp.MustCompile(`^-?(h|house|t|type|r|rarity):([a-z]+)$`)

// chipIcon returns the icon of a chip with one house, type, or rarity.
func chipIcon(text string) Icon {
	m := chipTermRe.FindStringSubmatch(strings.ToLower(text))
	if m == nil {
		return Icon{}
	}

	switch m[1] {
	case "h", "house":
		return HouseIcon(m[2], m[2])
	case "t", "type":
		return TypeIcon(m[2], m[2])
	default:
		return RarityIcon(m[2])
	}
}

// TextPart is a run of card text. Bold runs are the ability triggers.
type TextPart struct {
	Text string
	Bold bool
	// Link is the ID of the card that the text names, or empty.
	Link string
	// Icon is an icon in the text, for example the tide icon of {tide}.
	// The run then has no text.
	Icon Icon
}

// triggerRe finds a trigger list at the start of a line, a sentence, or a
// quote: "Play:", "Fight/Reap:", "Play/After Fight/After Reap:", "Before
// Fight:". A trigger in quotes is bold too: the card prints it bold, and it
// is the trigger of the card that gains it.
var triggerRe = regexp.MustCompile(
	`(?:^|[.!”"]\s+|[“"])((?:(?:After |Before )?(?:Play|Fight|Reap|Action|Omni|Destroyed|Leaves Play|Scrap)/?)+:)`,
)

// TextParts splits a line of card text into runs, with the trigger lists
// bold.
func TextParts(line string) []TextPart {
	var out []TextPart

	last := 0

	for _, m := range triggerRe.FindAllStringSubmatchIndex(line, -1) {
		start, end := m[2], m[3]
		if start > last {
			out = append(out, TextPart{Text: line[last:start]})
		}

		out = append(out, TextPart{Text: line[start:end], Bold: true})
		last = end
	}

	if last < len(line) {
		out = append(out, TextPart{Text: line[last:]})
	}

	return out
}

// LinkParts splits the plain runs of parts at the names of the mentioned
// cards. Each name becomes a run with the card's ID as its link. A longer
// name wins over a shorter name that it contains. The card's own name is
// not a link.
func LinkParts(parts []TextPart, mentions []card.Mention, self string) []TextPart {
	ms := slices.Clone(mentions)
	ms = slices.DeleteFunc(ms, func(m card.Mention) bool { return m.ID == self || m.Title == "" })
	slices.SortFunc(ms, func(a, b card.Mention) int { return len(b.Title) - len(a.Title) })

	if len(ms) == 0 {
		return parts
	}

	var out []TextPart

	for _, p := range parts {
		if p.Bold || p.Link != "" {
			out = append(out, p)

			continue
		}

		out = append(out, splitMentions(p.Text, ms)...)
	}

	return out
}

// splitMentions splits text at the first, longest mention, then splits the
// rest the same way.
func splitMentions(text string, ms []card.Mention) []TextPart {
	if text == "" {
		return nil
	}

	best, at := -1, len(text)

	for i, m := range ms {
		if j := strings.Index(text, m.Title); j >= 0 && j < at {
			best, at = i, j
		}
	}

	if best < 0 {
		return []TextPart{{Text: text}}
	}

	m := ms[best]

	var out []TextPart
	if at > 0 {
		out = append(out, TextPart{Text: text[:at]})
	}

	out = append(out, TextPart{Text: m.Title, Link: m.ID})

	return append(out, splitMentions(text[at+len(m.Title):], ms)...)
}

// textIcons are the icon tokens in the card text of the dataset.
var textIcons = map[string]Icon{
	"{tide}": {URL: iconBase + "stat/tide.svg", Mask: true, Label: "Tide"},
}

// IconParts splits the plain runs of parts at the icon tokens of the card
// text: "{tide}" becomes a run with the tide icon.
func IconParts(parts []TextPart) []TextPart {
	var out []TextPart

	for _, p := range parts {
		if p.Bold || p.Link != "" || !strings.Contains(p.Text, "{") {
			out = append(out, p)

			continue
		}

		out = append(out, splitIcons(p.Text)...)
	}

	return out
}

func splitIcons(text string) []TextPart {
	best, at := "", len(text)

	for tok := range textIcons {
		if j := strings.Index(text, tok); j >= 0 && j < at {
			best, at = tok, j
		}
	}

	if best == "" {
		if text == "" {
			return nil
		}

		return []TextPart{{Text: text}}
	}

	var out []TextPart
	if at > 0 {
		out = append(out, TextPart{Text: text[:at]})
	}

	out = append(out, TextPart{Icon: textIcons[best]})

	return append(out, splitIcons(text[at+len(best):])...)
}
