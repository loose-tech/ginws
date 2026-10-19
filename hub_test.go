package ws_test

import (
	"encoding/json"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	ws "github.com/kainkent69/ginws"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestActionChaining verifies that named actions chain properly across steps
func TestActionChaining(t *testing.T) {
	hub := ws.NewHub()

	var executedSteps []string

	hub.On("auth", func(m *ws.Message, c *ws.Client) string {
		executedSteps = append(executedSteps, "auth")
		m.Data = "authenticated"
		return "validate"
	})

	hub.On("validate", func(m *ws.Message, c *ws.Client) string {
		executedSteps = append(executedSteps, "validate")
		if m.Data != "authenticated" {
			t.Fatalf("expected data to be authenticated, got %v", m.Data)
		}
		m.Data = "validated"
		return "process"
	})

	hub.On("process", func(m *ws.Message, c *ws.Client) string {
		executedSteps = append(executedSteps, "process")
		m.Data = "processed"
		return "" // terminal
	})

	mockClient := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("test-client-1"),
		MessageChan: make(chan ws.Message, 10),
	}

	msg := &ws.Message{
		Title: "auth",
	}

	err := hub.ProcessMessage(mockClient, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(executedSteps) != 3 {
		t.Fatalf("expected 3 steps executed, got %d: %v", len(executedSteps), executedSteps)
	}

	expectedSteps := []string{"auth", "validate", "process"}
	for i, step := range expectedSteps {
		if executedSteps[i] != step {
			t.Errorf("step %d: expected %s, got %s", i, step, executedSteps[i])
		}
	}
}

// TestActionTimeout verifies that an action taking longer than DurationMs times out
func TestActionTimeout(t *testing.T) {
	hub := ws.NewHub()

	hub.On("slow", func(m *ws.Message, c *ws.Client) string {
		time.Sleep(100 * time.Millisecond)
		return ""
	})

	mockClient := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("test-client-slow"),
		MessageChan: make(chan ws.Message, 10),
	}

	msg := &ws.Message{
		Title:      "slow",
		DurationMs: 20, // 20ms timeout, action takes 100ms
	}

	err := hub.ProcessMessage(mockClient, msg)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// TestActionCycleProtection verifies that cyclical chains do not cause an infinite loop
func TestActionCycleProtection(t *testing.T) {
	hub := ws.NewHub()

	hub.On("loopA", func(m *ws.Message, c *ws.Client) string {
		return "loopB"
	})

	hub.On("loopB", func(m *ws.Message, c *ws.Client) string {
		return "loopA"
	})

	mockClient := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("test-client-loop"),
		MessageChan: make(chan ws.Message, 10),
	}

	msg := &ws.Message{
		Title: "loopA",
	}

	err := hub.ProcessMessage(mockClient, msg)
	if err != ws.ErrMaxChainDepth {
		t.Fatalf("expected ErrMaxChainDepth, got %v", err)
	}
}

// TestRoomJoinAndBroadcast verifies room registration and targeting
func TestRoomJoinAndBroadcast(t *testing.T) {
	hub := ws.NewHub()

	c1 := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("user-1"),
		MessageChan: make(chan ws.Message, 10),
	}
	c2 := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("user-2"),
		MessageChan: make(chan ws.Message, 10),
	}

	hub.Register(c1)
	hub.Register(c2)

	hub.JoinRoom(c1, "lobby")
	hub.JoinRoom(c2, "lobby")

	// Broadcast to room
	broadcastMsg := ws.Message{
		Room:    "lobby",
		Title:   "announcement",
		Payload: "hello lobby",
	}
	hub.Send(broadcastMsg)

	// Both clients should receive the message
	select {
	case m := <-c1.MessageChan:
		if m.Title != "announcement" {
			t.Errorf("c1 expected title announcement, got %s", m.Title)
		}
	default:
		t.Fatal("c1 did not receive room message")
	}

	select {
	case m := <-c2.MessageChan:
		if m.Title != "announcement" {
			t.Errorf("c2 expected title announcement, got %s", m.Title)
		}
	default:
		t.Fatal("c2 did not receive room message")
	}
}

// TestBroadcastExclusion verifies that msg.Broadcast skips the sender
func TestBroadcastExclusion(t *testing.T) {
	hub := ws.NewHub()

	c1 := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("sender-1"),
		MessageChan: make(chan ws.Message, 10),
	}
	c2 := &ws.Client{
		Handshaker:  ws.NewDefaultHandshaker("receiver-2"),
		MessageChan: make(chan ws.Message, 10),
	}

	hub.Register(c1)
	hub.Register(c2)

	// Broadcast message from c1
	msg := ws.Message{
		Title:     "chat",
		Payload:   "hey everyone",
		Broadcast: "sender-1", // skip sender
	}
	hub.Send(msg)

	// c1 should NOT have received it
	select {
	case <-c1.MessageChan:
		t.Fatal("c1 received broadcast despite being the sender in Broadcast field")
	default:
		// expected
	}

	// c2 SHOULD have received it
	select {
	case m := <-c2.MessageChan:
		if m.Payload != "hey everyone" {
			t.Errorf("c2 received unexpected payload: %v", m.Payload)
		}
	default:
		t.Fatal("c2 did not receive broadcast message")
	}
}

// TestEndToEndWebSocket verifies full HTTP upgrade, action chaining, and request-reply
func TestEndToEndWebSocket(t *testing.T) {
	hub := ws.NewHub()

	// Setup chained actions on the hub
	hub.On("checkout:start", func(m *ws.Message, c *ws.Client) string {
		m.Data = "checkout:verified"
		return "checkout:complete"
	})

	hub.On("checkout:complete", func(m *ws.Message, c *ws.Client) string {
		c.Send(ws.Message{
			Title:   "checkout:success",
			Payload: "order-12345",
			Reply:   m.Reply,
		})
		return ""
	})

	engine := gin.New()
	engine.GET("/ws", hub.WSHandler(false))

	server := httptest.NewServer(engine)
	defer server.Close()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Connect client
	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Send request with reply ID
	req := ws.Message{
		Title: "checkout:start",
		Reply: "reply-key-777",
	}

	reqBytes, _ := json.Marshal(req)
	if err := conn.WriteMessage(websocket.TextMessage, reqBytes); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Wait for response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, respBytes, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp ws.Message
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Title != "checkout:success" {
		t.Errorf("expected response title 'checkout:success', got %s", resp.Title)
	}

	if resp.Reply != "reply-key-777" {
		t.Errorf("expected reply id 'reply-key-777', got %s", resp.Reply)
	}

	if resp.Payload != "order-12345" {
		t.Errorf("expected payload 'order-12345', got %v", resp.Payload)
	}
}

// TestNodeClientIntegration runs the compiled TypeScript client (client/dist/ws.js) against the Go server
func TestNodeClientIntegration(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found in PATH, skipping Node client integration test")
	}

	hub := ws.NewHub()

	hub.On("ping", func(m *ws.Message, c *ws.Client) string {
		c.Send(ws.Message{
			Title:   "pong",
			Payload: "pong",
			Reply:   m.Reply,
		})
		return ""
	})

	hub.On("order:create", func(m *ws.Message, c *ws.Client) string {
		m.Payload = map[string]any{"item": m.Payload}
		return "order:validate"
	})

	hub.On("order:validate", func(m *ws.Message, c *ws.Client) string {
		return "order:finalize"
	})

	hub.On("order:finalize", func(m *ws.Message, c *ws.Client) string {
		c.Send(ws.Message{
			Title:   "order:completed",
			Payload: map[string]any{"orderId": "ORD-7749"},
			Reply:   m.Reply,
		})
		return ""
	})

	hub.On("slow_echo", func(m *ws.Message, c *ws.Client) string {
		time.Sleep(1 * time.Second)
		c.Send(ws.Message{
			Title:   "slow_echo_response",
			Payload: m.Payload,
			Reply:   m.Reply,
		})
		return ""
	})

	engine := gin.New()
	engine.GET("/ws", hub.WSHandler(false))

	server := httptest.NewServer(engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	cmd := exec.Command(nodePath, "/home/lluie/.gemini/antigravity-cli/brain/56ff4a13-4696-42ef-824e-03f49d3c2bf3/scratch/test_e2e_node.mjs", wsURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Node client integration test failed: %v\nOutput:\n%s", err, string(out))
	}
}
