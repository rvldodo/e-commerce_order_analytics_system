package command

import "testing"

func TestNeedsDatabase(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{nil, false},
		{[]string{"help"}, false},
		{[]string{"types"}, false},
		{[]string{"send", "--url", "https://x.io"}, false},
		{[]string{"get"}, true},
		{[]string{"get", "--type", "all"}, true},
		{[]string{"get", "--dry-run"}, false},
		{[]string{"get", "-dry-run"}, false},
		{[]string{"get", "--dry-run=true"}, false},
		{[]string{"get", "--dry-run=false"}, true},
		{[]string{"export", "--date", "2024-11-29"}, true},
		{[]string{"export", "--dry-run"}, false},
		{[]string{"get", "--out", "dry-run"}, true},
	}
	for _, c := range cases {
		if got := NeedsDatabase(c.args); got != c.want {
			t.Errorf("NeedsDatabase(%v) = %v, want %v", c.args, got, c.want)
		}
	}
}
