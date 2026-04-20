package ws

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var ErrFaildToProcess = errors.New("failed to process")
var ErrNoClient = errors.New("no client found")

// 7e2a904
var ErrDeadline = errors.New("command timedout")

// message sent and recieved
type Message struct {
	// encoding is always utf8 unles provided
	Encoding string `json:"encoding"`
	// payload is the  data best to be json
	Payload any `json:"payload"`
	// id is the id of the mesage
	Id string `json:"id"`
	// room is the room that the user can join or something
	Room string `json:"room"`
	// title is the  name of the mesage sent
	Title string `json:"title"`
	// duration ms is if it needs a timeout of sort.
	DurationMs time.Duration `json:"duration"`
	// to reply to everyone but not oneself
	Broadcast string `json:"broadcast"`
	// reply
	Reply string `json:"reply"`

	// external data to be mutated
	Data any
}

// the client type
type Client struct {
	Handshaker
	*websocket.Conn
	// the rooms that you join by name
	Joined      []string
	Credential  []string
	MessageChan chan Message
}

// the room
type Room struct {
	Name   string
	Window map[string]Client
}

// the clients
// later or unrecored client is not a client
var Clients = map[string]Client{}

// the rooms
var Rooms = map[string]*Room{}
var roomMu = sync.Mutex{}

// join a room
func getOrMakeRoom(name string) *Room {
	roomMu.Lock()
	defer roomMu.Unlock()
	if room, ok := Rooms[name]; ok {
		return room
	} else {
		room := &Room{
			Name:   name,
			Window: map[string]Client{},
		}
		Rooms[name] = room
		return room

	}
}

// join a new room
func JoinRoom(client Client, name string) *Room {
	room := getOrMakeRoom(name)
	client.Joined = append(client.Joined, name)
	room.Window[client.Id()] = client
	return room

}

// send a message
func send(msg Message) {
	to := msg.Id
	room := msg.Room
	target := Clients

	// get the room
	if room != "" {
		if rm, ok := Rooms[room]; ok {
			target = rm.Window
		} else {
			return
		}
	}

	if to != "" {
		if client, ok := target[to]; ok {
			client.Conn.WriteJSON(msg)
			return
		}
		return
	}

	for _, client := range target {
		go func(m Message, client Client) {
			if err := client.Conn.WriteJSON(msg); err != nil {
				slog.Error("failed to write json", "err", err)
			}
		}(msg, client)
	}

}

// actions should be return 0 as ok successful
type Action func(m *Message, client Client) string

// the list of actions
var ActionMap = map[string]Action{}

// staring a new one
func On(route string, action Action) {
	ActionMap[route] = action
}

// the processor
