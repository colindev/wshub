package wshub

import (
	"log"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Receiver func(*Client, string)

type Hub interface {
	Kick(*Client)
	Send(*Client, string)
	Each(func(*Client))
	Shutdown()
	HandleReceive(Receiver)
}

type hub struct {
	*log.Logger
	*sync.Mutex
	online   bool
	list     map[*Client]bool
	add      chan *Client
	del      chan *Client
	shutdown chan bool
	receiver Receiver
}

func (h *hub) run() {
	for {
		select {
		case c := <-h.del:
			delete(h.list, c)
			c.quiteW <- true

		case c := <-h.add:
			h.list[c] = true

		case <-h.shutdown:
			h.Println("try shutdown...")
			h.online = false
			for c := range h.list {
				delete(h.list, c)
				c.quiteW <- true
			}
		}
	}
}

func (h *hub) Kick(c *Client) {
	h.del <- c
}

func (h *hub) HandleReceive(r Receiver) {
	h.receiver = r
}

func (h *hub) Each(f func(*Client)) {
	for c := range h.list {
		f(c)
	}
}

func (h *hub) Send(c *Client, m string) {
	c.msg <- m
}

func (h *hub) Shutdown() {
	h.shutdown <- true
}

func Handler(f func(Hub), l *log.Logger) http.Handler {

	// 裝飾模式, 調用 x/net/websocketi.Handler 取得 http.Handler
	h := &hub{
		Logger:   l,
		Mutex:    &sync.Mutex{},
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
		c := newClient(h, conn)
		h.add <- c
		h.Println("h.add <- c")
		go c.write()
		h.Println("go c.write()")
		h.Println("c.read()")
		c.read(func(s string) {
			if h.receiver != nil {
				h.receiver(c, s)
			}
		})
		h.Println("connection end")
	})
}
