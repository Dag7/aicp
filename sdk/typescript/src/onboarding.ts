import type { Transport } from "./transport.js";
import type {
  OnboardInitRequest,
  OnboardInitResponse,
  NegotiateRequest,
  NegotiateResponse,
  AuthorizeRequest,
  AuthorizeResponse,
  ProvisionRequest,
  ProvisionResponse,
  VerifiablePresentation,
  DIDAuthResponse,
  CapabilityOffer,
  OnboardingState,
} from "./types.js";
import { createProof } from "./crypto.js";

/** Orchestrates the 4-phase onboarding flow. */
export class OnboardingManager {
  constructor(private readonly transport: Transport) {}

  /** Phase 1: Initiate onboarding with identity presentation. */
  async initiate(
    agentDID: string,
    presentation: VerifiablePresentation,
    requestedCapabilities: string[],
  ): Promise<OnboardInitResponse> {
    return this.transport.post<OnboardInitResponse>("/aicp/v1/onboard", {
      agent_did: agentDID,
      presentation,
      requested_capabilities: requestedCapabilities,
    } satisfies OnboardInitRequest);
  }

  /** Phase 2: Negotiate capabilities with challenge response. */
  async negotiate(
    sessionID: string,
    offeredCapabilities: CapabilityOffer[],
    challengeResponse?: DIDAuthResponse,
  ): Promise<NegotiateResponse> {
    return this.transport.post<NegotiateResponse>(
      `/aicp/v1/onboard/${sessionID}/negotiate`,
      {
        session_id: sessionID,
        challenge_response: challengeResponse,
        offered_capabilities: offeredCapabilities,
      } satisfies NegotiateRequest,
    );
  }

  /** Phase 3: Authorize payment. */
  async authorize(
    sessionID: string,
    paymentToken: string,
  ): Promise<AuthorizeResponse> {
    return this.transport.post<AuthorizeResponse>(
      `/aicp/v1/onboard/${sessionID}/authorize`,
      {
        session_id: sessionID,
        payment_token: paymentToken,
      } satisfies AuthorizeRequest,
    );
  }

  /** Phase 4: Provision the agent's account. */
  async provision(sessionID: string): Promise<ProvisionResponse> {
    return this.transport.post<ProvisionResponse>(
      `/aicp/v1/onboard/${sessionID}/provision`,
      {
        session_id: sessionID,
      } satisfies ProvisionRequest,
    );
  }

  /**
   * Run the full 4-phase onboarding flow.
   *
   * This helper orchestrates all four phases, handling
   * challenge-response automatically if a private key is provided.
   */
  async runFullFlow(params: {
    agentDID: string;
    presentation: VerifiablePresentation;
    requestedCapabilities: string[];
    privateKey: Uint8Array;
    paymentToken: string;
  }): Promise<{
    state: OnboardingState;
    accountID: string;
    credentials: Record<string, string>;
  }> {
    // Phase 1: Initiate
    const initResp = await this.initiate(
      params.agentDID,
      params.presentation,
      params.requestedCapabilities,
    );

    // Build challenge response if server issued a challenge.
    let challengeResponse: DIDAuthResponse | undefined;
    if (initResp.challenge) {
      const proof = createProof(
        { challenge: initResp.challenge.challenge, did: params.agentDID },
        params.privateKey,
        `${params.agentDID}#key-1`,
        "authentication",
        {
          challenge: initResp.challenge.challenge,
          domain: initResp.challenge.domain,
        },
      );

      challengeResponse = {
        type: "DIDAuthResponse",
        challenge: initResp.challenge.challenge,
        did: params.agentDID,
        proof,
      };
    }

    // Phase 2: Negotiate
    await this.negotiate(
      initResp.session_id,
      params.requestedCapabilities.map((c) => ({ name: c })),
      challengeResponse,
    );

    // Phase 3: Authorize
    await this.authorize(initResp.session_id, params.paymentToken);

    // Phase 4: Provision
    const provisionResp = await this.provision(initResp.session_id);

    return {
      state: provisionResp.state,
      accountID: provisionResp.account_id,
      credentials: provisionResp.credentials,
    };
  }
}
