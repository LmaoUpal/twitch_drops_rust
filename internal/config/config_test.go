package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Twitch.ClientID == "" || len(cfg.Games) != 1 || cfg.Games[0] != "Rust" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	want := Default()
	want.Games = []string{"Rust", "Escape from Tarkov"}
	want.ExcludedChannels = []string{"someone"}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Games) != 2 || got.ExcludedChannels[0] != "someone" || got.Twitch.Hashes["Inventory"] != want.Twitch.Hashes["Inventory"] {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}

func TestLoadKeepsDefaultHashesForMissingOps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "[twitch]\nclient_id = \"abc\"\n[twitch.hashes]\nInventory = \"custom\"\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Twitch.ClientID != "abc" || cfg.Twitch.Hashes["Inventory"] != "custom" {
		t.Fatalf("overrides lost: %+v", cfg.Twitch)
	}
	if cfg.Twitch.Hashes["ViewerDropsDashboard"] == "" {
		t.Fatal("default hash for unlisted op missing")
	}
}
