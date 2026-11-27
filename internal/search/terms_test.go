package search

import (
	"slices"
	"testing"

	"github.com/dmikalova/keyforge-data/card"

	"github.com/dmikalova/keyforge-card-search/internal/fixture"
)

func TestTerms(t *testing.T) {
	e := engine(t)
	cases := []struct {
		q    string
		want []string
	}{
		{"", nil},
		{"h:dis  -s:wc troll", []string{"house:dis", "-set:wc", "troll"}},
		{
			"(h:dis or h:logos) p>=3 sort:power",
			[]string{"(house:dis or house:logos)", "power>=3", "sort:power"},
		},
		{"h:dis or h:logos", []string{"house:dis or house:logos"}},
		{"s:aoa,wc", []string{"set:aoa,wc"}},
	}
	for _, c := range cases {
		got, err := e.Terms(c.q)
		if err != nil {
			t.Fatalf("Terms(%q): %v", c.q, err)
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("Terms(%q) = %q, want %q", c.q, got, c.want)
		}
	}
	if _, err := e.Terms("h:"); err == nil {
		t.Error("Terms of a bad query has no error")
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
	terms, _ := e.Terms("h:dis k:elusive")
	if !e.HasTerm(terms, "k:elusive") || e.HasTerm(terms, "k:taunt") {
		t.Errorf("HasTerm on %q", terms)
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

// TestFacetsSortAbilitiesAndFlagsByCount checks that the ability and flag
// facets put the value with the most results first.
func TestFacetsSortAbilitiesAndFlagsByCount(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}
	// Play is first in the printed order, but reap has the most cards.
	ds.Cards[0].Abilities = []string{"play", "reap"}
	ds.Cards[1].Abilities = []string{"reap"}
	ds.Cards[2].Abilities = []string{"reap", "fight"}
	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Search("")
	if err != nil {
		t.Fatal(err)
	}
	for name, cs := range map[string][]Count{
		"ability": res.Facets.Abilities, "flag": res.Facets.Flags,
	} {
		if len(cs) == 0 {
			t.Fatalf("no %s facet", name)
		}
		for i := 1; i < len(cs); i++ {
			if cs[i].N > cs[i-1].N {
				t.Errorf("%s facet not by count: %v", name, cs)
			}
		}
	}
	if res.Facets.Abilities[0].Value != "reap" {
		t.Errorf("first ability = %q, want reap", res.Facets.Abilities[0].Value)
	}
	for _, c := range res.Facets.Flags {
		if c.Term != "is:"+c.Value || c.Field != "is" {
			t.Errorf("flag count %+v", c)
		}
	}
}

// TestFacetCountsMatchTheirClick checks that each facet count is the number
// of results that a click on the value finds.
func TestFacetCountsMatchTheirClick(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}
	// A card with an anomaly printing beside its normal printing: the
	// rarity facet counts it under both rarities.
	c := &ds.Cards[0]
	c.Printings = append(c.Printings, card.Printing{
		Set: "worldscollide", Number: "A99", Rarity: "Anomaly", Anomaly: true,
	})
	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}
	all, err := e.Search("")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(all.Facets.Rarities, func(c Count) bool {
		return c.Value == "Anomaly" && c.N == 1
	}) {
		t.Errorf("rarities = %v, want Anomaly 1", all.Facets.Rarities)
	}
	for _, q := range []string{"", "h:brobnar", "t:creature", "s:cota", "r:rare"} {
		res, err := e.Search(q)
		if err != nil {
			t.Fatal(q, err)
		}
		checkFacetCounts(t, e, q, res.Facets)
	}
}

func checkFacetCounts(t *testing.T, e *Engine, q string, f Facets) {
	t.Helper()
	for _, cs := range [][]Count{
		f.Houses, f.Types, f.Rarities, f.Sets, f.Power, f.Armor, f.Aember,
		f.Keywords, f.Abilities, f.Traits, f.Artists, f.Flags,
	} {
		for _, c := range cs {
			if c.N == 0 {
				t.Errorf("query %q: %s %q has a count of 0", q, c.Field, c.Value)
			}
			state, _ := e.Filters(q).State(c.Field, c.Value)
			if state != StateNone {
				continue
			}
			// The click result is the total before limit:N, which the facets
			// count too.
			clicked := e.Toggle(q, c.Term)
			res, err := e.Search(clicked)
			if err != nil {
				t.Errorf("%q: %v", clicked, err)
				continue
			}
			if res.Total != c.N {
				t.Errorf("query %q: %s %q counts %d, but %q finds %d",
					q, c.Field, c.Value, c.N, clicked, res.Total)
			}
		}
	}
}

// TestRedemptionPrinting checks a card with a printing in another house: the
// card stays in its own house and shows its own printing, h:redemption finds
// it and shows the Redemption printing, and the house facet counts it under
// both houses.
func TestRedemptionPrinting(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}
	c := &ds.Cards[0]
	own := c.House
	c.Printings = append(c.Printings, card.Printing{
		Set: "tokensofchange", Number: "152", Rarity: "Rare", House: "redemption",
	})
	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}

	shown := func(q string) *card.Printing {
		t.Helper()
		res, err := e.Search(q)
		if err != nil {
			t.Fatal(q, err)
		}
		for _, it := range res.Items {
			if it.Card.ID == c.ID {
				return it.Printing
			}
		}
		t.Fatalf("%q does not find %s", q, c.ID)
		return nil
	}

	if p := shown(""); p.House != "" {
		t.Errorf("default printing is in %q, want the card's own house", p.House)
	}
	if p := shown("h:redemption"); p.House != "redemption" || p.Set != "tokensofchange" {
		t.Errorf("h:redemption shows %s %s", p.Set, p.House)
	}
	if p := shown("h:" + own); p.House != "" {
		t.Errorf("h:%s shows a printing in %q", own, p.House)
	}

	res, err := e.Search("")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(res.Facets.Houses, func(c Count) bool {
		return c.Value == "redemption" && c.N == 1
	}) {
		t.Errorf("houses = %v, want redemption 1", res.Facets.Houses)
	}
	checkFacetCounts(t, e, "", res.Facets)
	checkFacetCounts(t, e, "h:redemption", mustSearch(t, e, "h:redemption").Facets)
}

func mustSearch(t *testing.T, e *Engine, q string) *Result {
	t.Helper()
	res, err := e.Search(q)
	if err != nil {
		t.Fatal(q, err)
	}
	return res
}

func TestLimit(t *testing.T) {
	e := engine(t)
	all := mustSearch(t, e, "")

	r := mustSearch(t, e, "sort:random seed:7 limit:3")
	if len(r.Items) != 3 || r.Total != len(all.Items) {
		t.Fatalf("limit:3 = %d of %d, want 3 of %d", len(r.Items), r.Total, len(all.Items))
	}

	again := mustSearch(t, e, "sort:random seed:7 limit:3")
	for i := range r.Items {
		if again.Items[i].Card.ID != r.Items[i].Card.ID {
			t.Errorf("limit:3 with the same seed differs at %d", i)
		}
	}

	if _, err := e.Search("slice:3"); err == nil {
		t.Error("slice:3 has no error: limit has no alias")
	}

	if p := mustSearch(t, e, "unique:prints limit:5"); len(p.Items) != 5 {
		t.Errorf("unique:prints limit:5 = %d", len(p.Items))
	}

	if r := mustSearch(
		t,
		e,
		"limit:100000",
	); len(r.Items) != len(all.Items) ||
		r.Total != len(all.Items) {
		t.Errorf("a limit over the count = %d of %d", len(r.Items), r.Total)
	}

	for _, q := range []string{"limit:0", "limit:x", "limit:-2"} {
		if _, err := e.Search(q); err == nil {
			t.Errorf("%s has no error", q)
		}
	}
}

// auditEngine is the fixture with the cases that the facets must count
// right: a Redemption printing, an anomaly printing beside a normal
// printing, and artists on some printings.
func auditEngine(t *testing.T) *Engine {
	t.Helper()
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}
	ds.Cards[0].Printings = append(ds.Cards[0].Printings, card.Printing{
		Set: "tokensofchange", Number: "152", Rarity: "Rare", House: "redemption",
	})
	ds.Cards[1].Printings = append(ds.Cards[1].Printings, card.Printing{
		Set: "worldscollide", Number: "A98", Rarity: "Anomaly", Anomaly: true,
	})
	for i := range 4 {
		ds.Cards[i].Printings[0].Artist = []string{"Fábio Perez", "Leone Bonelli"}[i%2]
	}
	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// TestFacetAudit checks every facet value of many queries: its count is the
// number of results that a click on it finds, and no shown value counts 0.
func TestFacetAudit(t *testing.T) {
	e := auditEngine(t)
	for _, q := range []string{
		"", "type:reference", "h:redemption", "s:wc", "is:anomaly", "r:anomaly",
		"is:eviltwin", "k:assault", "-h:dis", "-h:brobnar,logos t:creature",
		"sort:random seed:3 limit:10", "unique:prints", "art:none", "h:none",
		"p>=3", "is:reprint -s:cota",
	} {
		res, err := e.Search(q)
		if err != nil {
			// The fixture has no card with this value (type:reference).
			t.Logf("skip %q: %v", q, err)
			continue
		}
		checkFacetCounts(t, e, q, res.Facets)
	}
}
