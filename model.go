package ws

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrFailedToProcess = errors.New("failed to process")
	ErrFaildToProcess  = ErrFailedToProcess // backward compatibility alias
	ErrNoClient        = errors.New("no client found")
	ErrDeadline        = errors.New("command timed out")
	ErrMaxChainDepth   = errors.New("maximum action chain depth exceeded")
)

// Message sent and received over WebSocket
type Message struct {
	// encoding is always utf8 unless provided
	Encoding string `json:"encoding"`
	// payload is the data, best to be json
	Payload any `json:"payload"`
	// id is the id of the target recipient or sender
	Id string `json:"id"`
	// room is the room that the user can join or broadcast to
	Room string `json:"room"`
	// title is the name of the message sent (action name)
	Title string `json:"title"`
	// duration ms is if it needs a timeout
	DurationMs time.Duration `json:"duration"`
	// to reply to everyone but not oneself
	Broadcast string `json:"broadcast"`
	// reply correlation id
	Reply string `json:"reply"`

	// external data to be mutated across chained actions
	Data any `json:"data,omitempty"`
}

// Client represents a connected websocket client
type Client struct {
	Handshaker
	Conn        *websocket.Conn
	Joined      []string
	Credential  []string
	MessageChan chan Message
	hub         *Hub
	mu          sync.Mutex
	closed      bool
}

// Send enqueues a message to the client's write pump
func (c *Client) Send(msg Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.MessageChan <- msg:
		return true
	default:
		slog.Warn("client message buffer full, message dropped", "id", c.Id())
		return false
	}
}

// Close gracefully closes the client connection and message channel
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.MessageChan)
	c.mu.Unlock()

	if c.Conn != nil {
		c.Conn.Close()
	}
}

// JoinRoom adds the client to the specified room
func (c *Client) JoinRoom(name string) *Room {
	if c.hub != nil {
		return c.hub.JoinRoom(c, name)
	}
	return DefaultHub.JoinRoom(c, name)
}

// LeaveRoom removes the client from the specified room
func (c *Client) LeaveRoom(name string) {
	if c.hub != nil {
		c.hub.LeaveRoom(c, name)
		return
	}
	DefaultHub.LeaveRoom(c, name)
}

// Room represents a grouping of clients for targeted broadcasting
type Room struct {
	Name    string
	Window  map[string]*Client
	mu      sync.RWMutex
}

// Action represents a named handler. Returning a non-empty string chains to the next action.
type Action func(m *Message, client *Client) string

// Hub manages active clients, rooms, and action routing
type Hub struct {
	clients   map[string]*Client
	clientsMu sync.RWMutex
	rooms     map[string]*Room
	roomsMu   sync.RWMutex
	actions   map[string]Action
	actionsMu sync.RWMutex
	auther    Auther
	upgrader  websocket.Upgrader
	onDisconnect func(client *Client)
}

// NewHub creates an independent Hub instance
func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
		rooms:   make(map[string]*Room),
		actions: make(map[string]Action),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// On registers a named action handler on the hub
func (h *Hub) On(route string, action Action) {
	h.actionsMu.Lock()
	defer h.actionsMu.Unlock()
	h.actions[route] = action
}

// GetAction retrieves a registered action by name
func (h *Hub) GetAction(route string) (Action, bool) {
	h.actionsMu.RLock()
	defer h.actionsMu.RUnlock()
	a, ok := h.actions[route]
	return a, ok
}

// SetAuther sets the authentication provider for the hub
func (h *Hub) SetAuther(a Auther) {
	h.auther = a
}

// SetUpgrader sets custom websocket upgrader options
func (h *Hub) SetUpgrader(u websocket.Upgrader) {
	h.upgrader = u
}

// SetOnDisconnect sets a callback invoked when any client disconnects
func (h *Hub) SetOnDisconnect(fn func(client *Client)) {
	h.onDisconnect = fn
}

// Register adds a client to the hub
func (h *Hub) Register(c *Client) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.clients[c.Id()] = c
}

// unregister removes a client from the hub and any joined rooms
func (h *Hub) unregister(c *Client) {
	id := c.Id()

	h.clientsMu.Lock()
	delete(h.clients, id)
	h.clientsMu.Unlock()

	h.roomsMu.Lock()
	for _, roomName := range c.Joined {
		if rm, ok := h.rooms[roomName]; ok {
			rm.mu.Lock()
			delete(rm.Window, id)
			rm.mu.Unlock()
		}
	}
	h.roomsMu.Unlock()

	if h.onDisconnect != nil {
		go h.onDisconnect(c)
	}
}

// GetClient retrieves an active client by ID
func (h *Hub) GetClient(id string) (*Client, bool) {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	c, ok := h.clients[id]
	return c, ok
}

// ClientCount returns the number of active clients
func (h *Hub) ClientCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// JoinRoom adds a client to a room, creating the room if it does not exist
func (h *Hub) JoinRoom(client *Client, name string) *Room {
	h.roomsMu.Lock()
	room, ok := h.rooms[name]
	if !ok {
		room = &Room{
			Name:   name,
			Window: make(map[string]*Client),
		}
		h.rooms[name] = room
	}
	h.roomsMu.Unlock()

	room.mu.Lock()
	room.Window[client.Id()] = client
	room.mu.Unlock()

	client.Joined = append(client.Joined, name)
	return room
}

// LeaveRoom removes a client from a room
func (h *Hub) LeaveRoom(client *Client, name string) {
	h.roomsMu.Lock()
	room, ok := h.rooms[name]
	h.roomsMu.Unlock()

	if ok {
		room.mu.Lock()
		delete(room.Window, client.Id())
		room.mu.Unlock()
	}
}

// Send delivers a message to a room, client ID, or all connected clients
func (h *Hub) Send(msg Message) {
	if msg.Room != "" {
		h.roomsMu.RLock()
		rm, ok := h.rooms[msg.Room]
		h.roomsMu.RUnlock()
		if !ok {
			return
		}

		rm.mu.RLock()
		defer rm.mu.RUnlock()
		for id, target := range rm.Window {
			if msg.Broadcast != "" && id == msg.Broadcast {
				continue
			}
			target.Send(msg)
		}
		return
	}

	if msg.Id != "" {
		h.clientsMu.RLock()
		target, ok := h.clients[msg.Id]
		h.clientsMu.RUnlock()
		if ok {
			target.Send(msg)
		}
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	for id, target := range h.clients {
		if msg.Broadcast != "" && id == msg.Broadcast {
			continue
		}
		target.Send(msg)
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(msg Message) {
	msg.Id = ""
	msg.Room = ""
	h.Send(msg)
}

// DefaultHub provides package-level convenience functions
var DefaultHub = NewHub()

// On registers an action on the DefaultHub
func On(route string, action Action) {
	DefaultHub.On(route, action)
}

// JoinRoom adds a client to a room on the DefaultHub
func JoinRoom(client *Client, name string) *Room {
	return DefaultHub.JoinRoom(client, name)
}

// Send dispatches a message via the DefaultHub
func Send(msg Message) {
	DefaultHub.Send(msg)
}
