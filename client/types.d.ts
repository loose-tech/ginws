export * from "./types.js";

declare global {
  interface Message {
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

  type actionFunc = (msg: Message) => string | undefined | void | Promise<string | undefined | void>;
}
