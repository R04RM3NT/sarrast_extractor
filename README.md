# Sarrast Extractor

Track series from [sarrast.com](https://sarrast.com) — extract episode links and metadata, cache posters, and browse your library from a terminal UI, the command line, or a web app.

Sarrast Extractor is a personal library tracker for series on the Persian site سرراست. It fetches a series page, pulls out the episode links (which arrive via the `ouo.io` URL shortener, with the real episode URL embedded in the `s` query parameter), parses the series metadata (name, description, rating, status, category), caches the poster thumbnail, and incrementally merges everything into a single local SQLite database. Re-scanning a series updates its episodes in place instead of duplicating them.

Everything ships as **one self-contained binary** — the React frontend is embedded into the Go executable with `go:embed`, and the database is a pure-Go SQLite driver, so no CGO or external services are needed.

## Features

- **Three interfaces** — an interactive [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI, a batch command-line extraction flow, and a web UI.
- **Web library browser** — an animated poster grid, live search, category filter chips, and a per-series detail page with a full episode list ([React](https://react.dev) + [Vite](https://vite.dev) + [Tailwind CSS](https://tailwindcss.com)).
- **Incremental scans** — episodes are deduplicated by a stable key; re-scans update `last_seen` timestamps rather than duplicating rows.
- **Poster caching** — series thumbnails are downloaded to `data/thumbnails/<slug>/thumb.webp`.
- **Proxy support** — route extraction requests through `http://`, `https://`, or `socks5://host:port` proxies.
- **One embedded binary** — no runtime Go or Node dependencies; the frontend bundle is compiled in.
- **Automatic migration** — a legacy `series.json` database is imported into SQLite on first run and kept as backup.
- **No CGO** — uses the pure-Go [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) driver.

## Quick start

Build the binary (see [Building from source](#building-from-source)) and run it with no arguments:

```sh
extract          # or: extract.exe on Windows — opens the TUI
```

Or jump straight to the other two modes:

```sh
# Extract a series and save it to the database
extract -url "https://sarrast.com/series/<slug>"

# Browse your library in the browser
extract -serve   # opens http://127.0.0.1:8080
```

## Usage

There are three ways to run the extractor, selected by the first argument.

### 1. Terminal UI (default)

```sh
extract
extract -tui
```

An interactive [Bubble Tea](https://github.com/charmbracelet/bubbletea) interface for the extraction flow, with a form for the series URL and options, a settings screen (where the default proxy can be changed and saved), and a results view.

### 2. Command line (batch)

```sh
extract -url "https://sarrast.com/series/my-series"
```

Fetches the page, extracts matching episode links and metadata, stores them in the database, and prints a summary:

```
Series: My Series
New episodes: 12
Total episodes: 12
Database: centralized SQLite database (data/series.db)
```

Extra options:

```sh
extract -url "https://sarrast.com/series/my-series" \
    -series "My Series" \      # name to store under (otherwise derived from the URL)
    -proxy socks5://127.0.0.1:9050 \   # http://, https:// or socks5://host:port
    -filter sarrast.com \      # substring a link href must contain (default: ouo.io)
    -quiet                     # suppress progress output
```

### 3. Web application

```sh
extract -serve
```

Starts a read-only HTTP server on `http://127.0.0.1:8080` serving the embedded React app. **The web app only browses** the database — extraction still happens through the TUI or the CLI.

## CLI reference

| Flag | Description |
| --- | --- |
| `-tui`, `--tui` | Launch the interactive TUI (default with no arguments). |
| `-serve`, `--serve`, `serve` | Start the web server on `127.0.0.1:8080`. |
| `-url <page>` | (CLI) Series page URL to extract links from; **required** for CLI mode. |
| `-series <name>` | (CLI) Name to store the series under; derived from the URL when omitted. |
| `-proxy <addr>` | (CLI) Proxy address, e.g. `http://host:8080`, `https://host:8080` or `socks5://host:1080`. Falls back to the saved settings proxy. |
| `-filter <host>` | (CLI) Substring that a link's `href` must contain to be considered; default `ouo.io`. |
| `-quiet` | (CLI) Suppress progress output. |

## Web UI & HTTP API

The web server exposes a small JSON API on top of the same database. Endpoints:

| Endpoint | Description |
| --- | --- |
| `GET /api/series` | All series, ordered by name. |
| `GET /api/series/{slug}` | A single series with its full episode list. |
| `GET /thumbnails/{slug}/thumb.webp` | The cached poster image. |
| `GET /` | The SPA. Any non-API path serves `index.html`, so client-side routes deep-link. |

## Storage & configuration

| Path | Purpose |
| --- | --- |
| `data/series.db` | The SQLite database (WAL mode). Created automatically on first run. |
| `data/thumbnails/<slug>/thumb.webp` | Cached series posters. |
| `%AppData%\sarrast\settings.json` (Windows) / `~/.config/sarrast/settings.json` (Unix) | Persistent settings — currently the default proxy. Format: `{ "proxy": "..." }`. |

The database path and thumbnails directory are fixed internal constants and are not configurable. On first open, a legacy `series.json` database (if present) is imported into SQLite and kept as a backup.

## Building from source

### Prerequisites

- **Go** ≥ 1.26
- **Node.js** and **npm** (only to build the frontend)

### Build the frontend first

The React app bundles into `internal/web/static/`, which the Go server embeds at compile time:

```sh
cd frontend
npm install
npm run build      # outputs to ../internal/web/static
cd ..
```

> **Important:** because the frontend is embedded at build time, you must re-run `npm install && npm run build` and then rebuild the Go binary whenever you change the UI. If the bundle hasn't been built, `extract -serve` shows a placeholder page telling you to build it.

### Build the binary

```sh
go build ./cmd/extract
```

This produces `extract` (or `extract.exe` on Windows) in the current directory.

### Frontend development

Run the Vite dev server alongside `extract -serve`; Vite proxies `/api` and `/thumbnails` to the Go server and hot-reloads the UI:

```sh
cd frontend
npm run dev        # http://localhost:5173
```

### Testing

```sh
go vet ./...
go test ./...
```

## Project layout

```
.
├── cmd/
│   └── extract/            # Entry point; dispatches to TUI / CLI / web
├── frontend/               # React SPA (Vite + Tailwind)
│   └── src/
│       ├── pages/          # Home (library), SeriesDetail
│       └── components/     # Layout, SeriesCard, Aurora, ui
├── internal/
│   ├── config/             # Application configuration (defaults, timeouts)
│   ├── database/           # SQLite layer: schema, migrations, repositories
│   ├── extractor/          # HTTP + HTML link extraction helpers
│   ├── logging/            # log/slog wrapper
│   ├── metadata/           # Series-page HTML metadata parsing
│   ├── scanner/            # Orchestrates a scan: fetch → parse → merge → thumbnail
│   ├── seriesdb/           # Legacy JSON database (migration source)
│   ├── settings/           # Persistent per-user settings (default proxy)
│   ├── thumbnail/          # Poster caching to data/thumbnails/
│   ├── tui/                # Bubble Tea terminal UI
│   └── web/                # HTTP server, JSON API, embedded SPA
└── data/                   # Runtime data: series.db, thumbnails/ (gitignored)
```

## Tech stack

| Layer | Technology |
| --- | --- |
| Backend | [Go](https://go.dev) 1.26 |
| Database | [SQLite](https://www.sqlite.org) via [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (pure Go, no CGO) |
| Terminal UI | [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Bubbles](https://github.com/charmbracelet/bubbles) + [Lip Gloss](https://github.com/charmbracelet/lipgloss) |
| HTML / proxy | [`golang.org/x/net`](https://pkg.go.dev/golang.org/x/net) |
| Frontend | [React](https://react.dev) 18 · [Vite](https://vite.dev) 5 · [Tailwind CSS](https://tailwindcss.com) 3 · [Framer Motion](https://www.framer.com/motion/) · React Router |

## Acknowledgments

Created by **R04RM3NT** — built with ❤️, Go, and React.