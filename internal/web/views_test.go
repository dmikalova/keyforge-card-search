package web

import "testing"

func TestFirstCount(t *testing.T) {
	cases := []struct{ at, total, want int }{
		{0, 1000, BatchSize},
		{0, 10, 10},
		{59, 1000, 120},
		{60, 1000, 180},
		{240, 1000, 360},
		{240, 250, 250},
		{1150, 100_000, MaxRestore},
		{-5, 1000, BatchSize},
		{1_000_000, 100_000, MaxRestore},
	}
	for _, c := range cases {
		if got := FirstCount(c.at, c.total); got != c.want {
			t.Errorf("FirstCount(%d, %d) = %d, want %d", c.at, c.total, got, c.want)
		}
	}
}
