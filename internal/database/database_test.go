package database

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sarrast/internal/seriesdb"
)

// openTemp opens a fresh SQLite database in a temporary directory and closes it
// when the test finishes.
func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"), nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// fixedNow is a stable timestamp pinned for tests instead of time.Now().
func fixedNow() time.Time {
	return time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
}

// TestOpenCreatesSchema checks that a fresh database has the series and
// episodes tables plus the expected indexes.
func TestOpenCreatesSchema(t *testing.T) {
	db := openTemp(t)

	var n int
	if err := db.sql.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'series'").Scan(&n); err != nil {
		t.Fatalf("count series table: %v", err)
	}
	if n != 1 {
		t.Fatalf("series table missing, found %d", n)
	}

	if err := db.sql.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'episodes'").Scan(&n); err != nil {
		t.Fatalf("count episodes table: %v", err)
	}
	if n != 1 {
		t.Fatalf("episodes table missing, found %d", n)
	}

	if err := db.sql.QueryRow(`SELECT count(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_episodes_series_id'`).Scan(&n); err != nil {
		t.Fatalf("count idx index: %v", err)
	}
	if n != 1 {
		t.Fatalf("idx_episodes_series_id missing, found %d", n)
	}

	if err := db.sql.QueryRow(`SELECT count(*) FROM sqlite_master
		WHERE type = 'index' AND name = 'idx_episodes_number'`).Scan(&n); err != nil {
		t.Fatalf("count idx_episodes_number index: %v", err)
	}
	if n != 1 {
		t.Fatalf("idx_episodes_number missing, found %d", n)
	}
}

// TestOpenEnablesForeignKeys checks the foreign_keys pragma is switched on so
// ON DELETE CASCADE actually works.
func TestOpenEnablesForeignKeys(t *testing.T) {
	db := openTemp(t)

	var fk int
	if err := db.sql.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
}

// TestSchemaVersion checks a fresh database starts at the latest migration
// version and that reopening does not re-migrate.
func TestSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "version.db")
	db, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	var version int
	if err := db.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != migrationVersion {
		t.Fatalf("user_version = %d, want %d", version, migrationVersion)
	}
	db.Close()

	// Reopening an existing database (no migration / no JSON file) must succeed.
	db2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	db2.Close()
}

// TestMigrationV2NormalizesNonceKeys seeds episodes with the legacy nonce-style
// keys the parser used to produce ("episode-156-FdsE4"), then reopens the
// database so migration v2 rewrites them to their stable identity ("episode-156")
// and fills in the true episode number (156). Plain episode-1 slugs and numeric
// 156-thor keys are left untouched.
func TestMigrationV2NormalizesNonceKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v2.db")

	// Build a v1-shaped database with raw nonce keys, then roll it back to
	// user_version 1 so migration v2 runs on the next Open.
	db, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.MergeScan(SeriesUpdate{Slug: "secret", Name: "Secret", SourceURL: "https://example.test/series/secret"}, []EpisodeUpdate{
		{Key: "episode-156-FdsE4", Number: 0, Title: "", Link: "https://example.test/156"},
		{Key: "episode-099-vhNJ9", Number: 0, Title: "", Link: "https://example.test/99"},
		{Key: "episode-1", Number: 0, Title: "", Link: "https://example.test/1"},
		{Key: "156-thor", Number: 156, Title: "episode 156", Link: "https://example.test/thor"},
	}, TimeSourceFunc(fixedNow)); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("downgrade user_version: %v", err)
	}
	db.Close()

	// The reopen applies migration v2.
	db2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("reopen for migration v2: %v", err)
	}
	defer db2.Close()

	byKey := map[string]int{}
	rows, err := db2.sql.Query(`SELECT episode_key, COALESCE(episode_number, 0) FROM episodes`)
	if err != nil {
		t.Fatalf("query episodes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			t.Fatalf("scan episode: %v", err)
		}
		byKey[k] = n
	}

	want := map[string]int{
		"episode-156": 156,
		"episode-099": 99,
		"episode-1":   0,
		"156-thor":    156,
	}
	if len(byKey) != len(want) {
		t.Fatalf("episode keys = %v, want %v", keysOfInt(byKey), keysOfInt(want))
	}
	for k, n := range want {
		got, ok := byKey[k]
		if !ok {
			t.Errorf("missing episode key %q", k)
			continue
		}
		if got != n {
			t.Errorf("episode %q number = %d, want %d", k, got, n)
		}
	}
}

func keysOfInt(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestMigrationV3NormalizesLowerCaseNonceAndTrailingNumbers seeds episodes with
// (a) a legacy lowercase-nonce key "episode-020-jq8y6" (left over when migration
// v2 only stripped mixed/uppercase nonces) and (b) a {random}-{ep} key
// "pink-pussy-121" with number 0. Reopening applies migration v3: the first is
// rewritten to "episode-020" with number 20, the second keeps its key but gets
// number 121.
func TestMigrationV3NormalizesLowerCaseNonceAndTrailingNumbers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v3.db")

	db, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.MergeScan(SeriesUpdate{Slug: "secret", Name: "Secret", SourceURL: "https://example.test/series/secret"}, []EpisodeUpdate{
		{Key: "episode-020-jq8y6", Number: 0, Title: "", Link: "https://example.test/a"},
		{Key: "episode-019", Number: 19, Title: "episode 19", Link: "https://example.test/b"},
		{Key: "pink-pussy-121", Number: 0, Title: "", Link: "https://example.test/c"},
		{Key: "292-xxx", Number: 292, Title: "episode 292", Link: "https://example.test/d"},
	}, TimeSourceFunc(fixedNow)); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	if _, err := db.sql.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatalf("downgrade user_version: %v", err)
	}
	db.Close()

	db2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("reopen for migration v3: %v", err)
	}
	defer db2.Close()

	byKey := map[string]int{}
	rows, err := db2.sql.Query(`SELECT episode_key, COALESCE(episode_number, 0) FROM episodes`)
	if err != nil {
		t.Fatalf("query episodes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			t.Fatalf("scan episode: %v", err)
		}
		byKey[k] = n
	}

	want := map[string]int{
		"episode-020":  20,
		"episode-019":  19,
		"pink-pussy-121": 121,
		"292-xxx":      292,
	}
	if len(byKey) != len(want) {
		t.Fatalf("episode keys = %v, want %v", keysOfInt(byKey), keysOfInt(want))
	}
	for k, n := range want {
		got, ok := byKey[k]
		if !ok {
			t.Errorf("missing episode key %q", k)
			continue
		}
		if got != n {
			t.Errorf("episode %q number = %d, want %d", k, got, n)
		}
	}
}

// TestReopenIsIdempotent checks migrations are not re-applied on a second open.
func TestReopenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")
	db, err := Open(path, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := db.MergeScan(SeriesUpdate{
		Slug: "demo", Name: "Demo", SourceURL: "https://example.test/series/demo",
	}, nil, TimeSourceFunc(fixedNow)); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	db.Close()

	db2, err := Open(path, nil)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := db2.GetSeries("demo")
	if err != nil {
		t.Fatalf("GetSeries after reopen: %v", err)
	}
	if got.Name != "Demo" {
		t.Fatalf("series name = %q, want Demo", got.Name)
	}
	db2.Close()
}

// TestMigrationImportsJSON writes a legacy series.json, opens a fresh database
// pointing at it, and checks all series + episodes land in SQLite with the
// original URLs, links and timestamps preserved.
func TestMigrationImportsJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := seriesdb.Database{Version: 1, Series: map[string]seriesdb.Series{
		"legacy-show": {
			Name:          "Legacy Show",
			SourceURL:     "https://example.test/series/legacy-show",
			LastScannedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Episodes: []seriesdb.Episode{
				{ID: "ep-1", Link: "https://shorter.test/?s=ep-1", SourceHref: "/go?s=ep-1", FirstSeen: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), LastSeen: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
				{ID: "ep-2", Link: "https://shorter.test/?s=ep-2", FirstSeen: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
			},
		},
	}}

	jsonPath := filepath.Join(dir, "series.json")
	if err := writeLegacyJSON(jsonPath, legacy); err != nil {
		t.Fatalf("write legacy json: %v", err)
	}

	old := legacyJSONPath
	legacyJSONPath = jsonPath
	t.Cleanup(func() { legacyJSONPath = old })

	db, err := Open(filepath.Join(dir, "out.db"), nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	series, err := db.ListSeries()
	if err != nil {
		t.Fatalf("ListSeries: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("series count = %d, want 1", len(series))
	}
	s := series[0]
	if s.Name != "Legacy Show" {
		t.Errorf("name = %q, want Legacy Show", s.Name)
	}
	if s.SourceURL != "https://example.test/series/legacy-show" {
		t.Errorf("source_url = %q", s.SourceURL)
	}
	// The migration should have preserved the original last_scanned_at.
	if !s.LastScannedAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("last_scanned_at = %v, want 2026-01-01", s.LastScannedAt)
	}
	if s.EpisodeCount != 2 {
		t.Errorf("episode_count = %d, want 2", s.EpisodeCount)
	}

	got, err := db.GetSeries(s.Slug)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if len(got.Episodes()) != 2 {
		t.Fatalf("episodes = %d, want 2", len(got.Episodes()))
	}

	// Preserve the episode links and their timestamps.
	byKey := map[string]*Episode{}
	for _, e := range got.Episodes() {
		byKey[e.EpisodeKey] = e
	}
	e1 := byKey["ep-1"]
	if e1 == nil {
		t.Fatalf("episode ep-1 missing")
	}
	if e1.Link != "https://shorter.test/?s=ep-1" {
		t.Errorf("ep-1 link = %q", e1.Link)
	}
	if e1.SourceHref != "/go?s=ep-1" {
		t.Errorf("ep-1 source_href = %q", e1.SourceHref)
	}
	if !e1.FirstSeenAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("ep-1 first_seen = %v", e1.FirstSeenAt)
	}
}

// TestMigrationKeepsOriginalFile checks the legacy series.json is left in place
// as a backup after migration.
func TestMigrationKeepsOriginalFile(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "series.json")
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Fatalf("temp json exists before it should")
	}
	if err := writeLegacyJSON(jsonPath, seriesdb.Database{Version: 1, Series: map[string]seriesdb.Series{
		"x": {Name: "X", Episodes: []seriesdb.Episode{{ID: "one", Link: "one"}}},
	}}); err != nil {
		t.Fatalf("write legacy json: %v", err)
	}

	old := legacyJSONPath
	legacyJSONPath = jsonPath
	t.Cleanup(func() { legacyJSONPath = old })

	db, err := Open(filepath.Join(dir, "out.db"), nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("original series.json was removed: %v", err)
	}
}

// TestMigrationIdempotent runs the migration twice against the same SQLite
// file and checks no duplicate rows are created.
func TestMigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "series.json")
	if err := writeLegacyJSON(jsonPath, seriesdb.Database{Version: 1, Series: map[string]seriesdb.Series{
		"show": {
			Name: "Show",
			Episodes: []seriesdb.Episode{
				{ID: "a", Link: "https://shorter.test/?s=a"},
				{ID: "b", Link: "https://shorter.test/?s=b"},
			},
		},
	}}); err != nil {
		t.Fatalf("write legacy json: %v", err)
	}

	old := legacyJSONPath
	legacyJSONPath = jsonPath
	t.Cleanup(func() { legacyJSONPath = old })

	outPath := filepath.Join(dir, "out.db")
	db, err := Open(outPath, nil)
	if err != nil {
		t.Fatalf("Open #1: %v", err)
	}
	db.Close()

	// Open again: migration runs against the same data but must not duplicate.
	db2, err := Open(outPath, nil)
	if err != nil {
		t.Fatalf("Open #2: %v", err)
	}
	defer db2.Close()

	series, err := db2.ListSeries()
	if err != nil {
		t.Fatalf("ListSeries: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("series count = %d, want 1", len(series))
	}
	if series[0].EpisodeCount != 2 {
		t.Fatalf("episode_count = %d, want 2", series[0].EpisodeCount)
	}

	s, err := db2.GetSeries(series[0].Slug)
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if len(s.Episodes()) != 2 {
		t.Fatalf("episodes = %d, want 2", len(s.Episodes()))
	}
}

// TestSeriesInsertAndUpdate scans a fresh series, then re-scans it with new
// metadata and checks the row updates rather than duplicating.
func TestSeriesInsertAndUpdate(t *testing.T) {
	db := openTemp(t)

	r1, err := db.MergeScan(SeriesUpdate{
		Slug: "demo", Name: "Demo", SourceURL: "https://example.test/series/demo",
		Description: "before",
	}, []EpisodeUpdate{{Key: "one", Number: 1, Title: "One", Link: "one"}}, TimeSourceFunc(fixedNow))
	if err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}
	if r1.Series.EpisodeCount != 1 {
		t.Fatalf("episode_count after scan 1 = %d, want 1", r1.Series.EpisodeCount)
	}

	r2, err := db.MergeScan(SeriesUpdate{
		Slug: "demo", Name: "Demo", SourceURL: "https://example.test/series/demo",
		Description: "after",
		Rating:      "8.0",
	}, []EpisodeUpdate{{Key: "two", Number: 2, Title: "Two", Link: "two"}}, TimeSourceFunc(fixedNow))
	if err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}
	// Same series row: episode_count reflects the accumulated list, not a reset.
	if r2.Series.EpisodeCount != 2 {
		t.Fatalf("episode_count after scan 2 = %d, want 2", r2.Series.EpisodeCount)
	}
	if r2.Series.Description != "after" {
		t.Errorf("description = %q, want after", r2.Series.Description)
	}
	if r2.Series.Rating != "8.0" {
		t.Errorf("rating = %q, want 8.0", r2.Series.Rating)
	}

	// There is only one series row.
	var count int
	if err := db.sql.QueryRow("SELECT count(*) FROM series").Scan(&count); err != nil {
		t.Fatalf("count series rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("series rows = %d, want 1", count)
	}
}

// TestEpisodeDeduplication scans the same episode twice and checks it is only
// stored once.
func TestEpisodeDeduplication(t *testing.T) {
	db := openTemp(t)

	first := []EpisodeUpdate{
		{Key: "one", Number: 1, Title: "One", Link: "one"},
		{Key: "two", Number: 2, Title: "Two", Link: "two"},
	}
	if _, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, first, TimeSourceFunc(fixedNow)); err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}

	second := []EpisodeUpdate{
		{Key: "two", Number: 2, Title: "Two", Link: "two"},
		{Key: "one", Number: 1, Title: "One", Link: "one"},
		{Key: "three", Number: 3, Title: "Three", Link: "three"},
	}
	r, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, second, TimeSourceFunc(fixedNow))
	if err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}
	if len(r.NewEpisodes) != 1 || r.NewEpisodes[0].EpisodeKey != "three" {
		t.Fatalf("new episodes = %v, want only three", keysOf(r.NewEpisodes))
	}
	if r.Series.EpisodeCount != 3 {
		t.Fatalf("episode_count = %d, want 3", r.Series.EpisodeCount)
	}

	got, err := db.GetSeries("s")
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if len(got.Episodes()) != 3 {
		t.Fatalf("stored episodes = %d, want 3", len(got.Episodes()))
	}
}

// TestUpdatesLastSeenAt verifies re-scanning a known episode refreshes its
// last_seen_at (and preserves first_seen_at).
func TestUpdatesLastSeenAt(t *testing.T) {
	db := openTemp(t)

	r1, err := db.MergeScan(
		SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"},
		[]EpisodeUpdate{{Key: "one", Number: 1, Title: "One", Link: "one"}},
		TimeSourceFunc(fixedNow),
	)
	if err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}
	if r1.Series.Episodes()[0].LastSeenAt.IsZero() {
		t.Fatal("last_seen_at not set on first scan")
	}

	later := fixedNow().Add(time.Hour)
	r2, err := db.MergeScan(
		SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"},
		[]EpisodeUpdate{{Key: "one", Number: 1, Title: "One", Link: "one"}},
		at(later),
	)
	if err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}

	ep := r2.Series.Episodes()[0]
	if !ep.LastSeenAt.Equal(later) {
		t.Errorf("last_seen_at = %v, want %v", ep.LastSeenAt, later)
	}
	if !ep.FirstSeenAt.Equal(fixedNow()) {
		t.Errorf("first_seen_at = %v, want preserved %v", ep.FirstSeenAt, fixedNow())
	}
}

// TestUpdatesLastScannedAt verifies the series row's last_scanned_at is bumped
// on every scan.
func TestUpdatesLastScannedAt(t *testing.T) {
	db := openTemp(t)

	r1, err := db.MergeScan(
		SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"},
		nil, TimeSourceFunc(fixedNow),
	)
	if err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}
	if !r1.Series.LastScannedAt.Equal(fixedNow()) {
		t.Errorf("last_scanned_at = %v, want %v", r1.Series.LastScannedAt, fixedNow())
	}

	later := fixedNow().Add(2 * time.Hour)
	r2, err := db.MergeScan(
		SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"},
		nil, at(later),
	)
	if err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}
	if !r2.Series.LastScannedAt.Equal(later) {
		t.Errorf("last_scanned_at = %v, want %v", r2.Series.LastScannedAt, later)
	}
}

// TestQueryCompleteEpisodeList verifies GetSeries returns every stored episode
// newest-first (by episode_number DESC), not just the latest scan. Gaps in
// numbering are preserved.
func TestQueryCompleteEpisodeList(t *testing.T) {
	db := openTemp(t)

	if _, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, []EpisodeUpdate{
		{Key: "b", Number: 2, Title: "B", Link: "b"},
		{Key: "a", Number: 1, Title: "A", Link: "a"},
		{Key: "c", Number: 48, Title: "C", Link: "c"},
	}, TimeSourceFunc(fixedNow)); err != nil {
		t.Fatalf("MergeScan: %v", err)
	}
	// A later scan with a subset must not truncate the stored history, and a
	// rescan of an existing number must not duplicate it.
	if _, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, []EpisodeUpdate{
		{Key: "b", Number: 2, Title: "B", Link: "b"},
		{Key: "a", Number: 1, Title: "A", Link: "a"},
	}, at(fixedNow().Add(time.Hour))); err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}

	got, err := db.GetSeries("s")
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	eps := got.Episodes()
	if len(eps) != 3 {
		t.Fatalf("episodes = %d, want 3", len(eps))
	}
	// Newest first: 48, 2, 1.
	wantKeys := []string{"c", "b", "a"}
	for i, e := range eps {
		if e.EpisodeKey != wantKeys[i] {
			t.Fatalf("episode %d key = %s; want %s (order: %v)", i, e.EpisodeKey, wantKeys[i], wantKeys)
		}
	}
}

// TestRescanRefreshesNumberOnExistingEpisode verifies that re-scanning an
// already-stored episode updates its episode_number (and title) rather than
// duplicating.
func TestRescanRefreshesNumberOnExistingEpisode(t *testing.T) {
	db := openTemp(t)

	if _, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, []EpisodeUpdate{
		{Key: "1-x", Number: 0, Title: "Old", Link: "https://ouo.io/?s=1-x"},
	}, at(fixedNow())); err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}

	// Second scan reports the same key with a truer number and title.
	if _, err := db.MergeScan(SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}, []EpisodeUpdate{
		{Key: "1-x", Number: 1, Title: "Real", Link: "https://ouo.io/?s=1-x"},
	}, at(fixedNow().Add(time.Hour))); err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}

	got, err := db.GetSeries("s")
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if len(got.Episodes()) != 1 {
		t.Fatalf("episodes = %d, want 1 (no duplicate on rescan)", len(got.Episodes()))
	}
	ep := got.Episodes()[0]
	if ep.Number != 1 {
		t.Errorf("episode_number = %d, want 1 (refreshed on rescan)", ep.Number)
	}
	if ep.Title != "Real" {
		t.Errorf("title = %q, want Real (refreshed on rescan)", ep.Title)
	}
}

// TestGetSeriesNotFound checks a missing slug returns ErrNotFound.
func TestGetSeriesNotFound(t *testing.T) {
	db := openTemp(t)
	if _, err := db.GetSeries("nope"); err != ErrNotFound {
		t.Fatalf("GetSeries missing = %v, want ErrNotFound", err)
	}
}

// TestSeriesValidation checks MergeScan rejects updates without an identity.
func TestSeriesValidation(t *testing.T) {
	db := openTemp(t)
	if _, err := db.MergeScan(SeriesUpdate{}, nil, nil); err == nil {
		t.Fatal("MergeScan with empty series update should fail")
	}
}

// keysOf returns the episode keys of the given episodes (test helper).
func keysOf(eps []*Episode) []string {
	out := make([]string, 0, len(eps))
	for _, e := range eps {
		out = append(out, e.EpisodeKey)
	}
	return out
}

// writeLegacyJSON writes a legacy seriesdb.Database in the on-disk format.
func writeLegacyJSON(path string, db seriesdb.Database) error {
	data, err := json.Marshal(db)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// TimeSourceFunc adapts a function to the TimeSource interface.
type TimeSourceFunc func() time.Time

func (f TimeSourceFunc) Now() time.Time { return f() }

// at pins the clock to a fixed instant for scan tests.
func at(t time.Time) TimeSourceFunc { return func() time.Time { return t } }
