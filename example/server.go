package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"golang.org/x/net/websocket"

	"github.com/colindev/wshub"
)

type collector struct {
	*sync.Mutex
	hub  *wshub.Hub
	list map[string]map[*wshub.Client]bool
	info map[*wshub.Client]map[string]interface{}
}

func (clt *collector) Push(id string, c *wshub.Client) {
	clt.Lock()
	defer clt.Unlock()

	if m, ok := clt.list[id]; ok {
		m[c] = true
	} else {
		clt.list[id] = map[*wshub.Client]bool{c: true}
	}
}

func (clt *collector) Find(id string) (map[*wshub.Client]bool, bool) {
	clt.Lock()
	defer clt.Unlock()
	m, ok := clt.list[id]
	return m, ok
}

func (clt *collector) Del(c *wshub.Client) {
	clt.Lock()
	defer clt.Unlock()
	if m, ok := clt.info[c]; ok {
		delete(clt.info, c)
		if id, ok := m["id"]; ok {
			delete(clt.list, id.(string))
		}
	}
}

func (clt *collector) Kick(id string) {
	clt.Lock()
	defer clt.Unlock()
	if m, ok := clt.list[id]; ok {
		delete(clt.list, id)
		for c := range m {
			delete(clt.info, c)
			clt.hub.Kick(c)
		}
	}
}

func main() {

	hub := wshub.New(log.New(os.Stdout, "wshub:", log.Lshortfile))

	hub.MessageObserver = func(c *wshub.Client, msg string) {
		fmt.Println(">>", c, msg)
		hub.Send(c, "echo:"+msg)
	}

	http.Handle("/ws", hub.Handler(func(conn *websocket.Conn) {

		fmt.Println(conn)

	}))

	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
		
		<script>
		
		var ws = new WebSocket('ws://'+location.host+'/ws');
		ws.onopen = function(e){console.info('opened', e);};
		ws.onclose = function(e){console.info('closed', e);};
		ws.onmessage = function(e){console.info('msg <--', e.data);};

		</script>
		
		`))
	})

	go hub.Run()
	http.ListenAndServe(":8000", nil)
}
