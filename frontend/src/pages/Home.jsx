import { useEffect, useMemo, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Search, SlidersHorizontal, Sparkles, Library, RefreshCw, Play } from "lucide-react";
import { api } from "../api";
import SeriesCard, { CardSkeleton } from "../components/SeriesCard";
import { Reveal, Counter, EmptyState, listStagger } from "../components/ui";

// Home is the library overview. It has an animated hero with a live search that
// filters the grid in real time, plus a staggered poster grid.
export default function Home() {
  const [series, setSeries] = useState(null); // null = loading
  const [error, setError] = useState(null);
  const [query, setQuery] = useState("");
  const [category, setCategory] = useState("all");

  useEffect(() => {
    let cancelled = false;
    api
      .listSeries()
      .then((data) => !cancelled && setSeries(data))
      .catch((e) => !cancelled && setError(e.message));
    return () => {
      cancelled = true;
    };
  }, []);

  const categories = useMemo(() => {
    if (!series) return [];
    return [...new Set(series.map((s) => s.category).filter(Boolean))].sort();
  }, [series]);

  const filtered = useMemo(() => {
    if (!series) return [];
    const q = query.trim().toLowerCase();
    return series
      .filter((s) => (category === "all" ? true : s.category === category))
      .filter((s) => (q ? s.name.toLowerCase().includes(q) : true))
      .sort((a, b) => (b.rating || "0").localeCompare(a.rating || "0", undefined, { numeric: true }));
  }, [series, query, category]);

  const totalEpisodes = useMemo(
    () => (series ? series.reduce((sum, s) => sum + s.episodeCount, 0) : 0),
    [series]
  );

  const reset = () => {
    setQuery("");
    setCategory("all");
  };

  return (
    <div className="mx-auto max-w-7xl px-4 sm:px-6">
      {/* ===== Hero ===== */}
      <section className="relative py-16 text-center sm:py-24">
        <motion.div
          initial={{ opacity: 0, y: 40 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, ease: [0.21, 0.47, 0.32, 0.98] }}
        >
          <motion.p
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ delay: 0.2, duration: 0.6 }}
            className="mb-4 inline-flex items-center gap-2 rounded-full border border-brand/30 bg-brand/10 px-4 py-1.5 text-xs font-semibold uppercase tracking-widest text-brand"
          >
            <Sparkles className="h-3.5 w-3.5" />
            Scanned &amp; indexed
          </motion.p>

          <h1
            className="mx-auto max-w-3xl font-display text-5xl font-bold leading-[1.05] tracking-tight sm:text-7xl"
            style={{ letterSpacing: "-0.02em" }}
          >
            <span className="shimmer-text">Your sarrast series,</span>
            <br />
            <span className="text-gradient">beautifully organized.</span>
          </h1>

          <motion.p
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.35, duration: 0.6 }}
            className="mx-auto mt-6 max-w-xl text-base text-ink-soft sm:text-lg"
          >
            Every sarrast series you extracted, kept in one central database with its
            posters and the complete episode history.
          </motion.p>
        </motion.div>

        {/* Hero stat chips */}
        {(series || error) && (
          <motion.div
            initial={{ opacity: 0, y: 24 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.5, duration: 0.7 }}
            className="mt-10 flex flex-wrap items-center justify-center gap-3 text-sm"
          >
            <span className="flex items-center gap-2 rounded-full border border-edge/60 bg-surface/50 px-5 py-2.5 backdrop-blur-sm">
              <Library className="h-4 w-4 text-brand" />
              <Counter value={series?.length ?? 0} />
              <span className="text-ink-soft">series</span>
            </span>
            <span className="flex items-center gap-2 rounded-full border border-edge/60 bg-surface/50 px-5 py-2.5 backdrop-blur-sm">
              <Play className="h-4 w-4 text-sky-400" />
              <Counter value={totalEpisodes} />
              <span className="text-ink-soft">episodes</span>
            </span>
          </motion.div>
        )}

        {/* ===== Search + filter bar ===== */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.7, duration: 0.6 }}
          className="mx-auto mt-12 max-w-xl"
        >
          <div className="flex items-center gap-3 rounded-2xl border border-edge/60 bg-surface/60 p-2.5 shadow-card backdrop-blur-xl transition focus-within:border-brand/60 focus-within:shadow-glow transition-all duration-300">
            <Search className="ml-2 h-5 w-5 shrink-0 text-ink-faint" />
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search your library…"
              className="w-full bg-transparent text-sm text-ink outline-none placeholder:text-ink-faint"
            />
            {query && (
              <button
                onClick={reset}
                className="mr-1 rounded-full bg-white/5 px-2.5 py-1 text-xs text-ink-soft hover:bg-white/10 hover:text-ink"
              >
                Clear
              </button>
            )}
          </div>

          {/* Category chips */}
          {categories.length > 0 && (
            <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
              <SlidersHorizontal className="h-4 w-4 text-ink-faint" />
              <Chip active={category === "all"} onClick={() => setCategory("all")}>
                All
              </Chip>
              {categories.map((c) => (
                <Chip key={c} active={category === c} onClick={() => setCategory(c)}>
                  {c}
                </Chip>
              ))}
            </div>
          )}
        </motion.div>
      </section>

      {/* ===== Results ===== */}
      <section className="pb-16 sm:pb-24">
        {error ? (
          <div className="mx-auto max-w-md text-center">
            <p className="text-red-300">Couldn't reach the API: {error}</p>
            <p className="mt-2 text-sm text-ink-soft">Make sure the Go server is running with `extract.exe -serve`.</p>
          </div>
        ) : !series ? (
          // Loading skeleton grid
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {Array.from({ length: 12 }).map((_, i) => (
              <CardSkeleton key={i} />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState />
        ) : (
          <>
            <Reveal className="mb-5 flex items-center justify-between">
              <h2 className="font-display text-xl font-bold">
                {filtered.length} {filtered.length === 1 ? "series" : "series"}
                {query && (
                  <span className="text-ink-faint"> matching “{query}”</span>
                )}
              </h2>
              <span className="text-xs uppercase tracking-widest text-ink-faint">
                <RefreshCw className="mr-1 inline h-3.5 w-3.5" />
                live
              </span>
            </Reveal>

            <motion.div
              key={`${category}-${query}`}
              variants={listStagger}
              initial="hidden"
              animate="show"
              className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
            >
              <AnimatePresence mode="popLayout">
                {filtered.map((s) => (
                  <SeriesCard key={s.slug} series={s} />
                ))}
              </AnimatePresence>
            </motion.div>
          </>
        )}
      </section>
    </div>
  );
}

function Chip({ active, children, onClick }) {
  return (
    <button
      onClick={onClick}
      className={`rounded-full px-3.5 py-1.5 text-xs font-medium transition ${
        active
          ? "bg-brand text-white shadow-glow"
          : "border border-edge/60 bg-surface/40 text-ink-soft hover:border-brand/50 hover:text-ink"
      }`}
    >
      {children}
    </button>
  );
}