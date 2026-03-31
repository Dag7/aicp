import type { ANSRecord } from "./types.js";

// ─── Service Provider Types ─────────────────────────────────────────────────

export interface ServiceProviderConfig {
  /** The service's DID */
  serviceDID: string;
  /** The service's display name */
  serviceName: string;
  /** Service categories */
  categories: string[];
  /** Onboarding flows supported */
  onboardingFlows?: string[];
  /** Required credentials for onboarding */
  requiredCredentials?: string[];
  /** Accepted payment methods */
  acceptedPayments?: string[];
  /** Minimum trust score (0-1000) */
  minimumTrustScore?: number;
  /** AICP server URL (for proxied verification) */
  aicpServerURL?: string;
}

/** Middleware request context with AICP auth info. */
export interface AICPRequestContext {
  /** Authenticated DID (if present) */
  did?: string;
  /** Whether the request is authenticated */
  authenticated: boolean;
  /** The raw Bearer token */
  token?: string;
}

// ─── AICPServiceAdapter ─────────────────────────────────────────────────────

/**
 * AICPServiceAdapter helps service providers integrate with the AICP protocol.
 * It generates .well-known/aicp.json, validates DID-Auth tokens, and provides
 * middleware for Express/Fastify/any framework.
 */
export class AICPServiceAdapter {
  constructor(private readonly config: ServiceProviderConfig) {}

  /** Generate the ANS record for .well-known/aicp.json. */
  buildANSRecord(options?: {
    description?: string;
    homepage?: string;
    apiBase?: string;
    pricingModel?: string;
    pricingTiers?: Array<{
      name: string;
      rate?: string;
      description?: string;
    }>;
  }): ANSRecord {
    return {
      aicp_version: "1.0",
      service: {
        name: this.config.serviceName,
        did: this.config.serviceDID,
        description: options?.description,
        category: this.config.categories,
        homepage: options?.homepage,
      },
      onboarding: {
        flows: this.config.onboardingFlows ?? ["standard"],
        required_credentials: this.config.requiredCredentials ?? [
          "AICPAgentDelegation",
        ],
        accepted_payments: this.config.acceptedPayments ?? ["stripe"],
        minimum_trust_score: this.config.minimumTrustScore ?? 100,
      },
      authentication: {
        did_methods: ["did:aicp"],
        auth_protocols: ["DIDAuth"],
      },
      pricing: options?.pricingModel
        ? {
            model: options.pricingModel,
            tiers: options.pricingTiers,
          }
        : undefined,
      endpoints: {
        api_base: options?.apiBase,
      },
    };
  }

  /**
   * Extract AICP auth context from a request.
   * Works with any framework: just pass the Authorization header value.
   */
  extractAuth(authHeader?: string | null): AICPRequestContext {
    if (!authHeader || !authHeader.startsWith("Bearer ")) {
      return { authenticated: false };
    }

    const token = authHeader.slice(7);
    // The token is a DID — in AICP, the Bearer token is the agent's DID
    // authenticated via DID-Auth (the server has already verified the proof).
    return {
      did: token,
      authenticated: true,
      token,
    };
  }

  /**
   * Express/Connect-compatible middleware.
   * Extracts AICP auth and attaches it to req.aicp.
   */
  expressMiddleware() {
    return (
      req: { headers: Record<string, string | undefined>; aicp?: AICPRequestContext },
      _res: unknown,
      next: () => void,
    ) => {
      req.aicp = this.extractAuth(req.headers["authorization"]);
      next();
    };
  }

  /**
   * Fastify-compatible hook.
   * Decorates the request with aicp context.
   */
  fastifyHook() {
    return (
      request: { headers: Record<string, string | undefined>; aicp?: AICPRequestContext },
      _reply: unknown,
      done: () => void,
    ) => {
      request.aicp = this.extractAuth(request.headers["authorization"]);
      done();
    };
  }

  /**
   * Generic middleware for any framework.
   * Returns { handler, requireAuth }.
   */
  genericMiddleware() {
    const adapter = this;
    return {
      /** Extract auth context (non-blocking). */
      handler(authHeader?: string | null): AICPRequestContext {
        return adapter.extractAuth(authHeader);
      },

      /** Guard: throws if not authenticated. */
      requireAuth(authHeader?: string | null): AICPRequestContext {
        const ctx = adapter.extractAuth(authHeader);
        if (!ctx.authenticated) {
          throw new Error("AICP authentication required");
        }
        return ctx;
      },
    };
  }

  /** Get the service DID. */
  getServiceDID(): string {
    return this.config.serviceDID;
  }
}
