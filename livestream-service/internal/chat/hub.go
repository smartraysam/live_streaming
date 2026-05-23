package chat

import (
	"sync"

	"github.com/gorilla/websocket"
)

type client struct {
	conn   *websocket.Conn
	userID string
}

type Hub struct {
	register   chan *client
	unregister chan *client
	broadcast  chan Message
	clients    map[*client]struct{}
	private    bool
	creatorID  string
}

func (h *Hub) Publish(msg Message) {
	h.broadcast <- msg
}

func NewHub(private bool, creatorID string) *Hub {
	return &Hub{
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan Message, 32),
		clients:    map[*client]struct{}{},
		private:    private,
		creatorID:  creatorID,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			if h.private && h.viewerCount() >= 1 && c.userID != h.creatorID {
				_ = c.conn.WriteJSON(map[string]string{"error": "session_full"})
				_ = c.conn.Close()
				continue
			}
			h.clients[c] = struct{}{}
		case c := <-h.unregister:
			delete(h.clients, c)
			_ = c.conn.Close()
		case m := <-h.broadcast:
			for c := range h.clients {
				_ = c.conn.WriteJSON(m)
			}
		}
	}
}

func (h *Hub) viewerCount() int {
	n := 0
	for c := range h.clients {
		if c.userID != h.creatorID {
			n++
		}
	}
	return n
}

type HubManager struct {
	hubs sync.Map
}

func (m *HubManager) GetOrCreate(streamID string, private bool, creatorID string) *Hub {
	if v, ok := m.hubs.Load(streamID); ok {
		return v.(*Hub)
	}
	h := NewHub(private, creatorID)
	actual, loaded := m.hubs.LoadOrStore(streamID, h)
	if loaded {
		return actual.(*Hub)
	}
	go h.Run()
	return h
}
