export type Claims = {
  sub: string;
  org: string;
  role: string;
  sid: string;
  exp: number;
};

/** Decodes the payload of a JWT without verifying it (display only). */
export function decodeClaims(token: string): Claims | null {
  const parts = token.split(".");
  if (parts.length !== 3) return null;
  try {
    const payload = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = payload.padEnd(payload.length + ((4 - (payload.length % 4)) % 4), "=");
    const json = atob(padded);
    const claims = JSON.parse(json) as Partial<Claims>;
    if (!claims.sub || !claims.org || !claims.role) return null;
    return claims as Claims;
  } catch {
    return null;
  }
}
