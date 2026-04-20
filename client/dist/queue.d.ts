export declare class Queue {
    buffer: EventBuffer[];
    vacancy: Map<number, any>;
    vacancies: number;
    queue: QueueMessage[];
    addBuffer(data: any): Promise<unknown>;
    consumeBuffer(title: string): Promise<unknown>;
    cleanBuffer(): Promise<EventBuffer[]>;
    addQueue(msg: QueueMessage): void;
    do(): Promise<unknown> | undefined;
    process(): Promise<void>;
}
//# sourceMappingURL=queue.d.ts.map