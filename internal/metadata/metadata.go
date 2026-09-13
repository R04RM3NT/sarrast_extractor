// Package metadata parses a series page: it extracts the series title,
// description, main thumbnail, category, rating, status, episode count, and the
// episode links together with their titles. It deliberately ignores episode
// thumbnails — only the main series thumbnail is ever cached.
package metadata

import (
	"fmt"
	"io"
	"net/url"
	"path"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// Series holds the metadata parsed from one series page.
type Series struct {
	Name         string
	Description  string
	ThumbnailURL string
	Category     string
	Rating       string
	Status       string
	EpisodeCount int
}

// Episode is one link extracted from a series page. Key is the final path
// segment of the source URL (the stable identity across scans), and Number is
// the true episode number derived from that segment's leading digits — for
// example "12-thor" → Key "12-thor", Number 12.
type Episode struct {
	Key        string
	Title      string
	Link       string
	SourceHref string
	Number     int
}

// Result is everything Parse was able to collect from one page.
type Result struct {
	Series Series
	// Episodes contains every episode found on the page, in document order,
	// with duplicates removed.
	Episodes []Episode
}

// Parse reads the page body and extracts series metadata plus episodes. It
// needs the page URL for resolving relative thumbnail/links and the filter host
// used to recognise candidate episode links.
func Parse(r io.Reader, pageURL, filterHost string) (Result, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return Result{}, err
	}

	res := Result{}
	p := &parser{pageURL: pageURL, filterHost: filterHost}
	p.walk(doc)
	p.finish()

	res.Series = p.series
	res.Episodes = p.episodes
	return res, nil
}

type parser struct {
	pageURL    string
	filterHost string

	series Series

	// episodeContainerDepth tracks how deep we are inside the episodes
	// container. Only anchors inside it are collected.
	episodeContainerDepth int

	// raw candidate values discovered while walking; deduplicated in finish().
	episodeLinks []episodeRaw
	seenKey      map[string]struct{}
	episodes     []Episode
}

// episodeContainerClass is the distinctive class of the episodes container div.
const episodeContainerClass = "text-white mb-20 mt-8 relative px-4"

// episodeContainerXData is the distinctive x-data attribute of that div.
const episodeContainerXData = "{showEpisode:false}"

type episodeRaw struct {
	src    string // anchor href (the ouo.io URL)
	key    string // episode identity (path segment of the s value)
	link   string // the episode's real URL (the full s value)
	title  string // anchor text
	pos    int
	number int // true episode number derived from the key
	thumbs int
}

// handleHeading captures the series name from an <h1>. The real series title is
// the <h1> just above the episode list — recognisable by the "font-black" class
// (e.g. "خواهران ناتنی") — while a site-brand <h1> (e.g. "سرراست") can appear
// above it in the page header. A "font-black" h1 is a strong signal and wins;
// otherwise the most recently seen plain h1 wins (the episode-section title
// appears after the top-of-page brand).
func (p *parser) handleHeading(n *html.Node) {
	text := strings.TrimSpace(textContent(n))
	if text == "" {
		return
	}
	p.series.Name = text
}

func (p *parser) walk(n *html.Node) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "meta":
			p.handleMeta(n)
		case "h1":
			p.handleHeading(n)
		case "p":
			p.handleParagraph(n)
		case "img":
			p.handleImage(n)
		case "a":
			if p.episodeContainerDepth > 0 {
				p.handleAnchor(n)
			}
		case "span":
			p.handleSpan(n)
		case "div":
			if isEpisodeContainer(n) {
				p.episodeContainerDepth++
			}
		}
	}

	// meta description lives in <head>; walk everything.
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.walk(c)
	}

	// When the walk of this div's subtree is complete, we leave it.
	if n.Type == html.ElementNode && n.Data == "div" && isEpisodeContainer(n) && p.episodeContainerDepth > 0 {
		p.episodeContainerDepth--
	}
}

// isEpisodeContainer reports whether n is the episodes container div, matched
// on both its distinctive class and its x-data attribute so a similar-looking
// div elsewhere on the page cannot leak episode links in.
func isEpisodeContainer(n *html.Node) bool {
	// Compare the class list on a whitespace-normalized basis: the real page
	// class is " text-white mb-20 mt-8 relative px-4" (leading space).
	classes := strings.Join(strings.Fields(attr(n, "class")), " ")
	if !strings.Contains(classes, episodeContainerClass) {
		return false
	}
	return strings.Contains(strings.ReplaceAll(attr(n, "x-data"), " ", ""), episodeContainerXData)
}

// handleMeta picks up the <meta name="description"> tag when the page does not
// expose a narrative description block.
func (p *parser) handleMeta(n *html.Node) {
	if p.series.Description != "" {
		return
	}
	name := strings.ToLower(strings.TrimSpace(attr(n, "name")))
	if name != "description" {
		return
	}
	if content := strings.TrimSpace(attr(n, "content")); content != "" {
		p.series.Description = content
	}
}

// handleParagraph captures a narrative description block and a category from
// <p> elements. A description is taken from a <p> whose class carries a
// description hint; a category is taken from a <p> whose class carries a genre
// hint (stripping a leading "Genre:" label). Both run independently so one
// kind of <p> does not prevent the other from being read.
func (p *parser) handleParagraph(n *html.Node) {
	classes := strings.ToLower(attr(n, "class"))
	text := strings.TrimSpace(textContent(n))

	if p.series.Description == "" && strings.Contains(classes, "description") && text != "" {
		p.series.Description = text
	}
	if p.series.Category == "" && strings.Contains(classes, "genre") && text != "" {
		p.series.Category = strings.TrimSpace(strings.TrimPrefix(text, "Genre:"))
	}
}

// handleSpan captures rating, status and category from elements whose classes
// carry semantic hints.
func (p *parser) handleSpan(n *html.Node) {
	classes := strings.ToLower(attr(n, "class"))
	if p.series.Rating == "" && strings.Contains(classes, "rating") {
		if text := strings.TrimSpace(textContent(n)); text != "" {
			p.series.Rating = strings.TrimSpace(strings.TrimPrefix(text, "★"))
		}
	}
	if p.series.Status == "" && strings.Contains(classes, "status") {
		if text := strings.TrimSpace(textContent(n)); text != "" {
			p.series.Status = text
		}
	}
	if p.series.Category == "" && strings.Contains(classes, "genre") {
		if text := strings.TrimSpace(textContent(n)); text != "" {
			p.series.Category = strings.TrimPrefix(text, "Genre:")
		}
	}
}

// handleImage captures the main series thumbnail URL. The main poster follows
// /public/img/series/<slug>/thumb.webp; episode posters add another path
// segment and are ignored, so we only accept a three-segment match.
func (p *parser) handleImage(n *html.Node) {
	src := attr(n, "src")
	if src == "" {
		return
	}
	if _, isPoster := mainThumbnail(src); isPoster {
		if p.series.ThumbnailURL == "" {
			p.series.ThumbnailURL = abs(p.pageURL, src)
		}
	}
}

// mainThumbnail reports whether src is the main series poster
// /public/img/series/<slug>/thumb.webp.
func mainThumbnail(src string) (slug string, ok bool) {
	u, err := url.Parse(src)
	if err != nil {
		return "", false
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	// segments: public, img, series, <slug>, thumb.webp
	if len(parts) != 5 || parts[0] != "public" || parts[1] != "img" || parts[2] != "series" {
		return "", false
	}
	if path.Base(u.Path) != "thumb.webp" {
		return "", false
	}
	return parts[3], true
}

// handleAnchor collects a candidate episode link. It is only called for
// anchors inside the episodes container; a link is recognised when the href
// contains the filter host. The href is an ouo.io shortener whose "s" query
// parameter is the full episode URL
// (https://sarrast.com/series/{series_name}/{ep}-{random}). That URL becomes
// the episode's link; its final path segment (e.g. "12-thor") is the identity
// and yields the true episode number.
func (p *parser) handleAnchor(n *html.Node) {
	href := attr(n, "href")
	if !strings.Contains(href, p.filterHost) {
		return
	}

	link, ok := sParam(href)
	if !ok {
		return
	}
	link = strings.TrimSpace(link)
	if link == "" {
		return
	}
	link = abs(p.pageURL, link) // resolve any relative s value against the page

	key, ok := episodeSegment(link)
	if !ok || strings.TrimSpace(key) == "" {
		return
	}

	if p.seenKey == nil {
		p.seenKey = make(map[string]struct{})
	}
	if _, dup := p.seenKey[key]; dup {
		return
	}
	p.seenKey[key] = struct{}{}

	p.episodeLinks = append(p.episodeLinks, episodeRaw{
		src:    href,
		key:    key,
		link:   link,
		title:  strings.TrimSpace(textContent(n)),
		pos:    len(p.episodeLinks),
		number: episodeNumberFromSegment(key),
	})
}

// finish turns the raw candidates into the final ordered, deduplicated list and
// derives the remaining series fields. Link and SourceHref are the episode's
// real URL (the full "s" query value of the shortener, e.g.
// https://sarrast.com/series/x/12-thor); the title is the human-readable
// "episode {ep}" derived from the true number.
func (p *parser) finish() {
	for _, raw := range p.episodeLinks {
		link := raw.link
		if link == "" {
			link = raw.key
		}
		p.episodes = append(p.episodes, Episode{
			Key:        raw.key,
			Title:      episodeTitle(raw.number, raw.title),
			Link:       link,
			SourceHref: link,
			Number:     raw.number,
		})
	}
	p.series.EpisodeCount = len(p.episodes)
	p.series.Category = strings.TrimSpace(p.series.Category)
	p.series.Rating = strings.TrimSpace(p.series.Rating)
	p.series.Status = strings.TrimSpace(p.series.Status)
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// textContent returns the descendant text of n, trimmed of surrounding space.
func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// episodeSegment returns the stable identity of an episode from rawURL. It
// prefers the "s" query parameter (a shortener that embeds the real episode
// path) when it carries a non-trivial value, and otherwise uses the URL's final
// path segment. "https://ouo.io/go?s=12-thor" → "12-thor";
// "https://sarrast.com/series/x/12-thor" → "12-thor"; for an absolute-URL s
// value with a trailing random nonce, "…/episode-004-CtlLM" → "episode-004".
// The result is URL-unescaped and must be non-empty and not a bare root path.
func episodeSegment(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}

	seg, ok := bestSegment([]string{u.Query().Get("s"), u.Path})
	if ok {
		return seg, true
	}

	// No usable segment anywhere: fall back to the s value's final path segment
	// so something stable is recorded even for odd shortener shapes.
	for _, c := range []string{u.Query().Get("s"), u.Path} {
		if seg := cleanSegment(path.Base(strings.TrimRight(c, "/"))); seg != "" {
			return seg, true
		}
	}
	return "", false
}

// bestSegment runs through candidate slot values and returns the first slot
// that yields a usable episode segment. For a shortener URL the s query value is
// preferred, then the URL path itself. A candidate can be a short slug
// ("12-thor", "episode-1") or a full URL
// ("https://sarrast.com/series/x/episode-004-CtlLM"); for a full URL the
// identifying segment is the final path segment. A trailing random nonce that a
// shortener appends to a release (episode-004-CtlLM) is stripped by stripNonce.
func bestSegment(candidates []string) (string, bool) {
	for _, c := range candidates {
		ref := strings.TrimSpace(c)
		if ref == "" {
			continue
		}
		u, err := url.Parse(ref)
		if err != nil {
			continue
		}
		if !u.IsAbs() {
			// Page-relative value: the whole candidate is the episode slug.
			if seg := cleanSegment(path.Base(strings.TrimRight(u.Path, "/"))); seg != "" {
				return stripNonce(seg), true
			}
			continue
		}
		segs := strings.Split(strings.Trim(u.Path, "/"), "/")

		// Scan from the end; the episode name usually sits at the back.
		for i := len(segs) - 1; i >= 0; i-- {
			seg := cleanSegment(segs[i])
			if seg == "" {
				continue
			}
			return stripNonce(seg), true
		}
	}
	return "", false
}

// stripNonce removes a trailing random nonce appended by a URL shortener (e.g.
// "episode-004-CtlLM" → "episode-004"). A nonce is a trailing -XYZ part of a
// segment whose XYZ contains at least one uppercase letter (a hallmark of random
// nonces in ouo.io shorteners). When no such suffix is found the segment is
// returned unchanged.
func stripNonce(seg string) string {
	// Find the rightmost dash that could separate the real identity from a nonce.
	for i := len(seg) - 1; i > 0; i-- {
		if seg[i] != '-' {
			continue
		}
		suffix := seg[i+1:]
		// A nonce has mixed case or at least one uppercase letter.
		hasUpper := false
		hasLower := false
		for _, c := range suffix {
			if c >= 'A' && c <= 'Z' {
				hasUpper = true
			}
			if c >= 'a' && c <= 'z' {
				hasLower = true
			}
			if hasUpper && hasLower {
				return seg[:i]
			}
		}
		// Also strip if the suffix is purely non-lowercase random letters
		// (all uppercase, like "CTL") — real episode names are lowercase.
		if hasUpper && !hasLower {
			return seg[:i]
		}
	}
	return seg
}

// cleanSegment trims and URL-unescapes a raw path segment, returning "" for
// values that are effectively empty or root markers.
func cleanSegment(raw string) string {
	seg := strings.TrimSpace(raw)
	if seg == "" || seg == "." || seg == "/" {
		return ""
	}
	if decoded, err := url.PathUnescape(seg); err == nil {
		seg = decoded
	}
	return strings.TrimSpace(seg)
}

// episodeTitle returns the human-readable episode title. It prefers the
// "episode {ep}" form derived from the true number, falling back to the page's
// anchor text when the number is unknown (0).
func episodeTitle(number int, fallback string) string {
	if number > 0 {
		return fmt.Sprintf("episode %d", number)
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return ""
}

// episodeNumberFromSegment derives the true episode number from a URL segment.
// It handles three shapes:
//
//   - leading digits:  "12-thor" → 12, "3-something" → 3
//   - zero-padded prefix: "episode-004-<nonce>" → 4 (a plain "episode-1" slug
//     stays 0)
//   - trailing digits ({random}-{ep} links): "pink-pussy-121" → 121
//
// A segment with no parseable number (for example "thor") yields 0, which
// callers treat as "unknown"; ordering falls back to the stored key.
func episodeNumberFromSegment(seg string) int {
	seg = strings.TrimSpace(seg)

	// Zero-padded "episode-NNN" prefix (real numbered releases).
	if low := strings.ToLower(seg); strings.HasPrefix(low, "episode-") {
		digits := seg[len("episode-"):]
		if len(digits) >= 2 && digits[0] == '0' {
			i := 0
			for i < len(digits) && digits[i] >= '0' && digits[i] <= '9' {
				i++
			}
			if i > 0 {
				if n, err := strconv.Atoi(digits[:i]); err == nil {
					return n
				}
			}
		}
		return 0
	}

	// Leading digits: "12-thor".
	i := 0
	for i < len(seg) && seg[i] >= '0' && seg[i] <= '9' {
		i++
	}
	if i > 0 {
		if n, err := strconv.Atoi(seg[:i]); err == nil {
			return n
		}
	}

	// Trailing digits ({random}-{ep}): "pink-pussy-121".
	j := len(seg)
	for j > 0 && seg[j-1] >= '0' && seg[j-1] <= '9' {
		j--
	}
	if j < len(seg) {
		if n, err := strconv.Atoi(seg[j:]); err == nil {
			return n
		}
	}
	return 0
}

// sParam extracts the "s" query parameter from a shortener URL. It reports
// false when the URL cannot be parsed.
func sParam(rawURL string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", false
	}
	return u.Query().Get("s"), true
}

// abs resolves a possibly-relative reference against the page URL.
func abs(base, ref string) string {
	bu, err := url.Parse(base)
	if err != nil {
		return ref
	}
	ru, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return bu.ResolveReference(ru).String()
}
