import { useEffect, useRef, useState } from "react";
import { motion, useInView, useMotionValue, useSpring } from "framer-motion";

// Reveal lifts content in from below once it scrolls into view. Delays allow
// staggered grids.
export function Reveal({ children, delay = 0, y = 24, className = "", once = true }) {
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once, margin: "-80px" }}
      transition={{ duration: 0.6, delay, ease: [0.21, 0.47, 0.32, 0.98] }}
    >
      {children}
    </motion.div>
  );
}

// StaggerItem is meant to be a direct child of a motion <motion.div> parent with
// initial/animate/transition = "show"/"show" variants. Simpler: give the parent
// a staggerChildren variant.
export const listStagger = {
  hidden: {},
  show: { transition: { staggerChildren: 0.07 } },
};

export const itemFade = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: [0.21, 0.47, 0.32, 0.98] },
  },
};

// Counter eases an integer up to `value` as it scrolls into view.
export function Counter({ value, className = "" }) {
  const ref = useRef(null);
  const inView = useInView(ref, { once: true, margin: "-40px" });
  const mv = useMotionValue(0);
  const spring = useSpring(mv, { stiffness: 60, damping: 18 });
  const [display, setDisplay] = useState(0);

  useEffect(() => {
    if (!inView) return;
    mv.set(value);
  }, [inView, value, mv]);

  useEffect(() => {
    const unsub = spring.on("change", (v) => setDisplay(Math.round(v)));
    return unsub;
  }, [spring]);

  return (
    <span ref={ref} className={className}>
      {display}
    </span>
  );
}

// Shimmer skeleton while data loads.
export function Skeleton({ className = "" }) {
  return (
    <div
      className={`animate-shimmer rounded-xl bg-gradient-to-r from-surface-light/60 via-white/5 to-surface-light/60 bg-[length:200%_100%] ${className}`}
    />
  );
}

// EmptyState shows a friendly placeholder for an empty library.
export function EmptyState() {
  return (
    <Reveal className="mx-auto max-w-md py-24 text-center">
      <div className="mx-auto mb-6 grid h-20 w-20 place-items-center rounded-3xl bg-surface/60 shadow-glow">
        <span className="text-3xl">🎬</span>
      </div>
      <h2 className="font-display text-2xl font-bold">Your library is empty</h2>
      <p className="mt-2 text-ink-soft">
        No series scanned yet. Run the extractor against a series page to start
        populating the database.
      </p>
    </Reveal>
  );
}