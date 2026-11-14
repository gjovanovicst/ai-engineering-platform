import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "AI Engineering Platform",
    short_name: "AIP",
    description: "AI agents executing the software lifecycle",
    start_url: "/overview",
    display: "standalone",
    background_color: "#0b0b0f",
    theme_color: "#4f46e5",
    icons: [
      { src: "/icons/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "any" },
      { src: "/icons/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "maskable" },
    ],
  };
}
