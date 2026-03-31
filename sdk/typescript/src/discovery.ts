import type { Transport } from "./transport.js";
import type { ANSRecord } from "./types.js";

/** Manages service discovery via ANS (Agent Name System). */
export class DiscoveryManager {
  constructor(private readonly transport: Transport) {}

  /**
   * Fetch the .well-known/aicp.json from a domain.
   * This is a direct HTTP fetch, not through the AICP server.
   */
  async fetchWellKnown(
    domain: string,
    fetchFn?: typeof globalThis.fetch,
  ): Promise<ANSRecord> {
    const f = fetchFn ?? globalThis.fetch.bind(globalThis);
    const url = `https://${domain}/.well-known/aicp.json`;
    const resp = await f(url, {
      headers: { Accept: "application/json" },
    });
    if (!resp.ok) {
      throw new Error(
        `failed to fetch .well-known/aicp.json from ${domain}: ${resp.status}`,
      );
    }
    return (await resp.json()) as ANSRecord;
  }

  /** Fetch the server's own .well-known/aicp.json. */
  async getServerInfo(): Promise<ANSRecord> {
    return this.transport.get<ANSRecord>("/.well-known/aicp.json");
  }

  /**
   * Resolve a service by its DID or domain.
   * Attempts domain resolution first, then falls back to the AICP server.
   */
  async resolve(
    serviceIdentifier: string,
    fetchFn?: typeof globalThis.fetch,
  ): Promise<ANSRecord> {
    // If it looks like a domain (not a DID), try .well-known first.
    if (!serviceIdentifier.startsWith("did:")) {
      try {
        return await this.fetchWellKnown(serviceIdentifier, fetchFn);
      } catch {
        // Fall through to server lookup.
      }
    }

    return this.getServerInfo();
  }
}
