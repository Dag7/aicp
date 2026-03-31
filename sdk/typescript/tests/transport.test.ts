import { describe, it, expect, vi } from "vitest";
import { Transport, AICPError } from "../src/transport.js";

function mockFetch(
  status: number,
  body: unknown,
): typeof globalThis.fetch {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    statusText: status === 200 ? "OK" : "Error",
    json: () => Promise.resolve(body),
  }) as unknown as typeof globalThis.fetch;
}

describe("Transport", () => {
  it("makes GET requests", async () => {
    const fetchFn = mockFetch(200, { success: true, data: { ok: true } });
    const transport = new Transport({
      serverURL: "https://example.com",
      fetch: fetchFn,
    });

    const result = await transport.get<{ ok: boolean }>("/health");
    expect(result.ok).toBe(true);
    expect(fetchFn).toHaveBeenCalledWith(
      "https://example.com/health",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("makes POST requests with body", async () => {
    const fetchFn = mockFetch(201, {
      success: true,
      data: { token: "abc" },
    });
    const transport = new Transport({
      serverURL: "https://example.com",
      fetch: fetchFn,
    });

    const result = await transport.post<{ token: string }>("/api/token", {
      did: "did:aicp:test",
    });
    expect(result.token).toBe("abc");
    expect(fetchFn).toHaveBeenCalledWith(
      "https://example.com/api/token",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ did: "did:aicp:test" }),
      }),
    );
  });

  it("sends auth token in headers", async () => {
    const fetchFn = mockFetch(200, { success: true, data: {} });
    const transport = new Transport({
      serverURL: "https://example.com",
      authToken: "my-token",
      fetch: fetchFn,
    });

    await transport.get("/test");
    const call = (fetchFn as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(call[1].headers["Authorization"]).toBe("Bearer my-token");
  });

  it("throws AICPError on API error", async () => {
    const fetchFn = mockFetch(400, {
      success: false,
      error: { code: "BAD_REQUEST", message: "invalid input" },
    });
    const transport = new Transport({
      serverURL: "https://example.com",
      fetch: fetchFn,
    });

    await expect(transport.get("/test")).rejects.toThrow(AICPError);
    try {
      await transport.get("/test");
    } catch (err) {
      const e = err as AICPError;
      expect(e.statusCode).toBe(400);
      expect(e.code).toBe("BAD_REQUEST");
      expect(e.message).toBe("invalid input");
    }
  });

  it("strips trailing slashes from base URL", () => {
    const transport = new Transport({
      serverURL: "https://example.com///",
      fetch: mockFetch(200, { success: true }),
    });
    expect(transport.getBaseURL()).toBe("https://example.com");
  });

  it("updates auth token", async () => {
    const fetchFn = mockFetch(200, { success: true, data: {} });
    const transport = new Transport({
      serverURL: "https://example.com",
      fetch: fetchFn,
    });

    transport.setAuthToken("new-token");
    await transport.get("/test");
    const call = (fetchFn as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(call[1].headers["Authorization"]).toBe("Bearer new-token");
  });
});
