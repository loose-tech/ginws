package ws

import (
	"context"
	"log/slog"
	"time"
)

func (h *Hub) process(client *Client, msg *Message, ctx context.Context) error {
	currentName := msg.Title
	reschan := make(chan error, 1)

	go func() {
		depth := 0
		const maxDepth = 50

		for {
			if ctx.Err() != nil {
				reschan <- ctx.Err()
				return
			}

			if depth >= maxDepth {
				reschan <- ErrMaxChainDepth
				return
			}
			depth++

			action, ok := h.GetAction(currentName)
			if !ok {
				slog.Debug("no action available", "name", currentName)
				reschan <- ErrFailedToProcess
				return
			}

			nextRoute := action(msg, client)

			if nextRoute == "" {
				reschan <- nil // completed chain
				return
			}

			// If the action returned a new route name, loop to the next action
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

// ProcessMessage runs the action pipeline for a message within an optional timeout context
func (h *Hub) ProcessMessage(client *Client, message *Message) error {
	slog.Info("processing new message", "title", message.Title, "room", message.Room, "durationMs", int64(message.DurationMs))
	ctx := context.Background()
	var cancel context.CancelFunc = func() {}

	if message.DurationMs > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), message.DurationMs*time.Millisecond)
	}
	defer cancel()

	return h.process(client, message, ctx)
}

// ProcessMessage processes a message using the DefaultHub
func ProcessMessage(client *Client, message *Message) error {
	return DefaultHub.ProcessMessage(client, message)
}
