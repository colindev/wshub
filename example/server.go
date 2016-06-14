package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/net/websocket"

	"github.com/colindev/wshub"
)

type Message struct {
	Message string `json:"message"`
}

func main() {

	hub := wshub.New(log.New(os.Stdout, "wshub:", log.Lshortfile))

	hub.MessageObserver = func(c *wshub.Client, msg string) {
		fmt.Println(">>", c, msg)
		c.Send("echo:" + msg)
	}

	http.Handle("/ws", hub.Handler(func(conn *websocket.Conn) {

		fmt.Println(">>", conn)

	}))

	http.HandleFunc("/client", func(w http.ResponseWriter, r *http.Request) {
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
	go http.ListenAndServe(":8000", nil)

	reader := bufio.NewReader(os.Stdin)
	for {

		line, _, err := reader.ReadLine()
		if err != nil {
			fmt.Println(err)
			continue
		}
		hub.Broadcast(Message{string(line)}, nil)
		hub.Broadcast(string(line), nil)
		hub.Broadcast(line, nil)
	}
}
