package web

import (
	"testing"

	"github.com/dmikalova/keyforge-data/query"
	"github.com/dop251/goja"
)

// TestTextFoldingMatchesGo runs static/js/text.js and checks that the search
// bar folds text like the query package does. The bar marks and completes
// values with it, so a difference would hide a match.
func TestTextFoldingMatchesGo(t *testing.T) {
	src, err := embedded.ReadFile("static/js/text.js")
	if err != nil {
		t.Fatal(err)
	}

	vm := goja.New()
	if _, err := vm.RunString(string(src)); err != nil {
		t.Fatal(err)
	}

	var fold, norm func(string) string

	kf := vm.Get("kfText").ToObject(vm)
	if err := vm.ExportTo(kf.Get("fold"), &fold); err != nil {
		t.Fatal(err)
	}

	if err := vm.ExportTo(kf.Get("norm"), &norm); err != nil {
		t.Fatal(err)
	}

	for _, s := range []string{
		"Troll", "Fábio Perez", "Décio Júnior", "Æmber", "æmber", "Ironyx Rebels",
		"Star Alliance", "Ol’ Paddy", "Z.Y.X.", "Dæmo-Thief", "token creature",
		"Mateusz Mizak", "Basith Ibrahim", "Ēkwidon", "naïve café", "1-2 Punch", "",
		"“Quoted”", "non‑Star — Alliance", "Œuvre", "Łódź", "Straße", "3×", "Wait…",
	} {
		if got, want := fold(s), query.Fold(s); got != want {
			t.Errorf("fold(%q) = %q, Go %q", s, got, want)
		}

		if got, want := norm(s), query.Normalize(s); got != want {
			t.Errorf("norm(%q) = %q, Go %q", s, got, want)
		}
	}
}
