package search

import (
	"errors"
	"slices"
	"testing"

	"github.com/dmikalova/keyforge-data/query"
)

func TestSortListIsCanonical(t *testing.T) {
	e := engine(t)
	cases := map[string]string{
		"sort:set,-power,name":       "sort:set,-power,name",
		"sort:Set,-POWER":            "sort:set,-power",
		"h:dis sort:type,house":      "house:dis sort:type,house",
		"sort:random seed:4":         "sort:random seed:4",
		"sort:house,type,power,name": "sort:house,type,power,name",
	}

	for in, want := range cases {
		q, err := e.Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)

			continue
		}

		if got := q.String(); got != want {
			t.Errorf("Parse(%q).String() = %q, want %q", in, got, want)
		}

		again, err := e.Parse(q.String())
		if err != nil || again.String() != want {
			t.Errorf("%q does not round-trip: %v %v", want, again, err)
		}
	}
}

func TestSortListErrors(t *testing.T) {
	e := engine(t)
	cases := map[string]string{
		"sort:sett":        "set",
		"sort:set,,name":   "",
		"sort:name,name":   "",
		"sort:random,name": "random",
		"sort:-random":     "random",
		"sort:power,hosue": "house",
	}

	for in, suggest := range cases {
		_, err := e.Parse(in)

		var qe *query.Error
		if !errors.As(err, &qe) {
			t.Errorf("Parse(%q) error = %v, want a query error", in, err)

			continue
		}

		if qe.Suggest != suggest {
			t.Errorf("Parse(%q) suggests %q, want %q", in, qe.Suggest, suggest)
		}
	}
}

func TestSortListOrders(t *testing.T) {
	e := engine(t)

	r, err := e.Search("t:creature,action sort:type,-power")
	if err != nil {
		t.Fatal(err)
	}

	prevType, prevPower := "", 1<<30

	for _, it := range r.Items {
		p := powerRank(it.Card)
		if it.Card.Type != prevType {
			if it.Card.Type < prevType {
				t.Fatalf("type %q after %q", it.Card.Type, prevType)
			}

			prevType, prevPower = it.Card.Type, 1<<30
		}

		if p > prevPower {
			t.Fatalf("%s power %d after %d in %s", it.Card.ID, p, prevPower, prevType)
		}

		prevPower = p
	}

	asc, _ := e.Search("t:creature sort:power")
	desc, _ := e.Search("t:creature sort:power order:desc")

	if n := len(asc.Items); n == 0 || asc.Items[0].Card.ID != desc.Items[n-1].Card.ID {
		t.Error("order:desc does not reverse the whole order")
	}
}

func TestGroupBuckets(t *testing.T) {
	if got := HouseLabel(groupBucket + "archonpower"); got != "Archon power" {
		t.Errorf("label = %q", got)
	}

	order := []string{
		"dis",
		groupBucket + "anomaly",
		groupBucket + "skybeast",
		groupBucket + "thetide",
	}
	for i := 1; i < len(order); i++ {
		if bucketRank(order[i-1]) >= bucketRank(order[i]) {
			t.Errorf("%s ranks after %s", order[i-1], order[i])
		}
	}
}

func TestCompletions(t *testing.T) {
	c := engine(t).Completions()

	find := func(name string) CompleteField {
		for _, f := range c.Fields {
			if f.Name == name {
				return f
			}
		}

		t.Fatalf("no field %q in %+v", name, c.Fields)

		return CompleteField{}
	}

	if h := find("house"); h.Alias != "h" || !slices.Contains(h.Values, "dis") {
		t.Errorf("house = %+v", h)
	}

	if s := find("sort"); !slices.Contains(s.Values, "power") || !s.List {
		t.Errorf("sort = %+v", s)
	}

	if is := find(query.FlagField); !slices.Contains(is.Values, "houseless") {
		t.Errorf("is = %+v", is)
	}

	if tr := find("trait"); len(tr.Values) == 0 {
		t.Errorf("trait = %+v", tr)
	}
}

// TestSetFacetMatchesSetTerm checks that each set facet count is the number
// of results of the query with that set term.
func TestSetFacetMatchesSetTerm(t *testing.T) {
	e := engine(t)

	for _, q := range []string{"", "t:creature", "h:brobnar,untamed"} {
		r, err := e.Search(q)
		if err != nil {
			t.Fatal(err)
		}

		if len(r.Facets.Sets) == 0 {
			t.Fatalf("%q has no set facet", q)
		}

		for _, c := range r.Facets.Sets {
			with, err := e.Search(AddTerm(q, c.Term))
			if err != nil {
				t.Fatal(err)
			}

			if c.N != len(with.Items) {
				t.Errorf(
					"%q: facet %s = %d, but %q gives %d",
					q,
					c.Term,
					c.N,
					AddTerm(q, c.Term),
					len(with.Items),
				)
			}
		}
	}
}
