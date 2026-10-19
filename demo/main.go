package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	ws.On("ping", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "pong",
			Payload: "pong",
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("echo", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "echo_response",
			Payload: m.Payload,
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("slow_echo", func(m *ws.Message, client *ws.Client) string {
		// This will test the timeout logic if the client sets a short duration
		time.Sleep(2 * time.Second)
		client.Send(ws.Message{
			Title:   "slow_echo_response",
			Payload: m.Payload,
			Reply:   m.Reply,
		})
		return ""
	})

	// --- USER IDENTITY REGISTRY ---
	type UserProfile struct {
		ID       string `json:"id"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
		Online   bool   `json:"online"`
		Status   string `json:"status,omitempty"`
	}

	var usersMu sync.RWMutex
	users := map[string]*UserProfile{
		"sys-bot": {
			ID:       "sys-bot",
			Username: "GinWS Bot (AI)",
			Avatar:   "🤖",
			Online:   true,
			Status:   "⚡ Online & Auto-Reply",
		},
		"user-sarah": {
			ID:       "user-sarah",
			Username: "Sarah (Lead Dev)",
			Avatar:   "👩‍💻",
			Online:   true,
			Status:   "🚀 Core Contributor",
		},
		"user-alex": {
			ID:       "user-alex",
			Username: "Alex (Architect)",
			Avatar:   "🧙‍♂️",
			Online:   true,
			Status:   "🎮 Ready for Game & Calls",
		},
	}

	getOnlineUsers := func() []*UserProfile {
		usersMu.RLock()
		defer usersMu.RUnlock()
		list := make([]*UserProfile, 0, len(users))
		for _, u := range users {
			if u.Online {
				list = append(list, u)
			}
		}
		return list
	}

	// --- MESSAGE MEMORY, RECALL & FACEBOOK-STYLE READ RECEIPTS ---
	type ReadReceipt struct {
		UserID   string `json:"userId"`
		Username string `json:"username"`
		Avatar   string `json:"avatar"`
		ReadAt   string `json:"readAt"`
	}

	type StoredMessage struct {
		ID           string         `json:"id"`
		Room         string         `json:"room"`
		SenderID     string         `json:"senderId"`
		SenderName   string         `json:"senderName"`
		SenderAvatar string         `json:"senderAvatar"`
		Content      any            `json:"content"`
		Time         string         `json:"time"`
		Timestamp    int64          `json:"timestamp"`
		Recalled     bool           `json:"recalled"`
		ReadBy       []*ReadReceipt `json:"readBy"`
	}

	var roomMessagesMu sync.RWMutex
	nowInit := time.Now().Format("15:04:05")
	nowTsInit := time.Now().UnixMilli()
	roomMessages := map[string][]*StoredMessage{
		"general": {
			{
				ID:           "msg-seed-1",
				Room:         "general",
				SenderID:     "sys-bot",
				SenderName:   "GinWS Bot",
				SenderAvatar: "🤖",
				Content:      map[string]any{"text": "⚡ Welcome to GinWS! Messages sent in any room are stored in server memory and automatically recalled upon reconnect or refresh."},
				Time:         nowInit,
				Timestamp:    nowTsInit - 60000,
				Recalled:     false,
				ReadBy: []*ReadReceipt{
					{UserID: "sys-bot", Username: "GinWS Bot", Avatar: "🤖", ReadAt: nowInit},
				},
			},
			{
				ID:           "msg-seed-2",
				Room:         "general",
				SenderID:     "user-sarah",
				SenderName:   "Sarah (Lead Dev)",
				SenderAvatar: "👩‍💻",
				Content:      map[string]any{"text": "Hey everyone! The offline catch-up and Facebook-style seen indicators are live. Try disconnecting and reconnecting to test automatic recovery!"},
				Time:         nowInit,
				Timestamp:    nowTsInit - 30000,
				Recalled:     false,
				ReadBy: []*ReadReceipt{
					{UserID: "user-sarah", Username: "Sarah (Lead Dev)", Avatar: "👩‍💻", ReadAt: nowInit},
					{UserID: "sys-bot", Username: "GinWS Bot", Avatar: "🤖", ReadAt: nowInit},
				},
			},
		},
		"public-wall": {
			{
				ID:           "msg-seed-wall-1",
				Room:         "public-wall",
				SenderID:     "sys-bot",
				SenderName:   "GinWS Bot",
				SenderAvatar: "📢",
				Content:      map[string]any{"text": "🚀 Welcome to the Public Broadcast Wall! Shouts and announcements posted here are synced live across all clients and visible on the global wall."},
				Time:         nowInit,
				Timestamp:    nowTsInit - 50000,
				Recalled:     false,
				ReadBy: []*ReadReceipt{
					{UserID: "sys-bot", Username: "GinWS Bot", Avatar: "🤖", ReadAt: nowInit},
				},
			},
			{
				ID:           "msg-seed-wall-2",
				Room:         "public-wall",
				SenderID:     "user-sarah",
				SenderName:   "Sarah (Lead Dev)",
				SenderAvatar: "👩‍💻",
				Content:      map[string]any{"text": "📢 The public wall and channels are now completely integrated in one cohesive structure! Post anytime."},
				Time:         nowInit,
				Timestamp:    nowTsInit - 25000,
				Recalled:     false,
				ReadBy: []*ReadReceipt{
					{UserID: "user-sarah", Username: "Sarah (Lead Dev)", Avatar: "👩‍💻", ReadAt: nowInit},
					{UserID: "sys-bot", Username: "GinWS Bot", Avatar: "🤖", ReadAt: nowInit},
				},
			},
		},
		"engineering": {
			{
				ID:           "msg-seed-eng-1",
				Room:         "engineering",
				SenderID:     "user-alex",
				SenderName:   "Alex (Architect)",
				SenderAvatar: "🧙‍♂️",
				Content:      map[string]any{"text": "WebSocket action chaining pipeline and offline recovery verified at 60fps."},
				Time:         nowInit,
				Timestamp:    nowTsInit - 45000,
				Recalled:     false,
				ReadBy: []*ReadReceipt{
					{UserID: "user-alex", Username: "Alex (Architect)", Avatar: "🧙‍♂️", ReadAt: nowInit},
					{UserID: "sys-bot", Username: "GinWS Bot", Avatar: "🤖", ReadAt: nowInit},
				},
			},
		},
	}

	addRoomMessage := func(room string, msg *StoredMessage) {
		roomMessagesMu.Lock()
		defer roomMessagesMu.Unlock()
		list := roomMessages[room]
		if len(list) >= 200 {
			list = list[1:]
		}
		roomMessages[room] = append(list, msg)
	}

	getRoomMessages := func(room string) []*StoredMessage {
		roomMessagesMu.RLock()
		defer roomMessagesMu.RUnlock()
		list := roomMessages[room]
		result := make([]*StoredMessage, len(list))
		copy(result, list)
		return result
	}

	// --- DM STORAGE & READ TRACKING ---
	type StoredDmMessage struct {
		ID         string `json:"id"`
		FromID     string `json:"fromId"`
		FromName   string `json:"fromName"`
		FromAvatar string `json:"fromAvatar"`
		ToID       string `json:"toId"`
		Content    any    `json:"content"`
		Time       string `json:"time"`
		Timestamp  int64  `json:"timestamp"`
		Recalled   bool   `json:"recalled"`
		Read       bool   `json:"read"`
		ReadAt     string `json:"readAt,omitempty"`
	}

	var dmMessagesMu sync.RWMutex
	dmMessages := make(map[string][]*StoredDmMessage)

	getDmKey := func(u1, u2 string) string {
		if u1 < u2 {
			return u1 + ":" + u2
		}
		return u2 + ":" + u1
	}

	addDmMessage := func(u1, u2 string, msg *StoredDmMessage) {
		dmMessagesMu.Lock()
		defer dmMessagesMu.Unlock()
		k := getDmKey(u1, u2)
		list := dmMessages[k]
		if len(list) >= 200 {
			list = list[1:]
		}
		dmMessages[k] = append(list, msg)
	}

	getDmHistory := func(u1, u2 string) []*StoredDmMessage {
		dmMessagesMu.RLock()
		defer dmMessagesMu.RUnlock()
		k := getDmKey(u1, u2)
		list := dmMessages[k]
		result := make([]*StoredDmMessage, len(list))
		copy(result, list)
		return result
	}

	// --- 3. THE PUBLIC BROADCAST WALL STORAGE ---
	var wallMu sync.RWMutex
	wallPosts := []map[string]any{
		{
			"id":           "wall-seed-1",
			"authorId":     "sys-bot",
			"authorName":   "GinWS Bot",
			"authorAvatar": "📢",
			"content":      "⚡ Welcome to the Public Wall! Broadcast your announcements, multimedia, and status live across the GinWS network.",
			"time":         nowInit,
		},
		{
			"id":           "wall-seed-2",
			"authorId":     "user-sarah",
			"authorName":   "Sarah (Lead Dev)",
			"authorAvatar": "👩‍💻",
			"content":      "Channels and Public Wall are now fully unified. Check out #public-wall or shout right from the broadcast banner!",
			"time":         nowInit,
		},
	}

	// Chat broadcast and persistence
	ws.On("chat", func(m *ws.Message, client *ws.Client) string {
		room := m.Room
		if room == "" {
			room = "general"
		}

		usersMu.RLock()
		senderProfile := users[client.Id()]
		usersMu.RUnlock()

		senderName := client.Id()
		senderAvatar := "👤"
		if senderProfile != nil {
			senderName = senderProfile.Username
			senderAvatar = senderProfile.Avatar
		}

		msgID := fmt.Sprintf("msg-%d-%s", time.Now().UnixNano(), client.Id())
		nowTime := time.Now().Format("15:04:05")
		nowTs := time.Now().UnixMilli()

		stored := &StoredMessage{
			ID:           msgID,
			Room:         room,
			SenderID:     client.Id(),
			SenderName:   senderName,
			SenderAvatar: senderAvatar,
			Content:      m.Payload,
			Time:         nowTime,
			Timestamp:    nowTs,
			Recalled:     false,
			ReadBy: []*ReadReceipt{
				{
					UserID:   client.Id(),
					Username: senderName,
					Avatar:   senderAvatar,
					ReadAt:   nowTime,
				},
			},
		}

		addRoomMessage(room, stored)

		out := *m
		out.Title = "chat_message"
		out.Room = room
		out.Id = msgID
		out.Reply = ""
		out.Data = client.Id()
		out.Payload = stored
		ws.Send(out)

		// If posted in public-wall, also synchronize with wallPosts and broadcast to wall
		if room == "public-wall" {
			wallContent := ""
			if cMap, ok := m.Payload.(map[string]any); ok {
				if t, ok := cMap["text"].(string); ok {
					wallContent = t
				}
			} else if t, ok := m.Payload.(string); ok {
				wallContent = t
			}
			if wallContent != "" {
				wPost := map[string]any{
					"id":           msgID,
					"authorId":     client.Id(),
					"authorName":   senderName,
					"authorAvatar": senderAvatar,
					"content":      wallContent,
					"time":         nowTime,
				}
				wallMu.Lock()
				wallPosts = append(wallPosts, wPost)
				wallMu.Unlock()
				ws.Send(ws.Message{
					Title:   "wall:new_post",
					Payload: wPost,
				})
			}
		}

		// In demo mode: automatically simulate a peer read receipt after 1.2s to showcase Facebook-style Seen indicators
		go func(targetMsgID, targetRoom, senderID string) {
			time.Sleep(1200 * time.Millisecond)
			receipt := &ReadReceipt{
				UserID:   "sys-bot",
				Username: "GinWS Bot",
				Avatar:   "🤖",
				ReadAt:   time.Now().Format("15:04:05"),
			}
			roomMessagesMu.Lock()
			list := roomMessages[targetRoom]
			var updated []*ReadReceipt
			for _, sm := range list {
				if sm.ID == targetMsgID {
					already := false
					for _, rb := range sm.ReadBy {
						if rb.UserID == "sys-bot" {
							already = true
							break
						}
					}
					if !already {
						sm.ReadBy = append(sm.ReadBy, receipt)
					}
					updated = sm.ReadBy
					break
				}
			}
			roomMessagesMu.Unlock()

			if updated != nil {
				ws.Send(ws.Message{
					Title: "message:read_receipt",
					Room:  targetRoom,
					Payload: map[string]any{
						"room":      targetRoom,
						"messageId": targetMsgID,
						"receipt":   receipt,
						"readBy":    updated,
					},
				})
			}
		}(msgID, room, client.Id())

		return ""
	})

	// Historical room messages catch-up / fetch
	ws.On("room:messages:get", func(m *ws.Message, client *ws.Client) string {
		room := m.Room
		if room == "" {
			if data, ok := m.Payload.(map[string]any); ok {
				if r, ok := data["room"].(string); ok {
					room = r
				}
			}
		}
		if room == "" {
			room = "general"
		}

		history := getRoomMessages(room)
		client.Send(ws.Message{
			Title: "room:messages:history",
			Room:  room,
			Payload: map[string]any{
				"room":     room,
				"messages": history,
			},
			Reply: m.Reply,
		})
		return ""
	})

	// Message read receipt (Facebook-style read indicator)
	ws.On("message:read", func(m *ws.Message, client *ws.Client) string {
		data, ok := m.Payload.(map[string]any)
		if !ok || data == nil {
			return ""
		}
		room, _ := data["room"].(string)
		if room == "" {
			room = m.Room
		}
		msgID, _ := data["messageId"].(string)
		if room == "" || msgID == "" {
			return ""
		}

		usersMu.RLock()
		uProfile := users[client.Id()]
		usersMu.RUnlock()

		uName := client.Id()
		uAvatar := "👤"
		if uProfile != nil {
			uName = uProfile.Username
			uAvatar = uProfile.Avatar
		}

		receipt := &ReadReceipt{
			UserID:   client.Id(),
			Username: uName,
			Avatar:   uAvatar,
			ReadAt:   time.Now().Format("15:04:05"),
		}

		roomMessagesMu.Lock()
		list := roomMessages[room]
		var updatedReadBy []*ReadReceipt
		for _, sm := range list {
			if sm.ID == msgID {
				alreadyRead := false
				for _, r := range sm.ReadBy {
					if r.UserID == client.Id() {
						alreadyRead = true
						break
					}
				}
				if !alreadyRead {
					sm.ReadBy = append(sm.ReadBy, receipt)
				}
				updatedReadBy = sm.ReadBy
				break
			}
		}
		roomMessagesMu.Unlock()

		if updatedReadBy != nil {
			ws.Send(ws.Message{
				Title: "message:read_receipt",
				Room:  room,
				Payload: map[string]any{
					"room":      room,
					"messageId": msgID,
					"receipt":   receipt,
					"readBy":    updatedReadBy,
				},
			})
		}
		return ""
	})

	// Message recall (unsend / redact)
	ws.On("message:recall", func(m *ws.Message, client *ws.Client) string {
		data, ok := m.Payload.(map[string]any)
		if !ok || data == nil {
			return ""
		}
		room, _ := data["room"].(string)
		if room == "" {
			room = m.Room
		}
		msgID, _ := data["messageId"].(string)
		if room == "" || msgID == "" {
			return ""
		}

		roomMessagesMu.Lock()
		list := roomMessages[room]
		var recalledSm *StoredMessage
		for _, sm := range list {
			if sm.ID == msgID {
				if sm.SenderID == client.Id() {
					sm.Recalled = true
					sm.Content = "[This message was recalled]"
					recalledSm = sm
				}
				break
			}
		}
		roomMessagesMu.Unlock()

		if recalledSm != nil {
			ws.Send(ws.Message{
				Title: "message:recalled",
				Room:  room,
				Payload: map[string]any{
					"room":       room,
					"messageId":  msgID,
					"recalledBy": client.Id(),
					"time":       time.Now().Format("15:04:05"),
				},
			})
		}
		return ""
	})

	ws.On("hello", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "hello_response",
			Payload: "Welcome!",
			Reply:   m.Reply,
		})
		return ""
	})

	// Demonstrate Action Chaining (named middleware pattern):
	// Step 1: "order:create" -> Step 2: "order:validate" -> Step 3: "order:finalize"
	ws.On("order:create", func(m *ws.Message, client *ws.Client) string {
		println("[Chain 1/3] order:create processing...")
		m.Payload = map[string]any{
			"item":      m.Payload,
			"step":      "order:create",
			"createdAt": time.Now().Format(time.RFC3339),
		}
		return "order:validate" // Chain to next named action
	})

	ws.On("order:validate", func(m *ws.Message, client *ws.Client) string {
		println("[Chain 2/3] order:validate verifying stock & auth...")
		data, _ := m.Payload.(map[string]any)
		data["step"] = "order:validate"
		data["validated"] = true
		m.Payload = data
		return "order:finalize" // Chain to next named action
	})

	ws.On("order:finalize", func(m *ws.Message, client *ws.Client) string {
		println("[Chain 3/3] order:finalize completing and replying...")
		data, _ := m.Payload.(map[string]any)
		data["step"] = "order:finalize"
		data["status"] = "confirmed"
		data["orderId"] = "ORD-7749"

		client.Send(ws.Message{
			Title:   "order:completed",
			Payload: data,
			Reply:   m.Reply,
		})
		return "" // Chain finished
	})

	// User Login & Profile registration
	ws.On("auth:login", func(m *ws.Message, client *ws.Client) string {
		payload, _ := m.Payload.(map[string]any)
		username, _ := payload["username"].(string)
		avatar, _ := payload["avatar"].(string)
		status, _ := payload["status"].(string)
		if username == "" {
			username = fmt.Sprintf("User-%s", client.Id())
		}
		if avatar == "" {
			avatar = "👤"
		}

		profile := &UserProfile{
			ID:       client.Id(),
			Username: username,
			Avatar:   avatar,
			Online:   true,
			Status:   status,
		}

		usersMu.Lock()
		users[client.Id()] = profile
		usersMu.Unlock()

		// Confirm login to the user
		client.Send(ws.Message{
			Title:   "auth:success",
			Payload: profile,
			Reply:   m.Reply,
		})

		// Broadcast updated user list to everyone
		ws.Send(ws.Message{
			Title:   "users:updated",
			Payload: getOnlineUsers(),
		})
		return ""
	})

	ws.On("profile:update", func(m *ws.Message, client *ws.Client) string {
		payload, _ := m.Payload.(map[string]any)
		if payload == nil {
			return ""
		}
		username, _ := payload["username"].(string)
		avatar, _ := payload["avatar"].(string)
		status, _ := payload["status"].(string)

		usersMu.Lock()
		p, exists := users[client.Id()]
		if !exists {
			p = &UserProfile{ID: client.Id(), Online: true}
			users[client.Id()] = p
		}
		if username != "" {
			p.Username = username
		}
		if avatar != "" {
			p.Avatar = avatar
		}
		if status != "" {
			p.Status = status
		}
		p.Online = true
		usersMu.Unlock()

		ws.Send(ws.Message{
			Title:   "users:updated",
			Payload: getOnlineUsers(),
		})
		return ""
	})

	ws.On("users:list", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "users:list",
			Payload: getOnlineUsers(),
			Reply:   m.Reply,
		})
		return ""
	})

	// Track disconnects in real-time
	ws.DefaultHub.SetOnDisconnect(func(c *ws.Client) {
		usersMu.Lock()
		if u, ok := users[c.Id()]; ok {
			// Don't mark permanent demo personas offline
			if c.Id() != "sys-bot" && c.Id() != "user-sarah" && c.Id() != "user-alex" {
				u.Online = false
			}
		}
		usersMu.Unlock()

		ws.Send(ws.Message{
			Title:   "users:updated",
			Payload: getOnlineUsers(),
		})
	})

	// Direct Messaging with History & Read Status
	ws.On("dm", func(m *ws.Message, client *ws.Client) string {
		targetID := m.Id
		if targetID == "" {
			return ""
		}

		usersMu.RLock()
		senderProfile := users[client.Id()]
		usersMu.RUnlock()

		senderName := client.Id()
		senderAvatar := "👤"
		if senderProfile != nil {
			senderName = senderProfile.Username
			senderAvatar = senderProfile.Avatar
		}

		msgID := fmt.Sprintf("dm-%d-%s", time.Now().UnixNano(), client.Id())
		nowTime := time.Now().Format("15:04:05")
		nowTs := time.Now().UnixMilli()

		stored := &StoredDmMessage{
			ID:         msgID,
			FromID:     client.Id(),
			FromName:   senderName,
			FromAvatar: senderAvatar,
			ToID:       targetID,
			Content:    m.Payload,
			Time:       nowTime,
			Timestamp:  nowTs,
			Recalled:   false,
			Read:       false,
		}
		addDmMessage(client.Id(), targetID, stored)

		dmPayload := map[string]any{
			"id":         msgID,
			"fromId":     client.Id(),
			"fromName":   senderName,
			"fromAvatar": senderAvatar,
			"toId":       targetID,
			"content":    m.Payload,
			"time":       nowTime,
			"timestamp":  nowTs,
			"read":       false,
		}

		// Send to recipient
		ws.Send(ws.Message{
			Title:   "dm_message",
			Id:      targetID,
			Payload: dmPayload,
		})

		// Confirm to sender
		client.Send(ws.Message{
			Title:   "dm_sent",
			Payload: dmPayload,
			Reply:   m.Reply,
		})

		// In demo mode: If chatting with a bot or demo peer, simulate interactive read receipt and auto-reply!
		if targetID == "sys-bot" || targetID == "user-sarah" || targetID == "user-alex" {
			go func(partnerID, userClientID, userMsgID string, userContent any) {
				// 1. Send read receipt after 400ms
				time.Sleep(400 * time.Millisecond)
				readTime := time.Now().Format("15:04:05")
				dmMessagesMu.Lock()
				k := getDmKey(userClientID, partnerID)
				for _, dm := range dmMessages[k] {
					if dm.ID == userMsgID {
						dm.Read = true
						dm.ReadAt = readTime
						break
					}
				}
				dmMessagesMu.Unlock()

				client.Send(ws.Message{
					Title: "dm:read_receipt",
					Id:    userClientID,
					Payload: map[string]any{
						"messageId": userMsgID,
						"readBy":    partnerID,
						"readAt":    readTime,
					},
				})

				// 2. Prepare automated intelligent reply after 800ms
				time.Sleep(800 * time.Millisecond)
				replyTime := time.Now().Format("15:04:05")
				replyID := fmt.Sprintf("dm-%d-%s", time.Now().UnixNano(), partnerID)

				usersMu.RLock()
				botProfile := users[partnerID]
				usersMu.RUnlock()
				botName := partnerID
				botAvatar := "🤖"
				if botProfile != nil {
					botName = botProfile.Username
					botAvatar = botProfile.Avatar
				}

				userText := ""
				hasFiles := false
				if contentMap, ok := userContent.(map[string]any); ok {
					if t, ok := contentMap["text"].(string); ok {
						userText = t
					}
					if f, ok := contentMap["files"].([]any); ok && len(f) > 0 {
						hasFiles = true
					}
				} else if t, ok := userContent.(string); ok {
					userText = t
				}

				replyText := ""
				switch partnerID {
				case "sys-bot":
					if hasFiles {
						replyText = fmt.Sprintf("🤖 GinWS Bot here! Received your file attachment and text: \"%s\". Real-time multimedia messaging active!", userText)
					} else if strings.Contains(strings.ToLower(userText), "hello") || strings.Contains(strings.ToLower(userText), "hi") {
						replyText = "🤖 Hello! I'm the GinWS simulated peer. You can DM me, send files, or challenge me in Tic-Tac-Toe!"
					} else {
						replyText = fmt.Sprintf("🤖 Received: \"%s\" via GinWS private WebSocket channel!", userText)
					}
				case "user-sarah":
					if hasFiles {
						replyText = "👩‍💻 Sarah here! Got your attached multimedia file. WebRTC calling & DM buffers are operating with 0 latency."
					} else {
						replyText = fmt.Sprintf("👩‍💻 Hey! Sarah here. Got your message: \"%s\". Feel free to click 📞 to test video calling anytime!", userText)
					}
				case "user-alex":
					replyText = fmt.Sprintf("🧙‍♂️ Alex here! Message received. Check out the Mini-Game Arena to test automated AI move replies!")
				default:
					replyText = fmt.Sprintf("👋 Received your message: %s", userText)
				}

				botStored := &StoredDmMessage{
					ID:         replyID,
					FromID:     partnerID,
					FromName:   botName,
					FromAvatar: botAvatar,
					ToID:       userClientID,
					Content:    map[string]any{"text": replyText},
					Time:       replyTime,
					Timestamp:  time.Now().UnixMilli(),
					Recalled:   false,
					Read:       false,
				}
				addDmMessage(userClientID, partnerID, botStored)

				client.Send(ws.Message{
					Title: "dm_message",
					Payload: map[string]any{
						"id":         replyID,
						"fromId":     partnerID,
						"fromName":   botName,
						"fromAvatar": botAvatar,
						"toId":       userClientID,
						"content":    map[string]any{"text": replyText},
						"time":       replyTime,
						"timestamp":  botStored.Timestamp,
						"read":       false,
					},
				})
			}(targetID, client.Id(), msgID, m.Payload)
		}

		return ""
	})

	ws.On("dm:messages:get", func(m *ws.Message, client *ws.Client) string {
		data, _ := m.Payload.(map[string]any)
		var partnerID string
		if data != nil {
			partnerID, _ = data["partnerId"].(string)
		}
		if partnerID == "" {
			partnerID = m.Id
		}
		if partnerID == "" {
			return ""
		}

		history := getDmHistory(client.Id(), partnerID)
		if len(history) == 0 && (partnerID == "sys-bot" || partnerID == "user-sarah" || partnerID == "user-alex") {
			welcomeText := "🤖 Hey there! Welcome to GinWS private direct messaging. Send me any message or attach files (images, audio, video) to test multimedia!"
			if partnerID == "user-sarah" {
				welcomeText = "👩‍💻 Hi! I'm Sarah. Direct messages support instant delivery, Facebook-style read indicators, and 1-on-1 WebRTC calling!"
			} else if partnerID == "user-alex" {
				welcomeText = "🧙‍♂️ Greetings! Ready to test the Mini-Game Arena or WebRTC video calling? Let me know!"
			}
			usersMu.RLock()
			botProfile := users[partnerID]
			usersMu.RUnlock()
			bName := partnerID
			bAvatar := "🤖"
			if botProfile != nil {
				bName = botProfile.Username
				bAvatar = botProfile.Avatar
			}
			initDm := &StoredDmMessage{
				ID:         fmt.Sprintf("dm-seed-%s-%s", partnerID, client.Id()),
				FromID:     partnerID,
				FromName:   bName,
				FromAvatar: bAvatar,
				ToID:       client.Id(),
				Content:    map[string]any{"text": welcomeText},
				Time:       time.Now().Format("15:04:05"),
				Timestamp:  time.Now().UnixMilli() - 30000,
				Recalled:   false,
				Read:       true,
			}
			addDmMessage(client.Id(), partnerID, initDm)
			history = getDmHistory(client.Id(), partnerID)
		}

		client.Send(ws.Message{
			Title: "dm:messages:history",
			Payload: map[string]any{
				"partnerId": partnerID,
				"messages":  history,
			},
			Reply: m.Reply,
		})
		return ""
	})

	ws.On("dm:read", func(m *ws.Message, client *ws.Client) string {
		data, _ := m.Payload.(map[string]any)
		if data == nil {
			return ""
		}
		partnerID, _ := data["partnerId"].(string)
		msgID, _ := data["messageId"].(string)
		if partnerID == "" || msgID == "" {
			return ""
		}

		dmMessagesMu.Lock()
		k := getDmKey(client.Id(), partnerID)
		list := dmMessages[k]
		for _, dm := range list {
			if dm.ID == msgID && dm.ToID == client.Id() {
				dm.Read = true
				dm.ReadAt = time.Now().Format("15:04:05")
				break
			}
		}
		dmMessagesMu.Unlock()

		ws.Send(ws.Message{
			Title: "dm:read_receipt",
			Id:    partnerID,
			Payload: map[string]any{
				"messageId": msgID,
				"readBy":    client.Id(),
				"readAt":    time.Now().Format("15:04:05"),
			},
		})
		return ""
	})

	ws.On("wall:post", func(m *ws.Message, client *ws.Client) string {
		post, _ := m.Payload.(map[string]any)
		if post == nil {
			post = make(map[string]any)
		}

		usersMu.RLock()
		profile := users[client.Id()]
		usersMu.RUnlock()

		authorName := client.Id()
		authorAvatar := "📢"
		if profile != nil {
			authorName = profile.Username
			authorAvatar = profile.Avatar
		}

		postID := fmt.Sprintf("wall-%d-%s", time.Now().UnixNano(), client.Id())
		nowTime := time.Now().Format("15:04:05")

		post["id"] = postID
		post["authorId"] = client.Id()
		post["authorName"] = authorName
		post["authorAvatar"] = authorAvatar
		post["time"] = nowTime

		wallMu.Lock()
		wallPosts = append(wallPosts, post)
		wallMu.Unlock()

		// Broadcast to all connected clients
		ws.Send(ws.Message{
			Title:   "wall:new_post",
			Payload: post,
		})

		// Mirror into #public-wall room messages so channels & wall are fully unified
		wallStored := &StoredMessage{
			ID:           postID,
			Room:         "public-wall",
			SenderID:     client.Id(),
			SenderName:   authorName,
			SenderAvatar: authorAvatar,
			Content:      map[string]any{"text": post["content"]},
			Time:         nowTime,
			Timestamp:    time.Now().UnixMilli(),
			Recalled:     false,
			ReadBy: []*ReadReceipt{
				{UserID: client.Id(), Username: authorName, Avatar: authorAvatar, ReadAt: nowTime},
			},
		}
		addRoomMessage("public-wall", wallStored)
		ws.Send(ws.Message{
			Title:   "chat_message",
			Room:    "public-wall",
			Id:      postID,
			Data:    client.Id(),
			Payload: wallStored,
		})
		return ""
	})

	ws.On("wall:get", func(m *ws.Message, client *ws.Client) string {
		wallMu.RLock()
		posts := make([]map[string]any, len(wallPosts))
		copy(posts, wallPosts)
		wallMu.RUnlock()

		client.Send(ws.Message{
			Title:   "wall:posts",
			Payload: posts,
			Reply:   m.Reply,
		})
		return ""
	})

	// --- 4. WEBRTC SIGNALING & PHOTO CALL ---
	ws.On("signal:call", func(m *ws.Message, client *ws.Client) string {
		targetID := m.Id
		if targetID != "" {
			ws.Send(ws.Message{
				Title:   "signal:incoming_call",
				Id:      targetID,
				Data:    client.Id(),
				Payload: m.Payload,
			})
		}
		return ""
	})

	ws.On("signal:answer", func(m *ws.Message, client *ws.Client) string {
		targetID := m.Id
		if targetID != "" {
			ws.Send(ws.Message{
				Title:   "signal:call_answered",
				Id:      targetID,
				Data:    client.Id(),
				Payload: m.Payload,
			})
		}
		return ""
	})

	ws.On("signal:ice", func(m *ws.Message, client *ws.Client) string {
		targetID := m.Id
		if targetID != "" {
			ws.Send(ws.Message{
				Title:   "signal:ice_candidate",
				Id:      targetID,
				Payload: m.Payload,
			})
		}
		return ""
	})

	ws.On("signal:hangup", func(m *ws.Message, client *ws.Client) string {
		targetID := m.Id
		if targetID != "" {
			ws.Send(ws.Message{
				Title:   "signal:call_ended",
				Id:      targetID,
				Payload: client.Id(),
			})
		}
		return ""
	})

	ws.On("photo:share", func(m *ws.Message, client *ws.Client) string {
		m.Title = "photo:received"
		if m.Data == nil {
			m.Data = client.Id()
		}
		ws.Send(*m)
		return ""
	})

	// --- 5. REAL-TIME MINI-GAME (MULTIPLAYER TIC-TAC-TOE VIA ACTION CHAINING) ---
	type GameState struct {
		Board   [9]string `json:"board"`
		Turn    string    `json:"turn"`
		Winner  string    `json:"winner"`
		ScoreX  int       `json:"scoreX"`
		ScoreO  int       `json:"scoreO"`
		LastBy  string    `json:"lastBy"`
	}

	var gameMu sync.RWMutex
	game := &GameState{Turn: "X"}

	// Chained Game Pipeline: game:move -> game:validate -> game:evaluate
	ws.On("game:move", func(m *ws.Message, client *ws.Client) string {
		return "game:validate"
	})

	ws.On("game:validate", func(m *ws.Message, client *ws.Client) string {
		gameMu.Lock()
		defer gameMu.Unlock()

		moveData, _ := m.Payload.(map[string]any)
		if moveData == nil || game.Winner != "" {
			return ""
		}

		idxFloat, ok := moveData["index"].(float64)
		if !ok {
			return ""
		}
		idx := int(idxFloat)
		if idx < 0 || idx > 8 || game.Board[idx] != "" {
			return ""
		}

		player := game.Turn
		game.Board[idx] = player
		game.LastBy = client.Id()

		// Alternate turn
		if game.Turn == "X" {
			game.Turn = "O"
		} else {
			game.Turn = "X"
		}

		m.Payload = map[string]any{"player": player}
		return "game:evaluate" // Chain to winner evaluation
	})

	ws.On("game:evaluate", func(m *ws.Message, client *ws.Client) string {
		gameMu.Lock()
		b := game.Board
		lines := [][3]int{
			{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, // rows
			{0, 3, 6}, {1, 4, 7}, {2, 5, 8}, // cols
			{0, 4, 8}, {2, 4, 6},             // diagonals
		}

		for _, line := range lines {
			if b[line[0]] != "" && b[line[0]] == b[line[1]] && b[line[1]] == b[line[2]] {
				game.Winner = b[line[0]]
				if game.Winner == "X" {
					game.ScoreX++
				} else {
					game.ScoreO++
				}
				break
			}
		}

		// Check draw
		if game.Winner == "" {
			full := true
			for _, cell := range b {
				if cell == "" {
					full = false
					break
				}
			}
			if full {
				game.Winner = "Draw"
			}
		}

		turnNow := game.Turn
		winnerNow := game.Winner
		gameMu.Unlock()

		// Broadcast game update to all players
		ws.Send(ws.Message{
			Title:   "game:updated",
			Payload: game,
		})

		// --- AUTOMATED OPPONENT MOVE ---
		// When it's O's turn and no winner yet, simulate automated AI opponent move after 600ms
		if winnerNow == "" && turnNow == "O" {
			go func() {
				time.Sleep(600 * time.Millisecond)
				gameMu.Lock()
				if game.Turn != "O" || game.Winner != "" {
					gameMu.Unlock()
					return
				}

				var emptySlots []int
				for i, cell := range game.Board {
					if cell == "" {
						emptySlots = append(emptySlots, i)
					}
				}
				if len(emptySlots) == 0 {
					gameMu.Unlock()
					return
				}

				// Check if O can win immediately
				chosen := -1
				for _, slot := range emptySlots {
					testB := game.Board
					testB[slot] = "O"
					for _, l := range lines {
						if testB[l[0]] == "O" && testB[l[1]] == "O" && testB[l[2]] == "O" {
							chosen = slot
							break
						}
					}
					if chosen != -1 {
						break
					}
				}

				// Check if X is about to win: block X!
				if chosen == -1 {
					for _, slot := range emptySlots {
						testB := game.Board
						testB[slot] = "X"
						for _, l := range lines {
							if testB[l[0]] == "X" && testB[l[1]] == "X" && testB[l[2]] == "X" {
								chosen = slot
								break
							}
						}
						if chosen != -1 {
							break
						}
					}
				}

				// Prefer center or corners
				if chosen == -1 {
					if game.Board[4] == "" {
						chosen = 4
					} else {
						for _, c := range []int{0, 2, 6, 8} {
							if game.Board[c] == "" {
								chosen = c
								break
							}
						}
					}
				}
				if chosen == -1 {
					chosen = emptySlots[0]
				}

				game.Board[chosen] = "O"
				game.Turn = "X"
				game.LastBy = "sys-bot (AI Opponent)"

				// Check win for O
				b2 := game.Board
				for _, line := range lines {
					if b2[line[0]] != "" && b2[line[0]] == b2[line[1]] && b2[line[1]] == b2[line[2]] {
						game.Winner = b2[line[0]]
						if game.Winner == "O" {
							game.ScoreO++
						}
						break
					}
				}
				if game.Winner == "" {
					full := true
					for _, cell := range b2 {
						if cell == "" {
							full = false
							break
						}
					}
					if full {
						game.Winner = "Draw"
					}
				}
				gameMu.Unlock()

				ws.Send(ws.Message{
					Title:   "game:updated",
					Payload: game,
				})
			}()
		}

		return ""
	})

	ws.On("game:get", func(m *ws.Message, client *ws.Client) string {
		gameMu.RLock()
		defer gameMu.RUnlock()
		client.Send(ws.Message{
			Title:   "game:updated",
			Payload: game,
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("game:reset", func(m *ws.Message, client *ws.Client) string {
		gameMu.Lock()
		game.Board = [9]string{}
		game.Turn = "X"
		game.Winner = ""
		gameMu.Unlock()

		ws.Send(ws.Message{
			Title:   "game:updated",
			Payload: game,
		})
		return ""
	})

	// --- 7. ROOM & GROUP MANAGEMENT WITH PERMISSION ---
	type RoomMetadata struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		Topic       string   `json:"topic"`
		Icon        string   `json:"icon"`
		CreatorID   string   `json:"creatorId"`
		CreatorName string   `json:"creatorName"`
		IsProtected bool     `json:"isProtected"` // true if requires permission to join
		Members     []string `json:"members"`     // User IDs allowed / members
		Pending     []string `json:"pending"`     // User IDs awaiting approval
		CreatedAt   string   `json:"createdAt"`
	}

	var roomsMu sync.RWMutex
	roomsMap := map[string]*RoomMetadata{
		"general": {
			ID:          "general",
			Name:        "general",
			Topic:       "Welcome to GinWS & open discussion",
			Icon:        "💬",
			CreatorID:   "system",
			CreatorName: "System",
			IsProtected: false,
			Members:     []string{},
			Pending:     []string{},
			CreatedAt:   time.Now().Format("15:04"),
		},
		"public-wall": {
			ID:          "public-wall",
			Name:        "public-wall",
			Topic:       "Public Shouts, Announcements & Live Broadcast Feed",
			Icon:        "📢",
			CreatorID:   "system",
			CreatorName: "System",
			IsProtected: false,
			Members:     []string{},
			Pending:     []string{},
			CreatedAt:   time.Now().Format("15:04"),
		},
		"engineering": {
			ID:          "engineering",
			Name:        "engineering",
			Topic:       "Tech discussions, architecture, and WebSocket tuning",
			Icon:        "⚙️",
			CreatorID:   "system",
			CreatorName: "System",
			IsProtected: false,
			Members:     []string{},
			Pending:     []string{},
			CreatedAt:   time.Now().Format("15:04"),
		},
		"gaming": {
			ID:          "gaming",
			Name:        "gaming",
			Topic:       "Multiplayer Tic-Tac-Toe & game matchmaking",
			Icon:        "🎮",
			CreatorID:   "system",
			CreatorName: "System",
			IsProtected: false,
			Members:     []string{},
			Pending:     []string{},
			CreatedAt:   time.Now().Format("15:04"),
		},
		"vip-squad": {
			ID:          "vip-squad",
			Name:        "vip-squad",
			Topic:       "Exclusive squad channel - requires owner permission",
			Icon:        "🍸",
			CreatorID:   "system",
			CreatorName: "System",
			IsProtected: true,
			Members:     []string{},
			Pending:     []string{},
			CreatedAt:   time.Now().Format("15:04"),
		},
	}

	getRoomsList := func() []*RoomMetadata {
		roomsMu.RLock()
		defer roomsMu.RUnlock()
		list := make([]*RoomMetadata, 0, len(roomsMap))
		order := []string{"general", "public-wall", "engineering", "gaming", "vip-squad"}
		for _, id := range order {
			if r, ok := roomsMap[id]; ok {
				list = append(list, r)
			}
		}
		for id, r := range roomsMap {
			found := false
			for _, o := range order {
				if o == id {
					found = true
					break
				}
			}
			if !found {
				list = append(list, r)
			}
		}
		return list
	}

	ws.On("rooms:get", func(m *ws.Message, client *ws.Client) string {
		client.Send(ws.Message{
			Title:   "rooms:list",
			Payload: getRoomsList(),
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("room:create", func(m *ws.Message, client *ws.Client) string {
		data, _ := m.Payload.(map[string]any)
		if data == nil {
			return ""
		}
		rawName, _ := data["name"].(string)
		cleanName := strings.ToLower(strings.TrimSpace(rawName))
		if cleanName == "" {
			return ""
		}
		topic, _ := data["topic"].(string)
		icon, _ := data["icon"].(string)
		if icon == "" {
			icon = "💬"
		}
		isProtected, _ := data["isProtected"].(bool)

		usersMu.RLock()
		creatorProfile := users[client.Id()]
		usersMu.RUnlock()
		creatorName := client.Id()
		if creatorProfile != nil {
			creatorName = creatorProfile.Username
		}

		roomsMu.Lock()
		if _, exists := roomsMap[cleanName]; !exists {
			roomsMap[cleanName] = &RoomMetadata{
				ID:          cleanName,
				Name:        cleanName,
				Topic:       topic,
				Icon:        icon,
				CreatorID:   client.Id(),
				CreatorName: creatorName,
				IsProtected: isProtected,
				Members:     []string{client.Id()},
				Pending:     []string{},
				CreatedAt:   time.Now().Format("15:04"),
			}
		} else {
			if roomsMap[cleanName].CreatorID == client.Id() {
				if topic != "" {
					roomsMap[cleanName].Topic = topic
				}
				if icon != "" {
					roomsMap[cleanName].Icon = icon
				}
				roomsMap[cleanName].IsProtected = isProtected
			}
		}
		createdRoom := roomsMap[cleanName]
		roomsMu.Unlock()

		ws.Send(ws.Message{
			Title:   "rooms:updated",
			Payload: getRoomsList(),
		})

		client.Send(ws.Message{
			Title:   "room:created",
			Payload: createdRoom,
			Reply:   m.Reply,
		})
		return ""
	})

	ws.On("room:request_access", func(m *ws.Message, client *ws.Client) string {
		data, _ := m.Payload.(map[string]any)
		if data == nil {
			return ""
		}
		roomID, _ := data["roomId"].(string)
		if roomID == "" {
			return ""
		}

		roomsMu.Lock()
		room := roomsMap[roomID]
		if room == nil || !room.IsProtected {
			roomsMu.Unlock()
			return ""
		}

		for _, mId := range room.Members {
			if mId == client.Id() {
				roomsMu.Unlock()
				client.Send(ws.Message{
					Title:   "room:access_granted",
					Payload: map[string]any{"roomId": roomID, "approved": true, "alreadyMember": true},
					Reply:   m.Reply,
				})
				return ""
			}
		}

		alreadyPending := false
		for _, pId := range room.Pending {
			if pId == client.Id() {
				alreadyPending = true
				break
			}
		}
		if !alreadyPending {
			room.Pending = append(room.Pending, client.Id())
		}
		creatorID := room.CreatorID
		roomName := room.Name
		roomsMu.Unlock()

		usersMu.RLock()
		requesterProfile := users[client.Id()]
		usersMu.RUnlock()
		reqName := client.Id()
		reqAvatar := "👤"
		if requesterProfile != nil {
			reqName = requesterProfile.Username
			reqAvatar = requesterProfile.Avatar
		}

		if creatorID != "" && creatorID != client.Id() {
			ws.Send(ws.Message{
				Title: "room:permission_request",
				Id:    creatorID,
				Payload: map[string]any{
					"roomId":          roomID,
					"roomName":        roomName,
					"requesterId":     client.Id(),
					"requesterName":   reqName,
					"requesterAvatar": reqAvatar,
					"time":            time.Now().Format("15:04:05"),
				},
			})
		}

		client.Send(ws.Message{
			Title: "room:request_pending",
			Payload: map[string]any{
				"roomId": roomID,
				"status": "pending",
			},
			Reply: m.Reply,
		})

		ws.Send(ws.Message{
			Title:   "rooms:updated",
			Payload: getRoomsList(),
		})
		return ""
	})

	ws.On("room:approve_access", func(m *ws.Message, client *ws.Client) string {
		data, _ := m.Payload.(map[string]any)
		if data == nil {
			return ""
		}
		roomID, _ := data["roomId"].(string)
		requesterID, _ := data["requesterId"].(string)
		approved, _ := data["approved"].(bool)
		if roomID == "" || requesterID == "" {
			return ""
		}

		roomsMu.Lock()
		room := roomsMap[roomID]
		if room == nil {
			roomsMu.Unlock()
			return ""
		}

		if room.CreatorID != client.Id() && room.CreatorID != "system" {
			roomsMu.Unlock()
			return ""
		}

		newPending := make([]string, 0, len(room.Pending))
		for _, p := range room.Pending {
			if p != requesterID {
				newPending = append(newPending, p)
			}
		}
		room.Pending = newPending

		if approved {
			alreadyMember := false
			for _, mb := range room.Members {
				if mb == requesterID {
					alreadyMember = true
					break
				}
			}
			if !alreadyMember {
				room.Members = append(room.Members, requesterID)
			}
		}
		roomName := room.Name
		roomsMu.Unlock()

		if approved {
			ws.Send(ws.Message{
				Title: "room:access_granted",
				Id:    requesterID,
				Payload: map[string]any{
					"roomId":   roomID,
					"roomName": roomName,
					"approved": true,
				},
			})
		} else {
			ws.Send(ws.Message{
				Title: "room:access_denied",
				Id:    requesterID,
				Payload: map[string]any{
					"roomId":   roomID,
					"roomName": roomName,
					"approved": false,
				},
			})
		}

		ws.Send(ws.Message{
			Title:   "rooms:updated",
			Payload: getRoomsList(),
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
