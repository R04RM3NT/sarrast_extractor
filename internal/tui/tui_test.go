package tui

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"sarrast/internal/config"
	"sarrast/internal/database"
)

func keyRune(r rune) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

// setup opens a TUI model backed by a temporary centralized SQLite database so
// tests never create data/series.db inside the package directory.
func setup(t *testing.T) Model {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	old := openDB
	openDB = func(log *slog.Logger) (*database.DB, error) {
		return database.Open(filepath.Join(t.TempDir(), "tui.db"), log)
	}
	t.Cleanup(func() { openDB = old })

	mm, err := New(config.Default(), logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = mm.Close() })
	return mm
}

// TestUsesCentralizedDatabase checks that the TUI opens a single centralized
// SQLite database (the same the CLI and web server use) rather than a separate
// per-series or per-scan database.
func TestUsesCentralizedDatabase(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "central.db")
	old := openDB
	openDB = func(log *slog.Logger) (*database.DB, error) {
		return database.Open(path, log)
	}
	t.Cleanup(func() { openDB = old })

	mm, err := New(config.Default(), logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mm.Close()
	if mm.db == nil {
		t.Fatal("model has no database")
	}
	if got := mm.db.Path(); got != path {
		t.Fatalf("model database path = %q, want %q", got, path)
	}
}

func TestSettingsScreenNavigation(t *testing.T) {
	mm := setup(t)

	step := func(msg tea.Msg) Model {
		nm, _ := mm.Update(msg)
		mm = nm.(Model)
		return mm
	}

	// Menu → down ×1 → Settings.
	step(tea.KeyMsg{Type: tea.KeyDown})
	if m := step(tea.KeyMsg{Type: tea.KeyEnter}); m.screen != screenSettings {
		t.Fatalf("expected settings screen, got %d", m.screen)
	}

	// Type a socks5 proxy.
	for _, r := range []rune("socks5://127.0.0.1:1080") {
		step(keyRune(r))
	}
	// Pressing enter saves: the settings file would be written, so only check
	// that the proxy value landed on the input and validation accepts it.
	if got := mm.settings.proxyInput.Value(); got != "socks5://127.0.0.1:1080" {
		t.Errorf("proxy input = %q", got)
	}
	if msg, ok := validateProxy("socks5://127.0.0.1:1080"); !ok || msg == "" {
		t.Errorf("validateProxy rejected valid socks5 proxy: %q", msg)
	}

	// esc moves to the Quit section.
	if m := step(tea.KeyMsg{Type: tea.KeyEsc}); m.screen != screenQuit {
		t.Fatalf("expected quit screen after esc, got %d", m.screen)
	}

	// Arrow keys in the sidebar switch sections: up from Quit -> Settings.
	if m := step(tea.KeyMsg{Type: tea.KeyUp}); m.screen != screenSettings {
		t.Fatalf("expected settings after sidebar up, got %d", m.screen)
	}
}

func TestValidateProxy(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"", true},
		{"http://127.0.0.1:8080", true},
		{"https://proxy.example:8443", true},
		{"socks5://127.0.0.1:1080", true},
		{"socks5h://127.0.0.1:1080", true},
		{"127.0.0.1:8080", false},
		{"ftp://x", false},
		{"http://", false},
	}
	for _, c := range cases {
		if _, ok := validateProxy(c.in); ok != c.ok {
			t.Errorf("validateProxy(%q) ok=%v, want %v", c.in, ok, c.ok)
		}
	}
}
