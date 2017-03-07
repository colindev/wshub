package wshub

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"sync"

	"github.com/gorilla/websocket"
)

type MessageObserver func(*websocket.Conn, []byte)
type ErrorObserver func(error)
type ShutdownObserver func(*Hub)

var (
	defaultShutdownHandler ShutdownObserver = func(h *Hub) {
		log.Println("Hub shutdown...")
	}
	defaultErrorHandler ErrorObserver = func(e error) {
		_, f, l, _ := runtime.Caller(1)
		log.Printf("wshub.Hub: %s:%d %s\n", f, l, e)
	}
)

type Hub struct {
	sync.RWMutex
	running  bool
	stack    []Middleware
	list     map[*websocket.Conn]bool
	add      chan *websocket.Conn
	del      chan *websocket.Conn
	shutdown chan bool
	ErrorObserver
	MessageObserver
	ShutdownObserver
}

func New() *Hub {
	hub := &Hub{
		stack:            make([]Middleware, 0),
		list:             make(map[*websocket.Conn]bool),
		add:              make(chan *websocket.Conn, 1),
		del:              make(chan *websocket.Conn, 1),
		shutdown:         make(chan bool, 1),
		ErrorObserver:    defaultErrorHandler,
		ShutdownObserver: defaultShutdownHandler,
	}

	hub.MessageObserver = func(_ *websocket.Conn, p []byte) {
		hub.Broadcast(p)
	}

	return hub
}

func (h *Hub) Use(ms ...Middleware) {
	h.stack = append(h.stack, ms...)
}

func (h *Hub) isRunning() bool {
	h.RLock()
	defer h.RUnlock()

	return h.running
}

func (h *Hub) setRunning(t bool) {
	h.Lock()
	defer h.Unlock()

	h.running = t
}

func (h *Hub) Run() {

	if h.isRunning() {
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
			c.WriteMessage(websocket.TextMessage, []byte("bye"))
			delete(h.list, c)
			c.Close()

		case c := <-h.add:
			h.list[c] = true

		case <-h.shutdown:
			h.Lock()
			h.running = false
			h.Unlock()
			for c := range h.list {
				c.Close()
			}
			return
		}
	}
}

func (h *Hub) Kick(c *websocket.Conn) {
	h.del <- c
}

func (h *Hub) Each(handler func(*websocket.Conn)) {
	for c := range h.list {
		handler(c)
	}
}

func (h *Hub) Count() int {
	h.RLock()
	defer h.RUnlock()
	return len(h.list)
}

func (h *Hub) Broadcast(m interface{}) (n int) {
	var (
		pm  *websocket.PreparedMessage
		err error
	)
	h.RLock()
	errObserver := h.ErrorObserver
	h.RUnlock()

	switch m := m.(type) {
	case []byte:
		pm, err = websocket.NewPreparedMessage(websocket.TextMessage, m)
	case string:
		pm, err = websocket.NewPreparedMessage(websocket.TextMessage, []byte(m))
	default:
		b, e := json.Marshal(m)
		if e != nil {
			err = e
			break
		}
		pm, err = websocket.NewPreparedMessage(websocket.TextMessage, b)
	}

	if err != nil {
		errObserver(err)
		return
	}

	h.Each(func(c *websocket.Conn) {
		if err := c.WritePreparedMessage(pm); err != nil {
			errObserver(err)
		}
	})

	return
}

func (h *Hub) Shutdown() {
	h.shutdown <- true
}

func (h *Hub) Handler() http.Handler {

	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
	}

	handler := func(w http.ResponseWriter, r *http.Request) {

		h.RLock()
		errObserver := h.ErrorObserver
		h.RUnlock()

		if !h.isRunning() {
			errObserver(errors.New("wshub is not running"))
			return
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			errObserver(err)
			return
		}

		h.add <- c

		h.RLock()
		msgObserver := h.MessageObserver
		h.RUnlock()

		for {
			msgType, p, err := c.ReadMessage()
			if err != nil {
				errObserver(err)
				break
			}
			if msgType != websocket.TextMessage {
				errObserver(fmt.Errorf("receive message type: %#v", msgType))
				break
			}

			msgObserver(c, p)

		}

		h.del <- c

	}

	h.RLock()
	stack := h.stack
	h.RUnlock()

	for i := len(stack) - 1; i >= 0; i-- {
		handler = stack[i].Wrap(handler)
	}

	return http.HandlerFunc(handler)
}
