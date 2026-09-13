/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      colors: {
        brand: {
          DEFAULT: "#c97bff", // primary purple
          dark: "#b14aff",
          deep: "#7c3aed",
        },
        surface: {
          DEFAULT: "#12121c",
          light: "#1a1a2a",
          dark: "#0b0b12",
        },
        edge: "#26263a",
        ink: {
          DEFAULT: "#e8e8f4",
          soft: "#a6a6bf",
          faint: "#6f6f8a",
        },
      },
      fontFamily: {
        display: ["Space Grotesk", "system-ui", "-apple-system", "sans-serif"],
        body: ["Inter", "system-ui", "-apple-system", "sans-serif"],
      },
      boxShadow: {
        glow: "0 0 24px rgba(201, 123, 255, 0.35)",
        "glow-lg": "0 0 48px rgba(201, 123, 255, 0.45)",
        card: "0 10px 30px rgba(0, 0, 0, 0.45)",
      },
      backgroundImage: {
        "gradient-radial": "radial-gradient(var(--tw-gradient-stops))",
      },
      keyframes: {
        shimmer: {
          "0%": { backgroundPosition: "0% 0%" },
          "100%": { backgroundPosition: "200% 0%" },
        },
        float: {
          "0%, 100%": { transform: "translateY(0px)" },
          "50%": { transform: "translateY(-20px)" },
        },
        "pulse-slow": {
          "0%, 100%": { opacity: "0.6" },
          "50%": { opacity: "1" },
        },
        spinSlow: {
          from: { transform: "rotate(0deg)" },
          to: { transform: "rotate(360deg)" },
        },
      },
      animation: {
        shimmer: "shimmer 2.5s linear infinite",
        float: "float 6s ease-in-out infinite",
        "pulse-slow": "pulse-slow 4s ease-in-out infinite",
        "spin-slow": "spinSlow 30s linear infinite",
      },
    },
  },
  plugins: [],
};