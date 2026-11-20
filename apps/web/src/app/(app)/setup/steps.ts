/** The onboarding steps, kept out of page.tsx: Next.js allows only the reserved
 * Page exports, so a custom `steps` export fails the production build. */
export const steps = [
  { key: "provider", title: "Connect an AI provider", detail: "The model your agents may use." },
  { key: "repository", title: "Connect a repository", detail: "Where the agent opens its pull requests." },
  { key: "test", title: "Set the test command", detail: "The gate every development run must pass." },
  { key: "sandbox", title: "Confirm the sandbox", detail: "The isolated image the agent runs in." },
] as const;
