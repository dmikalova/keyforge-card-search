package web

import (
	"slices"
	"strings"
)

// lineBreak is the token of a line break in a word diff.
const lineBreak = "\n"

// DiffParts returns text as lines of parts, with the words that are not in
// base marked Changed. It is a word diff with the longest common
// subsequence: a printing's text against the card text.
func DiffParts(text, base string) [][]TextPart {
	a, b := diffTokens(text), diffTokens(base)

	// common[i][j] is the length of the longest common subsequence of a[i:]
	// and b[j:].
	common := make([][]int, len(a)+1)
	for i := range common {
		common[i] = make([]int, len(b)+1)
	}

	for i, v := range slices.Backward(a) {
		for j := len(b) - 1; j >= 0; j-- {
			if v == b[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}

	var lines [][]TextPart

	var line []TextPart

	add := func(word string, changed bool) {
		if word == lineBreak {
			lines = append(lines, line)
			line = nil

			return
		}

		if n := len(line); n > 0 && line[n-1].Changed == changed {
			line[n-1].Text += " " + word

			return
		}

		// The space between two parts goes in the unchanged one, so a mark
		// covers only the changed words.
		if n := len(line); n > 0 && changed {
			line[n-1].Text += " "
		} else if n > 0 {
			word = " " + word
		}

		line = append(line, TextPart{Text: word, Changed: changed})
	}

	for i, j := 0, 0; i < len(a); {
		switch {
		case j < len(b) && a[i] == b[j]:
			add(a[i], false)
			i++
			j++
		case j < len(b) && common[i][j+1] > common[i+1][j]:
			j++
		default:
			add(a[i], a[i] != lineBreak)
			i++
		}
	}

	if len(line) > 0 {
		lines = append(lines, line)
	}

	return lines
}

// diffTokens returns the words of text, with lineBreak between the lines.
func diffTokens(text string) []string {
	var out []string

	for line := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		if len(out) > 0 {
			out = append(out, lineBreak)
		}

		out = append(out, words...)
	}

	return out
}
