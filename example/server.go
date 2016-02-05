package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/colin1124x/wshub"
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
	http.Handle("/ws", wshub.Handler(func(h *wshub.Hub) {

		clt := &collector{
			Mutex: &sync.Mutex{},
			list:  make(map[string]map[*wshub.Client]bool),
			info:  make(map[*wshub.Client]map[string]interface{}),
		}

		h.Validator = func(c *wshub.Client) error {
			query := c.Request().URL.Query()
			token := query.Get("token")

			fmt.Println(token)

			//return fmt.Errorf("x")
			return nil
			//	switch token {
			//	case "a":

			//		return nil
			//	default:
			//		return fmt.Errorf("error")
			//	}
		}
		h.ConnectedObserver = func(c *wshub.Client) {
			fmt.Printf("ConnectedObserver: %+v\n", c)
			clt.Push(c.Request().URL.Query().Get("id"), c)
			h.Send(c, fmt.Sprintf("conns: %d", h.Count()))
		}
		h.MessageObserver = func(c *wshub.Client, v string) {
			fmt.Printf("MessageObserver: %+v --> %s\n", c, v)

			h.Each(func(x *wshub.Client) {
				h.Send(c, fmt.Sprintf("%+v", x))
			})
		}
		h.ClosedObserver = func(c *wshub.Client) {
			fmt.Printf("ClosedObserver: %+v\n", c)
		}

	}, log.New(os.Stderr, "", log.Lshortfile)))

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

	http.ListenAndServe(":8000", nil)
}
