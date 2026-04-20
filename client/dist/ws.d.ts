interface Reply {
    key: string;
    action: actionFunc;
    resolver: [Function, Function];
    expiration: number;
    expiredAction: (reply: Reply, done: Function) => any;
}
export declare class Ws {
    url: string;
    private protocols?;
    private isIntentionalClose;
    initialize: boolean;
    reconnectInterval: number;
    attempts: number;
    socket: WebSocket;
    usedMessage: number[];
    listeners: any[];
    actionMap: Map<string, actionFunc>;
    replyMap: Map<string, Reply>;
    constructor(url: string | URL, protocols?: string | string[]);
    on(event: "open" | "close" | "error" | "message", handler?: Function): void;
    fire(event: "open" | "close" | "error" | "reconnect" | "reconnecting", data?: any): void;
    private reconnected;
    private fireAttempt;
    private initializeListeners;
    private attemptReconnect;
    closeSafe(): void;
    sendObj(msg: Message, reply?: actionFunc, expiredCb?: Reply["expiredAction"]): Promise<unknown>;
    sendRaw(title: string, room?: string | null, data?: any, duration?: number, reply?: actionFunc, expiredCb?: Reply["expiredAction"]): Promise<unknown>;
    sendTo(room: string, title: string, data?: any, duration?: number, reply?: actionFunc, expiredCb?: Reply["expiredAction"]): Promise<unknown>;
    use(title: string, action: actionFunc): void;
    runraw(map: Map<string, actionFunc> | Map<string, Reply>, title: string, msg: Message): Promise<void>;
    run(title: string, msg: Message): Promise<void>;
}
export {};
//# sourceMappingURL=ws.d.ts.map