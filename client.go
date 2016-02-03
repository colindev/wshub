package wshub

import (
	"io"
	"net/http"

	"golang.org/x/net/websocket"
)

type Client struct {
	hub    *hub
	conn   *websocket.Conn
	quiteR chan bool
	quiteW chan bool
	msg    chan string
}

func (c *Client) Request() *http.Request {
	return c.conn.Request()
}

func newClient(h *hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:    h,
		conn:   conn,
		quiteR: make(chan bool),
		quiteW: make(chan bool),
		msg:    make(chan string),
	}
}

func (c *Client) read(f func(string)) {
	defer c.hub.Println("quite read", c)
	for {
		select {
		case <-c.quiteR:
			c.hub.Println("reader try quite writer")
			return
		default:
			c.hub.Println("reader wait receive...")
			var s string
			err := websocket.Message.Receive(c.conn, &s)
			c.hub.Printf("reader receive %+v, err=%#v\n", s, err)
			if err == io.EOF {
				// 客端關閉連線
				c.hub.Println("client close conn")
				c.quiteW <- true
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

func (c *Client) write() {
	defer c.hub.Println("quite write", c)
	for {
		select {
		case m, ok := <-c.msg:
			if !ok {
				c.hub.Println("c.msg closed?")
				c.quiteR <- true
				return
			}
			if err := websocket.Message.Send(c.conn, m); err != nil {
				c.hub.Println("send error:", err)
			}
		case <-c.quiteW:
			return
		}
	}
}
