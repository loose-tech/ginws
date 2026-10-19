import { gen12p3 } from "./helper.js";
import type {
  ActionFunc,
  Message,
  ReplyHandler,
  RequestOptions,
  WsEventType,
  WsOptions,
} from "./types.js";

export type { ActionFunc, Message, RequestOptions, WsEventType, WsOptions };

export class Ws {
  url: string;
  private protocols: string | string[] | undefined;
  private isIntentionalClose: boolean = false;
  private reconnectTimer: any = null;

  initialize: boolean = false;
  reconnectInterval: number = 2000;
  maxReconnectAttempts: number = Infinity;
  autoReconnect: boolean = true;
  attempts: number = 0;

  socket!: WebSocket;
  private outboundQueue: string[] = [];
  private listeners: Map<WsEventType, Set<(data?: any) => void>> = new Map();
  actionMap: Map<string, ActionFunc> = new Map();
  replyMap: Map<string, ReplyHandler> = new Map();

  constructor(url: string | URL, protocolsOrOptions?: string | string[] | WsOptions) {
    this.url = url.toString();

    if (
      protocolsOrOptions &&
      typeof protocolsOrOptions === "object" &&
      !Array.isArray(protocolsOrOptions)
    ) {
      const opts = protocolsOrOptions as WsOptions;
      this.protocols = opts.protocols;
      if (opts.reconnectInterval !== undefined) this.reconnectInterval = opts.reconnectInterval;
      if (opts.maxReconnectAttempts !== undefined) this.maxReconnectAttempts = opts.maxReconnectAttempts;
      if (opts.autoReconnect !== undefined) this.autoReconnect = opts.autoReconnect;
    } else {
      this.protocols = protocolsOrOptions as string | string[] | undefined;
    }

    this.connect();
  }

  // Connects or reconnects the WebSocket
  private connect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }

    try {
      this.socket = new WebSocket(this.url, this.protocols);
      this.initializeListeners();
    } catch (err) {
      this.fire("error", err);
      if (!this.isIntentionalClose && this.autoReconnect) {
        this.scheduleReconnect();
      }
    }
  }

  private initializeListeners() {
    this.socket.addEventListener("open", () => {
      const wasInitialized = this.initialize;
      this.initialize = true;
      this.isIntentionalClose = false;
      this.attempts = 0;

      this.fire("open");
      if (wasInitialized) {
        this.fire("reconnect", this);
      }

      // Flush queued messages that were called before connection opened
      this.flushOutboundQueue();
    });

    this.socket.addEventListener("message", (event: MessageEvent) => {
      try {
        const raw = typeof event.data === "string" ? event.data : new TextDecoder().decode(event.data);
        const msg: Message = JSON.parse(raw);
        this.fire("message", msg);
        this.run(msg.title, msg);
      } catch (err) {
        this.fire("error", err);
      }
    });

    this.socket.addEventListener("close", () => {
      this.fire("close");
      if (!this.isIntentionalClose && this.autoReconnect) {
        this.scheduleReconnect();
      }
    });

    this.socket.addEventListener("error", (err) => {
      this.fire("error", err);
    });
  }

  private scheduleReconnect() {
    if (this.reconnectTimer || this.isIntentionalClose) return;
    if (this.attempts >= this.maxReconnectAttempts) return;

    this.attempts++;
    this.fire("reconnecting", { attempt: this.attempts });

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectInterval);
  }

  private flushOutboundQueue() {
    while (this.outboundQueue.length > 0 && this.socket.readyState === WebSocket.OPEN) {
      const next = this.outboundQueue.shift();
      if (next) {
        this.socket.send(next);
      }
    }
  }

  private sendSafe(json: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(json);
    } else {
      this.outboundQueue.push(json);
    }
  }

  // Register event listener (open, close, error, reconnect, reconnecting, message)
  on(event: WsEventType, handler: (data?: any) => void): () => void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(handler);
    return () => this.off(event, handler);
  }

  // Remove event listener
  off(event: WsEventType, handler?: (data?: any) => void) {
    if (!handler) {
      this.listeners.delete(event);
      return;
    }
    const set = this.listeners.get(event);
    if (set) {
      set.delete(handler);
    }
  }

  // Register one-time event listener
  once(event: WsEventType, handler: (data?: any) => void): () => void {
    const unbind = this.on(event, (data) => {
      unbind();
      handler(data);
    });
    return unbind;
  }

  // Trigger event listeners
  fire(event: WsEventType, data?: any) {
    const set = this.listeners.get(event);
    if (set) {
      set.forEach((fn) => {
        try {
          fn(data);
        } catch (err) {
          console.error(`Error in ws event handler [${event}]:`, err);
        }
      });
    }
  }

  // Register a named action handler for client-side routing & chaining
  use(title: string, action: ActionFunc) {
    this.actionMap.set(title, action);
  }

  // Modern Request-Reply: send request and await server reply directly
  request<T = any>(title: string, payload?: any, options?: RequestOptions): Promise<Message & { payload: T }> {
    const duration = options?.timeout || 0;
    const room = options?.room || null;

    return this.sendRaw(title, room, payload, duration) as Promise<Message & { payload: T }>;
  }

  // Fire-and-forget message send
  send(title: string, payload?: any, room?: string): void {
    const msg: Message = {
      title,
      payload,
      room,
    };
    this.sendSafe(JSON.stringify(msg));
  }

  // Send an object message with optional reply and expiration handler
  sendObj(
    msg: Message,
    reply?: ActionFunc,
    expiredCb?: ReplyHandler["expiredAction"],
  ): Promise<any> {
    return new Promise((resolve, reject) => {
      const id = gen12p3();
      msg.reply = id;

      let timeout: any = null;
      const duration = msg.duration || 0;

      const replyer: ReplyHandler = {
        key: id,
        action: reply ? reply.bind(this) : undefined,
        resolve: (arg: any) => {
          if (timeout) clearTimeout(timeout);
          resolve(arg);
        },
        reject: (arg: any) => {
          if (timeout) clearTimeout(timeout);
          reject(arg);
        },
        expiration: duration > 0 ? Date.now() + duration : 0,
        expiredAction: expiredCb,
      };

      this.replyMap.set(id, replyer);
      this.sendSafe(JSON.stringify(msg));

      if (duration > 0) {
        timeout = setTimeout(() => {
          this.replyMap.delete(id);
          if (expiredCb) {
            expiredCb(replyer, () => {
              resolve(null);
            });
          } else {
            reject(new Error(`WebSocket request '${msg.title}' timed out after ${duration}ms`));
          }
        }, duration);
      }
    });
  }

  // Send raw message format (compatible with existing code)
  sendRaw(
    title: string,
    room?: string | null,
    data?: any,
    duration?: number,
    reply?: ActionFunc,
    expiredCb?: ReplyHandler["expiredAction"],
  ): Promise<any> {
    const msg: Message = {
      title,
      payload: data,
      room: room || undefined,
      duration: duration || 0,
    };

    return this.sendObj(msg, reply, expiredCb);
  }

  // Send to room with reply
  sendTo(
    room: string,
    title: string,
    data?: any,
    duration?: number,
    reply?: ActionFunc,
    expiredCb?: ReplyHandler["expiredAction"],
  ): Promise<any> {
    return this.sendRaw(title, room, data, duration, reply, expiredCb);
  }

  // Execute named actions sequentially with support for async functions
  async runraw(
    map: Map<string, ActionFunc> | Map<string, ReplyHandler>,
    title: string,
    msg: Message,
  ) {
    let currentTitle: string = title;
    let depth = 0;
    const maxDepth = 50;

    while (currentTitle && depth < maxDepth) {
      depth++;
      const handler = map.get(currentTitle);
      if (!handler) {
        return;
      }

      if (typeof handler === "function") {
        const next = await handler(msg);
        currentTitle = typeof next === "string" ? next : "";
      } else {
        const replyHandler = handler as ReplyHandler;
        this.replyMap.delete(currentTitle);

        try {
          if (replyHandler.action) {
            const res = await replyHandler.action(msg);
            replyHandler.resolve(res !== undefined ? res : msg);
          } else {
            if (msg.title === "error") {
              const errMsg = msg.payload?.error || "Server returned an error";
              replyHandler.reject(new Error(errMsg));
            } else {
              replyHandler.resolve(msg);
            }
          }
        } catch (err) {
          replyHandler.reject(err);
        }
        return;
      }
    }
  }

  // Dispatch incoming message to reply handlers and/or named action pipeline
  run(title: string, msg: Message) {
    if (msg.reply && this.replyMap.has(msg.reply)) {
      this.runraw(this.replyMap, msg.reply, msg);
    }
    if (title && this.actionMap.has(title)) {
      this.runraw(this.actionMap, title, msg);
    }
  }

  // Method to close connection intentionally
  public closeSafe() {
    this.isIntentionalClose = true;
    this.autoReconnect = false;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.socket) {
      this.socket.close();
    }
    this.initialize = false;
    this.outboundQueue = [];

    // Reject pending replies
    for (const reply of this.replyMap.values()) {
      reply.reject(new Error("WebSocket closed intentionally"));
    }
    this.replyMap.clear();
  }
}
