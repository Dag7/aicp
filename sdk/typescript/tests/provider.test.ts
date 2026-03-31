import { describe, it, expect } from "vitest";
import { AICPServiceAdapter } from "../src/provider.js";

describe("AICPServiceAdapter", () => {
  const adapter = new AICPServiceAdapter({
    serviceDID: "did:aicp:myservice",
    serviceName: "Test Service",
    categories: ["ai", "nlp"],
    requiredCredentials: ["AICPAgentDelegation"],
    acceptedPayments: ["stripe", "paypal"],
    minimumTrustScore: 200,
  });

  describe("buildANSRecord", () => {
    it("generates a valid ANS record", () => {
      const record = adapter.buildANSRecord({
        description: "A test AI service",
        homepage: "https://example.com",
        apiBase: "https://api.example.com",
      });

      expect(record.aicp_version).toBe("1.0");
      expect(record.service.name).toBe("Test Service");
      expect(record.service.did).toBe("did:aicp:myservice");
      expect(record.service.category).toEqual(["ai", "nlp"]);
      expect(record.onboarding.required_credentials).toEqual([
        "AICPAgentDelegation",
      ]);
      expect(record.onboarding.accepted_payments).toEqual([
        "stripe",
        "paypal",
      ]);
      expect(record.onboarding.minimum_trust_score).toBe(200);
      expect(record.authentication.did_methods).toEqual(["did:aicp"]);
      expect(record.endpoints?.api_base).toBe("https://api.example.com");
    });

    it("includes pricing when provided", () => {
      const record = adapter.buildANSRecord({
        pricingModel: "usage",
        pricingTiers: [
          { name: "basic", rate: "0.01", description: "Per request" },
        ],
      });

      expect(record.pricing?.model).toBe("usage");
      expect(record.pricing?.tiers).toHaveLength(1);
      expect(record.pricing?.tiers?.[0].name).toBe("basic");
    });
  });

  describe("extractAuth", () => {
    it("extracts DID from Bearer token", () => {
      const ctx = adapter.extractAuth("Bearer did:aicp:agent123");
      expect(ctx.authenticated).toBe(true);
      expect(ctx.did).toBe("did:aicp:agent123");
      expect(ctx.token).toBe("did:aicp:agent123");
    });

    it("returns unauthenticated for missing header", () => {
      const ctx = adapter.extractAuth(undefined);
      expect(ctx.authenticated).toBe(false);
      expect(ctx.did).toBeUndefined();
    });

    it("returns unauthenticated for non-Bearer header", () => {
      const ctx = adapter.extractAuth("Basic dXNlcjpwYXNz");
      expect(ctx.authenticated).toBe(false);
    });
  });

  describe("expressMiddleware", () => {
    it("attaches aicp context to request", () => {
      const middleware = adapter.expressMiddleware();
      const req: Record<string, unknown> = {
        headers: { authorization: "Bearer did:aicp:test" },
      };
      let nextCalled = false;
      middleware(
        req as Parameters<ReturnType<typeof adapter.expressMiddleware>>[0],
        {},
        () => {
          nextCalled = true;
        },
      );
      expect(nextCalled).toBe(true);
      expect(
        (req as { aicp?: { authenticated: boolean; did?: string } }).aicp
          ?.authenticated,
      ).toBe(true);
      expect(
        (req as { aicp?: { authenticated: boolean; did?: string } }).aicp?.did,
      ).toBe("did:aicp:test");
    });
  });

  describe("genericMiddleware", () => {
    it("handler extracts auth", () => {
      const mw = adapter.genericMiddleware();
      const ctx = mw.handler("Bearer did:aicp:agent");
      expect(ctx.authenticated).toBe(true);
    });

    it("requireAuth throws when unauthenticated", () => {
      const mw = adapter.genericMiddleware();
      expect(() => mw.requireAuth(undefined)).toThrow(
        "AICP authentication required",
      );
    });

    it("requireAuth returns context when authenticated", () => {
      const mw = adapter.genericMiddleware();
      const ctx = mw.requireAuth("Bearer did:aicp:agent");
      expect(ctx.authenticated).toBe(true);
    });
  });

  it("returns service DID", () => {
    expect(adapter.getServiceDID()).toBe("did:aicp:myservice");
  });
});
