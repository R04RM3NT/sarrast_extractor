package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ---- queries ----

// ListSeries returns every series row, without their episodes, ordered by name.
func (db *DB) ListSeries() ([]*Series, error) {
	rows, err := db.sql.Query(`SELECT id, slug, name, source_url, description, thumbnail_url,
		thumbnail_path, category, rating, status, episode_count, created_at, updated_at, last_scanned_at
		FROM series ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list series: %w", err)
	}
	defer rows.Close()

	var out []*Series
	for rows.Next() {
		s, err := scanSeries(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetSeries returns one series by slug with its complete episode list, or
// ErrNotFound when nothing matches.
func (db *DB) GetSeries(slug string) (*Series, error) {
	row := db.sql.QueryRow(`SELECT id, slug, name, source_url, description, thumbnail_url,
		thumbnail_path, category, rating, status, episode_count, created_at, updated_at, last_scanned_at
		FROM series WHERE slug = ?`, slug)
	s, err := scanSeries(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	eps, err := db.listEpisodes(s.ID)
	if err != nil {
		return nil, err
	}
	s.setEpisodes(eps)
	return s, nil
}

// getSeriesBySlug is the internal variant returning the series row without its
// episodes; used by the JSON migration.
func (db *DB) getSeriesBySlug(slug string) (*Series, error) {
	row := db.sql.QueryRow(`SELECT id, slug, name, source_url, description, thumbnail_url,
		thumbnail_path, category, rating, status, episode_count, created_at, updated_at, last_scanned_at
		FROM series WHERE slug = ?`, slug)
	s, err := scanSeries(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s, nil
}

// SeriesExists reports whether a series row exists for the slug. It is used to
// make the JSON migration idempotent.
func (db *DB) SeriesExists(slug string) (bool, error) {
	s, err := db.getSeriesBySlug(slug)
	if err == ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return s != nil, nil
}

// SetThumbnailPath records the local thumbnail cache path for a series after
// the thumbnail has been cached to disk. It is a light update used by the
// scanner once caching succeeds; it is a no-op for an unknown slug.
func (db *DB) SetThumbnailPath(slug, path string) error {
	if strings.TrimSpace(slug) == "" {
		return nil
	}
	if _, err := db.sql.Exec(`UPDATE series SET thumbnail_path = ? WHERE slug = ?`, path, slug); err != nil {
		return fmt.Errorf("set thumbnail path: %w", err)
	}
	return nil
}

// listEpisodes returns every episode of a series ordered newest-first: by
// episode_number descending (highest first), then key, then id.
func (db *DB) listEpisodes(seriesID int64) ([]*Episode, error) {
	rows, err := db.sql.Query(`SELECT id, series_id, episode_key, episode_number, title, link,
		source_href, first_seen_at, last_seen_at
		FROM episodes WHERE series_id = ? ORDER BY COALESCE(episode_number, 0) DESC, episode_key DESC, id DESC`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("list episodes: %w", err)
	}
	defer rows.Close()

	var out []*Episode
	for rows.Next() {
		e, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---- scan transaction (series + episodes updated atomically) ----

// MergeScan merges one scan of a single series into the database inside a
// transaction: it upserts the series row with the fresh metadata, inserts
// newly-discovered episodes, refreshes last_seen_at for known ones and
// recomputes episode_count. The returned ScanResult carries the complete stored
// episode list.
func (db *DB) MergeScan(series SeriesUpdate, episodes []EpisodeUpdate, now TimeSource) (ScanResult, error) {
	if err := series.Validate(); err != nil {
		return ScanResult{}, err
	}
	if now == nil {
		now = Clock{}
	}
	ts := now.Now()

	var result ScanResult
	err := withTx(db.sql, func(tx *sql.Tx) error {
		id, _, err := upsertSeries(tx, series.Slug, series.Name, series.SourceURL, ts)
		if err != nil {
			return err
		}

		if err := updateSeriesMetadata(tx, id, series, ts); err != nil {
			return err
		}

		newCount := 0
		newKeys := make(map[string]struct{})
		seen := make(map[string]struct{}, len(episodes))
		for _, ep := range episodes {
			key := strings.TrimSpace(ep.Key)
			if key == "" {
				continue
			}
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}

			res, err := tx.Exec(`INSERT OR IGNORE INTO episodes
				(series_id, episode_key, episode_number, title, link, source_href, first_seen_at, last_seen_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				id, key, ep.Number, ep.Title, ep.Link, ep.SourceHref, ts, ts)
			if err != nil {
				return fmt.Errorf("insert episode: %w", err)
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return fmt.Errorf("episode rows affected: %w", err)
			}
			if affected == 1 {
				newCount++
				newKeys[key] = struct{}{}
			} else {
				// Existing episode: refresh its number/title (matching by the
				// actual episode identity) and bump last_seen_at.
				if _, err := tx.Exec(`UPDATE episodes SET
					episode_number = ?, title = ?, last_seen_at = ?,
					source_href = COALESCE(NULLIF(?, ''), source_href)
					WHERE series_id = ? AND episode_key = ?`,
					ep.Number, ep.Title, ts, ep.SourceHref, id, key); err != nil {
					return fmt.Errorf("touch episode: %w", err)
				}
			}
		}

		if err := updateCount(tx, id, ts); err != nil {
			return err
		}

		complete, err := listEpisodesTx(tx, id)
		if err != nil {
			return err
		}
		loaded, err := getSeriesTx(tx, id)
		if err != nil {
			return err
		}
		loaded.setEpisodes(complete)

		var newEps []*Episode
		for _, e := range complete {
			if _, ok := newKeys[e.EpisodeKey]; ok {
				newEps = append(newEps, e)
			}
		}

		result = ScanResult{Series: loaded, NewEpisodes: newEps}
		return nil
	})
	if err != nil {
		return ScanResult{}, err
	}
	return result, nil
}

// ---- scan row types ----

// SeriesUpdate carries the metadata to use for one series during a scan. It is
// produced by the scanner/metadata layers and consumed by MergeScan.
type SeriesUpdate struct {
	Slug          string
	Name          string
	SourceURL     string
	Description   string
	ThumbnailURL  string
	ThumbnailPath string
	Category      string
	Rating        string
	Status        string
}

// Validate rejects series updates that lack an identity.
func (u SeriesUpdate) Validate() error {
	if strings.TrimSpace(u.Slug) == "" {
		return fmt.Errorf("series slug is empty")
	}
	if strings.TrimSpace(u.Name) == "" {
		return fmt.Errorf("series name is empty")
	}
	if strings.TrimSpace(u.SourceURL) == "" {
		return fmt.Errorf("series source URL is empty")
	}
	return nil
}

// EpisodeUpdate is one episode discovered during a scan.
type EpisodeUpdate struct {
	Key        string
	Number     int
	Title      string
	Link       string
	SourceHref string
}

// TimeSource abstracts the clock so tests can pin timestamps.
type TimeSource interface {
	Now() time.Time
}

// Clock is the production clock.
type Clock struct{}

// Now returns the current UTC time.
func (Clock) Now() time.Time { return time.Now().UTC() }

func updateSeriesMetadata(tx *sql.Tx, id int64, series SeriesUpdate, ts time.Time) error {
	if _, err := tx.Exec(`UPDATE series SET
		name = ?, source_url = ?, description = ?, thumbnail_url = ?, thumbnail_path = ?,
		category = ?, rating = ?, status = ?, updated_at = ?, last_scanned_at = ?
		WHERE id = ?`,
		series.Name, series.SourceURL, series.Description, series.ThumbnailURL, series.ThumbnailPath,
		series.Category, series.Rating, series.Status, ts, ts, id); err != nil {
		return fmt.Errorf("update series metadata: %w", err)
	}
	return nil
}

func listEpisodesTx(tx *sql.Tx, seriesID int64) ([]*Episode, error) {
	rows, err := tx.Query(`SELECT id, series_id, episode_key, episode_number, title, link,
		source_href, first_seen_at, last_seen_at
		FROM episodes WHERE series_id = ? ORDER BY COALESCE(episode_number, 0) DESC, episode_key DESC, id DESC`, seriesID)
	if err != nil {
		return nil, fmt.Errorf("list episodes in transaction: %w", err)
	}
	defer rows.Close()

	var out []*Episode
	for rows.Next() {
		e, err := scanEpisode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func getSeriesTx(tx *sql.Tx, id int64) (*Series, error) {
	row := tx.QueryRow(`SELECT id, slug, name, source_url, description, thumbnail_url,
		thumbnail_path, category, rating, status, episode_count, created_at, updated_at, last_scanned_at
		FROM series WHERE id = ?`, id)
	s, err := scanSeries(row)
	if err != nil {
		return nil, fmt.Errorf("load series in transaction: %w", err)
	}
	return s, nil
}

// ==== scanning helpers ====

func scanEpisode(rows interface {
	Scan(dest ...interface{}) error
}) (*Episode, error) {
	var (
		e          Episode
		number     sql.NullInt64
		title      sql.NullString
		sourceHref sql.NullString
	)
	err := rows.Scan(&e.ID, &e.SeriesID, &e.EpisodeKey, &number, &title, &e.Link,
		&sourceHref, &e.FirstSeenAt, &e.LastSeenAt)
	if err != nil {
		return nil, fmt.Errorf("scan episode: %w", err)
	}
	if number.Valid {
		e.Number = int(number.Int64)
	}
	e.Title = title.String
	e.SourceHref = sourceHref.String
	return &e, nil
}

func scanSeries(rows interface {
	Scan(dest ...interface{}) error
}) (*Series, error) {
	var (
		s           Series
		desc        sql.NullString
		thumbURL    sql.NullString
		thumbPath   sql.NullString
		category    sql.NullString
		rating      sql.NullString
		status      sql.NullString
		lastScanned sql.NullTime
	)
	err := rows.Scan(&s.ID, &s.Slug, &s.Name, &s.SourceURL, &desc, &thumbURL, &thumbPath,
		&category, &rating, &status, &s.EpisodeCount, &s.CreatedAt, &s.UpdatedAt, &lastScanned)
	if err != nil {
		return nil, fmt.Errorf("scan series: %w", err)
	}
	s.Description = desc.String
	s.ThumbnailURL = thumbURL.String
	s.ThumbnailPath = thumbPath.String
	s.Category = category.String
	s.Rating = rating.String
	s.Status = status.String
	if lastScanned.Valid {
		s.LastScannedAt = lastScanned.Time
	}
	return &s, nil
}
