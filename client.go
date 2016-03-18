package wshub

import (
	"io"
	"net/http"

	"golang.org/x/net/websocket"
)

type Client struct {
	hub   *Hub
	conn  *websocket.Conn
	quite chan string
	msg   chan interface{}
}

func (c *Client) Request() *http.Request {
	return c.conn.Request()
}

func newClient(h *Hub, conn *websocket.Conn) *Client {
	c := &Client{
		hub:   h,
		conn:  conn,
		quite: make(chan string),
		msg:   make(chan interface{}, 1),
	}
	c.msg <- nil
	return c
}

func (c *Client) receiverRun(f func(string)) {
	defer c.hub.Println("quite receiver", c)
	for {
		select {
		case <-c.quite:
			c.hub.Println("receiver try quite sender")
			c.Quite("receiver try quite sender")
			return
		default:
			c.hub.Println("reader wait receive...")
			var s string
			err := websocket.Message.Receive(c.conn, &s)
			c.hub.Printf("reader receive %+v, err=%#v\n", s, err)
			if err == io.EOF {
				// 客端關閉連線
				c.hub.Println("client close conn")
				c.Quite("client closed")
				return
			} else if err != nil {
				// 解析有錯
				c.hub.Println("receive error:", err)
			} else {
				c.hub.Println("receive:", s)
				f(s)
			}
		}
	}
}

func (c *Client) senderRun(f func(interface{}) (interface{}, error)) {
	defer c.hub.Println("quite sender")
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
					c.hub.Println("send string error:", err)
				}
			default:
				if err := websocket.JSON.Send(c.conn, m); err != nil {
					c.hub.Println("send json error:", err)
				}
			}
		case <-c.quite:
			c.Quite("sender try quite receiver")
			return
		}
	}
}

func (c *Client) Quite(s string) {
	c.hub.Printf("\033[31mquite <- [ %s ]\033[m\n", s)
	defer c.hub.Printf("\033[32mquite <- [ %s ] done\033[m\n", s)
	select {
	case c.quite <- s:
	default:
	}
}
