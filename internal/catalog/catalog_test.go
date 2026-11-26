package catalog

import "testing"

func TestFixtureCoversTheSpecialCases(t *testing.T) {
	c, err := Fixture()
	if err != nil {
		t.Fatal(err)
	}

	flags := map[string]bool{}
	xPower, anomaly, reprint := false, false, false

	for _, cd := range c.Cards() {
		for _, f := range cd.Flags {
			flags[f] = true
		}

		if cd.Power != nil && cd.Power.X {
			xPower = true
		}

		if len(cd.Printings) > 1 {
			reprint = true
		}

		for _, p := range cd.Printings {
			anomaly = anomaly || p.Anomaly
		}
	}

	for _, f := range []string{"gigantic", "eviltwin", "tokencreature", "reference", "skybeast"} {
		if !flags[f] {
			t.Errorf("the fixture has no card with the flag %q", f)
		}
	}

	if !xPower || !anomaly || !reprint {
		t.Errorf("x power %v, anomaly %v, reprint %v: want all true", xPower, anomaly, reprint)
	}
}

func TestImageURL(t *testing.T) {
	if got := ImageURL("grimreminders/239.webp"); got != ImageBaseURL+"grimreminders/239.webp" {
		t.Errorf("ImageURL = %q", got)
	}

	if got := ImageURL(""); got != "" {
		t.Errorf("ImageURL(\"\") = %q, want empty", got)
	}
}
