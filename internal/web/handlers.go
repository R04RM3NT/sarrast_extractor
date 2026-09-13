package web

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"sarrast/internal/database"
)

// stderr is used so Serve can report listen errors.
var stderr io.Writer = os.Stderr

// newServer builds the HTTP server. The frontend bundle is served from the
// embedded static filesystem; the JSON API and thumbnail routes work
// independently of it.
func newServer(db *database.DB, log *slog.Logger) (*server, error) {
	return &server{db: db, log: log}, nil
}

// apiSeries renders a series row as the JSON shape the frontend consumes.
func apiSeries(s *database.Series) seriesJSON {
	return seriesJSON{
		ID:            s.ID,
		Slug:          s.Slug,
		Name:          s.Name,
		SourceURL:     s.SourceURL,
		Description:   s.Description,
		ThumbnailURL:  s.ThumbnailURL,
		ThumbnailPath: s.ThumbnailPath,
		Category:      s.Category,
		Rating:        s.Rating,
		Status:        s.Status,
		EpisodeCount:  s.EpisodeCount,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
		LastScannedAt: s.LastScannedAt,
	}
}

func apiEpisode(e *database.Episode) episodeJSON {
	return episodeJSON{
		ID:          e.ID,
		SeriesID:    e.SeriesID,
		Key:         e.EpisodeKey,
		Number:      e.Number,
		Title:       e.Title,
		Link:        e.Link,
		SourceHref:  e.SourceHref,
		FirstSeenAt: e.FirstSeenAt,
		LastSeenAt:  e.LastSeenAt,
	}
}

// routes wires the URL space to the handlers.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()

	// JSON API (always present even without a built frontend).
	mux.HandleFunc("/api/series", s.handleListSeries)
	mux.HandleFunc("/api/series/", s.handleGetSeries)

	// Cached poster files on disk.
	mux.HandleFunc("/thumbnails/", s.handleThumbnail)

	// The React SPA handles everything else, including client-side routes.
	mux.HandleFunc("/", s.handleSPA)

	return mux
}

// writeJSON writes v as JSON with the given status.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleListSeries serves every series as JSON, ordered by name.
func (s *server) handleListSeries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	series, err := s.db.ListSeries()
	if err != nil {
		s.log.Error("list series", "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	out := make([]seriesJSON, 0, len(series))
	for _, item := range series {
		out = append(out, apiSeries(item))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetSeries serves one series with its full episode list as JSON.
func (s *server) handleGetSeries(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/api/series/")
	slug = strings.Trim(slug, "/")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}

	series, err := s.db.GetSeries(slug)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeErr(w, http.StatusNotFound, "series not found")
			return
		}
		s.log.Error("get series", "slug", slug, "error", err)
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	detail := apiSeries(series)
	detail.Episodes = make([]episodeJSON, 0, len(series.Episodes()))
	for _, e := range series.Episodes() {
		detail.Episodes = append(detail.Episodes, apiEpisode(e))
	}
	writeJSON(w, http.StatusOK, detail)
}

// handleThumbnail serves a cached poster from disk if it exists.
func (s *server) handleThumbnail(w http.ResponseWriter, r *http.Request) {
	slug := strings.TrimPrefix(r.URL.Path, "/thumbnails/")
	slug = strings.TrimSuffix(slug, "thumb.webp")
	slug = strings.Trim(slug, "/")
	if slug == "" || strings.Contains(slug, "/") {
		http.NotFound(w, r)
		return
	}
	file := filepath.Join(database.ThumbnailsDir, slug, "thumb.webp")
	if _, err := os.Stat(file); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, file)
}

// handleSPA serves the React bundle: direct static files, plus a catch-all that
// returns index.html so client-side routes deep-link. When the bundle has not
// been built the embedded filesystem holds nothing and a placeholder page is
// shown instead.
func (s *server) handleSPA(w http.ResponseWriter, r *http.Request) {
	assets, err := staticFS()
	if err != nil {
		http.Error(w, "frontend not built", http.StatusInternalServerError)
		return
	}

	if serveStatic(w, r, assets, r.URL.Path) {
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Deep links and the entry point both land on index.html.
	if !serveStatic(w, r, assets, "/index.html") {
		_, _ = w.Write([]byte(placeholderPage))
	}
}

// serveStatic serves Data from fsys when the (cleaned) file exists, writing the
// guessable content type. It reports whether a file was served.
func serveStatic(w http.ResponseWriter, r *http.Request, fsys fs.FS, filePath string) bool {
	name := strings.TrimPrefix(path.Clean("/"+filePath), "/")
	info, err := fs.Stat(fsys, name)
	if err != nil || info.IsDir() {
		return false
	}
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return false
	}
	w.Header().Set("Content-Type", contentTypeFor(name))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
	return true
}

// placeholderPage renders when the frontend bundle has not been built yet.
const placeholderPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sarrast Extractor</title>
  <style>
    :root { color-scheme: dark; }
    body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; background: #0b0b12; color: #e8e8f4; line-height: 1.5; display: grid; place-items: center; min-height: 100vh; }
    .card { text-align: center; max-width: 520px; padding: 48px; border: 1px solid #26263a; border-radius: 16px; background: #12121c; }
    code { background: #26263a; padding: 2px 6px; border-radius: 6px; font-size: 14px; }
    a { color: #c97bff; }
  </style>
</head>
<body>
  <div class="card">
    <h1>◆ Sarrast Extractor</h1>
    <p>The frontend bundle has not been built yet.</p>
    <p>Run <code>npm run build</code> inside <code>frontend/</code> and rebuild
    the Go binary, or use the Vite dev server with:</p>
    <p><code>cd frontend &amp;&amp; npm run dev</code></p>
    <p><a href="/api/series">API</a> and thumbnails keep working meanwhile.</p>
  </div>
</body>
</html>`

// contentTypeFor guesses the media type for a static asset from its path. It
// avoids depending on the OS registry for embedded files.
func contentTypeFor(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".map":
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

// formatTime renders a time as "2006-01-02 15:04" or "—" when zero. Kept for
// backend helpers that still present time text; the SPA formats client-side.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04")
}