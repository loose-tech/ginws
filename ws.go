package ws

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSHandler creates a Gin handler function bound to the hub
func (h *Hub) WSHandler(auth bool) gin.HandlerFunc {
	log := slog.With("component", "ws.WSHandler")
	return func(ctx *gin.Context) {
		log.Info("new client connecting")
		var handler Handshaker
		var err error

		auther := h.auther
		if auther == nil {
			auther = AuthHandler
		}

		if auth && auther != nil {
			handler, err = auther.Check(ctx)
			if err != nil {
				log.Error("failed to process auth", "error", err)
				ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}
		} else {
			handler = NewDefaultHandshaker(uuid.New().String())
		}

		conn, err := h.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
		if err != nil {
			log.Error("failed to upgrade websocket connection", "error", err)
			return
		}

		newClient := &Client{
			Handshaker:  handler,
			Conn:        conn,
			Joined:      []string{},
			MessageChan: make(chan Message, 256),
			hub:         h,
		}
		h.Register(newClient)

		go newClient.readPump()
		go newClient.writePump()
	}
}

// WSHandler creates a Gin handler using the DefaultHub
func WSHandler(auth bool) gin.HandlerFunc {
	return DefaultHub.WSHandler(auth)
}
