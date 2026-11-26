package search

import "testing"

func TestLabels(t *testing.T) {
	l := engine(t).Labels()
	cases := []struct{ got, want string }{
		{l.House("staralliance"), "Star Alliance"},
		{l.House(HouseNone), "Other"},
		{l.House("newhouse"), "Newhouse"},
		{l.Type("action"), "Action"},
		{l.Ability("beforefight"), "Before Fight"},
		{l.Group("thetide"), "The Tide"},
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("label %q, want %q", c.got, c.want)
		}
	}

	if got := (Labels(nil)).House("dis"); got != "Dis" {
		t.Errorf("label without data = %q", got)
	}
}

// TestChangedFlag checks that is:changed finds the card with a printing of
// other text, and shows that printing.
func TestChangedFlag(t *testing.T) {
	r, err := engine(t).Search("is:changed")
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Items) != 1 || r.Items[0].Card.ID != "troll-brobnar" ||
		r.Items[0].Printing.Text == "" {
		t.Errorf("is:changed = %d items", len(r.Items))
	}
}
