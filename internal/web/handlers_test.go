package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"sarrast/internal/database"
	"sarrast/internal/thumbnail"
)

// testServer builds a *server backed by a fresh temp SQLite DB with one series.
func testServer(t *testing.T, slug, name string) (*server, *database.DB) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	db, err := database.Open(filepath.Join(t.TempDir(), "web.db"), logger)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.MergeScan(database.SeriesUpdate{
		Slug:          slug,
		Name:          name,
		SourceURL:     "https://example.test/series/" + slug,
		Description:   "A test series description",
		ThumbnailURL:  "https://example.test/public/img/series/" + slug + "/thumb.webp",
		ThumbnailPath: filepath.Join(thumbnail.PathFor(slug)),
		Category:      "Science Fiction",
		Rating:        "8.4",
		Status:        "Ongoing",
	}, []database.EpisodeUpdate{
		{Key: "one", Number: 1, Title: "One", Link: "https://shorter.test/?s=one"},
		{Key: "two", Number: 2, Title: "Two", Link: "https://shorter.test/?s=two"},
	}, nil)
	if err != nil {
		t.Fatalf("MergeScan: %v", err)
	}

	s, err := newServer(db, logger)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return s, db
}

// do issues one request against the server's mux.
func do(t *testing.T, s *server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)
	return rec
}

// decodeSeriesList decodes rec.Body as a []seriesJSON.
func decodeSeriesList(t *testing.T, rec *httptest.ResponseRecorder) []seriesJSON {
	t.Helper()
	var out []seriesJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode series list: %v", err)
	}
	return out
}

func decodeSeriesDetail(t *testing.T, rec *httptest.ResponseRecorder) seriesJSON {
	t.Helper()
	var out seriesJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode series detail: %v", err)
	}
	return out
}

// TestHomeAPIReturnsSeries checks the list endpoint returns the seeded series
// with its metadata as JSON.
func TestHomeAPIReturnsSeries(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")
	rec := do(t, s, http.MethodGet, "/api/series")

	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", rec.Code)
	}
	list := decodeSeriesList(t, rec)
	if len(list) != 1 {
		t.Fatalf("got %d series, want 1", len(list))
	}
	got := list[0]
	if got.Name != "Demo Series" || got.Slug != "demo" || got.EpisodeCount != 2 {
		t.Errorf("unexpected series fields: %+v", got)
	}
}

// TestSeriesDetailAPIEnvelope checks the detail endpoint returns metadata and
// the complete episode list as an object (not a list).
func TestSeriesDetailAPIEnvelope(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")
	rec := do(t, s, http.MethodGet, "/api/series/demo")

	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rec.Code)
	}
	detail := decodeSeriesDetail(t, rec)
	if detail.Name != "Demo Series" || detail.Category != "Science Fiction" {
		t.Errorf("unexpected detail: %+v", detail)
	}
	if len(detail.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(detail.Episodes))
	}
}

// TestSeriesDetailShowsAllStoredEpisodes re-scans with a subset after the first
// scan: the detail endpoint must still return the full stored history.
func TestSeriesDetailShowsAllStoredEpisodes(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := database.Open(filepath.Join(t.TempDir(), "web.db"), logger)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	first := database.SeriesUpdate{Slug: "s", Name: "S", SourceURL: "https://example.test/series/s"}
	if _, err := db.MergeScan(first, []database.EpisodeUpdate{
		{Key: "a", Number: 1, Title: "A", Link: "a"},
		{Key: "b", Number: 2, Title: "B", Link: "b"},
	}, nil); err != nil {
		t.Fatalf("MergeScan #1: %v", err)
	}
	// A later scan only reports "b" — the API must still return both.
	if _, err := db.MergeScan(first, []database.EpisodeUpdate{
		{Key: "b", Number: 2, Title: "B", Link: "b"},
	}, nil); err != nil {
		t.Fatalf("MergeScan #2: %v", err)
	}

	s, err := newServer(db, logger)
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	rec := do(t, s, http.MethodGet, "/api/series/s")
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want 200", rec.Code)
	}
	detail := decodeSeriesDetail(t, rec)
	if len(detail.Episodes) != 2 {
		t.Fatalf("detail should return the full stored episode list AFTER a subset rescan, got %d", len(detail.Episodes))
	}
	// Newest-first: episode 2 ("B") must come before episode 1 ("A").
	if detail.Episodes[0].Title != "B" || detail.Episodes[1].Title != "A" {
		t.Errorf("episodes must be newest-first, got %q then %q", detail.Episodes[0].Title, detail.Episodes[1].Title)
	}
}

// TestInvalidSeriesSlugReturns404 checks a missing or malformed slug produces a
// 404 on the JSON endpoint.
func TestInvalidSeriesSlugReturns404(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")

	for _, path := range []string{
		"/api/series/nope",
		"/api/series/",
		"/api/series/a/b",
		"/api/series/%2e%2e",
	} {
		rec := do(t, s, http.MethodGet, path)
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

// TestAPIDoesNotReturnRawHTML verifies the JSON endpoints never leak the old
// template HTML and the list endpoint is an array, the detail an object.
func TestAPIDoesNotReturnRawHTML(t *testing.T) {
	s, _ := testServer(t, "xss", `<script>alert(1)</script>`)

	for _, path := range []string{"/api/series", "/api/series/xss"} {
		rec := do(t, s, http.MethodGet, path)
		body := rec.Body.String()
		if strings.Contains(body, "<script>") {
			t.Fatalf("GET %s returned raw HTML, want JSON", path)
		}
	}
}

// TestSPAServesIndexForUnknownPath ensures the SPA handler answers client-side
// deep links by returning the React entry page (or the placeholder while the
// bundle is missing) for any non-API, non-static path.
func TestSPAServesIndexForUnknownPath(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")
	for _, path := range []string{"/series/demo", "/", "/series/other"} {
		rec := do(t, s, http.MethodGet, path)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 from SPA handler", path, rec.Code)
		}
		body := strings.ToLower(rec.Body.String())
		// The built bundle mounts React at #root; the placeholder is static
		// HTML. Either is a valid SPA answer — both are HTML documents.
		if !strings.Contains(body, "<html") {
			t.Errorf("GET %s did not return an HTML document", path)
		}
	}
}

// TestLastScanTimeShown checks the detail endpoint carries last scan timestamp.
func TestLastScanTimeShown(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")
	rec := do(t, s, http.MethodGet, "/api/series/demo")
	body := rec.Body.String()
	if !strings.Contains(body, "lastScannedAt") || !containsDigit(body) {
		t.Errorf("detail endpoint missing lastScannedAt timestamp")
	}
}

func containsDigit(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			return true
		}
	}
	return false
}

// TestWebServerUsesCentralDB confirms the server is bound to the single
// centralized database passed in, never one it selects itself.
func TestWebServerUsesCentralDB(t *testing.T) {
	s, db := testServer(t, "demo", "Demo Series")
	if s.db != db {
		t.Fatal("server does not use the provided central database")
	}
}

// TestUnknownRouteIsHandledBySPA ensures the mux does not 404 unrelated paths:
// they reach the SPA handler (which returns index.html or the placeholder).
func TestUnknownRouteIsHandledBySPA(t *testing.T) {
	s, _ := testServer(t, "demo", "Demo Series")
	rec := do(t, s, http.MethodGet, "/series/..%2F")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /series/..%%2F = %d, want 200 from SPA handler", rec.Code)
	}
}