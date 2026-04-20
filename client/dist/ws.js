// TODO: reply and temporary listeners that would be deleted right after it is done
import { gen12p3 } from "./helper";
export class Ws {
    url;
    protocols;
    isIntentionalClose = false;
    initialize = false;
    reconnectInterval = 5000; // Try reconnecting every 5s
    attempts = 0;
    socket;
    /// for everything that needs a reply do it in fifo
    usedMessage = [];
    listeners = [];
    actionMap = new Map();
    replyMap = new Map();
    constructor(url, protocols) {
        this.socket = new WebSocket(url, protocols);
        this.url = url.toString();
        this.protocols = protocols;
        this.initializeListeners();
    }
    on(event, handler = () => { }) {
        this.listeners.push({ event, handler });
    }
    fire(event, data) {
        const listeners = this.listeners.filter((l) => l.event === event);
        console.log(this.listeners, listeners);
        listeners.forEach((l) => l.handler(data));
    }
    reconnected() {
        const self = this;
        this.fire("reconnect", self);
    }
    fireAttempt() {
        const self = this;
        self.attempts++;
        this.fire("reconnecting", self);
    }
    initializeListeners() {
        console.debug("initializing ws");
        this.socket.addEventListener("open", () => {
            this.fire("open");
            console.log("Connected!");
            this.isIntentionalClose = false;
            this.attempts = 0;
            if (this.initialize) {
                this.reconnected();
            }
        });
        this.socket.addEventListener("message", (data) => {
            try {
                console.log({ data });
                const msg = JSON.parse(data.data);
                this.run(msg.title, msg);
            }
            catch (err) {
                console.log(err);
                throw err;
            }
        });
        this.socket.addEventListener("close", (event) => {
            console.log("Closed.");
            this.fire("close");
            if (!this.isIntentionalClose) {
                this.attemptReconnect();
            }
        });
    }
    attemptReconnect() {
        console.log(`Attempting reconnect in ${this.reconnectInterval / 1000}s...`);
        setTimeout(() => {
            console.log("Reconnecting...");
            this.fireAttempt();
            const newSocket = new WebSocket(this.url, this.protocols);
            this.socket = newSocket;
            this.initializeListeners();
        }, this.reconnectInterval);
    }
    // Method to close properly
    closeSafe() {
        this.isIntentionalClose = true;
        this.socket.close();
        this.initialize = false;
    }
    // message them
    sendObj(msg, reply, expiredCb) {
        return new Promise((resolve, reject) => {
            if (!!reply) {
                const id = gen12p3();
                msg.reply = id;
                const json = JSON.stringify(msg);
                let timeout;
                const replyer = {
                    action: reply.bind(this),
                    key: id,
                    resolver: [
                        (arg) => {
                            if (timeout)
                                clearTimeout(timeout);
                            resolve(arg);
                        },
                        (arg) => {
                            if (timeout)
                                clearTimeout(timeout);
                            reject(arg);
                        },
                    ],
                    expiration: Date.now() + msg.duration,
                    expiredAction: expiredCb,
                };
                this.replyMap.set(id, replyer);
                this.socket.send(json);
                if (msg.duration > 0) {
                    timeout = setTimeout(() => {
                        console.warn("expired call... after: " + msg.duration);
                        expiredCb?.(replyer, (...args) => {
                            clearTimeout(timeout);
                            resolve(null);
                        });
                    }, msg.duration);
                }
            }
            else {
                const json = JSON.stringify(msg);
                this.socket.send(json);
                resolve(null);
            }
        });
    }
    // send
    sendRaw(title, room, data, duration, reply, expiredCb) {
        const msg = {
            duration: duration || 0,
            id: null,
            payload: data,
            room: room || null,
            title,
        };
        return this.sendObj(msg, reply, expiredCb);
    }
    // send to with room
    sendTo(room, title, data, duration, reply, expiredCb) {
        return this.sendRaw(title, room, data, duration, reply, expiredCb);
    }
    use(title, action) {
        this.actionMap.set(title, action);
    }
    async runraw(map, title, msg) {
        let action;
        for (;;) {
            if (!title) {
                return;
            }
            action = map.get(title);
            if (!action) {
                throw new Error(`action ${title} is not available`);
            }
            if (typeof action === "function") {
                title = action(msg);
            }
            else {
                const reply = action;
                try {
                    const f = reply.action;
                    const res = await f(msg);
                    if (reply.expiration < Date.now()) {
                        reply.resolver[0]("recieved but expired expired");
                        console.warn("has expired job");
                    }
                    else {
                        reply.resolver[0](res);
                    }
                    this.replyMap.delete(title);
                    return;
                }
                catch (err) {
                    throw err;
                }
            }
        }
    }
    // run with reply
    run(title, msg) {
        if (msg.reply) {
            console.log("msg");
            return this.runraw(this.replyMap, msg.reply, msg);
        }
        else {
            return this.runraw(this.actionMap, title, msg);
        }
    }
}
//# sourceMappingURL=ws.js.map