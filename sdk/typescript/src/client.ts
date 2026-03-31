import { Transport } from "./transport.js";
import { IdentityManager, Identity } from "./identity.js";
import { DiscoveryManager } from "./discovery.js";
import { OnboardingManager } from "./onboarding.js";
import { CommerceManager } from "./commerce.js";
import { CredentialWallet, MemoryWalletStorage } from "./wallet.js";
import type { WalletStorage } from "./wallet.js";
import { EventClient } from "./websocket.js";
import {
  generateKeyPair,
  signJSON,
  didIdentifier,
} from "./crypto.js";
import type {
  AICPAgentConfig,
  AICPClientConfig,
  VerifiableCredential,
  VerifiablePresentation,
  MoneyAmount,
  ANSRecord,
  OnboardingState,
  PaymentExecuteResponse,
  VerifyResponse,
} from "./types.js";

/**
 * AICPAgent is the primary entry point for the TypeScript SDK.
 *
 * It provides a high-level, 5-call experience for agent developers:
 *   1. Create identity (generateIdentity / fromKeyPair)
 *   2. Discover services (discover)
 *   3. Onboard to a service (onboard)
 *   4. Make payments (pay)
 *   5. Verify identities (verify)
 *
 * Example:
 * ```typescript
 * const agent = await AICPAgent.create({
 *   serverURL: "https://aicp.example.com",
 *   privateKey: myPrivateKey,
 * });
 *
 * const service = await agent.discover("example.com");
 * const account = await agent.onboard(service, ["api-access"]);
 * const payment = await agent.pay(service, { value: "10.00", currency: "USD" });
 * const result = await agent.verify("did:aicp:abc123");
 * ```
 */
export class AICPAgent {
  readonly transport: Transport;
  readonly identity: IdentityManager;
  readonly discovery: DiscoveryManager;
  readonly onboarding: OnboardingManager;
  readonly commerce: CommerceManager;
  readonly wallet: CredentialWallet;
  readonly events: EventClient;

  private readonly privateKey: Uint8Array;
  private readonly publicKey: Uint8Array;
  private readonly did: string;
  private constructor(
    config: AICPAgentConfig,
    wallet: CredentialWallet,
  ) {
    this.transport = new Transport(config);
    this.identity = new IdentityManager(this.transport);
    this.discovery = new DiscoveryManager(this.transport);
    this.onboarding = new OnboardingManager(this.transport);
    this.commerce = new CommerceManager(this.transport);
    this.wallet = wallet;
    this.events = new EventClient(this.transport);

    this.privateKey = config.privateKey;
    this.publicKey = Identity.getPublicKey(config.privateKey);

    if (config.agentDID) {
      this.did = config.agentDID;
    } else if (config.principalDID) {
      this.did = `${config.principalDID}:${didIdentifier(this.publicKey)}`;
    } else {
      this.did = Identity.principalDID(this.publicKey);
    }
  }

  /**
   * Create a new AICPAgent with a wallet backend.
   * Uses in-memory wallet by default.
   */
  static async create(
    config: AICPAgentConfig,
    storage?: WalletStorage,
  ): Promise<AICPAgent> {
    const wallet = new CredentialWallet(storage ?? new MemoryWalletStorage());
    const agent = new AICPAgent(config, wallet);

    // Store the key pair in the wallet.
    await wallet.storeKeyPair(agent.did, {
      publicKey: agent.publicKey,
      privateKey: agent.privateKey,
    });

    return agent;
  }

  /**
   * Generate a brand new agent identity and connect to a server.
   * Convenience factory for quick prototyping.
   */
  static async generateAndConnect(
    serverURL: string,
    storage?: WalletStorage,
  ): Promise<AICPAgent> {
    const keyPair = generateKeyPair();
    return AICPAgent.create(
      { serverURL, privateKey: keyPair.privateKey },
      storage,
    );
  }

  /** Get this agent's DID. */
  getDID(): string {
    return this.did;
  }

  /** Get this agent's public key. */
  getPublicKey(): Uint8Array {
    return this.publicKey;
  }

  // ─── High-Level API (5-call experience) ─────────────────────────────

  /**
   * 1. Discover a service by domain or DID.
   * Fetches the .well-known/aicp.json from the target.
   */
  async discover(serviceIdentifier: string): Promise<ANSRecord> {
    return this.discovery.resolve(serviceIdentifier);
  }

  /**
   * 2. Onboard to a service.
   * Runs the full 4-phase onboarding flow automatically.
   */
  async onboard(
    _service: ANSRecord,
    requestedCapabilities: string[],
    paymentToken: string,
    credentials?: VerifiableCredential[],
  ): Promise<{
    state: OnboardingState;
    accountID: string;
    credentials: Record<string, string>;
  }> {
    const presentation = this.identity.buildPresentation(
      this.did,
      credentials ?? [],
      this.privateKey,
    );

    return this.onboarding.runFullFlow({
      agentDID: this.did,
      presentation,
      requestedCapabilities,
      privateKey: this.privateKey,
      paymentToken,
    });
  }

  /**
   * 3. Make a payment.
   * Requests a token and executes the payment in one call.
   */
  async pay(
    service: ANSRecord,
    amount: MoneyAmount,
    purpose: string = "usage-payment",
  ): Promise<PaymentExecuteResponse> {
    return this.commerce.pay({
      agentDID: this.did,
      serviceDID: service.service.did,
      amount,
      purpose,
      serviceCategory: service.service.category[0] ?? "general",
    });
  }

  /**
   * 4. Verify a DID.
   * Resolves and validates the DID, delegation chain, and trust score.
   */
  async verify(did: string): Promise<VerifyResponse> {
    return this.identity.verify(did);
  }

  /**
   * 5. Subscribe to real-time events.
   * Connects to the WebSocket and returns the event client.
   */
  async subscribe(): Promise<EventClient> {
    if (!this.events.connected) {
      await this.events.connect();
    }
    return this.events;
  }

  // ─── Utility ────────────────────────────────────────────────────────

  /** Create a signed proof over arbitrary data. */
  sign(data: unknown): string {
    return signJSON(data, this.privateKey);
  }

  /** Build and sign a verifiable presentation. */
  buildPresentation(
    credentials: VerifiableCredential[],
  ): VerifiablePresentation {
    return this.identity.buildPresentation(
      this.did,
      credentials,
      this.privateKey,
    );
  }

  /** Check server health. */
  async health(): Promise<boolean> {
    try {
      await this.transport.get("/health");
      return true;
    } catch {
      return false;
    }
  }
}

/** Convenience re-export for creating a simple API client (no identity). */
export class AICPClient {
  readonly transport: Transport;
  readonly identity: IdentityManager;
  readonly discovery: DiscoveryManager;
  readonly commerce: CommerceManager;

  constructor(config: AICPClientConfig) {
    this.transport = new Transport(config);
    this.identity = new IdentityManager(this.transport);
    this.discovery = new DiscoveryManager(this.transport);
    this.commerce = new CommerceManager(this.transport);
  }

  /** Verify a DID. */
  async verify(did: string): Promise<VerifyResponse> {
    return this.identity.verify(did);
  }

  /** Discover a service. */
  async discover(serviceIdentifier: string): Promise<ANSRecord> {
    return this.discovery.resolve(serviceIdentifier);
  }

  /** Check server health. */
  async health(): Promise<boolean> {
    try {
      await this.transport.get("/health");
      return true;
    } catch {
      return false;
    }
  }
}
