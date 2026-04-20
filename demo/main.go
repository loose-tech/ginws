package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	ws "github.com/kainkent69/ginws"
)

type MyHandshaker struct {
	userID int64
}

func (m *MyHandshaker) Claims() any {
	return 0
}
func (m *MyHandshaker) HandShake() error {
	return nil
}
func (m *MyHandshaker) Id() string {
	return fmt.Sprintf("user-with-id=%d", m.userID)
}

type MyAuther struct {
	counter int64
}

func (a *MyAuther) Check(ctx *gin.Context) (ws.Handshaker, error) {
	id := atomic.AddInt64(&a.counter, 1)
	return &MyHandshaker{userID: id}, nil
}

func main() {
	// Initialize the AuthHandler
	ws.AuthHandler = &MyAuther{}

	// Add some actions
	ws.On("ping", func(m *ws.Message, client ws.Client) string {
		client.Send(ws.Message{
			Title:   "pong",
			Payload: "pong",
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("echo", func(m *ws.Message, client ws.Client) string {
		client.Send(ws.Message{
			Title:   "echo_response",
			Payload: m.Payload,
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("slow_echo", func(m *ws.Message, client ws.Client) string {
		// This will test the timeout logic if the client sets a short duration
		time.Sleep(2 * time.Second)
		client.Send(ws.Message{
			Title:   "slow_echo_response",
			Payload: m.Payload,
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("chat", func(m *ws.Message, client ws.Client) string {
		// Broadcast to room if provided, else to everyone
		m.Title = "chat_message"
		m.Id = "" // Reset ID to broadcast
		client.Send(*m)
		return ""
	})

	ws.On("hello", func(m *ws.Message, client ws.Client) string {
		client.Send(ws.Message{
			Title:   "hello_response",
			Payload: "Welcome!",
			Reply:   m.Reply,
		})
		return ""
	})

	r := gin.Default()

	// CORS-like behavior for origin * is handled by upgrader in ws package,
	// but for standard gin routes:
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// / route - reply hello world json
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello world"})
	})

	// /ws route
	r.GET("/ws", ws.WSHandler(true))

	// /check route - shows current handshaker id for verification
	r.GET("/check", func(c *gin.Context) {
		h, err := ws.AuthHandler.Check(c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": h.Id()})
	})

	// Serve the compiled JS from client/dist
	r.Static("/dist", "./client/dist")

	// Serve the main index.html
	r.StaticFile("/app", "index.html")

	// Listen and serve on port 5000
	r.Run(":5000")
}
