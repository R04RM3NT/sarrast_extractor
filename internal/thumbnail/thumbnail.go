// Package thumbnail downloads and caches the main series poster locally. It is
// deliberately narrow: only the main series thumbnail is ever fetched, never
// episode thumbnails, and caching never asks the user where to store files.
package thumbnail

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"sarrast/internal/database"
)

// cacheDir is a test seam: production always uses the fixed internal
// database.ThumbnailsDir; tests redirect it to a temporary directory so they
// never write into the working tree. It is not reachable from the CLI, TUI or
// web server.
var cacheDir = database.ThumbnailsDir

// PathFor is the fixed on-disk location for a series' cached poster.
func PathFor(slug string) string {
	return filepath.Join(cacheDir, slug, "thumb.webp")
}

// Exists reports whether the cached poster is already present on disk.
func Exists(slug string) bool {
	_, err := os.Stat(PathFor(slug))
	return err == nil
}

// Cache downloads the poster at thumbnailURL and stores it under
// data/thumbnails/<slug>/thumb.webp. It never redownloads an existing file and
// returns the stored path (or the previously stored one when already cached).
func Cache(client *http.Client, slug, thumbnailURL string) (string, error) {
	target := PathFor(slug)

	if Exists(slug) {
		return target, nil
	}
	if strings.TrimSpace(thumbnailURL) == "" {
		return "", nil
	}

	resp, err := client.Get(thumbnailURL)
	if err != nil {
		return "", fmt.Errorf("fetch thumbnail: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("thumbnail fetch returned %s", resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail directory: %w", err)
	}

	out, err := os.Create(filepath.Clean(target))
	if err != nil {
		return "", fmt.Errorf("create thumbnail file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(target)
		return "", fmt.Errorf("write thumbnail: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close thumbnail: %w", err)
	}

	return target, nil
}
