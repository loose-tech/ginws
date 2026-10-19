export class Queue {
    items = [];
    processing = false;
    enqueue(data, id) {
        const item = {
            id: id !== undefined ? id : Date.now() + Math.random(),
            data,
            timestamp: Date.now(),
        };
        this.items.push(item);
        return item;
    }
    dequeue() {
        return this.items.shift();
    }
    peek() {
        return this.items[0];
    }
    get length() {
        return this.items.length;
    }
    isEmpty() {
        return this.items.length === 0;
    }
    clear() {
        this.items = [];
    }
    flush() {
        const all = [...this.items];
        this.items = [];
        return all;
    }
    async processEach(handler) {
        if (this.processing)
            return;
        this.processing = true;
        try {
            while (this.items.length > 0) {
                const item = this.dequeue();
                if (item) {
                    await handler(item.data);
                }
            }
        }
        finally {
            this.processing = false;
        }
    }
}
//# sourceMappingURL=queue.js.map