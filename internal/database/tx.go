package database

import (
	"database/sql"
	"fmt"
	"time"
)

// ---- generic transaction + row helpers ----

// withTx runs fn inside a database transaction. If fn returns an error the
// transaction is rolled back, otherwise it is committed.
func withTx(db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	tx = nil
	return nil
}

// ---- series write helpers ----

// upsertSeries inserts a series when its slug is new and otherwise returns the
// existing row untouched (migration path: creates a series that really did not
// exist before). isNew reports whether the row was just inserted.
func upsertSeries(tx *sql.Tx, slug, name, sourceURL string, now time.Time) (int64, bool, error) {
	res, err := tx.Exec(`INSERT OR IGNORE INTO series (slug, name, source_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, slug, name, sourceURL, now, now)
	if err != nil {
		return 0, false, fmt.Errorf("insert series: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("series rows affected: %w", err)
	}

	if affected == 0 {
		var id int64
		err := tx.QueryRow(`SELECT id FROM series WHERE slug = ?`, slug).Scan(&id)
		if err != nil {
			return 0, false, fmt.Errorf("find existing series: %w", err)
		}
		return id, false, nil
	}

	var id int64
	if err := tx.QueryRow(`SELECT id FROM series WHERE slug = ?`, slug).Scan(&id); err != nil {
		return 0, false, fmt.Errorf("query inserted series id: %w", err)
	}
	return id, true, nil
}

// insertEpisode inserts an episode unless (series_id, episode_key) already
// exists; duplicates are ignored, not reported as errors.
func insertEpisode(tx *sql.Tx, seriesID int64, key, link, sourceHref string, firstSeen, lastSeen time.Time) error {
	if _, err := tx.Exec(`INSERT OR IGNORE INTO episodes
		(series_id, episode_key, link, source_href, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		seriesID, key, link, sourceHref, firstSeen, lastSeen); err != nil {
		return fmt.Errorf("insert episode: %w", err)
	}
	return nil
}

// updateCount sets episode_count to the true number of stored rows.
func updateCount(tx *sql.Tx, seriesID int64, now time.Time) error {
	if _, err := tx.Exec(`UPDATE series SET
		episode_count = (SELECT COUNT(*) FROM episodes WHERE series_id = ?),
		updated_at = ?
		WHERE id = ?`, seriesID, now, seriesID); err != nil {
		return fmt.Errorf("update episode count: %w", err)
	}
	return nil
}

// setLastScanned records a series' last_scanned_at. It is used by the JSON
// migration so a legacy last scan timestamp survives the import (when it is
// zero, the value is left untouched rather than overwritten).
func setLastScanned(tx *sql.Tx, seriesID int64, ts time.Time) error {
	if ts.IsZero() {
		return nil
	}
	if _, err := tx.Exec(`UPDATE series SET last_scanned_at = ? WHERE id = ?`, ts, seriesID); err != nil {
		return fmt.Errorf("set last scanned at: %w", err)
	}
	return nil
}
