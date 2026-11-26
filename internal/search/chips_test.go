package search

import (
	"slices"
	"testing"
)

func TestChips(t *testing.T) {
	e := engine(t)
	cases := []struct {
		q    string
		want []string
	}{
		{"", nil},
		{"h:dis  -s:wc troll", []string{"house:dis", "-set:WC", "troll"}},
		{
			"(h:dis or h:logos) p>=3 sort:power",
			[]string{"(house:dis or house:logos)", "power>=3", "sort:power"},
		},
		{"h:dis or h:logos", []string{"house:dis or house:logos"}},
		{"s:aoa,wc", []string{"set:AoA,WC"}},
	}
	for _, c := range cases {
		got, err := e.Chips(c.q)
		if err != nil {
			t.Fatalf("Chips(%q): %v", c.q, err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("Chips(%q) = %q, want %q", c.q, got, c.want)
		}
	}
	if _, err := e.Chips("h:"); err == nil {
		t.Error("Chips of a bad query has no error")
	}
}

func TestToggle(t *testing.T) {
	e := engine(t)
	cases := []struct{ q, term, want string }{
		{"", "h:dis", "house:dis"},
		{"h:dis p>=3", "h:dis", "power>=3"},
		{"house:dis", "h:dis", ""},
		{"p>=3", "h:dis", "power>=3 house:dis"},
		{"h:", "h:dis", "h: h:dis"},
	}
	for _, c := range cases {
		if got := e.Toggle(c.q, c.term); got != c.want {
			t.Errorf("Toggle(%q, %q) = %q, want %q", c.q, c.term, got, c.want)
		}
	}
	chips, _ := e.Chips("h:dis k:elusive")
	if !e.HasChip(chips, "k:elusive") || e.HasChip(chips, "k:taunt") {
		t.Errorf("HasChip on %q", chips)
	}
}

func TestAbilityFieldAndFacets(t *testing.T) {
	e := engine(t)
	for _, q := range []string{"ab:reap", "ability:play,fight", "-ab:omni"} {
		if _, err := e.Search(q); err != nil {
			t.Errorf("Search(%q): %v", q, err)
		}
	}
	if _, err := e.Search("ab:sleep"); err == nil {
		t.Error("ab:sleep has no error")
	}
	res, err := e.Search("")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Facets.Keywords) == 0 || len(res.Facets.Traits) == 0 {
		t.Errorf("keywords %v traits %v", res.Facets.Keywords, res.Facets.Traits)
	}
	if k := res.Facets.Keywords[0]; k.Term != "k:"+k.Value {
		t.Errorf("keyword term %q", k.Term)
	}
}

func TestAbilityOrder(t *testing.T) {
	got := []string{"reap", "action", "fight", "play", "omni"}
	slices.SortFunc(got, abilityOrder)

	if want := []string{"play", "fight", "reap", "action", "omni"}; !slices.Equal(got, want) {
		t.Errorf("ability order = %v, want %v", got, want)
	}
}
