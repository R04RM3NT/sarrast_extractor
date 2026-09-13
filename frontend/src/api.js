// API client for the Go JSON backend.
// Series/Episode mirror the JSON shapes returned by GET /api/series and
// GET /api/series/{slug}.

export const THUMB_PATH = (slug) => `/thumbnails/${slug}/thumb.webp`;

async function get(path) {
  const res = await fetch(path);
  if (!res.ok) {
    let detail = "";
    try {
      const body = await res.json();
      detail = body?.error ?? "";
    } catch {
      /* ignore non-JSON error bodies */
    }
    throw new Error(`${res.status}${detail ? ` ${detail}` : ""}`);
  }
  return res.json();
}

export const api = {
  listSeries: () => get("/api/series"),
  getSeries: (slug) => get(`/api/series/${slug}`),
};