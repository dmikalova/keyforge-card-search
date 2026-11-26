package search

import "testing"

func TestFiltersState(t *testing.T) {
	e := engine(t)
	cases := []struct {
		q            string
		field, value string
		state        FilterState
		without      string
	}{
		// Aliases, case, and the full field name.
		{"h:dis", "house", "dis", StateActive, ""},
		{"house:dis", "house", "dis", StateActive, ""},
		{"HOUSE:Dis", "house", "dis", StateActive, ""},
		{"h:dis", "house", "logos", StateNone, ""},
		// Set codes, slugs, and names.
		{"s:wc", "set", "WC", StateActive, ""},
		{"set:WC", "set", "WC", StateActive, ""},
		{"set:worldscollide", "set", "WC", StateActive, ""},
		{`set:"Worlds Collide"`, "set", "WC", StateActive, ""},
		{"s:wc", "set", "AoA", StateNone, ""},
		// A comma list: each value is active, and the removal keeps the other.
		{"h:dis,logos", "house", "dis", StateActive, "house:logos"},
		{"h:dis,logos", "house", "logos", StateActive, "house:dis"},
		{
			"h:dis,logos,mars t:creature",
			"house",
			"logos",
			StateActive,
			"house:dis,mars type:creature",
		},
		// A negated term is excluded; the removal drops the negation.
		{"-h:dis t:action", "house", "dis", StateExcluded, "type:action"},
		{"-h:dis,logos", "house", "dis", StateExcluded, "-house:logos"},
		// Numbers: exact values, ranges, and comparisons.
		{"p:3", "power", "3", StateActive, ""},
		{"power=3", "power", "3", StateActive, ""},
		{"p:2..4", "power", "3", StateActive, ""},
		{"p:2..4", "power", "5", StateNone, ""},
		{"p>=5", "power", "6", StateActive, ""},
		{"p>=5", "power", "4", StateNone, ""},
		{"p:x", "power", "X", StateActive, ""},
		{"a:1", "aember", "1", StateActive, ""},
		// Keywords with or without a comparison, and the other enums.
		{"k:elusive", "keyword", "elusive", StateActive, ""},
		{"keyword:assault>=2", "keyword", "assault", StateActive, ""},
		{"ab:reap", "ability", "reap", StateActive, ""},
		{"t:creature", "type", "creature", StateActive, ""},
		{"r:rare", "rarity", "Rare", StateActive, ""},
		{"h:none", "house", "none", StateActive, ""},
		// A unique prefix of a value is the value.
		{"h:ek", "house", "ekwidon", StateActive, ""},
		{"s:age", "set", "AoA", StateActive, ""},
		{"k:elu", "keyword", "elusive", StateActive, ""},
		// Directives stay when a filter is removed.
		{"h:dis sort:name", "house", "dis", StateActive, "sort:name"},
		// A term inside an OR group is not a top-level filter.
		{"(h:dis or t:action)", "house", "dis", StateNone, ""},
	}
	for _, c := range cases {
		state, without := e.Filters(c.q).State(c.field, c.value)
		if state != c.state || without != c.without {
			t.Errorf(
				"%q %s:%s = %d %q, want %d %q",
				c.q,
				c.field,
				c.value,
				state,
				without,
				c.state,
				c.without,
			)
		}
	}
}

func TestToggleRemovesAnySpelling(t *testing.T) {
	e := engine(t)
	cases := []struct{ q, term, want string }{
		{"set:worldscollide t:creature", "s:WC", "type:creature"},
		{`set:"Worlds Collide" -h:dis`, "h:dis", "set:worldscollide"},
		{"house:dis,logos", "h:dis", "house:logos"},
		{"-h:dis", "h:dis", ""},
		{"t:action", "h:dis", "type:action house:dis"},
	}
	for _, c := range cases {
		if got := e.Toggle(c.q, c.term); got != c.want {
			t.Errorf("Toggle(%q, %q) = %q, want %q", c.q, c.term, got, c.want)
		}
	}
}
