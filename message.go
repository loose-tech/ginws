package ws

import (
	"context"
	"log/slog"
	"time"
)

func process(client Client, msg *Message, ctx context.Context) error {
	currentName := msg.Title
	reschan := make(chan error, 1)
	// process everything
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			action, ok := ActionMap[currentName]
			if !ok {
				slog.Debug("no action available", "name", currentName)
				reschan <- ErrFaildToProcess
				return
			}

			nextRoute := action(msg, client)

			if nextRoute == "" {
				reschan <- nil // done
				return
			}

			// If the action returned a new route name, loop again
			currentName = nextRoute
			slog.Info("chaining to next action", "name", currentName)

		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-reschan:
		return err

	}

}

func processMessage(client Client, message *Message) error {
	slog.Info("processing new message", "title", message.Title, "room", message.Room, "duration", message.DurationMs)
	var cancel = func() {}
	var ctx = context.Background()
	if message.DurationMs > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), message.DurationMs*time.Microsecond)
	}
	defer cancel()
	return process(client, message, ctx)

}
