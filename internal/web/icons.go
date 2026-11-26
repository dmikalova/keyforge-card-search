package web

import (
	"regexp"
	"strings"

	"github.com/a-h/templ"
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
}

// triggerRe finds a trigger list at the start of a line or a sentence:
// "Play:", "Fight/Reap:", "Play/After Fight/After Reap:", "Before Fight:".
var triggerRe = regexp.MustCompile(
	`(?:^|[.!”"]\s+)((?:(?:After |Before )?(?:Play|Fight|Reap|Action|Omni|Destroyed|Leaves Play|Scrap)/?)+:)`,
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
