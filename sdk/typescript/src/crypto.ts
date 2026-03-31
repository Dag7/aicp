import * as ed from "@noble/ed25519";
import { sha512 } from "@noble/hashes/sha512";
import { sha256 } from "@noble/hashes/sha256";
import type { KeyPair, Proof } from "./types.js";

// noble/ed25519 v2 requires setting sha512 synchronously.
ed.etc.sha512Sync = (...m) => sha512(ed.etc.concatBytes(...m));

const BASE58_ALPHABET =
  "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";

// ─── Key Operations ─────────────────────────────────────────────────────────

/** Generate an Ed25519 key pair. */
export function generateKeyPair(): KeyPair {
  const privateKey = ed.utils.randomPrivateKey();
  const publicKey = ed.getPublicKey(privateKey);
  return { publicKey, privateKey };
}

/** Derive the public key from a private key. */
export function getPublicKey(privateKey: Uint8Array): Uint8Array {
  return ed.getPublicKey(privateKey);
}

// ─── Signing ────────────────────────────────────────────────────────────────

/** Sign a message with an Ed25519 private key. Returns a 64-byte signature. */
export function sign(message: Uint8Array, privateKey: Uint8Array): Uint8Array {
  return ed.sign(message, privateKey);
}

/** Verify an Ed25519 signature. */
export function verify(
  signature: Uint8Array,
  message: Uint8Array,
  publicKey: Uint8Array,
): boolean {
  return ed.verify(signature, message, publicKey);
}

/** Sign a JSON-serializable object. Returns base58-encoded signature. */
export function signJSON(data: unknown, privateKey: Uint8Array): string {
  const bytes = new TextEncoder().encode(JSON.stringify(data));
  const hash = sha256(bytes);
  const sig = sign(hash, privateKey);
  return base58Encode(sig);
}

/** Verify a base58-encoded signature over a JSON object. */
export function verifyJSON(
  data: unknown,
  signatureBase58: string,
  publicKey: Uint8Array,
): boolean {
  const sig = base58Decode(signatureBase58);
  const bytes = new TextEncoder().encode(JSON.stringify(data));
  const hash = sha256(bytes);
  return verify(sig, hash, publicKey);
}

// ─── DID Operations ─────────────────────────────────────────────────────────

/** Create a DID identifier from a public key (base58btc of first 16 bytes of SHA-256). */
export function didIdentifier(publicKey: Uint8Array): string {
  const hash = sha256(publicKey);
  return base58Encode(hash.slice(0, 16));
}

/** Build a principal DID from a public key. */
export function principalDID(publicKey: Uint8Array): string {
  return `did:aicp:${didIdentifier(publicKey)}`;
}

/** Build an agent DID from the principal and agent public keys. */
export function agentDID(
  principalPubKey: Uint8Array,
  agentPubKey: Uint8Array,
): string {
  return `did:aicp:${didIdentifier(principalPubKey)}:${didIdentifier(agentPubKey)}`;
}

/** Parse a did:aicp DID. Returns { method, principalID, agentID? }. */
export function parseDID(did: string): {
  method: string;
  principalID: string;
  agentID?: string;
} {
  const parts = did.split(":");
  if (parts.length < 3 || parts[0] !== "did" || parts[1] !== "aicp") {
    throw new Error(`invalid AICP DID: ${did}`);
  }
  const result: { method: string; principalID: string; agentID?: string } = {
    method: "aicp",
    principalID: parts[2],
  };
  if (parts.length >= 4) {
    result.agentID = parts[3];
  }
  return result;
}

/** Check if a DID is a principal DID (no agent segment). */
export function isPrincipalDID(did: string): boolean {
  const parts = did.split(":");
  return parts.length === 3 && parts[0] === "did" && parts[1] === "aicp";
}

/** Check if a DID is an agent DID (has agent segment). */
export function isAgentDID(did: string): boolean {
  const parts = did.split(":");
  return parts.length === 4 && parts[0] === "did" && parts[1] === "aicp";
}

// ─── Multibase Encoding ─────────────────────────────────────────────────────

/** Encode bytes as multibase (base58btc with 'z' prefix). */
export function multibaseEncode(data: Uint8Array): string {
  return "z" + base58Encode(data);
}

/** Decode a multibase string (expects 'z' prefix for base58btc). */
export function multibaseDecode(encoded: string): Uint8Array {
  if (!encoded.startsWith("z")) {
    throw new Error("unsupported multibase encoding: expected 'z' prefix");
  }
  return base58Decode(encoded.slice(1));
}

// ─── Proof Construction ─────────────────────────────────────────────────────

/** Create a proof over JSON data. */
export function createProof(
  data: unknown,
  privateKey: Uint8Array,
  verificationMethodID: string,
  purpose: string = "assertionMethod",
  options?: { challenge?: string; domain?: string },
): Proof {
  const proofValue = signJSON(data, privateKey);
  const proof: Proof = {
    type: "Ed25519Signature2020",
    created: new Date().toISOString(),
    verificationMethod: verificationMethodID,
    proofPurpose: purpose,
    proofValue,
  };
  if (options?.challenge) proof.challenge = options.challenge;
  if (options?.domain) proof.domain = options.domain;
  return proof;
}

/** Verify a proof over JSON data. */
export function verifyProof(
  data: unknown,
  proof: Proof,
  publicKey: Uint8Array,
): boolean {
  return verifyJSON(data, proof.proofValue, publicKey);
}

// ─── SHA-256 ────────────────────────────────────────────────────────────────

/** Compute SHA-256 hash. */
export function hash(data: Uint8Array): Uint8Array {
  return sha256(data);
}

/** Compute SHA-256 of a string. Returns hex. */
export function hashHex(data: string): string {
  const h = sha256(new TextEncoder().encode(data));
  return bytesToHex(h);
}

// ─── Base58 ─────────────────────────────────────────────────────────────────

export function base58Encode(data: Uint8Array): string {
  if (data.length === 0) return "";

  // Count leading zeros.
  let zeros = 0;
  while (zeros < data.length && data[zeros] === 0) zeros++;

  // Convert to base58.
  const b58 = new Uint8Array(data.length * 2);
  let length = 0;

  for (let i = zeros; i < data.length; i++) {
    let carry = data[i];
    let j = 0;
    for (let k = b58.length - 1; k >= 0; k--, j++) {
      if (carry === 0 && j >= length) break;
      carry += 256 * b58[k];
      b58[k] = carry % 58;
      carry = (carry / 58) | 0;
    }
    length = j;
  }

  let result = "1".repeat(zeros);
  let started = false;
  for (const byte of b58) {
    if (!started && byte === 0) continue;
    started = true;
    result += BASE58_ALPHABET[byte];
  }
  return result;
}

export function base58Decode(str: string): Uint8Array {
  if (str.length === 0) return new Uint8Array(0);

  // Count leading '1's (zeros).
  let zeros = 0;
  while (zeros < str.length && str[zeros] === "1") zeros++;

  const b256 = new Uint8Array(str.length * 2);
  let length = 0;

  for (let i = zeros; i < str.length; i++) {
    const idx = BASE58_ALPHABET.indexOf(str[i]);
    if (idx < 0) throw new Error(`invalid base58 character: ${str[i]}`);

    let carry = idx;
    let j = 0;
    for (let k = b256.length - 1; k >= 0; k--, j++) {
      if (carry === 0 && j >= length) break;
      carry += 58 * b256[k];
      b256[k] = carry % 256;
      carry = (carry / 256) | 0;
    }
    length = j;
  }

  // Skip leading zeros in b256.
  let start = b256.length - length;
  while (start < b256.length && b256[start] === 0) start++;

  const result = new Uint8Array(zeros + (b256.length - start));
  // Leading zeros are already 0 in the Uint8Array.
  result.set(b256.subarray(start), zeros);
  return result;
}

// ─── Hex ────────────────────────────────────────────────────────────────────

export function bytesToHex(data: Uint8Array): string {
  return Array.from(data)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

export function hexToBytes(hex: string): Uint8Array {
  const len = hex.length;
  const result = new Uint8Array(len / 2);
  for (let i = 0; i < len; i += 2) {
    result[i / 2] = parseInt(hex.substring(i, i + 2), 16);
  }
  return result;
}

/** Generate cryptographically random bytes. */
export function randomBytes(length: number): Uint8Array {
  return crypto.getRandomValues(new Uint8Array(length));
}
