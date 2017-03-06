package main

import (
	"bufio"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/colindev/wshub"
	"github.com/gorilla/websocket"
)

type Message struct {
	Message string `json:"message"`
}

type AccessMiddleware struct{}

func (x *AccessMiddleware) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Header)
		h(w, r)
	}
}

var (
	addr string
)

func init() {
	flag.StringVar(&addr, "addr", ":8000", "http address")

	log.SetFlags(log.Lshortfile)
}

func main() {

	flag.Parse()

	hub := wshub.New()

	hub.MessageObserver = func(c *websocket.Conn, p []byte) {
		msg := string(p)
		c.WriteMessage(websocket.TextMessage, []byte("echo:"+msg))
		log.Println(">>", c, msg)
	}

	hub.Use(&AccessMiddleware{})

	http.Handle("/ws", hub)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
		
		<script>
		
		var ws = new WebSocket('ws://'+location.host+'/ws');
		ws.onopen = function(e){console.info('opened', e);};
		ws.onclose = function(e){console.info('closed', e);};
		ws.onmessage = function(e){
			console.info('msg <--', e.data);

			if (e.data instanceof Blob) {
				var r1 = new FileReader;
				r1.onload = function(){
					console.log("as text", r1.result);
				}
				r1.readAsText(e.data);

				var r2 = new FileReader;
				r2.onload = function(){
					console.log("as binary string", r2.result);
				}
				r2.readAsBinaryString(e.data);

				var r3 = new FileReader;
				r3.onload = function(){
					console.log("as array buffer", r3.result);
				}
				r3.readAsArrayBuffer(e.data);
			}
		};

		</script>
		
		`))
	})

	go hub.Run()
	go http.ListenAndServe(addr, nil)

	reader := bufio.NewReader(os.Stdin)
	for {

		line, _, err := reader.ReadLine()
		if err != nil {
			log.Println(err)
			continue
		} else if len(line) == 0 {
			continue
		}

		hub.Broadcast(Message{"struct:" + string(line)})
		hub.Broadcast("string:" + string(line))
		hub.Broadcast(append([]byte("byte:"), line...))
	}
}
