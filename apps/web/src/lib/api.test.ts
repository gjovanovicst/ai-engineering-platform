// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ensureSession } from "./api";
import { setSession } from "./session";

describe("ensureSession", () => {
  beforeEach(() => {
    window.localStorage.clear();
    vi.restoreAllMocks();
  });
  afterEach(() => {
    window.localStorage.clear();
  });

  it("refreshes only once when called concurrently", async () => {
    // Seed a refresh token as if the page had been reloaded.
    window.localStorage.setItem("aip.refresh_token", "refresh-1");
    setSession({ access_token: "", refresh_token: "refresh-1", expires_in: 0, refresh_expires_in: 0, token_type: "Bearer" });

    const fetchMock = vi.fn<typeof fetch>(async () => {
      return new Response(
        JSON.stringify({
          access_token:
            "test-access-token",
          refresh_token: "refresh-2",
          expires_in: 900,
          refresh_expires_in: 1000,
          token_type: "Bearer",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      );
    });
    vi.stubGlobal("fetch", fetchMock);

    await Promise.all([ensureSession(), ensureSession()]);

    const refreshCalls = fetchMock.mock.calls.filter((call) => String(call[0]).includes("/auth/refresh"));
    expect(refreshCalls).toHaveLength(1);
    expect(window.localStorage.getItem("aip.refresh_token")).toBe("refresh-2");
  });
});
