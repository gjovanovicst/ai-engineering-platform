import { describe, expect, it } from "vitest";

import { decodeClaims } from "./auth";

function makeToken(payload: Record<string, unknown>): string {
  const encode = (v: unknown) =>
    Buffer.from(JSON.stringify(v)).toString("base64url");
  return `${encode({ alg: "EdDSA" })}.${encode(payload)}.signature`;
}

describe("decodeClaims", () => {
  it("decodes the payload", () => {
    const token = makeToken({ sub: "u1", org: "o1", role: "admin", sid: "s1", exp: 123 });
    expect(decodeClaims(token)).toMatchObject({ sub: "u1", org: "o1", role: "admin" });
  });

  it("returns null for malformed tokens", () => {
    expect(decodeClaims("not-a-token")).toBeNull();
    expect(decodeClaims("a.b.c")).toBeNull();
    expect(decodeClaims(makeToken({}))).toBeNull();
  });
});
