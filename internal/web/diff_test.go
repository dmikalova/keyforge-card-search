package web

import (
	"reflect"
	"testing"
)

func TestDiffParts(t *testing.T) {
	got := DiffParts("Reap: Gain 1 Æmber.\nElusive.", "After Reap: Gain 2 Æmber.\nElusive.")
	want := [][]TextPart{
		{{Text: "Reap: Gain "}, {Text: "1", Changed: true}, {Text: " Æmber."}},
		{{Text: "Elusive."}},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiffParts = %+v", got)
	}

	same := DiffParts("Play: Draw a card.", "Play: Draw a card.")
	if len(same) != 1 || len(same[0]) != 1 || same[0][0].Changed {
		t.Errorf("same text = %+v", same)
	}
}
