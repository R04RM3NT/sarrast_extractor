package scanner

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sarrast/internal/database"
)

func TestDeriveSeriesName(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://example.test/series/my-best-show/12-finale", "My Best Show"},
		{"https://example.test/series/another_show", "Another Show"},
		{"https://example.test/other/page", "example.test"},
	}
	for _, tc := range cases {
		if got := database.DeriveSeriesName(tc.url); got != tc.want {
			t.Errorf("DeriveSeriesName(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestScanMergesOnlyNewEpisodesAndReturnsCompleteList(t *testing.T) {
	// Run from a temporary working directory so the centralized database and
	// the thumbnail cache are written there, never into the source tree.
	t.Chdir(t.TempDir())

	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	page := `<html><head>
<meta name="description" content="demo series">
</head><body>
<h1>Demo</h1>
<img src="/public/img/series/demo/thumb.webp">
` + container + `
<a href="https://ouo.io/go?s=1-one">one</a>
<a href="https://ouo.io/go?s=2-two">two</a>
</div>
</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()

	db, err := database.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	first, err := Scan(db, server.URL+"/series/demo", "", "", "ouo.io", time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.NewEpisodes) != 2 || len(first.Series.Episodes()) != 2 {
		t.Fatalf("first scan = new %d, total %d", len(first.NewEpisodes), len(first.Series.Episodes()))
	}

	page = `<html><head>
<meta name="description" content="demo series">
</head><body>
<h1>Demo</h1>
<img src="/public/img/series/demo/thumb.webp">
` + container + `
<a href="https://ouo.io/go?s=1-one">one</a>
<a href="https://ouo.io/go?s=2-two">two</a>
<a href="https://ouo.io/go?s=3-three">three</a>
</div>
</body></html>`
	second, err := Scan(db, server.URL+"/series/demo", "", "", "ouo.io", time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.NewEpisodes) != 1 || second.NewEpisodes[0].EpisodeKey != "3-three" {
		t.Fatalf("second new episodes = %#v", second.NewEpisodes)
	}
	if len(second.Series.Episodes()) != 3 {
		t.Fatalf("second total episodes = %d, want 3", len(second.Series.Episodes()))
	}

	got, err := db.GetSeries(first.Series.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Episodes()) != 3 {
		t.Fatalf("stored episodes = %d, want 3", len(got.Episodes()))
	}
	// True episode numbers from the URL segments must be stored: 3, 2, 1
	// newest-first.
	eps := got.Episodes()
	for i, wantNum := range []int{3, 2, 1} {
		if eps[i].Number != wantNum {
			t.Errorf("stored episode %d number = %d, want %d", i, eps[i].Number, wantNum)
		}
	}
}

// TestScanWithNoEpisodesStillRecordsSeries is a regression test: a page that
// yields zero episodes must still persist the series row (with 0 episodes)
// rather than aborting. This is what previously left data/series.db empty: the
// scanner cached the thumbnail, hit "no episodes", and never wrote the DB.
func TestScanWithNoEpisodesStillRecordsSeries(t *testing.T) {
	t.Chdir(t.TempDir())

	page := `<html><head>
<meta name="description" content="empty demo">
</head><body>
<h1>Empty Demo</h1>
<img src="/public/img/series/empty-demo/thumb.webp">
</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()

	db, err := database.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := Scan(db, server.URL+"/series/empty-demo", "", "", "ouo.io", time.Second, nil)
	if err != nil {
		t.Fatalf("Scan with no episodes should succeed (series recorded): %v", err)
	}
	if len(result.NewEpisodes) != 0 {
		t.Fatalf("expected 0 new episodes, got %d", len(result.NewEpisodes))
	}

	// The series must exist in the DB.
	got, err := db.GetSeries(result.Series.Slug)
	if err != nil {
		t.Fatalf("series was not recorded for an empty scan: %v", err)
	}
	if got.Name != "Empty Demo" {
		t.Errorf("series name = %q, want Empty Demo", got.Name)
	}
	if len(got.Episodes()) != 0 {
		t.Errorf("stored episodes = %d, want 0", len(got.Episodes()))
	}
}

// TestScanCachesThumbnailAfterMerge guards the ordering requirement that a
// thumbnail is only cached once the series exists in the database. If caching
// ran first for a page that is later rejected, a thumb could exist with no row.
func TestScanCachesThumbnailAfterMerge(t *testing.T) {
	t.Chdir(t.TempDir())

	cacheHits := 0
	page := `<html><body>
<h1>Thumb Series</h1>
<img src="/public/img/series/thumb-series/thumb.webp">
</body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/public/img/series/thumb-series/thumb.webp" {
			cacheHits++
			w.Header().Set("Content-Type", "image/webp")
			_, _ = w.Write([]byte{0x52, 0x49, 0x46, 0x46}) // "RIFF" webp prefix
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()

	db, err := database.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = Scan(db, server.URL+"/series/thumb-series", "", "", "ouo.io", time.Second, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// The thumbnail must have been requested only once (cached after merge).
	if cacheHits != 1 {
		t.Fatalf("thumbnail requests = %d, want 1 (single cache after merge)", cacheHits)
	}

	// The stored series references the cached thumbnail path.
	got, err := db.GetSeries("thumb-series")
	if err != nil {
		t.Fatalf("GetSeries: %v", err)
	}
	if got.ThumbnailPath == "" {
		t.Errorf("thumbnail_path not persisted after caching")
	}
}
