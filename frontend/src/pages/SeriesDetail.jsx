import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { motion, AnimatePresence, useScroll, useTransform } from "framer-motion";
import {
  ArrowLeft,
  Calendar,
  ExternalLink,
  Film,
  Play,
  SearchX,
  Star,
  Tag,
  Clock,
} from "lucide-react";
import { api, THUMB_PATH } from "../api";
import { itemFade, listStagger, Reveal, Counter } from "../components/ui";

// SeriesDetail is the animated per-series page: a blurred parallax backdrop,
// a poster with hover tilt, metadata reveal-on-scroll, and a staggered episode
// list.
export default function SeriesDetail() {
  const { slug } = useParams();
  const [series, setSeries] = useState(null);
  const [error, setError] = useState(null);
  const [loaded, setLoaded] = useState(false);

  const { scrollY } = useScroll();
  const backdropY = useTransform(scrollY, [0, 600], [0, 120]);
  const heroOpacity = useTransform(scrollY, [0, 500], [1, 0.15]);

  useEffect(() => {
    let cancelled = false;
    setSeries(null);
    setError(null);
    setLoaded(false);
    api
      .getSeries(slug)
      .then((data) => !cancelled && setSeries(data))
      .catch((e) => !cancelled && setError(e.message));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  if (error) {
    return (
      <div className="mx-auto grid min-h-[60vh] max-w-md place-items-center text-center">
        <div>
          <SearchX className="mx-auto mb-4 h-12 w-12 text-ink-faint" />
          <h1 className="font-display text-2xl font-bold">Series not found</h1>
          <p className="mt-2 text-sm text-ink-soft">
            {error} — it may not have been scanned yet.
          </p>
          <Link
            to="/"
            className="mt-8 inline-flex items-center gap-2 rounded-full bg-brand px-6 py-2.5 text-sm font-semibold text-white transition hover:bg-brand-dark hover:shadow-glow"
          >
            <ArrowLeft className="h-4 w-4" />
            Back to library
          </Link>
        </div>
      </div>
    );
  }

  if (!series) return <DetailSkeleton />;

  const thumb = series.thumbnailPath ? THUMB_PATH(series.slug) : null;

  return (
    <div>
      {/* ===== Parallax hero backdrop ===== */}
      <motion.div style={{ y: backdropY, opacity: heroOpacity }} className="absolute inset-x-0 top-16 h-[46vh] overflow-hidden">
        <div className="absolute inset-0">
          <motion.div
            initial={{ scale: 1.15, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ duration: 1.2, ease: "easeOut" }}
            className="h-full w-full"
          >
            {thumb ? (
              <img
                src={thumb}
                alt=""
                aria-hidden
                className="h-full w-full object-cover object-top blur-2xl brightness-[0.35]"
                onLoad={() => setLoaded(true)}
              />
            ) : (
              <div className="h-full w-full bg-gradient-to-br from-[#1c1557]/60 via-[#312e81]/40 to-surface-dark" />
            )}
          </motion.div>
        </div>
        {/* gradient scrim back into the page */}
        <div className="absolute inset-x-0 bottom-0 h-40 bg-gradient-to-t from-surface-dark to-transparent" />
      </motion.div>

      <div className="relative z-10 mx-auto max-w-6xl px-4 sm:px-6">
        {/* Back link */}
        <motion.div
          initial={{ opacity: 0, x: -16 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.1, duration: 0.5 }}
          className="pt-8"
        >
          <Link
            to="/"
            className="inline-flex items-center gap-1.5 text-sm text-ink-soft transition hover:text-ink"
          >
            <ArrowLeft className="h-4 w-4" />
            Library
          </Link>
        </motion.div>

        {/* ===== Header ===== */}
        <section className="mt-6 flex flex-col gap-8 pb-12 sm:flex-row sm:items-start">
          {/* Poster */}
          <motion.div
            initial={{ opacity: 0, y: 40, rotate: -2 }}
            animate={{ opacity: 1, y: 0, rotate: 0 }}
            transition={{ delay: 0.2, duration: 0.7, ease: [0.21, 0.47, 0.32, 0.98] }}
            whileHover={{ y: -8 }}
            className="mx-auto shrink-0 sm:mx-0"
          >
            <div className="relative w-48 overflow-hidden rounded-2xl border border-edge/60 shadow-card sm:w-56">
              {thumb ? (
                <img
                  src={thumb}
                  alt={`${series.name} poster`}
                  className="aspect-[2/3] w-full object-cover"
                />
              ) : (
                <div className="grid aspect-[2/3] w-full place-items-center bg-gradient-to-br from-surface-light to-surface-dark">
                  <Film className="h-12 w-12 text-ink-faint" />
                </div>
              )}
              <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-inset ring-white/10" />
            </div>
          </motion.div>

          {/* Meta */}
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2 text-xs font-semibold uppercase tracking-widest">
              {series.status && (
                <span
                  className={`rounded-full px-3 py-1 text-[10px] font-bold ${
                    series.status.toLowerCase().includes("ongo") ||
                    series.status.toLowerCase().includes("airing")
                      ? "bg-emerald-500/90 text-white"
                      : series.status.toLowerCase().includes("finished") ||
                          series.status.toLowerCase().includes("complete")
                        ? "bg-sky-500/90 text-white"
                        : "bg-slate-500/90 text-white"
                  }`}
                >
                  {series.status}
                </span>
              )}
              {series.category && (
                <span className="flex items-center gap-1 rounded-full border border-edge/60 bg-surface/50 px-3 py-1 text-ink-soft">
                  <Tag className="h-3 w-3" />
                  {series.category}
                </span>
              )}
            </div>

            <motion.h1
              initial={{ opacity: 0, y: 24 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.32, duration: 0.7 }}
              className="mt-4 font-display text-4xl font-bold leading-tight tracking-tight sm:text-5xl"
            >
              {series.name}
            </motion.h1>

            <motion.div
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.42 }}
              className="mt-4 flex flex-wrap items-center gap-4 text-sm text-ink-soft"
            >
              <span className="flex items-center gap-1.5">
                <Star className="h-4 w-4 fill-amber-300 text-amber-300" />
                <span className="font-semibold text-ink">{series.rating || "—"}</span>
                rating
              </span>
              <span className="flex items-center gap-1.5">
                <Play className="h-4 w-4 text-brand" />
                <Counter value={series.episodeCount} className="font-semibold text-ink" />
                episodes
              </span>
              {series.sourceUrl && (
                <a
                  href={series.sourceUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1.5 rounded-full border border-edge/60 px-3 py-1 transition hover:border-brand/50 hover:text-ink"
                >
                  <ExternalLink className="h-3.5 w-3.5" />
                  Source
                </a>
              )}
            </motion.div>

            {series.description && (
              <Reveal delay={0.5} className="mt-6 max-w-2xl text-[15px] leading-relaxed text-ink-soft">
                {series.description}
              </Reveal>
            )}

            <Reveal delay={0.55} className="mt-6 flex flex-wrap items-center gap-x-6 gap-y-2 text-xs text-ink-faint">
              <span className="flex items-center gap-1.5">
                <Clock className="h-3.5 w-3.5" />
                Last scan {formatDate(series.lastScannedAt)}
              </span>
              <span className="flex items-center gap-1.5">
                <Calendar className="h-3.5 w-3.5" />
                Added {formatDate(series.createdAt)}
              </span>
            </Reveal>
          </div>
        </section>

        {/* ===== Episodes ===== */}
        <section className="pb-24">
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true, margin: "-60px" }}
            transition={{ duration: 0.6 }}
            className="mb-6 flex items-center gap-3"
          >
            <h2 className="font-display text-2xl font-bold">Episodes</h2>
            <span className="rounded-full bg-brand/15 px-2.5 py-0.5 text-sm font-semibold text-brand">
              {series.episodes?.length ?? 0}
            </span>
          </motion.div>

          {series.episodes && series.episodes.length > 0 ? (
            <motion.div variants={listStagger} initial="hidden" whileInView="show" viewport={{ once: true, margin: "-40px" }}>
              <AnimatePresence>
                {series.episodes.map((ep, i) => (
                  <EpisodeRow key={ep.key ?? ep.id} ep={ep} index={i} />
                ))}
              </AnimatePresence>
            </motion.div>
          ) : (
            <Reveal className="rounded-2xl border border-dashed border-edge/70 p-10 text-center text-ink-faint">
              No episodes recorded yet. Scan this series to populate its list.
            </Reveal>
          )}
        </section>
      </div>
    </div>
  );
}

// EpisodeRow is one animated row in the episode list.
function EpisodeRow({ ep, index }) {
  const hasTitle = Boolean(ep.title);
  const display = hasTitle ? ep.title : `Episode ${ep.number || "—"}`;
  return (
    <motion.a
      variants={itemFade}
      href={ep.link || undefined}
      target={ep.link ? "_blank" : undefined}
      rel={ep.link ? "noopener noreferrer" : undefined}
      className="group flex items-center gap-4 rounded-xl border border-edge/50 bg-surface/30 px-4 py-3 transition duration-300 hover:border-brand/50 hover:bg-surface/60 hover:shadow-glow"
    >
      <span className="w-10 shrink-0 text-right font-mono text-sm font-semibold text-ink-faint tabular-nums transition group-hover:text-brand">
        {ep.number || "—"}
      </span>
      <span className="min-w-0 flex-1 truncate text-[15px] text-ink transition group-hover:text-white">
        {display}
      </span>
      <span className="shrink-0 text-xs text-ink-faint">
        {formatSmallDate(ep.lastSeenAt)}
      </span>
      <span className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-white/5 opacity-0 transition duration-300 group-hover:opacity-100 group-hover:bg-brand/20">
        <Play className="h-3.5 w-3.5 fill-brand text-brand" />
      </span>
    </motion.a>
  );
}

// DetailSkeleton is the shimmering placeholder for a loading detail page.
function DetailSkeleton() {
  return (
    <div className="relative z-10 mx-auto max-w-6xl px-4 sm:px-6">
      <div className="mt-16 flex flex-col gap-8 sm:flex-row">
        <div className="mx-auto h-72 w-48 animate-shimmer rounded-2xl bg-gradient-to-r from-surface-light/50 via-white/5 to-surface-light/50 bg-[length:200%_100%] sm:mx-0 sm:w-56" />
        <div className="flex-1 space-y-4">
          <div className="h-3 w-40 rounded bg-white/5" />
          <div className="h-10 w-3/4 rounded bg-white/5" />
          <div className="h-3 w-56 rounded bg-white/5" />
          <div className="h-3 w-full max-w-md rounded bg-white/5" />
          <div className="h-3 w-2/3 rounded bg-white/5" />
        </div>
      </div>
      <div className="mt-10 grid gap-2">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="h-12 animate-shimmer rounded-xl bg-gradient-to-r from-surface-light/40 via-white/5 to-surface-light/40 bg-[length:200%_100%]" />
        ))}
      </div>
    </div>
  );
}

function formatDate(value) {
  if (!value) return "—";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? "—"
    : d.toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}

function formatSmallDate(value) {
  if (!value) return "";
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? ""
    : d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}