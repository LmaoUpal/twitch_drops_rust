package points

import (
	"testing"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

func live(logins ...string) []models.Channel {
	var out []models.Channel
	for _, l := range logins {
		out = append(out, models.Channel{Login: l, Online: true})
	}
	return out
}

func logins(chs []models.Channel) string {
	s := ""
	for i, ch := range chs {
		if i > 0 {
			s += ","
		}
		s += ch.Login
	}
	return s
}

func TestSlots(t *testing.T) {
	if Slots(false) != 2 || Slots(true) != 1 {
		t.Fatal("two channels without a drops watch, one next to it")
	}
}

func TestPick(t *testing.T) {
	offline := models.Channel{Login: "b", Online: false}
	tests := []struct {
		name    string
		tracked []string
		live    []models.Channel
		slots   int
		drops   string
		want    string
	}{
		{"priority order", []string{"a", "b", "c"}, live("c", "b", "a"), 2, "", "a,b"},
		{"offline skipped", []string{"a", "b", "c"}, append(live("a", "c"), offline), 2, "", "a,c"},
		{"drops channel skipped", []string{"a", "b", "c"}, live("a", "b", "c"), 1, "A", "b"},
		{"untracked live ignored", []string{"a"}, live("x", "a"), 2, "", "a"},
		{"case-insensitive and no duplicates", []string{"A", "a", "b"}, live("a", "b"), 2, "", "a,b"},
		{"nothing live", []string{"a"}, nil, 2, "", ""},
		{"no slots", []string{"a"}, live("a"), 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logins(Pick(tt.tracked, tt.live, tt.slots, tt.drops)); got != tt.want {
				t.Fatalf("want %q, got %q", tt.want, got)
			}
		})
	}
}
