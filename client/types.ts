export interface Message {
  id?: string | undefined;
  room?: string | undefined;
  title: string;
  duration?: number | undefined;
  payload?: any;
  reply?: string | undefined;
  broadcast?: string | undefined;
  data?: any;
  encoding?: string | undefined;
}

export type ActionFunc = (msg: Message) => string | undefined | void | Promise<string | undefined | void>;

export interface RequestOptions {
  room?: string | undefined;
  timeout?: number | undefined;
  onExpire?: (() => void) | undefined;
}

export type WsEventType = "open" | "close" | "error" | "message" | "reconnect" | "reconnecting";

export interface ReplyHandler {
  key: string;
  action?: ActionFunc | undefined;
  resolve: (value: any) => void;
  reject: (reason?: any) => void;
  expiration: number;
  expiredAction?: ((reply: ReplyHandler, done: () => void) => void) | undefined;
}

export interface WsOptions {
  protocols?: string | string[] | undefined;
  reconnectInterval?: number | undefined;
  maxReconnectAttempts?: number | undefined;
  autoReconnect?: boolean | undefined;
}
