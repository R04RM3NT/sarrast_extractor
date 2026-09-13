package metadata

import (
	"os"
	"strings"
	"testing"
)

// TestParseFullSeriesPage tests against the realistic testdata fixture: title,
// description, thumbnail, category, rating, status, and episode links are all
// extracted correctly, and episode thumbnails are deliberately ignored.
func TestParseFullSeriesPage(t *testing.T) {
	data, err := os.ReadFile("testdata/series_page.html")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	pageURL := "https://example.test/series/example-series"
	// The testdata episode links use relative /go?s= paths; the filter must
	// match those hrefs, not a domain like ouo.io.
	res, err := Parse(strings.NewReader(string(data)), pageURL, "/go")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if res.Series.Name != "Example Series" {
		t.Errorf("Name = %q, want Example Series", res.Series.Name)
	}
	if res.Series.Description != "A gripping sci-fi drama about the future." {
		t.Errorf("Description = %q", res.Series.Description)
	}

	wantThumb := "https://example.test/public/img/series/example-series/thumb.webp"
	if res.Series.ThumbnailURL != wantThumb {
		t.Errorf("ThumbnailURL = %q, want %q", res.Series.ThumbnailURL, wantThumb)
	}
	if res.Series.Rating != "8.4" {
		t.Errorf("Rating = %q, want 8.4", res.Series.Rating)
	}
	if res.Series.Status != "Ongoing" {
		t.Errorf("Status = %q, want Ongoing", res.Series.Status)
	}
	if res.Series.EpisodeCount != 3 {
		t.Errorf("EpisodeCount = %d, want 3", res.Series.EpisodeCount)
	}
	if len(res.Episodes) != 3 {
		t.Fatalf("Episodes = %d, want 3", len(res.Episodes))
	}

	// Verify episode keys, titles and numbers. The keys come from the "s"
	// param ("episode-1" etc.); they have no leading digits, so the true
	// episode number is 0 (unknown — ordering falls back to the key).
	type epCheck struct {
		Key   string
		Title string
		Num   int
	}
	wantEps := []epCheck{
		{Key: "episode-1", Title: "Episode 1", Num: 0},
		{Key: "episode-2", Title: "Episode 2", Num: 0},
		{Key: "episode-3", Title: "Episode 3", Num: 0},
	}
	for i, w := range wantEps {
		ep := res.Episodes[i]
		if ep.Key != w.Key {
			t.Errorf("ep %d Key = %q, want %q", i, ep.Key, w.Key)
		}
		if ep.Title != w.Title {
			t.Errorf("ep %d Title = %q, want %q", i, ep.Title, w.Title)
		}
		if ep.Number != w.Num {
			t.Errorf("ep %d Number = %d, want %d", i, ep.Number, w.Num)
		}
		// Link and SourceHref are the full resolved "s" value (the fixture's
		// relative /go?...&s=episode-N resolves against the page URL).
		wantLink := "https://example.test/series/" + w.Key
		if ep.Link != wantLink {
			t.Errorf("ep %d Link = %q, want %q", i, ep.Link, wantLink)
		}
		if ep.SourceHref != wantLink {
			t.Errorf("ep %d SourceHref = %q, want %q", i, ep.SourceHref, wantLink)
		}
	}
}

// TestMainThumbnailAccepted verifies /public/img/series/<slug>/thumb.webp is
// accepted as the main poster.
func TestMainThumbnailAccepted(t *testing.T) {
	slug, ok := mainThumbnail("/public/img/series/my-series/thumb.webp")
	if !ok || slug != "my-series" {
		t.Fatalf("mainThumbnail(%q) = (%q, %v), want (my-series, true)", "/public/img/series/my-series/thumb.webp", slug, ok)
	}
}

// TestEpisodeThumbnailRejected verifies that an episode-level thumbnail
// (extra path segment) is not treated as the main poster.
func TestEpisodeThumbnailRejected(t *testing.T) {
	_, ok := mainThumbnail("/public/img/series/my-series/episode-1/thumb.webp")
	if ok {
		t.Fatal("episode thumbnail should not match mainThumbnail")
	}
}

// TestOnlyCollectsEpisodesInsideContainer verifies that only <a> links inside
// the target container div are collected, and links elsewhere on the page are
// ignored.
func TestOnlyCollectsEpisodesInsideContainer(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	html := `<html><body>
<a href="https://ouo.io/go?s=outside">outside</a>` + container + `
<a href="https://ouo.io/go?s=one">Episode 1</a>
<a href="https://ouo.io/go?s=two">Episode 2</a>
</div>
<a href="https://ouo.io/go?s=after">after</a>
</body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Outside links are ignored: only the two inside the container count.
	wantKeys := []string{"one", "two"}
	if len(res.Episodes) != len(wantKeys) {
		t.Fatalf("Episodes = %d, want %d (%v)", len(res.Episodes), len(wantKeys), wantKeys)
	}
	for i, ep := range res.Episodes {
		if ep.Key != wantKeys[i] {
			t.Errorf("episode %d key = %q, want %q", i, ep.Key, wantKeys[i])
		}
	}
	if res.Series.EpisodeCount != len(wantKeys) {
		t.Errorf("EpisodeCount = %d, want %d", res.Series.EpisodeCount, len(wantKeys))
	}
}

// TestDuplicateEpisodesRemoved checks that two <a> tags with the same "s"
// parameter produce only one episode in the result.
func TestDuplicateEpisodesRemoved(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	html := `<html><body>` + container + `
<a href="https://ouo.io/go?s=dup-one">first</a>
<a href="https://ouo.io/go?s=dup-one">first again</a>
<a href="https://ouo.io/go?s=unique">second</a>
</div></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Episodes) != 2 {
		t.Fatalf("Episodes = %d, want 2 (duplicate removed)", len(res.Episodes))
	}
}

// TestGenreCategoryExtraction checks a category is picked up from a <p> whose
// class includes "genre" (the class carries the hint, the text carries the value).
func TestGenreCategoryExtraction(t *testing.T) {
	html := `<html><body>
<h1>Series</h1>
<p class="genre text-sm text-gray-400">Genre: Science Fiction</p>
</body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Series.Category != "Science Fiction" {
		t.Errorf("Category = %q, want Science Fiction", res.Series.Category)
	}
}

// TestMetaDescriptionFallback checks that when no <p class="description"> is
// present, the <meta name="description"> tag is used as the description.
func TestMetaDescriptionFallback(t *testing.T) {
	html := `<html><head>
<meta name="description" content="from the meta tag">
</head><body>
<h1>Series</h1>
</body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Series.Description != "from the meta tag" {
		t.Errorf("Description = %q, want 'from the meta tag'", res.Series.Description)
	}
}

// TestParagraphDescriptionBlocks checks that when both <p class="...description...">
// and <meta name="description"> exist, the first encountered (the <p>) wins.
func TestParagraphDescriptionBlocks(t *testing.T) {
	// In the walking order, <head> is visited before <body>, so the meta tag
	// is encountered first and takes precedence when the <p> is not marked
	// with a description class. Verify this invariant.
	html := `<html><head>
<meta name="description" content="meta-wins">
</head><body>
<h1>S</h1>
<p class="text-gray-300">from p</p>
</body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Series.Description != "meta-wins" {
		t.Errorf("Description = %q, want meta-wins", res.Series.Description)
	}
}

// TestEmptyPage parses a page with no episode links and gets an empty list
// (the guarded check lives in the scanner, not the parser).
func TestEmptyPage(t *testing.T) {
	html := `<html><body><h1>No episodes</h1></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Episodes) != 0 {
		t.Errorf("Episodes = %d, want 0", len(res.Episodes))
	}
	if res.Series.Name != "No episodes" {
		t.Errorf("Name = %q, want 'No episodes'", res.Series.Name)
	}
}

// TestNoSeriesTitle returns "unknown" when no <h1> is present, and
// the episode list is still correctly collected.
func TestNoSeriesTitle(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	html := `<html><body>` + container + `
<a href="https://ouo.io/go?s=ep-1">one</a>
</div></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Series.Name != "" {
		t.Errorf("Name = %q, want empty", res.Series.Name)
	}
	if len(res.Episodes) != 1 {
		t.Errorf("Episodes = %d, want 1", len(res.Episodes))
	}
}

// TestEpisodeNumberFromSegment verifies the true episode number derivation from
// URL path segments.
func TestEpisodeNumberFromSegment(t *testing.T) {
	cases := []struct {
		seg  string
		want int
	}{
		{"12-thor", 12},
		{"3-something", 3},
		{"48", 48},
		{"thor", 0},
		{"", 0},
		{"abc-9", 9}, // trailing digits ({random}-{ep}) are parsed
		{"01-pilot", 1},
	}
	for _, tc := range cases {
		if got := episodeNumberFromSegment(tc.seg); got != tc.want {
			t.Errorf("episodeNumberFromSegment(%q) = %d, want %d", tc.seg, got, tc.want)
		}
	}
}

// TestEpisodeNumberTrailingDigit verifies the trailing-digits ({random}-{ep})
// shape parses the number from the end.
func TestEpisodeNumberTrailingDigit(t *testing.T) {
	cases := []struct {
		seg  string
		want int
	}{
		{"pink-pussy-121", 121},
		{"pink-pussy-121", 121},
		{"foo-3", 3},
		{"some-ep-7", 7},   // {random}-ep form
		{"episode-9", 0},   // plain episode slug, not trailing (prefix form)
		{"12-thor", 12},    // leading still works
		{"episode-004-x", 4}, // padded prefix
		{"abc", 0},
		{"", 0},
	}
	for _, tc := range cases {
		if got := episodeNumberFromSegment(tc.seg); got != tc.want {
			t.Errorf("episodeNumberFromSegment(%q) = %d, want %d", tc.seg, got, tc.want)
		}
	}
}

// TestEpisodeSegmentShortenerKey preserves the historical identity behavior for
// shortener URLs whose s value is a bare slug ("12-thor") or a page-relative
// path.
func TestEpisodeSegmentShortenerKey(t *testing.T) {
	cases := []struct {
		rawURL string
		want   string
	}{
		{"https://ouo.io/go?s=12-thor", "12-thor"},
		{"https://example.test/go?s=episode-1", "episode-1"},
		{"https://sarrast.com/series/x/12-thor", "12-thor"},
		{"https://sarrast.com/go?v=abc&s=https://sarrast.com/series/x/12-twelve", "12-twelve"},
	}
	for _, tc := range cases {
		if got, ok := episodeSegment(tc.rawURL); !ok || got != tc.want {
			t.Errorf("episodeSegment(%q) = (%q, %v), want %q", tc.rawURL, got, ok, tc.want)
		}
	}
}

// TestEpisodeSegmentNonceKey verifies that an absolute-URL s value with a
// trailing random nonce yields the meaningful episode identity, not the nonce.
func TestEpisodeSegmentNonceKey(t *testing.T) {
	raw := "https://ouo.io/q?x=1&s=https://sarrast.com/series/secret-class/episode-004-CtlLM"
	got, ok := episodeSegment(raw)
	if !ok || got != "episode-004" {
		t.Fatalf("episodeSegment(%q) = (%q, %v), want episode-004", raw, got, ok)
	}
}

// TestParseRandomDashEpLink covers the {random}-{ep} shape: an s value whose
// final segment ends in the episode number (e.g. pink-pussy-121). The key must
// be the full segment and the number derived from the trailing digits.
func TestParseRandomDashEpLink(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	html := `<html><body>` + container + `
<a href="https://ouo.io/XY?s=https://sarrast.com/series/secret-class/pink-pussy-121">E121</a>
<a href="https://ouo.io/AB?s=https://sarrast.com/series/secret-class/some-ep-7">E7</a>
</div></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Episodes) != 2 {
		t.Fatalf("Episodes = %d, want 2", len(res.Episodes))
	}
	first := res.Episodes[0]
	if first.Key != "pink-pussy-121" {
		t.Errorf("Key = %q, want pink-pussy-121", first.Key)
	}
	if first.Number != 121 {
		t.Errorf("Number = %d, want 121", first.Number)
	}
	// Link/SourceHref are the full resolved URL.
	want := "https://sarrast.com/series/secret-class/pink-pussy-121"
	if first.Link != want || first.SourceHref != want {
		t.Errorf("Link=%q SourceHref=%q, want %q", first.Link, first.SourceHref, want)
	}
	if first.Title != "episode 121" {
		t.Errorf("Title = %q, want episode 121", first.Title)
	}
}

// TestParseSeriesNamePrefersFontBlackHeading verifies the series name is taken
// from the h1 that carries the "font-black" signal (the real title, e.g.
// "خواهران ناتنی"), not an earlier site-brand h1 (e.g. "سرراست").
func TestParseSeriesNamePrefersFontBlackHeading(t *testing.T) {
	html := `<html><head>
	<title>سرراست</title>
	</head><body>
	<h1 class="text-lg font-bold">سرراست</h1>
	<span class="series-meta">...</span>
	<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">
	<h1 class="text-xl font-black mt-2">خواهران ناتنی</h1>
	<a href="https://ouo.io/XY?s=https://sarrast.com/series/secret-class/pink-pussy-121">x</a>
	</div>
	</body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Series.Name != "خواهران ناتنی" {
		t.Errorf("Name = %q, want خواهران ناتنی", res.Series.Name)
	}
}

// TestParseNonceLinks records the user-reported link shape: an absolute-URL s
// value whose final path segment is a padded number plus random nonce
// (https://sarrast.com/series/secret-class/episode-004-CtlLM). The key and
// number must come from "episode-004" (the nonce "CtlLM" is ignored), while
// Link and SourceHref stay the full resolved URL.
func TestParseNonceLinks(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	html := `<html><body>` + container + `
<a href="https://ouo.io/GH12?s=https://sarrast.com/series/secret-class/episode-004-CtlLM">E4</a>
<a href="https://ouo.io/AB34?s=https://sarrast.com/series/secret-class/episode-010-aBcDe">E10</a>
</div></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(res.Episodes) != 2 {
		t.Fatalf("Episodes = %d, want 2; got %v", len(res.Episodes), keysOf(res.Episodes))
	}

	first := res.Episodes[0]
	if first.Key != "episode-004" {
		t.Errorf("first Key = %q, want episode-004", first.Key)
	}
	if first.Number != 4 {
		t.Errorf("first Number = %d, want 4", first.Number)
	}
	wantLink := "https://sarrast.com/series/secret-class/episode-004-CtlLM"
	if first.Link != wantLink {
		t.Errorf("first Link = %q, want %q", first.Link, wantLink)
	}
	if first.SourceHref != wantLink {
		t.Errorf("first SourceHref = %q, want %q", first.SourceHref, wantLink)
	}
	if first.Title != "episode 4" {
		t.Errorf("first Title = %q, want %q", first.Title, "episode 4")
	}

	second := res.Episodes[1]
	if second.Key != "episode-010" {
		t.Errorf("second Key = %q, want episode-010", second.Key)
	}
	if second.Number != 10 {
		t.Errorf("second Number = %d, want 10", second.Number)
	}
}

func keysOf(eps []Episode) []string {
	out := make([]string, len(eps))
	for i, e := range eps {
		out[i] = e.Key
	}
	return out
}

// TestParseDerivesNumbersFromSegment verifies that when a page lists episode
// links with leading-digit segments (including non-contiguous numbers and a
// missing-number segment), Parse records the true number from the URL segment
// regardless of scan order.
func TestParseDerivesNumbersFromSegment(t *testing.T) {
	const container = `<div class=" text-white mb-20 mt-8 relative px-4" x-data="{showEpisode:false}">`
	// Realistic ouo.io pattern: the s query value is a full episode URL. The
	// scan links are deliberately out of order (48, 1, 12) — scanning order
	// must not matter.
	html := `<html><body>` + container + `
<a href="https://ouo.io/qs/ABC123?s=https://sarrast.com/series/x/48-last">Latest</a>
<a href="https://ouo.io/qs/XYZ789?s=https://sarrast.com/series/x/1-first">First</a>
<a href="https://ouo.io/qs/DEF456?s=https://sarrast.com/series/x/12-twelve">Twelve</a>
<a href="https://ouo.io/qs/QWE123?s=numeric">Unknown</a>
</div></body></html>`
	res, err := Parse(strings.NewReader(html), "https://example.test/page", "ouo.io")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	wantNums := []int{48, 1, 12, 0} // in document order; scanning order is preserved
	if len(res.Episodes) != len(wantNums) {
		t.Fatalf("Episodes = %d, want %d", len(res.Episodes), len(wantNums))
	}
	for i, ep := range res.Episodes {
		if ep.Number != wantNums[i] {
			t.Errorf("episode %d (%s) Number = %d, want %d", i, ep.Key, ep.Number, wantNums[i])
		}
		// Key is the segment; Link/SourceHref is the full s URL.
		wantLink := map[int]string{
			48: "https://sarrast.com/series/x/48-last",
			1:  "https://sarrast.com/series/x/1-first",
			12: "https://sarrast.com/series/x/12-twelve",
			0:  "https://example.test/numeric", // relative s resolved against page
		}[wantNums[i]]
		if ep.Link != wantLink {
			t.Errorf("episode %d (%s) Link = %q, want %q", i, ep.Key, ep.Link, wantLink)
		}
		if ep.SourceHref != wantLink {
			t.Errorf("episode %d (%s) SourceHref = %q, want %q", i, ep.Key, ep.SourceHref, wantLink)
		}
		// Title for unknown-number episodes falls back to the anchor text.
		wantTitle := "Unknown"
		if ep.Number > 0 {
			wantTitle = map[int]string{48: "episode 48", 1: "episode 1", 12: "episode 12"}[ep.Number]
		}
		if ep.Title != wantTitle {
			t.Errorf("episode %d (%s) Title = %q, want %q", i, ep.Key, ep.Title, wantTitle)
		}
	}
}
