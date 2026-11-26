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
