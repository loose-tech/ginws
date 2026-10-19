import { gen12p3 } from "./helper.js";
export class Ws {
    url;
    protocols;
    isIntentionalClose = false;
    reconnectTimer = null;
    initialize = false;
    reconnectInterval = 2000;
    maxReconnectAttempts = Infinity;
    autoReconnect = true;
    attempts = 0;
    socket;
    outboundQueue = [];
    listeners = new Map();
    actionMap = new Map();
    replyMap = new Map();
    constructor(url, protocolsOrOptions) {
        this.url = url.toString();
        if (protocolsOrOptions &&
            typeof protocolsOrOptions === "object" &&
            !Array.isArray(protocolsOrOptions)) {
            const opts = protocolsOrOptions;
            this.protocols = opts.protocols;
            if (opts.reconnectInterval !== undefined)
                this.reconnectInterval = opts.reconnectInterval;
            if (opts.maxReconnectAttempts !== undefined)
                this.maxReconnectAttempts = opts.maxReconnectAttempts;
            if (opts.autoReconnect !== undefined)
                this.autoReconnect = opts.autoReconnect;
        }
        else {
            this.protocols = protocolsOrOptions;
        }
        this.connect();
    }
    // Connects or reconnects the WebSocket
    connect() {
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        try {
            this.socket = new WebSocket(this.url, this.protocols);
            this.initializeListeners();
        }
        catch (err) {
            this.fire("error", err);
            if (!this.isIntentionalClose && this.autoReconnect) {
                this.scheduleReconnect();
            }
        }
    }
    initializeListeners() {
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
        this.socket.addEventListener("message", (event) => {
            try {
                const raw = typeof event.data === "string" ? event.data : new TextDecoder().decode(event.data);
                const msg = JSON.parse(raw);
                this.fire("message", msg);
                this.run(msg.title, msg);
            }
            catch (err) {
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
    scheduleReconnect() {
        if (this.reconnectTimer || this.isIntentionalClose)
            return;
        if (this.attempts >= this.maxReconnectAttempts)
            return;
        this.attempts++;
        this.fire("reconnecting", { attempt: this.attempts });
        this.reconnectTimer = setTimeout(() => {
            this.reconnectTimer = null;
            this.connect();
        }, this.reconnectInterval);
    }
    flushOutboundQueue() {
        while (this.outboundQueue.length > 0 && this.socket.readyState === WebSocket.OPEN) {
            const next = this.outboundQueue.shift();
            if (next) {
                this.socket.send(next);
            }
        }
    }
    sendSafe(json) {
        if (this.socket && this.socket.readyState === WebSocket.OPEN) {
            this.socket.send(json);
        }
        else {
            this.outboundQueue.push(json);
        }
    }
    // Register event listener (open, close, error, reconnect, reconnecting, message)
    on(event, handler) {
        if (!this.listeners.has(event)) {
            this.listeners.set(event, new Set());
        }
        this.listeners.get(event).add(handler);
        return () => this.off(event, handler);
    }
    // Remove event listener
    off(event, handler) {
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
    once(event, handler) {
        const unbind = this.on(event, (data) => {
            unbind();
            handler(data);
        });
        return unbind;
    }
    // Trigger event listeners
    fire(event, data) {
        const set = this.listeners.get(event);
        if (set) {
            set.forEach((fn) => {
                try {
                    fn(data);
                }
                catch (err) {
                    console.error(`Error in ws event handler [${event}]:`, err);
                }
            });
        }
    }
    // Register a named action handler for client-side routing & chaining
    use(title, action) {
        this.actionMap.set(title, action);
    }
    // Modern Request-Reply: send request and await server reply directly
    request(title, payload, options) {
        const duration = options?.timeout || 0;
        const room = options?.room || null;
        return this.sendRaw(title, room, payload, duration);
    }
    // Fire-and-forget message send
    send(title, payload, room) {
        const msg = {
            title,
            payload,
            room,
        };
        this.sendSafe(JSON.stringify(msg));
    }
    // Send an object message with optional reply and expiration handler
    sendObj(msg, reply, expiredCb) {
        return new Promise((resolve, reject) => {
            const id = gen12p3();
            msg.reply = id;
            let timeout = null;
            const duration = msg.duration || 0;
            const replyer = {
                key: id,
                action: reply ? reply.bind(this) : undefined,
                resolve: (arg) => {
                    if (timeout)
                        clearTimeout(timeout);
                    resolve(arg);
                },
                reject: (arg) => {
                    if (timeout)
                        clearTimeout(timeout);
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
                    }
                    else {
                        reject(new Error(`WebSocket request '${msg.title}' timed out after ${duration}ms`));
                    }
                }, duration);
            }
        });
    }
    // Send raw message format (compatible with existing code)
    sendRaw(title, room, data, duration, reply, expiredCb) {
        const msg = {
            title,
            payload: data,
            room: room || undefined,
            duration: duration || 0,
        };
        return this.sendObj(msg, reply, expiredCb);
    }
    // Send to room with reply
    sendTo(room, title, data, duration, reply, expiredCb) {
        return this.sendRaw(title, room, data, duration, reply, expiredCb);
    }
    // Execute named actions sequentially with support for async functions
    async runraw(map, title, msg) {
        let currentTitle = title;
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
            }
            else {
                const replyHandler = handler;
                this.replyMap.delete(currentTitle);
                try {
                    if (replyHandler.action) {
                        const res = await replyHandler.action(msg);
                        replyHandler.resolve(res !== undefined ? res : msg);
                    }
                    else {
                        if (msg.title === "error") {
                            const errMsg = msg.payload?.error || "Server returned an error";
                            replyHandler.reject(new Error(errMsg));
                        }
                        else {
                            replyHandler.resolve(msg);
                        }
                    }
                }
                catch (err) {
                    replyHandler.reject(err);
                }
                return;
            }
        }
    }
    // Dispatch incoming message to reply handlers and/or named action pipeline
    run(title, msg) {
        if (msg.reply && this.replyMap.has(msg.reply)) {
            this.runraw(this.replyMap, msg.reply, msg);
        }
        if (title && this.actionMap.has(title)) {
            this.runraw(this.actionMap, title, msg);
        }
    }
    // Method to close connection intentionally
    closeSafe() {
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
//# sourceMappingURL=ws.js.map