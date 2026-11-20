import { FlatCompat } from "@eslint/eslintrc";

const compat = new FlatCompat({ baseDirectory: import.meta.dirname });

const config = [
  ...compat.extends("next/core-web-vitals", "next/typescript"),
  {
    ignores: [".next/**", "node_modules/**", "next-env.d.ts", "public/sw.js"],
  },
  {
    // A bare global resolves silently (onClick={close} calls window.close and
    // does nothing), so make the window globals explicit.
    rules: {
      "no-restricted-globals": [
        "error",
        { name: "close", message: "Use the panel's onClose: a bare close calls window.close and does nothing." },
        { name: "origin", message: "Use window.location.origin explicitly." },
        { name: "self", message: "Use window.self explicitly." },
        { name: "parent", message: "Use window.parent explicitly." },
      ],
    },
  },
];

export default config;
