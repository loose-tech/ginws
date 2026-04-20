// TODO: reply and temporary listeners that would be deleted right after it is done

import { gen12p3 } from "./helper";
interface Reply {
  key: string;
  action: actionFunc;
  resolver: [Function, Function];
  expiration: number;
  expiredAction: (reply: Reply, done: Function) => any;
}
export class Ws {
  url: string;
  private protocols?: string | string[];
  private isIntentionalClose: boolean = false;
  initialize: boolean = false;
  reconnectInterval: number = 5000; // Try reconnecting every 5s
  attempts = 0;
  socket: WebSocket;
  /// for everything that needs a reply do it in fifo
  usedMessage: number[] = [];
  listeners: any[] = [];
  actionMap: Map<string, actionFunc> = new Map();
  replyMap: Map<string, Reply> = new Map();

  constructor(url: string | URL, protocols?: string | string[]) {
    this.socket = new WebSocket(url, protocols);
    this.url = url.toString();
    this.protocols = protocols;
    this.initializeListeners();
  }

  on(
    event: "open" | "close" | "error" | "message",
    handler: Function = () => { },
  ) {
    this.listeners.push({ event, handler });
  }

  fire(
    event: "open" | "close" | "error" | "reconnect" | "reconnecting",
    data?: any,
  ) {
    const listeners = this.listeners.filter((l) => l.event === event);
    console.log(this.listeners, listeners);
    listeners.forEach((l) => l.handler(data));
  }

  private reconnected() {
    const self = this;
    this.fire("reconnect", self);
  }
  private fireAttempt() {
    const self = this;
    self.attempts++;
    this.fire("reconnecting", self);
  }

  private initializeListeners() {
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

    this.socket.addEventListener("message", (data: any) => {
      try {
        console.log({ data });
        const msg: Message = JSON.parse(data.data as any as string);
        this.run(msg.title, msg);
      } catch (err) {
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

  private attemptReconnect() {
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
  public closeSafe() {
    this.isIntentionalClose = true;
    this.socket.close();
    this.initialize = false;
  }

  // message them
  sendObj(
    msg: Message,
    reply?: actionFunc,
    expiredCb?: Reply["expiredAction"],
  ) {
    return new Promise((resolve, reject) => {
      if (!!reply) {
        const id = gen12p3();
        msg.reply = id;
        const json = JSON.stringify(msg);
        let timeout: any;

        const replyer: Reply = {
          action: reply.bind(this),
          key: id,
          resolver: [
            (arg: any) => {
              if (timeout) clearTimeout(timeout);
              resolve(arg);
            },
            (arg: any) => {
              if (timeout) clearTimeout(timeout);
              reject(arg);
            },
          ],
          expiration: Date.now() + msg.duration,
          expiredAction: expiredCb as any,
        };
        this.replyMap.set(id, replyer);
        this.socket.send(json);
        if (msg.duration > 0) {
          timeout = setTimeout(() => {
            console.warn("expired call... after: " + msg.duration);
            expiredCb?.(replyer, (...args: any[]) => {
              clearTimeout(timeout);
              resolve(null);
            });
          }, msg.duration);
        }
      } else {
        const json = JSON.stringify(msg);
        this.socket.send(json);
        resolve(null);
      }
    });
  }

  // send
  sendRaw(
    title: string,
    room?: string | null,
    data?: any,
    duration?: number,
    reply?: actionFunc,
    expiredCb?: Reply["expiredAction"],
  ) {
    const msg: Message = {
      duration: duration || 0,
      id: null as any,
      payload: data,
      room: room || (null as any),
      title,
    } as any;

    return this.sendObj(msg, reply, expiredCb);
  }

  // send to with room
  sendTo(
    room: string,
    title: string,
    data?: any,
    duration?: number,
    reply?: actionFunc,
    expiredCb?: Reply["expiredAction"],
  ) {
    return this.sendRaw(title, room, data, duration, reply, expiredCb);
  }

  use(title: string, action: actionFunc) {
    this.actionMap.set(title, action);
  }

  async runraw(
    map: Map<string, actionFunc> | Map<string, Reply>,
    title: string,
    msg: Message,
  ) {
    let action: actionFunc;
    for (; ;) {
      if (!title) {
        return;
      }
      action = map.get(title) as actionFunc;
      if (!action) {
        throw new Error(`action ${title} is not available`);
      }
      if (typeof action === "function") {
        title = action(msg) as string;
      } else {
        const reply = action as Reply;
        try {
          const f: (msg: Message) => Promise<any> = reply.action as any;
          const res = await f(msg);
          if (reply.expiration < Date.now()) {
            reply.resolver[0]("recieved but expired expired");
            console.warn("has expired job");
          } else {
            reply.resolver[0](res);
          }
          this.replyMap.delete(title);
          return;
        } catch (err) {
          throw err;
        }
      }
    }
  }

  // run with reply
  run(title: string, msg: Message) {
    if (msg.reply) {
      console.log("msg");
      return this.runraw(this.replyMap, msg.reply, msg);
    } else {
      return this.runraw(this.actionMap, title, msg);
    }
  }
}
