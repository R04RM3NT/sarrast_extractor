import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Build output lands in internal/web/static so the Go server can embed it with
// go:embed all:static and ship a single binary. The dev server proxies the JSON
// API to the running Go server on :8080.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/web/static",
    emptyOutDir: true,
    sourcemap: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/thumbnails": "http://127.0.0.1:8080",
    },
  },
});