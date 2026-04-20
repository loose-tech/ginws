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
		c.Conn.Close()
		// Cleanup: remove from Clients and Rooms maps here
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	// This handler triggers every time a Pong comes back
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	type RequestMsg struct {
		mType int
		msg   []byte
		err   error
	}

	msgChan := make(chan RequestMsg, 1)
	go func() {
		for request := range msgChan {
			var m = Message{}
			err := json.Unmarshal(request.msg, &m)
			if err != nil {
				slog.Error("failed to parse message", "error", err)
			}
			slog.Debug("info", "conn", c.Conn)
			if m.Title == "join" {
				JoinRoom(*c, m.Data.(string))
			} else {
				err = processMessage(*c, &m)
				if err != nil {
					if c.Conn != nil {
						c.Send(Message{
							Encoding: "utf8",
							Payload: map[string]any{
								"failed": err.Error(),
							},
							Id: c.Id(),
						})
					}

				} else {
					slog.Info("ws: succcessfully responded")
				}
			}
			slog.Info("process normallyy")
		}
	}()

	go func() {
		for msg := range c.MessageChan {
			slog.Info("recieved a message ")
			send(msg)
		}
	}()

	for {
		mtype, msg, err := c.Conn.ReadMessage()
		slog.Info("new message")
		if err != nil {
			break

		}
		// create a new request
		request := RequestMsg{
			mType: mtype,
			msg:   msg,
			err:   err,
		}
		msgChan <- request

	}
}

// do the rest
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	// do the tick
	for range ticker.C {
		// Send a Ping to the browser
		c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			return
		}
	}

}

func (c *Client) Send(msg Message) {
	c.MessageChan <- msg
}
