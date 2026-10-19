export interface QueueItem<T = any> {
  id: string | number;
  data: T;
  timestamp: number;
}

export class Queue<T = any> {
  private items: QueueItem<T>[] = [];
  private processing: boolean = false;

  enqueue(data: T, id?: string | number): QueueItem<T> {
    const item: QueueItem<T> = {
      id: id !== undefined ? id : Date.now() + Math.random(),
      data,
      timestamp: Date.now(),
    };
    this.items.push(item);
    return item;
  }

  dequeue(): QueueItem<T> | undefined {
    return this.items.shift();
  }

  peek(): QueueItem<T> | undefined {
    return this.items[0];
  }

  get length(): number {
    return this.items.length;
  }

  isEmpty(): boolean {
    return this.items.length === 0;
  }

  clear(): void {
    this.items = [];
  }

  flush(): QueueItem<T>[] {
    const all = [...this.items];
    this.items = [];
    return all;
  }

  async processEach(handler: (item: T) => Promise<void> | void): Promise<void> {
    if (this.processing) return;
    this.processing = true;
    try {
      while (this.items.length > 0) {
        const item = this.dequeue();
        if (item) {
          await handler(item.data);
        }
      }
    } finally {
      this.processing = false;
    }
  }
}
