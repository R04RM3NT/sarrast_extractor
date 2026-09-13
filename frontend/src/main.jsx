import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import "./index.css";
import Layout from "./components/Layout";
import Home from "./pages/Home";
import SeriesDetail from "./pages/SeriesDetail";

ReactDOM.createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<Home />} />
          <Route path="series/:slug" element={<SeriesDetail />} />
        </Route>
        <Route path="*" element={<NotFound />} />
      </Routes>
    </BrowserRouter>
  </React.StrictMode>
);

function NotFound() {
  return (
    <div className="grid min-h-screen place-items-center">
      <div className="text-center">
        <h1 className="font-display text-6xl font-bold text-gradient">404</h1>
        <p className="mt-3 text-ink-soft">This page drifted out of orbit.</p>
        <a
          href="/"
          className="mt-6 inline-block rounded-full bg-brand px-6 py-2.5 text-sm font-semibold text-white transition hover:bg-brand-dark hover:shadow-glow"
        >
          Back home
        </a>
      </div>
    </div>
  );
}