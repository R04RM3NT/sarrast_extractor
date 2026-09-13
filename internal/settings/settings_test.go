package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	old := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userConfigDir = old })

	want := Settings{Proxy: "socks5://127.0.0.1:1080"}
	if err := want.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "sarrast", "settings.json")); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Proxy != want.Proxy {
		t.Errorf("Proxy = %q, want %q", got.Proxy, want.Proxy)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	old := userConfigDir
	userConfigDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { userConfigDir = old })

	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Proxy != "" {
		t.Errorf("Proxy = %q, want empty", got.Proxy)
	}
}
