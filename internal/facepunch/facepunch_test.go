package facepunch

import (
	"os"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	f, err := os.Open("testdata/page.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 campaign, got %d", len(got))
	}
	c := got[0]
	if c.Round != "54" || c.Title != "Base Invaders" || c.Host != "Rustoria" || c.Status != "Upcoming" {
		t.Errorf("header: %+v", c)
	}
	if want := time.Date(2026, 10, 23, 18, 0, 0, 0, time.UTC); !c.StartAt.Equal(want) {
		t.Errorf("start %v, want %v", c.StartAt, want)
	}
	if want := time.Date(2026, 11, 1, 23, 59, 0, 0, time.UTC); !c.EndAt.Equal(want) {
		t.Errorf("end %v, want %v", c.EndAt, want)
	}
	if len(c.General) != 2 {
		t.Fatalf("want 2 general drops, got %+v", c.General)
	}
	if d := c.General[0]; d.Name != "Rug" || d.Minutes != 60 || d.ItemID != "11598" || d.ImageURL != "https://files.facepunch.com/rug.jpg" {
		t.Errorf("rug: %+v", d)
	}
	if d := c.General[1]; d.Name != "Garage Door" || d.Minutes != 180 {
		t.Errorf("garage door: %+v", d)
	}
	if len(c.Streamer) != 1 {
		t.Fatalf("want 1 streamer drop, got %+v", c.Streamer)
	}
	if s := c.Streamer[0]; s.Login != "twitchrivals" || s.DisplayName != "TwitchRivals" || s.Drop.Name != "Backpack" || s.Drop.Minutes != 30 || s.Drop.ImageURL != "https://files.facepunch.com/backpack.jpg" {
		t.Errorf("streamer drop: %+v", s)
	}
}

func TestParseMinutes(t *testing.T) {
	for in, want := range map[string]int{
		"1 Hour":            60,
		"2 Hours":           120,
		"30 minutes":        30,
		"1 Hour 30 Minutes": 90,
		"":                  0,
	} {
		if got := parseMinutes(in); got != want {
			t.Errorf("parseMinutes(%q) = %d, want %d", in, got, want)
		}
	}
}
