import type { Transport } from "./transport.js";
import type {
  MoneyAmount,
  PaymentTokenRequest,
  PaymentTokenResponse,
  PaymentExecuteRequest,
  PaymentExecuteResponse,
  PaymentStatusResponse,
} from "./types.js";

/** Manages payment operations (token request, execute, status). */
export class CommerceManager {
  constructor(private readonly transport: Transport) {}

  /** Request a payment token (Phase 1 of payment). */
  async requestToken(params: {
    agentDID: string;
    serviceDID: string;
    amount: MoneyAmount;
    maxAmount?: MoneyAmount;
    purpose: string;
    serviceCategory: string;
  }): Promise<PaymentTokenResponse> {
    return this.transport.post<PaymentTokenResponse>(
      "/aicp/v1/payment/token",
      {
        agent_did: params.agentDID,
        service_did: params.serviceDID,
        amount: params.amount,
        max_amount: params.maxAmount,
        purpose: params.purpose,
        service_category: params.serviceCategory,
      } satisfies PaymentTokenRequest,
    );
  }

  /** Execute a payment with a token (Phase 2 of payment). */
  async execute(token: string): Promise<PaymentExecuteResponse> {
    return this.transport.post<PaymentExecuteResponse>(
      "/aicp/v1/payment/execute",
      { token } satisfies PaymentExecuteRequest,
    );
  }

  /** Check payment status. */
  async getStatus(paymentID: string): Promise<PaymentStatusResponse> {
    return this.transport.get<PaymentStatusResponse>(
      `/aicp/v1/payment/status/${encodeURIComponent(paymentID)}`,
    );
  }

  /**
   * Convenience: request a token and immediately execute the payment.
   * Returns the execution response.
   */
  async pay(params: {
    agentDID: string;
    serviceDID: string;
    amount: MoneyAmount;
    purpose: string;
    serviceCategory: string;
  }): Promise<PaymentExecuteResponse> {
    const tokenResp = await this.requestToken(params);
    return this.execute(tokenResp.token);
  }
}
