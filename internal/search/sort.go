package search

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dmikalova/keyforge-data/query"
)

// SortRandom is the sort key that shuffles the results with the seed.
const SortRandom = "random"

// sortKey is one key of a sort list.
type sortKey struct {
	name string
	desc bool
}

// String returns the key as written in a query.
func (k sortKey) String() string {
	if k.desc {
		return "-" + k.name
	}

	return k.name
}

// sortList reads a sort value that normalizeSort made. An empty value is no
// keys.
func sortList(v string) []sortKey {
	var out []sortKey

	for part := range strings.SplitSeq(v, ",") {
		if part == "" {
			continue
		}

		name, desc := strings.CutPrefix(part, "-")
		out = append(out, sortKey{name: name, desc: desc})
	}

	return out
}

// normalizeSort checks the sort directives of the query and writes each one
// in canonical form: "Set, -POWER,name" becomes "set,-power,name".
func normalizeSort(q *query.Query) error {
	for i, d := range q.Directives {
		if d.Name != DirSort {
			continue
		}

		keys, err := parseSort(d)
		if err != nil {
			return err
		}

		parts := make([]string, len(keys))
		for j, k := range keys {
			parts[j] = k.String()
		}

		q.Directives[i].Value = strings.Join(parts, ",")
	}

	return nil
}

// parseSort reads the comma list of one sort directive.
func parseSort(d query.Directive) ([]sortKey, error) {
	fail := func(msg, suggest string) error {
		return &query.Error{Pos: d.Pos, Msg: msg, Suggest: suggest}
	}

	var keys []sortKey

	for part := range strings.SplitSeq(d.Value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fail("expected a sort key between the commas", "")
		}

		raw, desc := strings.CutPrefix(part, "-")
		name := query.Normalize(raw)

		if !slices.Contains(sortKeys, name) {
			return nil, fail(fmt.Sprintf("%q is not a sort key", raw), closestKey(name))
		}

		if slices.ContainsFunc(keys, func(k sortKey) bool { return k.name == name }) {
			return nil, fail(fmt.Sprintf("the sort key %q is in the list two times", name), "")
		}

		keys = append(keys, sortKey{name: name, desc: desc})
	}

	if slices.ContainsFunc(keys, func(k sortKey) bool { return k.name == SortRandom }) &&
		(len(keys) > 1 || keys[0].desc) {
		return nil, fail("sort:random cannot be in a list or descending", SortRandom)
	}

	return keys, nil
}

// closestKey returns the sort key nearest to s, or empty.
func closestKey(s string) string {
	best, dist := "", 3

	for _, k := range sortKeys {
		if d := levenshtein(s, k); d < dist {
			best, dist = k, d
		}
	}

	return best
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i

		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}

			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}

		prev = cur
	}

	return prev[len(b)]
}
