package wshub

import (
	"log"
	"net/http"

	"golang.org/x/net/websocket"
)

type Validator func(*http.Request) error
type ConnectedObserver func(*Client)
type MessageObserver func(*Client, string)
type ClosedObserver func(*Client)

type Hub struct {
	*log.Logger
	online   bool
	list     map[*Client]bool
	add      chan *Client
	del      chan *Client
	shutdown chan bool
	Validator
	ConnectedObserver
	MessageObserver
	ClosedObserver
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.del:
			h.Println("try delete list[c]...")
			delete(h.list, c)

		case c := <-h.add:
			h.Println("try set list[c] = true...")
			h.list[c] = true

		case <-h.shutdown:
			h.Println("try shutdown...")
			h.online = false
			for c := range h.list {
				go func(cc *Client) {
					cc.quite <- true
				}(c)
			}
		}
	}
}

func (h *Hub) Kick(c *Client) {
	h.del <- c
}

func (h *Hub) Each(f func(*Client)) {
	for c := range h.list {
		f(c)
	}
}

func (h *Hub) Count() int {
	return len(h.list)
}

func (h *Hub) Send(c *Client, m string) {
	c.msg <- m
}

func (h *Hub) Broadcast(m string, f func(*Client, string) (string, error)) {
	h.Each(func(c *Client) {
		if mm, err := f(c, m); err == nil && mm != "" {
			h.Send(c, mm)
		}
	})
}

func (h *Hub) Shutdown() {
	h.shutdown <- true
}

func Handler(f func(*Hub), l *log.Logger) http.Handler {

	// 裝飾模式, 調用 x/net/websocket.Handler 取得 http.Handler
	h := &Hub{
		Logger:   l,
		online:   true,
		list:     make(map[*Client]bool),
		add:      make(chan *Client),
		del:      make(chan *Client),
		shutdown: make(chan bool),
	}

	go h.run()
	h.Println("init hub")
	f(h)

	return websocket.Handler(func(conn *websocket.Conn) {
		h.Println("new Client")
		defer conn.Close()
		if !h.online {
			return
		}

		if h.Validator != nil {
			h.Println("fire Validator...")
			if err := h.Validator(conn.Request()); err != nil {
				return
			}
		}

		c := newClient(h, conn)
		h.add <- c

		go c.write()
		if h.ConnectedObserver != nil {
			h.Println("fire ConnectedObserver...")
			h.ConnectedObserver(c)
		}

		c.read(func(s string) {
			if h.MessageObserver != nil {
				h.MessageObserver(c, s)
			}
		})
		h.del <- c

		if h.ClosedObserver != nil {
			h.Println("fire closedObserver...")
			h.ClosedObserver(c)
		}

		h.Printf("quite websocket handler: %+v\n", c)
	})
}
