package wshub

import (
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"sync"

	"golang.org/x/net/websocket"
)

type Validator func(*Client) error
type ConnectedObserver func(*Client)
type MessageObserver func(*Client, string)
type ErrorObserver func(error)
type ClosedObserver func(*Client)
type ShutdownObserver func(*Hub)

var (
	defaultShutdownHandler ShutdownObserver = func(h *Hub) {
		fmt.Println("Hub quite running...")
	}
	defaultErrorHandler ErrorObserver = func(e error) {
		_, f, l, _ := runtime.Caller(1)
		fmt.Printf("wshub.Hub: %s:%d %s", f, l, e)
	}
)

type Hub struct {
	*sync.RWMutex
	running  bool
	stack    []Middleware
	list     map[*Client]bool
	add      chan *Client
	del      chan *Client
	shutdown chan bool
	Validator
	ErrorObserver
	ConnectedObserver
	MessageObserver
	ClosedObserver
	ShutdownObserver
}

func New() *Hub {
	return &Hub{
		RWMutex:          &sync.RWMutex{},
		stack:            make([]Middleware, 0),
		list:             make(map[*Client]bool),
		add:              make(chan *Client),
		del:              make(chan *Client),
		shutdown:         make(chan bool),
		ErrorObserver:    defaultErrorHandler,
		ShutdownObserver: defaultShutdownHandler,
	}
}

func (h *Hub) Use(ms ...Middleware) {
	h.stack = append(h.stack, ms...)
}

func (h *Hub) IsRunning() bool {
	h.RLock()
	defer h.RUnlock()

	return h.running
}

func (h *Hub) Run() {

	if h.IsRunning() {
		h.ErrorObserver(errors.New("already running"))
		return
	}

	h.Lock()
	h.running = true
	h.Unlock()

	defer h.ShutdownObserver(h)
	for {
		select {
		case c := <-h.del:
			delete(h.list, c)
			c.Quite("del chan receive [c]")

		case c := <-h.add:
			h.list[c] = true

		case <-h.shutdown:
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

func (h *Hub) Broadcast(m interface{}, f func(*Client, interface{}) (interface{}, error)) int {
	var n int

	if f == nil {
		f = func(c *Client, v interface{}) (interface{}, error) {
			return v, nil
		}
	}

	h.Each(func(c *Client) {
		if mm, err := f(c, m); err == nil {
			if e := c.Send(mm); e == nil {
				n++
			} else {
				h.ErrorObserver(e)
			}
		}
	})

	return n
}

func (h *Hub) Shutdown() {
	h.shutdown <- true
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	handler := func(conn *websocket.Conn) {

		defer conn.Close()

		if !h.IsRunning() {
			h.ErrorObserver(errors.New("wshub is not running!!"))
			return
		}

		// NOTE: 預設如果沒嵌入驗證方法就當允許連線
		var verified bool = true
		c, err := newClient(h, conn)
		if err != nil {
			h.ErrorObserver(err)
			return
		}

		// 必須啟動 sender 才能真正調用 Send 方法
		// 否則會堵住
		go c.senderRun(func(data interface{}) (interface{}, error) {
			if !verified {
				return "", fmt.Errorf("unverified")
			}
			return data, nil
		})

		if h.Validator != nil {
			if err := h.Validator(c); err != nil {
				h.ErrorObserver(fmt.Errorf("verified fail: %s", err))
				c.Quite("verified fail")
				return
			}
			verified = true
		}

		h.add <- c

		if h.ConnectedObserver != nil {
			h.ConnectedObserver(c)
		}

		c.receiverRun(func(s string) {
			if h.MessageObserver != nil {
				h.MessageObserver(c, s)
			}
		})

		h.del <- c

		if h.ClosedObserver != nil {
			h.ClosedObserver(c)
		}

	}

	for i := len(h.stack) - 1; i >= 0; i-- {
		handler = h.stack[i].Wrap(handler)
	}

	websocket.Handler(handler).ServeHTTP(w, r)
}
