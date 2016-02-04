package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/colin1124x/wshub"
)

func main() {
	http.Handle("/ws", wshub.Handler(func(h *wshub.Hub) {

		h.Validator = func(r *http.Request) error {
			fmt.Printf("Validator: %+v\n", r)
			return nil
		}
		h.ConnectedObserver = func(c *wshub.Client) {
			fmt.Printf("ConnectedObserver: %+v\n", c)
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
