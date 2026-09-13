package database

import "time"

// Series is one row of the series table, with all stored metadata.
type Series struct {
	ID            int64
	Slug          string
	Name          string
	SourceURL     string
	Description   string
	ThumbnailURL  string
	ThumbnailPath string
	Category      string
	Rating        string
	Status        string
	EpisodeCount  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	LastScannedAt time.Time

	// episodes is populated by getSeries. Episodes() exposes it read-only.
	episodes []*Episode
}

// Episodes returns the full stored episode list for this series. It is only
// non-empty when the series was loaded through a method that preloads episodes
// (for example the scan result or oneSeries queries).
func (s *Series) Episodes() []*Episode { return s.episodes }

func (s *Series) setEpisodes(episodes []*Episode) { s.episodes = episodes }

// Episode is one row of the episodes table. EpisodeKey is the stable identity
// of a single episode inside a series and prevents duplicates across scans.
type Episode struct {
	ID          int64
	SeriesID    int64
	EpisodeKey  string
	Number      int
	Title       string
	Link        string
	SourceHref  string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

// ScanResult is the result of one incremental scan of a single series.
type ScanResult struct {
	Series      *Series
	NewEpisodes []*Episode
}
