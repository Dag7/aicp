import { describe, it, expect } from "vitest";
import {
  generateKeyPair,
  getPublicKey,
  sign,
  verify,
  signJSON,
  verifyJSON,
  principalDID,
  agentDID,
  parseDID,
  isPrincipalDID,
  isAgentDID,
  didIdentifier,
  multibaseEncode,
  multibaseDecode,
  createProof,
  verifyProof,
  base58Encode,
  base58Decode,
  hash,
  hashHex,
  bytesToHex,
  hexToBytes,
  randomBytes,
} from "../src/crypto.js";

describe("Key Operations", () => {
  it("generates a valid Ed25519 key pair", () => {
    const kp = generateKeyPair();
    expect(kp.publicKey).toBeInstanceOf(Uint8Array);
    expect(kp.privateKey).toBeInstanceOf(Uint8Array);
    expect(kp.publicKey.length).toBe(32);
    expect(kp.privateKey.length).toBe(32);
  });

  it("derives public key from private key", () => {
    const kp = generateKeyPair();
    const derived = getPublicKey(kp.privateKey);
    expect(derived).toEqual(kp.publicKey);
  });

  it("generates unique key pairs", () => {
    const kp1 = generateKeyPair();
    const kp2 = generateKeyPair();
    expect(kp1.publicKey).not.toEqual(kp2.publicKey);
    expect(kp1.privateKey).not.toEqual(kp2.privateKey);
  });
});

describe("Signing and Verification", () => {
  it("signs and verifies a message", () => {
    const kp = generateKeyPair();
    const message = new TextEncoder().encode("hello AICP");
    const sig = sign(message, kp.privateKey);
    expect(sig).toBeInstanceOf(Uint8Array);
    expect(sig.length).toBe(64);
    expect(verify(sig, message, kp.publicKey)).toBe(true);
  });

  it("rejects signature with wrong key", () => {
    const kp1 = generateKeyPair();
    const kp2 = generateKeyPair();
    const message = new TextEncoder().encode("test");
    const sig = sign(message, kp1.privateKey);
    expect(verify(sig, message, kp2.publicKey)).toBe(false);
  });

  it("rejects signature with modified message", () => {
    const kp = generateKeyPair();
    const message = new TextEncoder().encode("original");
    const sig = sign(message, kp.privateKey);
    const modified = new TextEncoder().encode("modified");
    expect(verify(sig, modified, kp.publicKey)).toBe(false);
  });
});

describe("JSON Signing", () => {
  it("signs and verifies JSON objects", () => {
    const kp = generateKeyPair();
    const data = { foo: "bar", num: 42 };
    const sig = signJSON(data, kp.privateKey);
    expect(typeof sig).toBe("string");
    expect(verifyJSON(data, sig, kp.publicKey)).toBe(true);
  });

  it("rejects JSON with wrong key", () => {
    const kp1 = generateKeyPair();
    const kp2 = generateKeyPair();
    const data = { test: true };
    const sig = signJSON(data, kp1.privateKey);
    expect(verifyJSON(data, sig, kp2.publicKey)).toBe(false);
  });

  it("rejects JSON with modified data", () => {
    const kp = generateKeyPair();
    const data = { key: "value" };
    const sig = signJSON(data, kp.privateKey);
    expect(verifyJSON({ key: "changed" }, sig, kp.publicKey)).toBe(false);
  });
});

describe("DID Operations", () => {
  it("creates a valid principal DID", () => {
    const kp = generateKeyPair();
    const did = principalDID(kp.publicKey);
    expect(did).toMatch(/^did:aicp:[1-9A-HJ-NP-Za-km-z]+$/);
    expect(isPrincipalDID(did)).toBe(true);
    expect(isAgentDID(did)).toBe(false);
  });

  it("creates a valid agent DID", () => {
    const principalKP = generateKeyPair();
    const agentKP = generateKeyPair();
    const did = agentDID(principalKP.publicKey, agentKP.publicKey);
    expect(did).toMatch(
      /^did:aicp:[1-9A-HJ-NP-Za-km-z]+:[1-9A-HJ-NP-Za-km-z]+$/,
    );
    expect(isAgentDID(did)).toBe(true);
    expect(isPrincipalDID(did)).toBe(false);
  });

  it("parses a principal DID", () => {
    const kp = generateKeyPair();
    const did = principalDID(kp.publicKey);
    const parsed = parseDID(did);
    expect(parsed.method).toBe("aicp");
    expect(parsed.principalID).toBeTruthy();
    expect(parsed.agentID).toBeUndefined();
  });

  it("parses an agent DID", () => {
    const pKP = generateKeyPair();
    const aKP = generateKeyPair();
    const did = agentDID(pKP.publicKey, aKP.publicKey);
    const parsed = parseDID(did);
    expect(parsed.method).toBe("aicp");
    expect(parsed.principalID).toBeTruthy();
    expect(parsed.agentID).toBeTruthy();
  });

  it("throws on invalid DID", () => {
    expect(() => parseDID("not-a-did")).toThrow("invalid AICP DID");
    expect(() => parseDID("did:other:123")).toThrow("invalid AICP DID");
  });

  it("DID identifier is deterministic", () => {
    const kp = generateKeyPair();
    const id1 = didIdentifier(kp.publicKey);
    const id2 = didIdentifier(kp.publicKey);
    expect(id1).toBe(id2);
  });
});

describe("Multibase", () => {
  it("encodes and decodes round-trip", () => {
    const data = randomBytes(32);
    const encoded = multibaseEncode(data);
    expect(encoded.startsWith("z")).toBe(true);
    const decoded = multibaseDecode(encoded);
    expect(decoded).toEqual(data);
  });

  it("throws on unsupported multibase prefix", () => {
    expect(() => multibaseDecode("m" + "abc")).toThrow("unsupported multibase");
  });
});

describe("Base58", () => {
  it("encodes and decodes round-trip", () => {
    const data = new Uint8Array([1, 2, 3, 4, 5]);
    const encoded = base58Encode(data);
    const decoded = base58Decode(encoded);
    expect(decoded).toEqual(data);
  });

  it("handles leading zeros", () => {
    const data = new Uint8Array([0, 0, 0, 1, 2]);
    const encoded = base58Encode(data);
    expect(encoded.startsWith("111")).toBe(true);
    const decoded = base58Decode(encoded);
    expect(decoded).toEqual(data);
  });

  it("handles empty input", () => {
    expect(base58Encode(new Uint8Array(0))).toBe("");
    expect(base58Decode("")).toEqual(new Uint8Array(0));
  });

  it("throws on invalid base58 characters", () => {
    expect(() => base58Decode("0OIl")).toThrow("invalid base58 character");
  });
});

describe("Proof", () => {
  it("creates and verifies a proof", () => {
    const kp = generateKeyPair();
    const data = { message: "test" };
    const proof = createProof(data, kp.privateKey, "did:aicp:test#key-1");
    expect(proof.type).toBe("Ed25519Signature2020");
    expect(proof.verificationMethod).toBe("did:aicp:test#key-1");
    expect(proof.proofPurpose).toBe("assertionMethod");
    expect(proof.proofValue).toBeTruthy();
    expect(verifyProof(data, proof, kp.publicKey)).toBe(true);
  });

  it("creates a proof with challenge and domain", () => {
    const kp = generateKeyPair();
    const data = { challenge: "abc123" };
    const proof = createProof(
      data,
      kp.privateKey,
      "did:aicp:test#key-1",
      "authentication",
      { challenge: "abc123", domain: "example.com" },
    );
    expect(proof.proofPurpose).toBe("authentication");
    expect(proof.challenge).toBe("abc123");
    expect(proof.domain).toBe("example.com");
  });

  it("rejects proof with wrong key", () => {
    const kp1 = generateKeyPair();
    const kp2 = generateKeyPair();
    const data = { test: true };
    const proof = createProof(data, kp1.privateKey, "did:aicp:test#key-1");
    expect(verifyProof(data, proof, kp2.publicKey)).toBe(false);
  });
});

describe("Hash", () => {
  it("produces consistent SHA-256 hashes", () => {
    const data = new TextEncoder().encode("hello");
    const h1 = hash(data);
    const h2 = hash(data);
    expect(h1).toEqual(h2);
    expect(h1.length).toBe(32);
  });

  it("produces hex hash of strings", () => {
    const hex = hashHex("hello");
    expect(hex).toMatch(/^[0-9a-f]{64}$/);
    // Known SHA-256 of "hello".
    expect(hex).toBe(
      "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
    );
  });
});

describe("Hex", () => {
  it("converts bytes to hex and back", () => {
    const data = new Uint8Array([0xde, 0xad, 0xbe, 0xef]);
    expect(bytesToHex(data)).toBe("deadbeef");
    expect(hexToBytes("deadbeef")).toEqual(data);
  });
});

describe("Random", () => {
  it("generates random bytes of correct length", () => {
    const r = randomBytes(32);
    expect(r).toBeInstanceOf(Uint8Array);
    expect(r.length).toBe(32);
  });

  it("generates different bytes each time", () => {
    const r1 = randomBytes(16);
    const r2 = randomBytes(16);
    expect(r1).not.toEqual(r2);
  });
});
