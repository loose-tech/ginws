import type { ActionFunc, Message, ReplyHandler, RequestOptions, WsEventType, WsOptions } from "./types.js";
export type { ActionFunc, Message, RequestOptions, WsEventType, WsOptions };
export declare class Ws {
    url: string;
    private protocols;
    private isIntentionalClose;
    private reconnectTimer;
    initialize: boolean;
    reconnectInterval: number;
    maxReconnectAttempts: number;
    autoReconnect: boolean;
    attempts: number;
    socket: WebSocket;
    private outboundQueue;
    private listeners;
    actionMap: Map<string, ActionFunc>;
    replyMap: Map<string, ReplyHandler>;
    constructor(url: string | URL, protocolsOrOptions?: string | string[] | WsOptions);
    private connect;
    private initializeListeners;
    private scheduleReconnect;
    private flushOutboundQueue;
    private sendSafe;
    on(event: WsEventType, handler: (data?: any) => void): () => void;
    off(event: WsEventType, handler?: (data?: any) => void): void;
    once(event: WsEventType, handler: (data?: any) => void): () => void;
    fire(event: WsEventType, data?: any): void;
    use(title: string, action: ActionFunc): void;
    request<T = any>(title: string, payload?: any, options?: RequestOptions): Promise<Message & {
        payload: T;
    }>;
    send(title: string, payload?: any, room?: string): void;
    sendObj(msg: Message, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>;
    sendRaw(title: string, room?: string | null, data?: any, duration?: number, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>;
    sendTo(room: string, title: string, data?: any, duration?: number, reply?: ActionFunc, expiredCb?: ReplyHandler["expiredAction"]): Promise<any>;
    runraw(map: Map<string, ActionFunc> | Map<string, ReplyHandler>, title: string, msg: Message): Promise<void>;
    run(title: string, msg: Message): void;
    closeSafe(): void;
}
//# sourceMappingURL=ws.d.ts.map