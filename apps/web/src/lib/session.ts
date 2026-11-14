import type { TokenPair } from "./api";

const REFRESH_KEY = "aip.refresh_token";

// The access token lives only in memory (XSS blast radius is one page load).
// The refresh token is persisted until the BFF cookie flow lands (E2-S3).
let accessToken: string | null = null;

export function setSession(pair: TokenPair) {
  accessToken = pair.access_token;
  if (typeof window !== "undefined") {
    window.localStorage.setItem(REFRESH_KEY, pair.refresh_token);
  }
}

export function getAccessToken() {
  return accessToken;
}

export function getRefreshToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(REFRESH_KEY);
}

export function clearSession() {
  accessToken = null;
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(REFRESH_KEY);
  }
}

export function isAuthenticated() {
  return accessToken !== null || getRefreshToken() !== null;
}
