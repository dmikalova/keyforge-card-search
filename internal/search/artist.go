package search

import (
	"github.com/dmikalova/keyforge-data/card"
	"github.com/dmikalova/keyforge-data/query"
)

// FieldArtist is the artist field. Its alias is "art", because "ar" is
// armor.
const FieldArtist = "artist"

// ArtistNone is the value of "artist:none": a printing without an artist.
const ArtistNone = "none"

// noArtist stands for a missing artist in the artist field's strings. No
// folded artist name or search value has the control character, so only
// the rewritten "artist:none" term matches it.
const noArtist = "\x01"

// artists returns the artists of the entry's printings, with noArtist for
// a printing without one. Each artist comes with its other spellings, so
// "artist:matthew" finds Mateusz Mizak.
func (e *Engine) artists(en *Entry) []string {
	return each(en, func(p *card.Printing) []string {
		if p.Artist == "" {
			return []string{noArtist}
		}

		return append([]string{p.Artist}, e.artistAliases[p.Artist]...)
	})
}

// aliasesByArtist returns the other spellings of each artist from the
// dataset's map of spelling to artist.
func aliasesByArtist(m map[string]string) map[string][]string {
	out := map[string][]string{}
	for alias, name := range m {
		out[name] = append(out[name], alias)
	}

	return out
}

// forMatch returns the query that the matcher runs: "artist:none" becomes an
// exact test for a printing without an artist. The query text that people
// see does not change.
func forMatch(q *query.Query) *query.Query {
	return &query.Query{Root: rewriteNone(q.Root), Directives: q.Directives, Source: q.Source}
}

func rewriteNone(n query.Node) query.Node {
	switch n := n.(type) {
	case *query.Term:
		if n.Field != FieldArtist || query.Fold(n.Value.Text) != ArtistNone {
			return n
		}

		t := *n
		t.Value.Text = noArtist
		if t.Op != query.OpNe {
			t.Op = query.OpEq
		}

		return &t
	case *query.And:
		return &query.And{Children: rewriteAll(n.Children), Pos: n.Pos}
	case *query.Or:
		return &query.Or{Children: rewriteAll(n.Children), Pos: n.Pos, List: n.List}
	case *query.Not:
		return &query.Not{Child: rewriteNone(n.Child), Pos: n.Pos}
	}

	return n
}

func rewriteAll(nodes []query.Node) []query.Node {
	out := make([]query.Node, len(nodes))
	for i, n := range nodes {
		out[i] = rewriteNone(n)
	}

	return out
}
