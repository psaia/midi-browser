package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rakyll/portmidi"
)

var addr = flag.String("addr", "localhost:8080", "http service address")
var upgrader = websocket.Upgrader{}

func main() {
	messages := make(chan []byte)
	go midiDriver(messages)
	serve(messages)
}

func midiDriver(messages chan<- []byte) {
	portmidi.Initialize()
	in, err := portmidi.NewInputStream(portmidi.DefaultInputDeviceID(), 1024)
	handleErr(err, "portmidi connection err")

	defer in.Close()

	for e := range in.Listen() {
		d, err := json.Marshal(e)
		handleErr(err, "json marshal err")
		messages <- d
	}
}

func readPumpThenClose(conn *websocket.Conn, done chan<- int) {
	defer conn.Close()
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("breaking read loop because %s", err)
			done <- 1
			break
		}
	}
	log.Println("no longer listening for websocket connections")
}

func writePump(conn *websocket.Conn, messages <-chan []byte, done <-chan int) {
L:
	for {
		select {
		case <-done:
			break L
		case msg := <-messages:
			handleErr(conn.WriteMessage(websocket.TextMessage, msg), "msg write err")
		default:
		}
	}
	log.Println("no longer listening for incoming midi messages")
}

func ws(w http.ResponseWriter, r *http.Request, messages <-chan []byte) {
	conn, err := upgrader.Upgrade(w, r, nil)
	handleErr(err, "websocket connection err")

	done := make(chan int)

	go writePump(conn, messages, done)
	go readPumpThenClose(conn, done)

	log.Println("ws connected")
}

func serve(messages chan []byte) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "slash.html")
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws(w, r, messages)
	})
	log.Printf("listening on: %s\n", *addr)
	handleErr(http.ListenAndServe(*addr, nil), "srv listening err")
}

func handleErr(err error, m string) {
	if err != nil {
		log.Printf(">>>>>>>>>>>>>> %s\n", m)
		log.Fatal(err)
	}
}
