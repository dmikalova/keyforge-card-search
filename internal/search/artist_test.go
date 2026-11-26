package search

import (
	"slices"
	"testing"

	"github.com/dmikalova/keyforge-card-search/internal/fixture"
)

// artistEngine is the fixture with artists on the first printing of three
// cards. The other printings have no artist, as in the live data today.
func artistEngine(t *testing.T) (*Engine, []string) {
	t.Helper()

	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	names := []string{"Fábio Perez", "Décio Júnior", "Leone Bonelli"}
	var ids []string

	for i, n := range names {
		ds.Cards[i].Printings[0].Artist = n
		ids = append(ids, ds.Cards[i].ID)
	}

	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}

	return e, ids
}

func TestArtistSearch(t *testing.T) {
	e, ids := artistEngine(t)
	cases := []struct {
		q    string
		want []string
	}{
		{"artist:fabio", ids[:1]},
		{"art:decio", ids[1:2]},
		{`artist:"Fábio Perez"`, ids[:1]},
		{"artist:junior", ids[1:2]},
		// "one" is in Leone, and it must not match a printing without an
		// artist.
		{"artist:one", ids[2:3]},
	}

	for _, c := range cases {
		r, err := e.Search(c.q)
		if err != nil {
			t.Fatal(c.q, err)
		}

		if got := ids0(r); !slices.Equal(got, c.want) {
			t.Errorf("%s = %v, want %v", c.q, got, c.want)
		}
	}

	none, err := e.Search("artist:none")
	if err != nil {
		t.Fatal(err)
	}

	// The three cards with an artist have one printing each, so artist:none
	// finds every other card.
	all, _ := e.Search("")
	if len(none.Items) != len(all.Items)-3 {
		t.Errorf(
			"artist:none = %d of %d, want %d",
			len(none.Items),
			len(all.Items),
			len(all.Items)-3,
		)
	}

	for _, it := range none.Items {
		if it.Printing.Artist != "" && len(it.Card.Printings) == 1 {
			t.Errorf("artist:none shows %s with artist %q", it.Card.ID, it.Printing.Artist)
		}
	}

	if r, _ := e.Search("-artist:none"); len(r.Items) != 3 {
		t.Errorf("-artist:none = %d cards, want 3", len(r.Items))
	}
}

func TestArtistFacetAndSort(t *testing.T) {
	e, _ := artistEngine(t)

	r, err := e.Search("sort:artist")
	if err != nil {
		t.Fatal(err)
	}

	got := []string{
		r.Items[0].Printing.Artist,
		r.Items[1].Printing.Artist,
		r.Items[2].Printing.Artist,
	}
	if !slices.Equal(got, []string{"Décio Júnior", "Fábio Perez", "Leone Bonelli"}) {
		t.Errorf("sort:artist first = %v", got)
	}

	if last := r.Items[len(r.Items)-1].Printing.Artist; last != "" {
		t.Errorf("sort:artist last = %q, want no artist", last)
	}

	if len(r.Facets.Artists) != 3 || r.Facets.Artists[0].Field != FieldArtist {
		t.Errorf("artist facet = %+v", r.Facets.Artists)
	}

	f := e.Filters(`art:"fabio perez"`)
	if s, _ := f.State(FieldArtist, "Fábio Perez"); s != StateActive {
		t.Errorf("artist facet state = %d", s)
	}

	if s, _ := e.Filters("art:fabio").State(FieldArtist, "Fábio Perez"); s != StateActive {
		t.Errorf("art:fabio does not mark Fábio Perez")
	}

	if s, _ := e.Filters("art:fabio").State(FieldArtist, "Décio Júnior"); s != StateNone {
		t.Errorf("art:fabio marks Décio Júnior")
	}

	// The live data has no artists yet: no facet, and artist:none is all.
	plain := engine(t)

	all, err := plain.Search("")
	if err != nil {
		t.Fatal(err)
	}

	if len(all.Facets.Artists) != 0 {
		t.Errorf("artist facet without artists = %+v", all.Facets.Artists)
	}

	none, err := plain.Search("artist:none")
	if err != nil {
		t.Fatal(err)
	}

	if len(none.Items) != len(all.Items) {
		t.Errorf("artist:none = %d, want %d", len(none.Items), len(all.Items))
	}
}

func ids0(r *Result) []string {
	var out []string
	for _, it := range r.Items {
		out = append(out, it.Card.ID)
	}

	return out
}

// TestArtistAliases checks that another spelling of an artist finds the
// printings of that artist.
func TestArtistAliases(t *testing.T) {
	ds, err := fixture.Dataset()
	if err != nil {
		t.Fatal(err)
	}

	ds.Cards[0].Printings[0].Artist = "Mateusz Mizak"
	ds.ArtistAliases = map[string]string{"Matthew Mizak": "Mateusz Mizak"}

	e, err := New(ds)
	if err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{`artist:"Matthew Mizak"`, "art:matthew", "art:mateusz"} {
		r, err := e.Search(q)
		if err != nil {
			t.Fatal(q, err)
		}

		if got := ids0(r); !slices.Equal(got, []string{ds.Cards[0].ID}) {
			t.Errorf("%s = %v, want %v", q, got, ds.Cards[0].ID)
		}
	}
}
