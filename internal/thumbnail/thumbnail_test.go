package thumbnail

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"sarrast/internal/database"
)

// useTempCache redirects the cache directory to a temp dir for the test.
func useTempCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := cacheDir
	cacheDir = dir
	t.Cleanup(func() { cacheDir = old })
	return dir
}

// newImageServer serves a tiny stable PNG at every path.
func newImageServer(t *testing.T) *httptest.Server {
	t.Helper()
	img := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(img)
	}))
}

// TestCacheDownloadsOnce downloads a poster then re-caches it without a second
// HTTP round trip (the "never redownload" requirement).
func TestCacheDownloadsOnce(t *testing.T) {
	dir := useTempCache(t)

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
	}))
	defer server.Close()

	client := server.Client()

	p1, err := Cache(client, "demo", server.URL+"/thumb.webp")
	if err != nil {
		t.Fatalf("first Cache: %v", err)
	}
	if p1 == "" {
		t.Fatal("first Cache returned empty path")
	}
	if calls != 1 {
		t.Fatalf("downloads after first cache = %d, want 1", calls)
	}

	// Second call must hit the cache, not the network.
	p2, err := Cache(client, "demo", server.URL+"/thumb.webp")
	if err != nil {
		t.Fatalf("second Cache: %v", err)
	}
	if p2 != p1 {
		t.Fatalf("cached paths differ: %q != %q", p1, p2)
	}
	if calls != 1 {
		t.Fatalf("downloads after second cache = %d, want 1 (no redownload)", calls)
	}

	// The file exists at the expected fixed layout under the cache dir.
	if want := filepath.Join(dir, "demo", "thumb.webp"); p1 != want {
		t.Fatalf("cached path = %q, want %q", p1, want)
	}
}

// TestCacheSkipsEmptyURL ensures an empty thumbnail URL does not error and
// returns an empty path.
func TestCacheSkipsEmptyURL(t *testing.T) {
	useTempCache(t)
	client := &http.Client{}
	p, err := Cache(client, "demo", "")
	if err != nil {
		t.Fatalf("Cache with empty URL: %v", err)
	}
	if p != "" {
		t.Fatalf("Cache with empty URL returned %q, want empty", p)
	}
}

// TestCacheExists checks the Exists helper and that a cached file is
// recognized as present.
func TestCacheExists(t *testing.T) {
	useTempCache(t)
	server := newImageServer(t)
	defer server.Close()

	if Exists("demo") {
		t.Fatal("Exists(demo) = true before caching")
	}
	if _, err := Cache(server.Client(), "demo", server.URL+"/t.png"); err != nil {
		t.Fatalf("Cache: %v", err)
	}
	if !Exists("demo") {
		t.Fatal("Exists(demo) = false after caching")
	}
}

// TestCacheFixedLayout verifies the fixed internal directory is used
// (data/thumbnails/<slug>/thumb.webp) and is not user-configurable.
func TestCacheFixedLayout(t *testing.T) {
	// The production default is database.ThumbnailsDir; the seam only applies
	// in tests. Assert PathFor uses the configured cacheDir.
	if got := filepath.ToSlash(PathFor("alpha")); !gotInDefaultLayout(got) {
		t.Fatalf("default layout = %q", got)
	}
}

func gotInDefaultLayout(p string) bool {
	return p == filepath.ToSlash(filepath.Join(database.ThumbnailsDir, "alpha", "thumb.webp"))
}

// TestCacheDeletesPartialOnError verifies a failed body write removes the
// partially written file rather than leaving a corrupt one behind.
func TestCacheDeletesPartialOnError(t *testing.T) {
	dir := useTempCache(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Report 200 but then abort the body mid-stream.
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "1000")
		_, _ = w.Write([]byte{0x01, 0x02, 0x03}) // short write
	}))
	defer server.Close()

	_, err := Cache(server.Client(), "partial", server.URL+"/t.png")
	if err == nil {
		t.Fatal("Cache over short body should fail")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "partial", "thumb.webp")); !os.IsNotExist(statErr) {
		t.Fatalf("partial file should have been removed, stat err = %v", statErr)
	}
}
