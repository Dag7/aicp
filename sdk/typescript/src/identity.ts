import {
  generateKeyPair,
  getPublicKey,
  principalDID,
  agentDID,
  didIdentifier,
  signJSON,
  multibaseEncode,
  createProof,
  verifyProof,
  multibaseDecode,
  parseDID,
  isPrincipalDID,
  isAgentDID,
} from "./crypto.js";
import type { Transport } from "./transport.js";
import type {
  KeyPair,
  DIDResolutionResult,
  VerifiableCredential,
  VerifiablePresentation,
  VerifyRequest,
  VerifyResponse,
  Proof,
} from "./types.js";

/** Manages DID identity operations for the SDK. */
export class IdentityManager {
  constructor(private readonly transport: Transport) {}

  /** Generate a new principal identity (key pair + DID). */
  generatePrincipal(): { keyPair: KeyPair; did: string } {
    const keyPair = generateKeyPair();
    return { keyPair, did: principalDID(keyPair.publicKey) };
  }

  /** Generate a new agent identity under a principal. */
  generateAgent(principalKeyPair: KeyPair): {
    keyPair: KeyPair;
    did: string;
    principalDID: string;
  } {
    const agentKP = generateKeyPair();
    const pDID = principalDID(principalKeyPair.publicKey);
    return {
      keyPair: agentKP,
      did: agentDID(principalKeyPair.publicKey, agentKP.publicKey),
      principalDID: pDID,
    };
  }

  /** Resolve a DID via the AICP server. */
  async resolve(did: string): Promise<DIDResolutionResult> {
    return this.transport.post<DIDResolutionResult>("/aicp/v1/verify", {
      did,
    } satisfies VerifyRequest);
  }

  /** Verify a DID and optionally a verifiable presentation. */
  async verify(
    did: string,
    presentation?: VerifiablePresentation,
  ): Promise<VerifyResponse> {
    return this.transport.post<VerifyResponse>("/aicp/v1/verify", {
      did,
      presentation,
    } satisfies VerifyRequest);
  }

  /** Create a signed DID-Auth proof (for challenge-response). */
  createDIDAuthProof(
    privateKey: Uint8Array,
    did: string,
    challenge: string,
    domain: string,
  ): Proof {
    return createProof(
      { challenge, did },
      privateKey,
      `${did}#key-1`,
      "authentication",
      { challenge, domain },
    );
  }

  /** Build a verifiable presentation wrapping credentials. */
  buildPresentation(
    holderDID: string,
    credentials: VerifiableCredential[],
    privateKey: Uint8Array,
  ): VerifiablePresentation {
    const presentation: Omit<VerifiablePresentation, "proof"> = {
      "@context": [
        "https://www.w3.org/2018/credentials/v1",
        "https://aicp.dev/contexts/v1",
      ],
      type: ["VerifiablePresentation"],
      holder: holderDID,
      verifiableCredential: credentials,
    };

    const proof = createProof(
      presentation,
      privateKey,
      `${holderDID}#key-1`,
      "authentication",
    );

    return { ...presentation, proof };
  }
}

/** Static utility functions re-exported for convenience. */
export const Identity = {
  generateKeyPair,
  getPublicKey,
  principalDID,
  agentDID,
  didIdentifier,
  signJSON,
  parseDID,
  isPrincipalDID,
  isAgentDID,
  multibaseEncode,
  multibaseDecode,
  createProof,
  verifyProof,
} as const;
