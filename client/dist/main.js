import { Ws } from "./ws.js";
const ws = new Ws("ws://localhost:5000/ws");
let counter = 0;
ws.on("open", () => {
    // Make this async so we can 'await' the result
    async function ddrun() {
        counter++;
        if (counter > 20)
            counter = 1; // Prevent infinite growth
        const duration = 50 * counter;
        console.log(`Sending message. Expecting reply within ${duration}ms`);
        try {
            // sendRaw returns a Promise. Await it!
            await ws.sendRaw("hello", null, {}, duration, (msg) => {
                console.log("Success:", msg.payload);
                return "";
            }, (reply, done) => {
                console.log("Expired!");
                done();
            });
        }
        catch (e) {
            console.error(e);
        }
        // Now that the message is DONE (replied or expired), wait before next loop
        setTimeout(ddrun, counter * 50); // Constant 1s gap, or use counter * 50
    }
    ddrun();
});
//# sourceMappingURL=main.js.map