// TODO: reply and temporary listeners that would be deleted right after it is done

export class Queue {
  buffer: EventBuffer[] = [];
  vacancy: Map<number, any> = new Map();
  vacancies = 0;
  queue: QueueMessage[] = [];

  addBuffer(data: any) {
    return new Promise((resolve, reject) => {
      let id = 0;
      if (this.vacancies) {
        const keys = Array.from(this.vacancy.keys());
        id = keys[0];
        this.vacancy.set(id, id + 1);
        resolve(id);
      } else {
        this.vacancies++;
        id = this.vacancies - 1;
        this.vacancy.set(id, id + 1);
        resolve(id);
      }
      // means there has happend (unlikely)
      if (id) {
        this.buffer[id] = data;
      } else {
        reject(new Error("failed to add buffer"));
      }
    });
  }

  consumeBuffer(title: string) {
    return new Promise((resolve, reject) => {
      this.vacancies++;
      this.buffer.forEach((buf, i) => {
        // the title
        if (buf.title === title) {
          this.buffer[i] = null as any;
        }
      });
    });
  }

  cleanBuffer() {
    return Promise.resolve((this.buffer = this.buffer.filter((d) => !!d)));
  }

  addQueue(msg: QueueMessage) {
    this.queue.push(msg);
  }

  do() {
    const msg = this.queue.shift();
    if (msg !== undefined) {
      switch (msg.type) {
        case "add":
          return this.addBuffer(msg.args);
        case "consume":
          return this.consumeBuffer(msg.args[0] as string);
        default:
          /// TODO: do real work after
          return Promise.resolve(true);
      }
    }
  }

  async process() {
    for (; ;) {
      if (this.queue.length) {
        await this.do();
      } else {
        await new Promise((resolve) => {
          const tout = setTimeout(() => {
            resolve(null);
            clearTimeout(tout);
          }, 3); // safe wiat
        });
      }
    }
  }
}
