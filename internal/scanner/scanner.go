// Package scanner coordinates fetching a series page, parsing its metadata and
// episode links, caching the main thumbnail, and merging everything into the
// centralized SQLite database.
package scanner

import (
	"fmt"
	"strings"
	"time"

	"sarrast/internal/database"
	"sarrast/internal/extractor"
	"sarrast/internal/metadata"
	"sarrast/internal/thumbnail"
)

// ProgressFunc receives human-readable progress messages during a scan.
type ProgressFunc func(string)

// Result describes the effect of one scan. It carries the series (including
// the complete stored episode list) and the newly discovered episodes.
type Result struct {
	Series      *database.Series
	NewEpisodes []*database.Episode
}

// Scan fetches pageURL, parses it, and incrementally merges the result into the
// centralized SQLite database. Episodes are never duplicated; every stored
// episode for the series is included in the returned Result.
func Scan(db *database.DB, pageURL, seriesName, proxyURL, filterHost string, timeout time.Duration, progress ProgressFunc) (Result, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return Result{}, fmt.Errorf("page URL is empty")
	}
	if strings.TrimSpace(filterHost) == "" {
		return Result{}, fmt.Errorf("link filter is empty")
	}
	if db == nil {
		return Result{}, fmt.Errorf("database is nil")
	}

	if progress != nil {
		progress("fetching page …")
	}
	client, err := extractor.NewClient(proxyURL, timeout)
	if err != nil {
		return Result{}, err
	}
	body, err := client.FetchString(pageURL)
	if err != nil {
		return Result{}, err
	}

	if progress != nil {
		progress("parsing HTML …")
	}
	parsed, err := metadata.Parse(strings.NewReader(body), pageURL, filterHost)
	if err != nil {
		return Result{}, fmt.Errorf("parse HTML: %w", err)
	}

	if seriesName = strings.TrimSpace(seriesName); seriesName == "" {
		seriesName = parsed.Series.Name
	}
	if seriesName == "" {
		seriesName = database.DeriveSeriesName(pageURL)
	}

	slug := database.SlugFromName(seriesName)
	if parsed.Series.Name != "" {
		if urlSlug := database.SlugFromURL(pageURL); urlSlug != "" && urlSlug != "unknown" {
			slug = urlSlug
		}
	}

	if progress != nil && len(parsed.Episodes) == 0 {
		progress(fmt.Sprintf("no episodes matched filter %q; recording series only", filterHost))
	}

	episodes := make([]database.EpisodeUpdate, 0, len(parsed.Episodes))
	for _, ep := range parsed.Episodes {
		episodes = append(episodes, database.EpisodeUpdate{
			Key:        ep.Key,
			Number:     ep.Number,
			Title:      ep.Title,
			Link:       ep.Link,
			SourceHref: ep.SourceHref,
		})
	}

	// Merge into the database first: the series row and episode list must be
	// committed before any thumbnail is cached. This guarantees a thumbnail is
	// never left behind for a series that does not exist in SQLite.
	if progress != nil {
		progress("merging into database …")
	}
	update := database.SeriesUpdate{
		Slug:         slug,
		Name:         seriesName,
		SourceURL:    pageURL,
		Description:  parsed.Series.Description,
		ThumbnailURL: parsed.Series.ThumbnailURL,
		Category:     parsed.Series.Category,
		Rating:       parsed.Series.Rating,
		Status:       parsed.Series.Status,
	}
	result, err := db.MergeScan(update, episodes, nil)
	if err != nil {
		return Result{}, err
	}

	// Cache the main thumbnail only after the DB write succeeded, then persist
	// the local path. A thumbnail failure never loses the scan.
	if parsed.Series.ThumbnailURL != "" {
		if progress != nil {
			progress("caching thumbnail …")
		}
		thumbPath, cacheErr := thumbnail.Cache(client.Client(), slug, parsed.Series.ThumbnailURL)
		if cacheErr != nil {
			if progress != nil {
				progress("thumbnail skipped: " + cacheErr.Error())
			}
		} else if thumbPath != "" {
			if err := db.SetThumbnailPath(slug, thumbPath); err != nil {
				if progress != nil {
					progress("thumbnail path not saved: " + err.Error())
				}
			}
		}
	}

	if progress != nil {
		for _, episode := range result.NewEpisodes {
			progress("✓ new episode: " + episode.Link)
		}
		progress(fmt.Sprintf("scan complete: %d new, %d total episodes", len(result.NewEpisodes), len(result.Series.Episodes())))
	}

	return Result{
		Series:      result.Series,
		NewEpisodes: result.NewEpisodes,
	}, nil
}
