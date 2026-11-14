import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        bg: {
          canvas: "rgb(var(--bg-canvas) / <alpha-value>)",
          surface: "rgb(var(--bg-surface) / <alpha-value>)",
          elevated: "rgb(var(--bg-elevated) / <alpha-value>)",
          hover: "rgb(var(--bg-hover) / <alpha-value>)",
          selected: "rgb(var(--bg-selected) / <alpha-value>)",
        },
        border: {
          DEFAULT: "rgb(var(--border) / <alpha-value>)",
          strong: "rgb(var(--border-strong) / <alpha-value>)",
        },
        text: {
          primary: "rgb(var(--text-primary) / <alpha-value>)",
          secondary: "rgb(var(--text-secondary) / <alpha-value>)",
          muted: "rgb(var(--text-muted) / <alpha-value>)",
          inverse: "rgb(var(--text-inverse) / <alpha-value>)",
        },
        brand: {
          DEFAULT: "rgb(var(--brand) / <alpha-value>)",
          hover: "rgb(var(--brand-hover) / <alpha-value>)",
          subtle: "rgb(var(--brand-subtle) / <alpha-value>)",
        },
        ai: {
          DEFAULT: "rgb(var(--ai) / <alpha-value>)",
          subtle: "rgb(var(--ai-subtle) / <alpha-value>)",
        },
        status: {
          success: "rgb(var(--status-success) / <alpha-value>)",
          warning: "rgb(var(--status-warning) / <alpha-value>)",
          danger: "rgb(var(--status-danger) / <alpha-value>)",
          info: "rgb(var(--status-info) / <alpha-value>)",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "monospace"],
      },
      borderRadius: { sm: "6px", md: "8px", lg: "12px", xl: "16px" },
      boxShadow: {
        sm: "0 1px 2px rgba(0,0,0,0.05)",
        md: "0 4px 12px rgba(15,23,42,0.06)",
        lg: "0 10px 24px rgba(15,23,42,0.10)",
      },
      transitionDuration: { fast: "100ms", base: "150ms", slow: "200ms" },
    },
  },
};
export default config;
