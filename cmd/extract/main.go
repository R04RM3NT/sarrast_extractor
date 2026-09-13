// Command extract is the entry point for the sarrast extractor.
//
// Running it with no arguments (or with -tui) opens the interactive TUI.
// Passing extraction flags (-url ...) runs the batch CLI flow.
// Passing -serve starts the web application.
package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"sarrast/internal/config"
	"sarrast/internal/database"
	"sarrast/internal/logging"
	"sarrast/internal/scanner"
	"sarrast/internal/settings"
	"sarrast/internal/tui"
	"sarrast/internal/web"
)

func main() {
	args := os.Args[1:]
	for _, a := range args {
		switch a {
		case "-tui", "--tui":
			runTUI(config.Default())
			return
		case "-serve", "--serve", "serve":
			runServe()
			return
		}
	}

	if len(args) == 0 {
		runTUI(config.Default())
		return
	}
	runCLI(config.Default())
}

// runTUI starts the interactive terminal interface.
func runTUI(cfg config.Config) {
	logger := logging.Discard()

	// Apply the persisted default proxy (the TUI can also change it live from
	// the Settings screen).
	if s, err := loadSettings(); err == nil && s.Proxy != "" {
		cfg.Proxy = s.Proxy
	}

	model, err := tui.New(cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}

// runCLI runs the classic batch extraction flow from command-line flags. The
// database is always the centralized SQLite database; there is no -db or -out
// option.
func runCLI(cfg config.Config) {
	var pageURL, seriesName, proxyURL, filterHost string
	var quiet bool

	flag.StringVar(&pageURL, "url", "", "page URL to extract <a> tags from (required)")
	flag.StringVar(&seriesName, "series", "", "series name (derived from the URL when omitted)")
	flag.StringVar(&proxyURL, "proxy", "", "proxy address for requests (http://, https:// or socks5://host:port)")
	flag.StringVar(&filterHost, "filter", cfg.FilterHost, "substring that must be present in an <a> tag's href to be considered")
	flag.BoolVar(&quiet, "quiet", false, "if set, suppress normal progress output")
	flag.Parse()

	if pageURL == "" {
		fmt.Fprintln(os.Stderr, "error: the -url flag is required")
		flag.Usage()
		os.Exit(1)
	}

	// Fall back to the persisted default proxy unless -proxy was given.
	if proxyURL == "" {
		if s, err := loadSettings(); err == nil {
			proxyURL = s.Proxy
		}
	}

	db, err := database.New(logging.New(os.Stderr, defaultLogLevel()))
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	progress := scanner.ProgressFunc(nil)
	if !quiet {
		progress = func(message string) { log.Print(message) }
	}
	result, err := scanner.Scan(db, pageURL, seriesName, proxyURL, filterHost, cfg.HTTPTimeout, progress)
	if err != nil {
		log.Fatalf("scan failed: %v", err)
	}

	if !quiet {
		fmt.Printf("Series: %s\n", result.Series.Name)
		fmt.Printf("New episodes: %d\n", len(result.NewEpisodes))
		fmt.Printf("Total episodes: %d\n", len(result.Series.Episodes()))
		fmt.Printf("Database: centralized SQLite database (%s)\n", database.StoragePath)
	}
}

func defaultLogLevel() slog.Level { return slog.LevelInfo }

func runServe() {
	logger := logging.New(os.Stderr, defaultLogLevel())

	db, err := database.New(logger)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	ok := web.Serve(db, web.Options{Addr: web.DefaultAddr, Log: logger})
	if !ok {
		os.Exit(1)
	}
}

// loadSettings reads the persisted user settings, returning a zero-value
// struct when no file exists.
func loadSettings() (settings.Settings, error) {
	s, err := settings.Load()
	if err != nil {
		return settings.Settings{}, err
	}
	return s, nil
}
