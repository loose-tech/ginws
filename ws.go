package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// the handler
func WSHandler(auth bool) gin.HandlerFunc {
	log := slog.With("ws.WSHandler")
	return func(ctx *gin.Context) {
		log.Info("new client")
		handler, err := AuthHandler.Check(ctx)
		if err != nil {
			log.Error("failed to process auth")
			return
		}
		conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			ctx.AbortWithStatusJSON(400, gin.H{})
		}

		newClient := Client{
			Conn:        conn,
			Joined:      []string{},
			Handshaker:  handler,
			MessageChan: make(chan Message, 3),
		}
		Clients[newClient.Id()] = newClient

		go newClient.readPump()
		go newClient.writePump()

	}
}

// TODO: remove this later
func init() {
	On("hello", func(m *Message, client Client) string {
		slog.Info("reply", "reply", m.Reply)
		time.Sleep(time.Second)
		client.Send(Message{
			Encoding: "utf8",
			Payload:  "hello",
			Id:       client.Id(),
			Title:    "see-me",
			Reply:    m.Reply,
		})
		return ""
	})

}
