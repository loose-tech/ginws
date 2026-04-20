interface Message {
  id: string;
  room: string;
  title: string;
  duration: number;
  payload: any[];
  // to tell server we want reply
  reply: string;
}

// the function actions to be managed on every possible message
type actionFunc = (msg: Message) => string | undefined;

interface EventBuffer {
  title: string;
  expiry: number;
}

interface QueueMessage {
  type: "add" | "consume";
  args: any[];
}
