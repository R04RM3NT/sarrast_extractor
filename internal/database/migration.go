package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"sarrast/internal/seriesdb"
)

// migrationVersion is the latest schema version this package knows about.
const migrationVersion = 3

// migrationV1 is the initial schema: it builds the series and episodes tables
// plus their indexes. Everything uses IF NOT EXISTS so re-running on an
// already-migrated database is a no-op.
func migrationV1(tx *sql.Tx) error {
	const schema = `
CREATE TABLE IF NOT EXISTS series (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    source_url TEXT NOT NULL,
    description TEXT DEFAULT '',
    thumbnail_url TEXT DEFAULT '',
    thumbnail_path TEXT DEFAULT '',
    category TEXT DEFAULT '',
    rating TEXT DEFAULT '',
    status TEXT DEFAULT '',
    episode_count INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    last_scanned_at DATETIME
);

CREATE TABLE IF NOT EXISTS episodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    series_id INTEGER NOT NULL,
    episode_key TEXT NOT NULL,
    episode_number INTEGER,
    title TEXT DEFAULT '',
    link TEXT NOT NULL,
    source_href TEXT DEFAULT '',
    first_seen_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    FOREIGN KEY (series_id) REFERENCES series(id) ON DELETE CASCADE,
    UNIQUE(series_id, episode_key)
);

CREATE INDEX IF NOT EXISTS idx_episodes_series_id
    ON episodes(series_id);

CREATE INDEX IF NOT EXISTS idx_episodes_number
    ON episodes(series_id, episode_number);
`
	if _, err := tx.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// migrationV2 normalizes episode keys created before the parser learned to skip
// the trailing random nonce a shortener appends to a release URL. Keys like
// "episode-156-FdsE4" are rewritten to "episode-156" and their true episode
// number (156) is filled in, making the key the stable identity across scans.
// The rewritten key must not collide with an existing one; a collision is
// skipped so the existing (correct) row wins.
func migrationV2(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, series_id, episode_key FROM episodes
		WHERE episode_key LIKE 'episode-%-%'`)
	if err != nil {
		return fmt.Errorf("migration v2: select nonce keys: %w", err)
	}
	type row struct {
		id  int64
		sid int64
		key string
	}
	var toFix []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.sid, &r.key); err != nil {
			rows.Close()
			return fmt.Errorf("migration v2: scan: %w", err)
		}
		toFix = append(toFix, r)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("migration v2: close rows: %w", err)
	}

	for _, r := range toFix {
		newKey, number, ok := paddedNumberInfo(r.key)
		if !ok || newKey == r.key {
			continue
		}
		// Skip rows whose normalized key would collide with an existing one.
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM episodes WHERE series_id = ? AND episode_key = ?`,
			r.sid, newKey).Scan(&exists); err != nil {
			return fmt.Errorf("migration v2: collision check: %w", err)
		}
		if exists > 0 {
			continue
		}
		if _, err := tx.Exec(`UPDATE episodes SET episode_key = ?, episode_number = ? WHERE id = ?`,
			newKey, number, r.id); err != nil {
			return fmt.Errorf("migration v2: update key: %w", err)
		}
	}
	return nil
}

// paddedNumberInfo returns, for a nonce-style episode key like
// "episode-156-FdsE4", the stable key "episode-156" and the true episode number
// 156. It reports ok=false (and leaves the key untouched) when the key is not in
// the padded "episode-NNN-suffix" shape — a plain "episode-1" slug or a
// "156-html" style numeric key is never rewritten.
func paddedNumberInfo(key string) (newKey string, number int, ok bool) {
	if !strings.HasPrefix(strings.ToLower(key), "episode-") {
		return "", 0, false
	}
	rest := key[len("episode-"):]
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	// Require a padded digit run (a leading zero) and a suffix like "-FdsE4".
	if i == 0 || i >= len(rest) || rest[i] != '-' {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[:i])
	if err != nil {
		return "", 0, false
	}
	return "episode-" + rest[:i], n, true
}

// nonceStrippedKey returns the stable episode key for a nonce-style key
// ("episode-156-FdsE4" → "episode-156"). It reports ok=false when the key is not
// in the padded "episode-NNN-suffix" shape, leaving it untouched.
func nonceStrippedKey(key string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(key), "episode-") {
		return "", false
	}
	rest := key[len("episode-"):]
	if len(rest) < 3 {
		return "", false
	}
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	// Must be padded (a leading zero) with a suffix after the digits, e.g.
	// "156-FdsE4" not "156" or "1" from a human slug.
	if i == 0 || i >= len(rest) || rest[i] != '-' {
		return "", false
	}
	padded := rest[:i]
	// "episode-099-…" is 094-099 → real number 99. Keep the padded leading zeros
	// out of the rewritten key's meaning, but preserve the identity by keeping
	// the full padded form (leading zeros are significant for ordering here).
	return "episode-" + padded, true
}

// migrationV3 performs two cleanups on already-stored episode rows:
//
//  1. Strips a trailing lowercase random nonce after a padded "episode-NNN-"
//     key (e.g. "episode-020-jq8y6" → "episode-020"). Migration v2 handled
//     mixed/uppercase nonces; this one handles the all-lowercase kind.
//  2. Backfills the true episode number for {random}-{ep} links whose number
//     sits at the end (e.g. key "pink-pussy-121" → episode_number 121) but was
//     stored as 0 before the parser learned that shape.
func migrationV3(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, series_id, episode_key, episode_number FROM episodes`)
	if err != nil {
		return fmt.Errorf("migration v3: select episodes: %w", err)
	}
	type epRow struct {
		id      int64
		seriesID int64
		key     string
		num     sql.NullInt64
	}
	var toFix []epRow
	for rows.Next() {
		var r epRow
		if err := rows.Scan(&r.id, &r.seriesID, &r.key, &r.num); err != nil {
			rows.Close()
			return fmt.Errorf("migration v3: scan: %w", err)
		}
		toFix = append(toFix, r)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("migration v3: close rows: %w", err)
	}

	for _, r := range toFix {
		newKey, number, changed := normalizeLegacy(r.key, r.num)
		if !changed {
			continue
		}
		// Skip rows whose normalized key would collide with an existing one.
		if newKey != r.key {
			var exists int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM episodes WHERE series_id = ? AND episode_key = ?`,
				r.seriesID, newKey).Scan(&exists); err != nil {
				return fmt.Errorf("migration v3: collision check: %w", err)
			}
			if exists > 0 {
				continue
			}
		}
		if _, err := tx.Exec(`UPDATE episodes SET episode_key = ?, episode_number = ? WHERE id = ?`,
			newKey, number, r.id); err != nil {
			return fmt.Errorf("migration v3: update episode: %w", err)
		}
	}
	return nil
}

// normalizeLegacy returns the (possibly rewritten) key and true number for a
// legacy episode row, and whether either changed. It handles:
//
//   - "episode-020-jq8y6" → ("episode-020", 20, true) — lowercase nonce stripped
//   - "pink-pussy-121" → ("pink-pussy-121", 121, true) — trailing number backfilled
func normalizeLegacy(key string, num sql.NullInt64) (string, int, bool) {
	currentNum := 0
	if num.Valid {
		currentNum = int(num.Int64)
	}

	newKey := key
	number := currentNum

	// Lowercase nonce after a padded episode-NNN- prefix.
	if low := strings.ToLower(key); strings.HasPrefix(low, "episode-") {
		rest := key[len("episode-"):]
		// digits run
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		// require: some digits, then a dash, then a same-segment suffix that
		// contains a letter (the nonce) — e.g. "020-jq8y6".
		if i > 0 && i < len(rest) && rest[i] == '-' && i+1 < len(rest) {
			suffix := rest[i+1:]
			hasLetter := false
			for k := 0; k < len(suffix); k++ {
				c := suffix[k]
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					hasLetter = true
					break
				}
			}
			if hasLetter {
				newKey = "episode-" + rest[:i]
				if n, err := strconv.Atoi(rest[:i]); err == nil {
					number = n
				}
			}
		}
	} else {
		// {random}-{ep}: trailing number, currently 0.
		if currentNum == 0 {
			j := len(key)
			for j > 0 && key[j-1] >= '0' && key[j-1] <= '9' {
				j--
			}
			if j < len(key) {
				if n, err := strconv.Atoi(key[j:]); err == nil && n > 0 {
					number = n
				}
			}
		}
	}

	changed := newKey != key || number != currentNum
	return newKey, number, changed
}

// migrations maps a schema version number to the function that upgrades the
// database to that version.
var migrations = map[int]func(*sql.Tx) error{
	1: migrationV1,
	2: migrationV2,
	3: migrationV3,
}

// MigrateJSONIfPresent imports the legacy JSON database (series.json) into the
// current SQLite database, if the file exists. It is idempotent: series are
// upserted by slug and episodes are guarded by UNIQUE(series_id, episode_key),
// so repeated runs never duplicate data. The original series.json is kept as a
// backup.
func MigrateJSONIfPresent(db *DB) error {
	return migrateJSON(db, legacyJSONPath)
}

// legacyJSONPath is a test seam: production points at "series.json" in the
// working directory; tests redirect it to a temp file so they never touch a
// real series.json.
var legacyJSONPath = "series.json"

func migrateJSON(db *DB, jsonPath string) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy JSON database: %w", err)
	}

	legacy := seriesdb.Database{}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("parse legacy JSON database: %w", err)
	}

	now := time.Now().UTC()

	err = withTx(db.sql, func(tx *sql.Tx) error {
		seriesImported := 0
		episodesImported := 0
		for _, s := range legacy.Series {
			seriesID, isNew, err := upsertSeries(tx, SlugFromName(s.Name), s.Name, s.SourceURL, now)
			if err != nil {
				return fmt.Errorf("migrate series %q: %w", s.Name, err)
			}
			if isNew {
				seriesImported++
			}
			if err := setLastScanned(tx, seriesID, s.LastScannedAt); err != nil {
				return fmt.Errorf("migrate series %q: %w", s.Name, err)
			}

			for _, ep := range s.Episodes {
				firstSeen := ep.FirstSeen
				lastSeen := ep.LastSeen
				if firstSeen.IsZero() {
					firstSeen = now
				}
				if lastSeen.IsZero() {
					lastSeen = now
				}
				if err := insertEpisode(tx, seriesID, ep.ID, ep.Link, ep.SourceHref, firstSeen, lastSeen); err != nil {
					return fmt.Errorf("migrate episode %q: %w", ep.ID, err)
				}
				episodesImported++
			}

			if err := updateCount(tx, seriesID, now); err != nil {
				return err
			}
		}

		db.log.Info("json migration complete",
			slog.Int("series_imported", seriesImported),
			slog.Int("episodes_imported", episodesImported),
		)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
