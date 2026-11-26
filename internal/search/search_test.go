package search

import (
	"errors"
	"slices"
	"testing"

	"github.com/dmikalova/keyforge-data/query"

	"github.com/dmikalova/keyforge-card-search/internal/catalog"
)

func engine(t *testing.T) *Engine {
	t.Helper()

	c, err := catalog.Fixture()
	if err != nil {
		t.Fatal(err)
	}

	e, err := New(c.Cards())
	if err != nil {
		t.Fatal(err)
	}

	return e
}

func ids(r *Result) []string {
	out := make([]string, len(r.Items))
	for i, it := range r.Items {
		out[i] = it.Card.ID
	}

	return out
}

func TestSearchMatches(t *testing.T) {
	e := engine(t)
	cases := []struct {
		q    string
		want []string
	}{
		{"n:anthony", []string{"ant1-10ny-staralliance"}},
		{"ant1", []string{"ant1-10ny-staralliance"}},
		{"n=troll", []string{"troll-brobnar"}},
		{"p:x", []string{"mimic-gel-logos"}},
		{"p>=8", []string{"ultra-gravitron-logos", "troll-brobnar"}},
		{
			"h:brobnar t:creature -is:eviltwin -is:tokencreature p>=6",
			[]string{"krump-brobnar", "troll-brobnar"},
		},
		{"is:eviltwin", []string{"krump-brobnar-evil-twin"}},
		{"is:gigantic", []string{"ultra-gravitron-logos"}},
		{"is:tokencreature", []string{"berserker-brobnar"}},
		{"is:reference", []string{"expect-the-unexpected"}},
		{"is:skybeast", []string{"cloudrunner"}},
		{"h:none", []string{"cloudrunner", "expect-the-unexpected"}},
		{"is:anomaly", []string{"ghostform-saurian"}},
		{"k:assault>=2", []string{"ancient-bear-untamed"}},
		{"k:elusive h:mars", []string{"agent-hoo-man-mars"}},
		{"en:capture>=1", []string{"adaptoid-logos"}},
		{"en>=3", []string{"adaptoid-logos"}},
		{"s:vm23", []string{"troll-brobnar"}},
		{"s:cota is:reprint", nil},
		{"s:vm23 is:reprint", []string{"troll-brobnar"}},
		{"s:mm -is:reprint t:creature h:untamed", nil},
		{"fl:smash", []string{"troll-brobnar"}},
		{"#S03", []string{"cloudrunner"}},
		{
			"(h:dis or h:mars) t:action or t:artifact",
			[]string{"gateway-to-dis-dis", "mothergun-mars"},
		},
		{"tr:giant ar>=1", nil},
		{"r:token", []string{"berserker-brobnar"}},
	}

	for _, c := range cases {
		r, err := e.Search(c.q)
		if err != nil {
			t.Errorf("%q: %v", c.q, err)

			continue
		}

		got := ids(r)
		want := slices.Clone(c.want)
		slices.Sort(got)
		slices.Sort(want)

		if !slices.Equal(got, want) {
			t.Errorf("%q = %v, want %v", c.q, got, want)
		}
	}
}

func TestDefaultPrintingIsTheNewestSet(t *testing.T) {
	r, err := engine(t).Search("n=troll")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.Items[0].Printing.Set; got != "vaultmasters2023" {
		t.Errorf("Troll shows %s, want vaultmasters2023", got)
	}

	r, err = engine(t).Search("n=troll s:cota")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.Items[0].Printing.Set; got != "callofthearchons" {
		t.Errorf("Troll in CotA shows %s, want callofthearchons", got)
	}
}

func TestUniquePrints(t *testing.T) {
	r, err := engine(t).Search("n=troll unique:prints")
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Items) != 2 {
		t.Errorf("got %d prints, want 2", len(r.Items))
	}
}

func TestDefaultSortPutsReferenceAndHouselessLast(t *testing.T) {
	r, err := engine(t).Search("")
	if err != nil {
		t.Fatal(err)
	}

	got := ids(r)
	if first := got[0]; first != "1-2-punch-brobnar" {
		t.Errorf("first = %s, want 1-2-punch-brobnar", first)
	}

	tail := got[len(got)-2:]
	slices.Sort(tail)

	if !slices.Equal(tail, []string{"cloudrunner", "expect-the-unexpected"}) {
		t.Errorf("last two = %v", tail)
	}
}

func TestSortDirectives(t *testing.T) {
	e := engine(t)

	r, err := e.Search("t:creature sort:power order:desc")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.Items[0].Card.ID; got != "mimic-gel-logos" {
		t.Errorf("highest power = %s, want mimic-gel-logos (X sorts last)", got)
	}

	if got := r.Items[1].Card.ID; got != "ultra-gravitron-logos" {
		t.Errorf("second = %s, want ultra-gravitron-logos", got)
	}

	if _, err := e.Search("sort:random"); !errors.Is(err, ErrNoSeed) {
		t.Errorf("sort:random without a seed: %v, want ErrNoSeed", err)
	}

	a, err := e.Search("sort:random seed:7")
	if err != nil {
		t.Fatal(err)
	}

	b, err := e.Search("sort:random seed:7")
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(ids(a), ids(b)) {
		t.Error("the same seed gives a different order")
	}
}

func TestFuzzyNameRanks(t *testing.T) {
	r, err := engine(t).Search("n:troll")
	if err != nil {
		t.Fatal(err)
	}

	if got := r.Items[0].Card.ID; got != "troll-brobnar" || len(r.Items) < 2 {
		t.Errorf("best match = %s of %d, want troll-brobnar first", got, len(r.Items))
	}
}

func TestParseErrorSuggests(t *testing.T) {
	_, err := engine(t).Search("hose:brobnar")

	var qe *query.Error
	if !errors.As(err, &qe) || qe.Suggest != "house" {
		t.Errorf("error = %v, want a suggestion of house", err)
	}
}

func TestFacets(t *testing.T) {
	r, err := engine(t).Search("h:brobnar")
	if err != nil {
		t.Fatal(err)
	}

	f := r.Facets
	if len(f.Houses) != 1 || f.Houses[0].Term != "h:brobnar" || f.Houses[0].Label != "Brobnar" {
		t.Errorf("houses = %+v", f.Houses)
	}

	if len(f.Power) == 0 || f.Power[0].Value != "2" {
		t.Errorf("power = %+v", f.Power)
	}

	r, err = engine(t).Search("is:gigantic or n=troll")
	if err != nil {
		t.Fatal(err)
	}

	sets := r.Facets.Sets
	if len(sets) != 2 || sets[0].Term != "s:MM" || sets[1].Term != "s:VM23" {
		t.Errorf("sets = %+v", sets)
	}
}

func TestAddTerm(t *testing.T) {
	if got := AddTerm(" h:dis ", "t:action"); got != "h:dis t:action" {
		t.Errorf("AddTerm = %q", got)
	}

	if got := WithSeed("", 5); got != "seed:5" {
		t.Errorf("WithSeed = %q", got)
	}
}
