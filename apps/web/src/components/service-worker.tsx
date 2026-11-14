"use client";

import { useEffect } from "react";

// Registers the PWA worker in production builds only; `next dev` must stay free
// of stale caches (that bug cost us a day of CSS chasing).
export function ServiceWorkerRegister() {
  useEffect(() => {
    if (process.env.NODE_ENV !== "production") return;
    if (!("serviceWorker" in navigator)) return;
    navigator.serviceWorker.register("/sw.js").catch(() => undefined);
  }, []);
  return null;
}
