package wshub

import "net/http"

type Middleware interface {
	Wrap(http.HandlerFunc) http.HandlerFunc
}
