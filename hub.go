package wshub

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

type Validator func(*Client) error
type ConnectedObserver func(*Client)
type MessageObserver func(*Client, string)
type ClosedObserver func(*Client)
type ShutdownObserver func(*Hub)

var defaultShutdownHandler ShutdownObserver = func(h *Hub) {
	h.Println("Hub quite running...")
}

type Hub struct {
	*sync.RWMutex
	*log.Logger
	running  bool
	list     map[*Client]bool
	add      chan *Client
	del      chan *Client
	shutdown chan bool
	Validator
	ConnectedObserver
	MessageObserver
	ClosedObserver
	ShutdownObserver
}

func New(l *log.Logger) *Hub {
	return &Hub{
		RWMutex:          &sync.RWMutex{},
		Logger:           l,
		list:             make(map[*Client]bool),
		add:              make(chan *Client),
		del:              make(chan *Client),
		shutdown:         make(chan bool),
		ShutdownObserver: defaultShutdownHandler,
	}
}

func (h *Hub) IsRunning() bool {
	h.RLock()
	defer h.RUnlock()

	return h.running
}

func (h *Hub) Run() {
	if h.IsRunning() {
		panic("already running")
	}
	h.Lock()
	h.running = true
	h.Unlock()

	defer h.ShutdownObserver(h)
	for {
		select {
		case c := <-h.del:
			h.Println("try delete list[c]...")
			delete(h.list, c)
			c.Quite("del chan receive [c]")

		case c := <-h.add:
			h.Println("try set list[c] = true...")
			h.list[c] = true

		case <-h.shutdown:
			h.Println("try shutdown...")
			h.Lock()
			h.running = false
			h.Unlock()
			for c := range h.list {
				go func(cc *Client) {
					cc.Quite("hub shutdown")
				}(c)
			}
			return

		default:
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

func (h *Hub) Send(c *Client, data interface{}) {
	select {
	case c.msg <- data:
	default:
		h.Println("channel c.msg closed")
	}
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

func (h *Hub) Handler(connHandler func(*websocket.Conn)) http.Handler {

	return websocket.Handler(func(conn *websocket.Conn) {

		connHandler(conn)

		h.Println("new ws connection")
		defer h.Printf("quite websocket handler")
		defer conn.Close()

		if !h.IsRunning() {
			return
		}

		// NOTE: 預設如果沒嵌入驗證方法就當允許連線
		var verified bool = true
		c := newClient(h, conn)
		// 必須啟動 sender 才能真正調用 Send 方法
		// 否則會堵住
		go c.senderRun(func(data interface{}) (interface{}, error) {
			if !verified {
				return "", fmt.Errorf("unverified")
			}
			return data, nil
		})

		if h.Validator != nil {
			h.Println("fire Validator...")
			if err := h.Validator(c); err != nil {
				h.Println("verified fail")
				c.Quite("verified fail")
				h.Println("send quite signal to client")
				return
			}
			h.Println("verified success")
			verified = true
		}

		h.add <- c

		if h.ConnectedObserver != nil {
			h.Println("fire ConnectedObserver...")
			h.ConnectedObserver(c)
		}

		c.receiverRun(func(s string) {
			if h.MessageObserver != nil {
				h.MessageObserver(c, s)
			}
		})

		h.del <- c

		if h.ClosedObserver != nil {
			h.Println("fire closedObserver...")
			h.ClosedObserver(c)
		}

	})
}
