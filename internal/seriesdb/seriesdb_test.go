package seriesdb

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMergeKeepsCompleteListAndAddsOnlyNewEpisodes(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	db := New()

	first, err := db.Merge("Example Series", "https://example.test/series/example", []Episode{
		{ID: "one", Link: "https://short.test/?s=one"},
		{ID: "two", Link: "https://short.test/?s=two"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.NewEpisodes) != 2 || len(first.Series.Episodes) != 2 {
		t.Fatalf("first merge = new %d, total %d", len(first.NewEpisodes), len(first.Series.Episodes))
	}

	second, err := db.Merge("example   series", "https://example.test/series/example", []Episode{
		{ID: "two", Link: "https://short.test/?s=two"},
		{ID: "three", Link: "https://short.test/?s=three"},
	}, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.NewEpisodes) != 1 || second.NewEpisodes[0].ID != "three" {
		t.Fatalf("new episodes = %#v", second.NewEpisodes)
	}
	if len(second.Series.Episodes) != 3 {
		t.Fatalf("total episodes = %d, want 3", len(second.Series.Episodes))
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "series.json")
	db := New()
	if _, err := db.Merge("Example", "https://example.test", []Episode{{ID: "one", Link: "one"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Series["example"].Episodes) != 1 {
		t.Fatalf("loaded database = %#v", loaded)
	}
}
