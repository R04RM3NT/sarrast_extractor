package web

import "time"

// seriesJSON is the JSON shape of a series returned by the API. Episodes is
// populated only by the detail endpoint.
type seriesJSON struct {
	ID            int64         `json:"id"`
	Slug          string        `json:"slug"`
	Name          string        `json:"name"`
	SourceURL     string        `json:"sourceUrl"`
	Description   string        `json:"description"`
	ThumbnailURL  string        `json:"thumbnailUrl"`
	ThumbnailPath string        `json:"thumbnailPath"`
	Category      string        `json:"category"`
	Rating        string        `json:"rating"`
	Status        string        `json:"status"`
	EpisodeCount  int           `json:"episodeCount"`
	Episodes      []episodeJSON `json:"episodes,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	LastScannedAt time.Time     `json:"lastScannedAt"`
}

// episodeJSON is the JSON shape of a single episode.
type episodeJSON struct {
	ID          int64     `json:"id"`
	SeriesID    int64     `json:"seriesId"`
	Key         string    `json:"key"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	SourceHref  string    `json:"sourceHref"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}