import { describe, it, expect, beforeEach } from "vitest";
import {
  CredentialWallet,
  MemoryWalletStorage,
} from "../src/wallet.js";
import { generateKeyPair } from "../src/crypto.js";
import type { VerifiableCredential } from "../src/types.js";

describe("MemoryWalletStorage", () => {
  let storage: MemoryWalletStorage;

  beforeEach(() => {
    storage = new MemoryWalletStorage();
  });

  it("stores and retrieves values", async () => {
    await storage.set("key1", "value1");
    expect(await storage.get("key1")).toBe("value1");
  });

  it("returns null for missing keys", async () => {
    expect(await storage.get("missing")).toBeNull();
  });

  it("deletes values", async () => {
    await storage.set("key1", "value1");
    await storage.delete("key1");
    expect(await storage.get("key1")).toBeNull();
  });

  it("lists keys by prefix", async () => {
    await storage.set("cred:a", "1");
    await storage.set("cred:b", "2");
    await storage.set("key:x", "3");
    const creds = await storage.list("cred:");
    expect(creds).toHaveLength(2);
    expect(creds).toContain("cred:a");
    expect(creds).toContain("cred:b");
  });
});

describe("CredentialWallet", () => {
  let wallet: CredentialWallet;

  beforeEach(() => {
    wallet = new CredentialWallet(new MemoryWalletStorage());
  });

  describe("Key Management", () => {
    it("stores and retrieves key pairs", async () => {
      const kp = generateKeyPair();
      await wallet.storeKeyPair("did:aicp:test", kp);
      const retrieved = await wallet.getKeyPair("did:aicp:test");
      expect(retrieved).not.toBeNull();
      expect(retrieved!.publicKey).toEqual(kp.publicKey);
      expect(retrieved!.privateKey).toEqual(kp.privateKey);
    });

    it("returns null for missing key pair", async () => {
      expect(await wallet.getKeyPair("did:aicp:missing")).toBeNull();
    });

    it("deletes key pairs", async () => {
      const kp = generateKeyPair();
      await wallet.storeKeyPair("did:aicp:test", kp);
      await wallet.deleteKeyPair("did:aicp:test");
      expect(await wallet.getKeyPair("did:aicp:test")).toBeNull();
    });

    it("lists stored DIDs", async () => {
      const kp1 = generateKeyPair();
      const kp2 = generateKeyPair();
      await wallet.storeKeyPair("did:aicp:a", kp1);
      await wallet.storeKeyPair("did:aicp:b", kp2);
      const dids = await wallet.listDIDs();
      expect(dids).toHaveLength(2);
      expect(dids).toContain("did:aicp:a");
      expect(dids).toContain("did:aicp:b");
    });
  });

  describe("Credential Management", () => {
    const testCred: VerifiableCredential = {
      "@context": ["https://www.w3.org/2018/credentials/v1"],
      id: "urn:uuid:test-cred-1",
      type: ["VerifiableCredential", "AICPAgentDelegation"],
      issuer: "did:aicp:issuer",
      issuanceDate: "2025-01-01T00:00:00Z",
      credentialSubject: { id: "did:aicp:subject" },
    };

    it("stores and retrieves credentials", async () => {
      await wallet.storeCredential(testCred);
      const retrieved = await wallet.getCredential("urn:uuid:test-cred-1");
      expect(retrieved).not.toBeNull();
      expect(retrieved!.id).toBe("urn:uuid:test-cred-1");
      expect(retrieved!.issuer).toBe("did:aicp:issuer");
    });

    it("returns null for missing credential", async () => {
      expect(await wallet.getCredential("missing")).toBeNull();
    });

    it("deletes credentials", async () => {
      await wallet.storeCredential(testCred);
      await wallet.deleteCredential("urn:uuid:test-cred-1");
      expect(await wallet.getCredential("urn:uuid:test-cred-1")).toBeNull();
    });

    it("lists credential IDs", async () => {
      await wallet.storeCredential(testCred);
      const cred2 = { ...testCred, id: "urn:uuid:test-cred-2" };
      await wallet.storeCredential(cred2);
      const ids = await wallet.listCredentials();
      expect(ids).toHaveLength(2);
    });

    it("filters credentials by type", async () => {
      await wallet.storeCredential(testCred);
      const spending: VerifiableCredential = {
        ...testCred,
        id: "urn:uuid:spending-1",
        type: ["VerifiableCredential", "AICPSpendingAuthorization"],
      };
      await wallet.storeCredential(spending);

      const delegations = await wallet.getCredentialsByType(
        "AICPAgentDelegation",
      );
      expect(delegations).toHaveLength(1);
      expect(delegations[0].id).toBe("urn:uuid:test-cred-1");
    });
  });

  describe("Metadata", () => {
    it("stores and retrieves metadata", async () => {
      await wallet.storeMeta("accountID", "acc_123");
      expect(await wallet.getMeta("accountID")).toBe("acc_123");
    });

    it("returns null for missing metadata", async () => {
      expect(await wallet.getMeta("missing")).toBeNull();
    });

    it("deletes metadata", async () => {
      await wallet.storeMeta("key", "val");
      await wallet.deleteMeta("key");
      expect(await wallet.getMeta("key")).toBeNull();
    });
  });
});
