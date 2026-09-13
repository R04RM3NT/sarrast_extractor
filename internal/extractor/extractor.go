// Package extractor contains the low-level HTTP + HTML helpers used by the
// CLI link extractor and the TUI extract flow. It is the single source of
// truth for parsing candidate links out of a series page.
package extractor

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/proxy"
)

// DefaultUserAgent is sent with every discovery request.
const DefaultUserAgent = "Mozilla/5.0 (sarrast-extractor)"

// Client is a proxy-aware HTTP client.
type Client struct {
	http *http.Client
}

// NewClient builds an HTTP client. When proxyURL is non-empty it routes every
// request through the proxy; http, https and socks5 are supported.
func NewClient(proxyURL string, timeout time.Duration) (*Client, error) {
	httpClient := &http.Client{Timeout: timeout}
	if proxyURL == "" {
		return &Client{http: httpClient}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy address: %w", err)
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(parsed)}
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if parsed.User != nil {
			password, _ := parsed.User.Password()
			auth = &proxy.Auth{User: parsed.User.Username(), Password: password}
		}
		dialer, err := proxy.SOCKS5("tcp", parsed.Host, auth, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("create socks5 dialer: %w", err)
		}
		httpClient.Transport = &http.Transport{Dial: dialer.Dial}
	default:
		return nil, fmt.Errorf("unsupported proxy type %q (use http, https or socks5)", parsed.Scheme)
	}

	return &Client{http: httpClient}, nil
}

// Client returns the underlying *http.Client, letting other packages reuse the
// proxy/timeout configuration (for example when downloading thumbnails).
func (c *Client) Client() *http.Client { return c.http }

// FetchString GETs target and returns the response body as a string.
func (c *Client) FetchString(target string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unsuccessful response from server: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MatchingHrefs walks the HTML tree and returns the href of every <a> tag
// whose href contains filter.
func MatchingHrefs(r io.Reader, filter string) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	var results []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" && strings.Contains(attr.Val, filter) {
					results = append(results, attr.Val)
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results, nil
}

// SParam parses a URL and returns the value of its "s" query parameter.
func SParam(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}
	return parsed.Query().Get("s"), nil
}

// Resolve turns a possibly-relative reference into an absolute URL using base.
func Resolve(base, ref string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}
