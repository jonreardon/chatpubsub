package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
)

type ConnectRec struct {
	connected bool
	username  string
}

var connections map[*websocket.Conn]ConnectRec

func sendAll(msg, username []byte) {

	for conn := range connections {
		if err := conn.WriteMessage(websocket.TextMessage, append(username, msg...)); err != nil {
			delete(connections, conn)
			conn.Close()
		}
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	//from gorilla
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	var bufferName bytes.Buffer
	bufferName.WriteString("User" + strconv.Itoa(len(connections)) + ": ") //fastest for concatenation

	log.Println("Succesfully upgraded connection")
	connections[conn] = ConnectRec{true, bufferName.String()}
	log.Printf("%v", connections)
	log.Printf("%v", len(connections))

	for {
		// Blocks until a message is read
		_, msg, err := conn.ReadMessage()
		if err != nil {
			delete(connections, conn)
			conn.Close()
			return
		}
		log.Println(connections[conn].username + ": " + string(msg))
		sendAll(msg, []byte(bufferName.String()))
	}
}

func mainOLD() {
	// command line flags
	port := flag.Int("port", 8005, "port to serve on")
	dir := flag.String("directory", "web/", "directory of web files")
	flag.Parse()

	connections = make(map[*websocket.Conn]ConnectRec)

	// handle all requests by serving a file of the same name
	fs := http.Dir(*dir)
	fileHandler := http.FileServer(fs)
	http.Handle("/", fileHandler)
	http.HandleFunc("/ws", wsHandler)

	log.Printf("Running on port %d\n", *port)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	// this call blocks -- the progam runs here forever
	err := http.ListenAndServe(addr, nil)
	fmt.Println(err.Error())
}
