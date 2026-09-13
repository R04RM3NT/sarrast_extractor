import { motion } from "framer-motion";

// Aurora renders a slow, drifting, blurred aurora behind the whole app. Blobs
// animate gently to keep the page alive without being distracting.
export function Aurora() {
  return (
    <div aria-hidden className="pointer-events-none fixed inset-0 overflow-hidden">
      <motion.div
        className="blob -left-40 top-[-10%] h-[36rem] w-[36rem] bg-[#4c1d95]/40"
        animate={{ x: [0, 60, 0], y: [0, 40, 0], scale: [1, 1.15, 1] }}
        transition={{ duration: 22, repeat: Infinity, ease: "easeInOut" }}
      />
      <motion.div
        className="blob right-[-16rem] top-[15%] h-[30rem] w-[30rem] bg-[#1d4ed8]/30"
        animate={{ x: [0, -70, 0], y: [0, -30, 0], scale: [1.05, 0.9, 1.05] }}
        transition={{ duration: 26, repeat: Infinity, ease: "easeInOut" }}
      />
      <motion.div
        className="blob bottom-[-20%] left-1/2 h-[32rem] w-[40rem] -translate-x-1/2 bg-[#701a75]/25"
        animate={{ x: ["-50%", "-40%", "-50%"], y: [0, -40, 0] }}
        transition={{ duration: 30, repeat: Infinity, ease: "easeInOut" }}
      />
      {/* Fine grain overlay */}
      <div className="absolute inset-0 opacity-[0.03] mix-blend-overlay [background-image:url('data:image/svg+xml;utf8,<svg xmlns=%22http://www.w3.org/2000/svg%22 width=%22100%22 height=%22100%22><filter id=%22n%22><feTurbulence type=%22fractalNoise%22 baseFrequency=%220.85%22 numOctaves=%223%22/></filter><rect width=%22100%22 height=%22100%22 filter=%22url(%23n)%22/></svg>')]" />
    </div>
  );
}