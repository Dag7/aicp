import type { Transport } from "./transport.js";
import type { AICPEvent, EventType } from "./types.js";

type EventHandler = (event: AICPEvent) => void;

/** WebSocket client for real-time AICP event subscriptions. */
export class EventClient {
  private ws: WebSocket | null = null;
  private handlers = new Map<string, Set<EventHandler>>();
  private wildcardHandlers = new Set<EventHandler>();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private subscriptions = new Set<EventType>();

  constructor(
    private readonly transport: Transport,
    private readonly wsURL?: string,
  ) {}

  /** Connect to the AICP WebSocket. */
  async connect(): Promise<void> {
    // Get a ticket via the REST API.
    const ticketResp = await this.transport.post<{ ticket: string }>(
      "/aicp/v1/ws/ticket",
    );

    const base = this.wsURL ?? this.transport.getBaseURL().replace(/^http/, "ws");
    const url = `${base}/aicp/v1/ws?ticket=${encodeURIComponent(ticketResp.ticket)}`;

    return new Promise<void>((resolve, reject) => {
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        this.reconnectAttempts = 0;
        // Re-subscribe to any existing subscriptions.
        for (const eventType of this.subscriptions) {
          this.sendSubscribe(eventType);
        }
        resolve();
      };

      this.ws.onmessage = (msg) => {
        try {
          const event = JSON.parse(String(msg.data)) as AICPEvent;
          this.dispatch(event);
        } catch {
          // Ignore malformed messages.
        }
      };

      this.ws.onclose = () => {
        this.scheduleReconnect();
      };

      this.ws.onerror = () => {
        if (this.reconnectAttempts === 0) {
          reject(new Error("WebSocket connection failed"));
        }
      };
    });
  }

  /** Disconnect from the WebSocket. */
  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.maxReconnectAttempts = 0; // Prevent reconnection.
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  /** Subscribe to a specific event type. */
  on(eventType: EventType, handler: EventHandler): () => void {
    this.subscriptions.add(eventType);

    let handlers = this.handlers.get(eventType);
    if (!handlers) {
      handlers = new Set();
      this.handlers.set(eventType, handlers);
    }
    handlers.add(handler);

    // If already connected, send subscription message.
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.sendSubscribe(eventType);
    }

    // Return unsubscribe function.
    return () => {
      handlers!.delete(handler);
      if (handlers!.size === 0) {
        this.handlers.delete(eventType);
        this.subscriptions.delete(eventType);
      }
    };
  }

  /** Subscribe to all events. */
  onAll(handler: EventHandler): () => void {
    this.wildcardHandlers.add(handler);
    return () => {
      this.wildcardHandlers.delete(handler);
    };
  }

  /** Check if the WebSocket is connected. */
  get connected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  private dispatch(event: AICPEvent): void {
    // Type-specific handlers.
    const handlers = this.handlers.get(event.type);
    if (handlers) {
      for (const h of handlers) {
        try {
          h(event);
        } catch {
          // Handler errors don't propagate.
        }
      }
    }

    // Wildcard handlers.
    for (const h of this.wildcardHandlers) {
      try {
        h(event);
      } catch {
        // Handler errors don't propagate.
      }
    }
  }

  private sendSubscribe(eventType: EventType): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(
        JSON.stringify({ type: "subscribe", event_type: eventType }),
      );
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) return;
    const delay = Math.min(1000 * 2 ** this.reconnectAttempts, 30_000);
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => {
      this.connect().catch(() => {
        // Reconnect failure handled by onclose.
      });
    }, delay);
  }
}
