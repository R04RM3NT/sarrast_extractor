import { useState } from "react";
import { motion } from "framer-motion";
import { Link } from "react-router-dom";
import { Play, Star, Film } from "lucide-react";
import { THUMB_PATH } from "../api";
import { itemFade } from "./ui";

// SeriesCard is the animated poster tile shown in the library grid. It has a
// hover glow, a slide-up overlay with an inline play action, a shimmering
// reveal, and a stagger reveal keyed to the grid position.
export default function SeriesCard({ series, index = 0 }) {
  const [loaded, setLoaded] = useState(false);
  const thumb = series.thumbnailPath ? THUMB_PATH(series.slug) : null;

  return (
    <motion.article
      variants={itemFade}
      custom={index}
      className="group relative"
      layout
      whileHover={{ y: -6 }}
      transition={{ type: "spring", stiffness: 260, damping: 20 }}
    >
      <Link
        to={`/series/${series.slug}`}
        className="block overflow-hidden rounded-2xl border border-edge/60 bg-surface/40 backdrop-blur-sm transition duration-300 group-hover:border-brand/60 group-hover:shadow-glow-lg"
      >
        {/* Poster */}
        <div className="relative aspect-[2/3] overflow-hidden">
          {thumb ? (
            <>
              <img
                src={thumb}
                alt={`${series.name} poster`}
                loading="lazy"
                onLoad={() => setLoaded(true)}
                className={`h-full w-full object-cover transition duration-700 group-hover:scale-105 ${
                  loaded ? "opacity-100" : "opacity-0"
                }`}
              />
              <div className="absolute inset-0 bg-gradient-to-t from-surface-dark via-surface-dark/10 to-transparent opacity-80 transition group-hover:opacity-95" />
              <div className="pointer-events-none absolute inset-0 bg-gradient-to-tr from-brand/0 to-sky-400/0 mix-blend-overlay transition duration-500 group-hover:from-brand/25 group-hover:to-sky-400/20" />
            </>
          ) : (
            <div className="grid h-full w-full place-items-center bg-gradient-to-br from-surface-light to-surface-dark">
              <Film className="h-10 w-10 text-ink-faint" />
              <div className="absolute inset-0 bg-gradient-to-t from-surface-dark to-transparent" />
            </div>
          )}

          {/* Top row: rating + status */}
          <div className="absolute inset-x-3 top-3 flex items-start justify-between gap-2">
            {series.rating ? (
              <span className="flex items-center gap-1 rounded-full bg-black/50 px-2.5 py-1 text-xs font-semibold text-amber-300 backdrop-blur-sm">
                <Star className="h-3 w-3 fill-amber-300" />
                {series.rating}
              </span>
            ) : (
              <span />
            )}
            {series.status && (
              <span
                className={`rounded-full px-2.5 py-1 text-[10px] font-bold uppercase tracking-wider backdrop-blur-sm ${
                  series.status.toLowerCase().includes("ongo") ||
                  series.status.toLowerCase().includes("airing")
                    ? "bg-emerald-500/80 text-white"
                    : series.status.toLowerCase().includes("finished") ||
                        series.status.toLowerCase().includes("complete")
                      ? "bg-sky-500/80 text-white"
                      : "bg-slate-500/80 text-white"
                }`}
              >
                {series.status}
              </span>
            )}
          </div>

          {/* Bottom overlay: title + episodes */}
          <div className="absolute inset-x-0 bottom-0 p-3">
            <div className="mb-2 flex items-center gap-1.5 text-xs text-ink-soft">
              <Play className="h-3.5 w-3.5 fill-brand text-brand" />
              {series.episodeCount} {series.episodeCount === 1 ? "episode" : "episodes"}
            </div>
            <h3 className="line-clamp-2 font-display text-base font-bold leading-snug text-ink">
              {series.name}
            </h3>
          </div>

          {/* Hover ring */}
          <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-inset ring-white/0 transition duration-300 group-hover:ring-brand/40" />
        </div>
      </Link>
    </motion.article>
  );
}

// CardSkeleton is the loading placeholder matching the poster tile shape.
export function CardSkeleton() {
  return (
    <div className="overflow-hidden rounded-2xl border border-edge/40 bg-surface/20">
      <div className="aspect-[2/3] animate-shimmer bg-gradient-to-r from-surface-light/50 via-white/5 to-surface-light/50 bg-[length:200%_100%]" />
      <div className="space-y-2 p-3">
        <div className="h-3 w-3/4 rounded bg-white/5" />
        <div className="h-2.5 w-1/2 rounded bg-white/5" />
      </div>
    </div>
  );
}