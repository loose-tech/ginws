package ws

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

const (
	pongWait   = 60 * time.Second    // Max time to wait for next pong
	pingPeriod = (pongWait * 9) / 10 // Send pings at 90% of pongWait
	writeWait  = 10 * time.Second    // Max time to wait for a write
)

func (c *Client) readPump() {
	defer func() {
		if c.hub != nil {
			c.hub.unregister(c)
		}
		c.Close()
	}()

	// 16 MB max message size for multimedia and file sharing
	c.Conn.SetReadLimit(16 * 1024 * 1024)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, msgBytes, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Debug("websocket connection closed", "error", err, "id", c.Id())
			}
			break
		}

		var m Message
		if err := json.Unmarshal(msgBytes, &m); err != nil {
			slog.Error("failed to parse message", "error", err)
			continue
		}

		if m.Title == "join" {
			room := m.Room
			if room == "" {
				if s, ok := m.Payload.(string); ok {
					room = s
				} else if s, ok := m.Data.(string); ok {
					room = s
				}
			}
			if room != "" {
				c.JoinRoom(room)
				if m.Reply != "" {
					c.Send(Message{
						Title:   "joined",
						Room:    room,
						Payload: map[string]any{"room": room, "status": "joined"},
						Reply:   m.Reply,
						Id:      c.Id(),
					})
				}
			}
			continue
		}

		if m.Title == "leave" {
			room := m.Room
			if room == "" {
				if s, ok := m.Payload.(string); ok {
					room = s
				} else if s, ok := m.Data.(string); ok {
					room = s
				}
			}
			if room != "" {
				c.LeaveRoom(room)
				if m.Reply != "" {
					c.Send(Message{
						Title:   "left",
						Room:    room,
						Payload: map[string]any{"room": room, "status": "left"},
						Reply:   m.Reply,
						Id:      c.Id(),
					})
				}
			}
			continue
		}

		// Process each message asynchronously so slow actions do not block reading subsequent frames
		go func(msg Message) {
			var procErr error
			if c.hub != nil {
				procErr = c.hub.ProcessMessage(c, &msg)
			} else {
				procErr = DefaultHub.ProcessMessage(c, &msg)
			}

			if procErr != nil {
				slog.Error("failed to process message", "error", procErr, "title", msg.Title)
				if msg.Reply != "" {
					c.Send(Message{
						Encoding: "utf8",
						Title:    "error",
						Payload: map[string]any{
							"error": procErr.Error(),
						},
						Reply: msg.Reply,
						Id:    c.Id(),
					})
				}
			}
		}(m)
	}
}

// writePump is the sole concurrent writer to c.Conn
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case msg, ok := <-c.MessageChan:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub/client closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(msg); err != nil {
				slog.Error("failed to write json", "error", err, "id", c.Id())
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
