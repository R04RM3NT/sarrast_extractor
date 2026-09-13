// Package seriesdb stores series and episode links in a small file-based
// database. The JSON format keeps the application dependency-free while
// preserving all links between scans.
package seriesdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const currentVersion = 1

// Database is the on-disk collection of all known series.
type Database struct {
	Version int               `json:"version"`
	Series  map[string]Series `json:"series"`
}

// Series is a named collection of episode links discovered from a page.
type Series struct {
	Name          string    `json:"name"`
	SourceURL     string    `json:"source_url,omitempty"`
	Episodes      []Episode `json:"episodes"`
	LastScannedAt time.Time `json:"last_scanned_at"`
}

// Episode is one extracted link. ID is stable across scans and prevents
// duplicate entries when the same page is scanned again.
type Episode struct {
	ID         string    `json:"id"`
	Link       string    `json:"link"`
	SourceHref string    `json:"source_href,omitempty"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// ScanResult describes the effect of merging one scan into the database.
type ScanResult struct {
	Series      Series
	NewEpisodes []Episode
}

// New returns an empty database in the current format.
func New() Database {
	return Database{Version: currentVersion, Series: make(map[string]Series)}
}

// Load reads a database from path. A missing file is treated as an empty DB.
func Load(path string) (Database, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return Database{}, fmt.Errorf("read series database: %w", err)
	}

	db := New()
	if err := json.Unmarshal(data, &db); err != nil {
		return Database{}, fmt.Errorf("parse series database: %w", err)
	}
	if db.Version == 0 {
		db.Version = currentVersion
	}
	if db.Series == nil {
		db.Series = make(map[string]Series)
	}
	return db, nil
}

// Save writes the database to path using a temporary file and rename so a
// completed scan does not leave a partially written JSON file.
func (db Database) Save(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("series database path is empty")
	}
	if db.Version == 0 {
		db.Version = currentVersion
	}
	if db.Series == nil {
		db.Series = make(map[string]Series)
	}

	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("encode series database: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create series database directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".series-db-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary series database: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary series database: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary series database: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace series database: %w", err)
	}
	return nil
}

// Merge adds only previously unseen episodes to the named series and returns
// the complete episode list for display after the scan.
func (db *Database) Merge(name, sourceURL string, episodes []Episode, now time.Time) (ScanResult, error) {
	if db.Series == nil {
		db.Series = make(map[string]Series)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ScanResult{}, fmt.Errorf("series name is empty")
	}

	key := canonicalName(name)
	series, ok := db.Series[key]
	if !ok {
		series = Series{Name: name}
	}
	if series.Name == "" {
		series.Name = name
	}
	if sourceURL != "" {
		series.SourceURL = sourceURL
	}
	series.LastScannedAt = now

	known := make(map[string]int, len(series.Episodes))
	for i, episode := range series.Episodes {
		known[episode.ID] = i
	}

	var newEpisodes []Episode
	for _, episode := range episodes {
		if strings.TrimSpace(episode.ID) == "" || strings.TrimSpace(episode.Link) == "" {
			continue
		}
		if idx, exists := known[episode.ID]; exists {
			series.Episodes[idx].LastSeen = now
			if episode.SourceHref != "" {
				series.Episodes[idx].SourceHref = episode.SourceHref
			}
			continue
		}

		episode.FirstSeen = now
		episode.LastSeen = now
		known[episode.ID] = len(series.Episodes)
		series.Episodes = append(series.Episodes, episode)
		newEpisodes = append(newEpisodes, episode)
	}

	db.Series[key] = series
	return ScanResult{Series: series, NewEpisodes: newEpisodes}, nil
}

func canonicalName(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}
