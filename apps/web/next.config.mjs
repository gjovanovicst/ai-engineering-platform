/** @type {import('next').NextConfig} */
const nextConfig = {
  async headers() {
    // In development the RSC/HTML payloads must never be cached: a stale shell
    // (e.g. an older navigation) hydrating against new client bundles is the
    // hydration mismatch we kept hitting. Production keeps normal caching.
    if (process.env.NODE_ENV !== "development") return [];
    return [
      {
        source: "/:path*",
        headers: [{ key: "Cache-Control", value: "no-store, max-age=0, must-revalidate" }],
      },
    ];
  },
};

export default nextConfig;
