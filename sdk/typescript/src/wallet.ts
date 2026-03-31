import type { KeyPair, VerifiableCredential } from "./types.js";

/** Storage backend interface for the credential wallet. */
export interface WalletStorage {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  delete(key: string): Promise<void>;
  list(prefix: string): Promise<string[]>;
}

/** In-memory wallet storage (for testing or ephemeral agents). */
export class MemoryWalletStorage implements WalletStorage {
  private store = new Map<string, string>();

  async get(key: string): Promise<string | null> {
    return this.store.get(key) ?? null;
  }

  async set(key: string, value: string): Promise<void> {
    this.store.set(key, value);
  }

  async delete(key: string): Promise<void> {
    this.store.delete(key);
  }

  async list(prefix: string): Promise<string[]> {
    return Array.from(this.store.keys()).filter((k) => k.startsWith(prefix));
  }
}

/** File-system wallet storage (Node.js only). */
export class FileWalletStorage implements WalletStorage {
  constructor(private readonly basePath: string) {}

  private async getFS() {
    return await import("node:fs/promises");
  }

  private async getPath() {
    return await import("node:path");
  }

  async get(key: string): Promise<string | null> {
    const fs = await this.getFS();
    const path = await this.getPath();
    try {
      return await fs.readFile(
        path.join(this.basePath, encodeURIComponent(key)),
        "utf-8",
      );
    } catch {
      return null;
    }
  }

  async set(key: string, value: string): Promise<void> {
    const fs = await this.getFS();
    const path = await this.getPath();
    await fs.mkdir(this.basePath, { recursive: true });
    await fs.writeFile(
      path.join(this.basePath, encodeURIComponent(key)),
      value,
      "utf-8",
    );
  }

  async delete(key: string): Promise<void> {
    const fs = await this.getFS();
    const path = await this.getPath();
    try {
      await fs.unlink(path.join(this.basePath, encodeURIComponent(key)));
    } catch {
      // Ignore if doesn't exist.
    }
  }

  async list(prefix: string): Promise<string[]> {
    const fs = await this.getFS();
    try {
      const entries = await fs.readdir(this.basePath);
      return entries
        .map((e) => decodeURIComponent(e))
        .filter((k) => k.startsWith(prefix));
    } catch {
      return [];
    }
  }
}

/** Credential wallet: stores keys and verifiable credentials. */
export class CredentialWallet {
  constructor(private readonly storage: WalletStorage) {}

  // ─── Key Management ─────────────────────────────────────────────────

  /** Store a key pair. */
  async storeKeyPair(did: string, keyPair: KeyPair): Promise<void> {
    await this.storage.set(
      `key:${did}`,
      JSON.stringify({
        publicKey: Array.from(keyPair.publicKey),
        privateKey: Array.from(keyPair.privateKey),
      }),
    );
  }

  /** Retrieve a key pair by DID. */
  async getKeyPair(did: string): Promise<KeyPair | null> {
    const raw = await this.storage.get(`key:${did}`);
    if (!raw) return null;
    const data = JSON.parse(raw) as {
      publicKey: number[];
      privateKey: number[];
    };
    return {
      publicKey: new Uint8Array(data.publicKey),
      privateKey: new Uint8Array(data.privateKey),
    };
  }

  /** Delete a key pair. */
  async deleteKeyPair(did: string): Promise<void> {
    await this.storage.delete(`key:${did}`);
  }

  /** List all stored DIDs. */
  async listDIDs(): Promise<string[]> {
    const keys = await this.storage.list("key:");
    return keys.map((k) => k.slice(4)); // Remove "key:" prefix.
  }

  // ─── Credential Management ──────────────────────────────────────────

  /** Store a verifiable credential. */
  async storeCredential(credential: VerifiableCredential): Promise<void> {
    await this.storage.set(
      `cred:${credential.id}`,
      JSON.stringify(credential),
    );
  }

  /** Retrieve a credential by ID. */
  async getCredential(id: string): Promise<VerifiableCredential | null> {
    const raw = await this.storage.get(`cred:${id}`);
    if (!raw) return null;
    return JSON.parse(raw) as VerifiableCredential;
  }

  /** Delete a credential. */
  async deleteCredential(id: string): Promise<void> {
    await this.storage.delete(`cred:${id}`);
  }

  /** List all stored credential IDs. */
  async listCredentials(): Promise<string[]> {
    const keys = await this.storage.list("cred:");
    return keys.map((k) => k.slice(5)); // Remove "cred:" prefix.
  }

  /** Get all credentials of a specific type. */
  async getCredentialsByType(
    type: string,
  ): Promise<VerifiableCredential[]> {
    const ids = await this.listCredentials();
    const results: VerifiableCredential[] = [];
    for (const id of ids) {
      const cred = await this.getCredential(id);
      if (cred && cred.type.includes(type)) {
        results.push(cred);
      }
    }
    return results;
  }

  // ─── Metadata ───────────────────────────────────────────────────────

  /** Store arbitrary metadata (e.g., account IDs, service tokens). */
  async storeMeta(key: string, value: string): Promise<void> {
    await this.storage.set(`meta:${key}`, value);
  }

  /** Retrieve metadata. */
  async getMeta(key: string): Promise<string | null> {
    return this.storage.get(`meta:${key}`);
  }

  /** Delete metadata. */
  async deleteMeta(key: string): Promise<void> {
    await this.storage.delete(`meta:${key}`);
  }
}
