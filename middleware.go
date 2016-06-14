package wshub

import "golang.org/x/net/websocket"

type Middleware interface {
	Wrap(websocket.Handler) websocket.Handler
}
