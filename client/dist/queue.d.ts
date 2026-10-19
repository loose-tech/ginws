export interface QueueItem<T = any> {
    id: string | number;
    data: T;
    timestamp: number;
}
export declare class Queue<T = any> {
    private items;
    private processing;
    enqueue(data: T, id?: string | number): QueueItem<T>;
    dequeue(): QueueItem<T> | undefined;
    peek(): QueueItem<T> | undefined;
    get length(): number;
    isEmpty(): boolean;
    clear(): void;
    flush(): QueueItem<T>[];
    processEach(handler: (item: T) => Promise<void> | void): Promise<void>;
}
//# sourceMappingURL=queue.d.ts.map