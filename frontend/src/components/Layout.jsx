import { useEffect, useState } from "react";
import { Link, Outlet, useLocation } from "react-router-dom";
import { motion } from "framer-motion";
import { Clapperboard } from "lucide-react";
import { Aurora } from "./Aurora";

// Layout scaffolds the animated page chrome: a floating glass navbar that
// condenses on scroll, an aurora backdrop, and the routed page.
export default function Layout() {
  const [scrolled, setScrolled] = useState(false);
  const location = useLocation();

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  // Scroll to top on route change.
  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "instant" });
  }, [location.pathname]);

  return (
    <div className="relative min-h-screen overflow-x-clip">
      {/* Animated aurora backdrop */}
      <Aurora />

      {/* Floating glass navbar */}
      <motion.header
        initial={{ y: -80, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{ duration: 0.6, ease: "easeOut" }}
        className={`fixed inset-x-0 top-0 z-40 transition-all duration-300 ${
          scrolled
            ? "border-b border-edge/60 bg-surface-dark/70 backdrop-blur-xl shadow-[0_8px_40px_rgba(0,0,0,0.4)]"
            : "border-b border-transparent bg-transparent"
        }`}
      >
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6">
          <Link
            to="/"
            className="group flex items-center gap-2.5"
            aria-label="Sarrast home"
          >
            <motion.span
              className="grid h-9 w-9 place-items-center rounded-xl bg-gradient-to-br from-brand to-sky-400 shadow-glow"
              whileHover={{ rotate: -10, scale: 1.1 }}
              transition={{ type: "spring", stiffness: 300, damping: 15 }}
            >
              <Clapperboard className="h-5 w-5 text-white" />
            </motion.span>
            <span className="font-display text-lg font-bold tracking-tight">
              Sarrast
              <span className="text-gradient"> Extractor</span>
            </span>
          </Link>
          </div>
      </motion.header>

      {/* Routed content */}
      <main className="relative z-10 pt-16">
        <Outlet />
      </main>

      <footer className="relative z-10 border-t border-edge/50 py-8 text-center text-xs text-ink-faint">
        Sarrast Extractor · Created by <a href="https://github.com/R04RM3NT" className="hover:text-white transition-all duration-300">R04RM3NT</a>  **R04RM3NT** — built with ❤️, Go, and React. 
      </footer>
    </div>
  );
}