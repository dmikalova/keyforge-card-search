package web

import (
	"testing"

	"github.com/dmikalova/keyforge-card-search/internal/fixture"
	"github.com/dmikalova/keyforge-card-search/internal/search"
)

func TestFacetsMarkFilters(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	e, err := search.New(ds)
	if err != nil {
		t.Fatal(err)
	}

	find := func(gs []FacetGroup, group, label string) (FacetLink, bool) {
		for _, g := range gs {
			if g.Name != group {
				continue
			}

			for _, l := range append(g.Counts, g.More...) {
				if l.Label == label {
					return l, true
				}
			}
		}

		return FacetLink{}, false
	}

	q := "-h:dis s:wc"
	r, err := e.Search(q)
	if err != nil {
		t.Fatal(err)
	}

	gs := NewFacets(e, q, r.Facets)

	// The excluded house has no results, so it has a row with a count of
	// 0, and the row removes the exclusion.
	dis, ok := find(gs, "House", "Dis")
	if !ok || !dis.Excluded || dis.N != 0 || dis.URL != SearchURL("set:wc") {
		t.Errorf("Dis row = %+v (found %t)", dis, ok)
	}

	wc, ok := find(gs, "Set", "Worlds Collide")
	if !ok || !wc.Active || wc.URL != SearchURL("-house:dis") {
		t.Errorf("Worlds Collide row = %+v (found %t)", wc, ok)
	}
}
