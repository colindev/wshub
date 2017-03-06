package wshub

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"golang.org/x/net/websocket"
)

func init() {
	defaultMarshal := websocket.Message.Marshal
	websocket.Message.Marshal = func(v interface{}) (msg []byte, payloadType byte, err error) {
		if data, ok := v.([]byte); ok && len(data) == 0 {
			return nil, websocket.PingFrame, nil
		}
		return defaultMarshal(v)
	}
}

// Client connection
type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	quite chan string
	msg   chan interface{}
	valid bool
	sync.RWMutex
}

func newClient(h *Hub, conn *websocket.Conn) (*Client, error) {
	c := &Client{
		hub:   h,
		conn:  conn,
		quite: make(chan string),
		msg:   make(chan interface{}, 10),
		valid: true,
	}

	if err := c.Send(([]byte)(nil)); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) isValid() bool {
	c.RLock()
	defer c.RUnlock()
	return c.valid
}

func (c *Client) setValid(t bool) {
	c.Lock()
	defer c.Unlock()
	c.valid = t
}

func (c *Client) receiverRun(handler func(string)) {
	for {
		select {
		case <-c.quite:
			c.Quite("receiver try quite sender")
			return
		default:
			var s string
			err := websocket.Message.Receive(c.conn, &s)
			if !c.resolveInCome(s, err, handler, c.hub.ErrorObserver) {
				return
			}
		}
	}
}

func (c *Client) resolveInCome(s string, e error, accept func(string), reject func(error)) (keepOn bool) {
	if e == io.EOF {
		// 客端關閉連線
		c.Quite("client closed")
		return false
	} else if e != nil {
		// 解析有錯
		reject(fmt.Errorf("wshub client: %s", e))
	} else {
		accept(s)
	}
	return true
}

func (c *Client) senderRun(f func(interface{}) (interface{}, error)) {
	for {
		select {
		case m, ok := <-c.msg:
			if !ok {
				c.Quite("msg closed")
				return
			}
			m, e := f(m)
			if m == nil || e != nil {
				continue
			}
			switch m.(type) {
			case string, []byte:
				if err := websocket.Message.Send(c.conn, m); err != nil {
					c.hub.ErrorObserver(fmt.Errorf("wshub client: %s", err))
				}
			default:
				if err := websocket.JSON.Send(c.conn, m); err != nil {
					c.hub.ErrorObserver(fmt.Errorf("wshub client: %s", err))
				}
			}

		case <-c.quite:
			c.Quite("sender try quite receiver")
			return

		default:
		}
	}
}

func (c *Client) Request() *http.Request {
	return c.conn.Request()
}

func (c *Client) Send(data interface{}) error {
	select {
	case c.msg <- data:
		return nil
	default:
		return fmt.Errorf("client closed: %+v", c.Request().Header)
	}
}

func (c *Client) Quite(s string) {
	select {
	case c.quite <- s:
	default:
	}
}
