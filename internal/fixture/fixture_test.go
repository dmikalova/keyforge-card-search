package fixture

import "testing"

func TestDatasetCoversTheSpecialCases(t *testing.T) {
	ds, err := Dataset()
	if err != nil {
		t.Fatal(err)
	}

	flags := map[string]bool{}
	xPower, anomaly, reprint := false, false, false

	for _, cd := range ds.Cards {
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

	if len(ds.Sets) == 0 || ds.Version == "" {
		t.Errorf("sets %d, version %q: want sets and a version", len(ds.Sets), ds.Version)
	}
}
