import type { AICPClientConfig, APIResponse, APIError } from "./types.js";

/** Error thrown when an API request fails. */
export class AICPError extends Error {
  constructor(
    public readonly statusCode: number,
    public readonly code: string,
    message: string,
    public readonly detail?: string,
  ) {
    super(message);
    this.name = "AICPError";
  }

  static fromAPIError(statusCode: number, err: APIError): AICPError {
    return new AICPError(statusCode, err.code, err.message, err.detail);
  }
}

/** HTTP transport for AICP API calls. */
export class Transport {
  private readonly baseURL: string;
  private readonly timeout: number;
  private readonly fetchFn: typeof globalThis.fetch;
  private authToken?: string;

  constructor(config: AICPClientConfig) {
    this.baseURL = config.serverURL.replace(/\/+$/, "");
    this.timeout = config.timeout ?? 30_000;
    this.fetchFn = config.fetch ?? globalThis.fetch.bind(globalThis);
    this.authToken = config.authToken;
  }

  /** Update the auth token (e.g., after DID-Auth). */
  setAuthToken(token: string): void {
    this.authToken = token;
  }

  /** GET request. */
  async get<T>(path: string): Promise<T> {
    return this.request<T>("GET", path);
  }

  /** POST request with JSON body. */
  async post<T>(path: string, body?: unknown): Promise<T> {
    return this.request<T>("POST", path, body);
  }

  /** Raw request returning the full APIResponse envelope. */
  async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseURL}${path}`;
    const headers: Record<string, string> = {
      Accept: "application/json",
    };

    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
    }
    if (this.authToken) {
      headers["Authorization"] = `Bearer ${this.authToken}`;
    }

    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeout);

    try {
      const resp = await this.fetchFn(url, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      const json = (await resp.json()) as APIResponse<T>;

      if (!resp.ok || !json.success) {
        if (json.error) {
          throw AICPError.fromAPIError(resp.status, json.error);
        }
        throw new AICPError(
          resp.status,
          "UNKNOWN_ERROR",
          `HTTP ${resp.status}: ${resp.statusText}`,
        );
      }

      return json.data as T;
    } catch (err) {
      if (err instanceof AICPError) throw err;
      if (err instanceof DOMException && err.name === "AbortError") {
        throw new AICPError(0, "TIMEOUT", `request timed out after ${this.timeout}ms`);
      }
      throw new AICPError(0, "NETWORK_ERROR", String(err));
    } finally {
      clearTimeout(timer);
    }
  }

  /** Get the base URL. */
  getBaseURL(): string {
    return this.baseURL;
  }
}
