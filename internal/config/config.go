// Package config centralizes every tunable in the application. A single
// Config value is created once and injected into the components that need it,
// avoiding global state. The database path is deliberately NOT configurable:
// every component uses the fixed internal SQLite database.
package config

import "time"

// Config holds all application settings.
type Config struct {
	// FilterHost is the substring an <a> tag's href must contain to be
	// considered a candidate link on a series page.
	FilterHost string

	// HTTPTimeout bounds a single page-fetch HTTP request.
	HTTPTimeout time.Duration

	// Quiet suppresses CLI logging when true.
	Quiet bool

	// Proxy is the default proxy address used by every request (page
	// extraction). Supports http://, https:// and socks5://host:port.
	// Empty means direct connections.
	Proxy string
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		FilterHost:  "ouo.io",
		HTTPTimeout: 20 * time.Second,
	}
}
