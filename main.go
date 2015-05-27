package main

import (
	"flag"
	"fmt"
	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"log"
	"net/http"
)

type Message struct {
	msg string
}

func HandleChatRoot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	file := vars["filename"]
	// Logging for the example
	filename := "./web/" + file
	http.ServeFile(w, r, filename)
}

func HandleChatFolder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	file := vars["filename"]
	folder := vars["folder"]

	filename := "./web/" + folder + "/" + file
	fmt.Println("filename: %s", filename)
	log.Println(filename)

	http.ServeFile(w, r, filename)
}

func HandleBowerFolder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	file := vars["filename"]
	component := vars["component"]

	filename := "./web/bower_components/" + component + "/" + file
	fmt.Println("filename: %s", filename)
	log.Println(filename)

	http.ServeFile(w, r, filename)
}

func HandleBootstrapFolder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	file := vars["filename"]
	folder := vars["folder"]
	dist := vars["dist"]

	filename := "./web/bower_components/" + dist + "/" + folder + "/" + file
	fmt.Println("filename: %s", filename)
	log.Println(filename)

	http.ServeFile(w, r, filename)
}

func main() {
	fmt.Println("PubSub Service")
	port := flag.Int("port", 7373, "port to serve on")
	//dir := flag.String("directory", "web/", "directory of web files")
	flag.Parse()

	//fmt.Println("%v %v", port, dir)

	api := NewApi()

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTION"},
	})

	router := mux.NewRouter()

	router.HandleFunc("/specular/pub/{topic}", api.PubMessageHandler).Methods("GET")
	router.HandleFunc("/specular/{topic}", api.SubMessageHandler).Methods("GET")

	router.HandleFunc("/chat/{filename}", HandleChatRoot).Methods("GET")
	router.HandleFunc("/chat/{folder}/{filename}", HandleChatFolder).Methods("GET")
	router.HandleFunc("/chat/bower_components/{component}/{filename}", HandleBowerFolder).Methods("GET")
	router.HandleFunc("/chat/bower_components/bootstrap/{dist}/{folder}/{filename}", HandleBootstrapFolder).Methods("GET")

	log.Printf("Running on port %d\n", *port)

	n := negroni.Classic()
	n.Use(c)
	n.UseHandler(router)

	log.Fatal(http.ListenAndServe(":7373", n))
	//addr := fmt.Sprintf("127.0.0.1:%d", *port)
	// this call blocks -- the progam runs here forever
	//err := http.ListenAndServe(addr, n)
	//fmt.Println(err.Error())

}

type Api struct {
	ps *PubSub
}

func NewApi() *Api {
	api := &Api{ps: NewPubSub(10)}
	return api
}

func (api *Api) PubMessageHandler(w http.ResponseWriter, request *http.Request) {

	ws, err := websocket.Upgrade(w, request, nil, 1024, 1024)

	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}
	vars := mux.Vars(request)
	topic := vars["topic"]
	fmt.Println("topic {Pub}", topic)
	go func() {
		defer ws.Close()

		for {
			_, p, err := ws.ReadMessage()
			if err != nil {
				return
			}
			api.ps.Pub(p, topic)
		}
	}()

}

func (api *Api) SubMessageHandler(w http.ResponseWriter, request *http.Request) {

	ws, err := websocket.Upgrade(w, request, nil, 1024, 1024)

	if _, ok := err.(websocket.HandshakeError); ok {
		http.Error(w, "Not a websocket handshake", 400)
		return
	} else if err != nil {
		log.Println(err)
		return
	}

	vars := mux.Vars(request)
	topic := vars["topic"]
	fmt.Println("topic {Sub}", topic)

	go func() {
		defer ws.Close()
		for {
			_, p, err := ws.ReadMessage()
			if err != nil {
				return
			}
			api.ps.Pub(p, topic)
		}
	}()
	go func() {
		defer ws.Close()

		subscription := api.ps.Sub(topic)
		for message := range subscription {
			if err = ws.WriteMessage(1, message.([]byte)); err != nil {
				return
			}
		}
	}()

}

//func (api *Api) ListMessageHandler(w http.ResponseWriter, request *http.Request) {
//	topics := api.ps.ListTopics()
//}

type operation int

const (
	sub operation = iota
	subOnce
	list
	pub
	unsub
	unsubAll
	closeTopic
	shutdown
)

// PubSub is a collection of topics.
type PubSub struct {
	cmdChan  chan cmd
	capacity int
}

type cmd struct {
	op     operation
	topics []string
	ch     chan interface{}
	msg    interface{}
}

// New creates a new PubSub and starts a goroutine for handling operations.
// The capacity of the channels created by Sub and SubOnce will be as specified.
func NewPubSub(capacity int) *PubSub {
	ps := &PubSub{make(chan cmd), capacity}
	go ps.start()
	return ps
}

// Sub returns a channel on which messages published on any of
// the specified topics can be received.
func (ps *PubSub) Sub(topics ...string) chan interface{} {
	return ps.sub(sub, topics...)
}

// SubOnce is similar to Sub, but only the first message published, after subscription,
// on any of the specified topics can be received.
func (ps *PubSub) SubOnce(topics ...string) chan interface{} {
	return ps.sub(subOnce, topics...)
}

func (ps *PubSub) sub(op operation, topics ...string) chan interface{} {
	ch := make(chan interface{}, ps.capacity)
	ps.cmdChan <- cmd{op: op, topics: topics, ch: ch}
	return ch
}

// AddSub adds subscriptions to an existing channel.
func (ps *PubSub) AddSub(ch chan interface{}, topics ...string) {
	ps.cmdChan <- cmd{op: sub, topics: topics, ch: ch}
}

// Pub publishes the given message to all subscribers of
// the specified topics.
func (ps *PubSub) Pub(msg interface{}, topics ...string) {
	ps.cmdChan <- cmd{op: pub, topics: topics, msg: msg}
}

// Unsub unsubscribes the given channel from the specified
// topics. If no topic is specified, it is unsubscribed
// from all topics.
func (ps *PubSub) Unsub(ch chan interface{}, topics ...string) {
	if len(topics) == 0 {
		ps.cmdChan <- cmd{op: unsubAll, ch: ch}
		return
	}

	ps.cmdChan <- cmd{op: unsub, topics: topics, ch: ch}
}

// Close closes all channels currently subscribed to the specified topics.
// If a channel is subscribed to multiple topics, some of which is
// not specified, it is not closed.
func (ps *PubSub) Close(topics ...string) {
	ps.cmdChan <- cmd{op: closeTopic, topics: topics}
}

// Shutdown closes all subscribed channels and terminates the goroutine.
func (ps *PubSub) Shutdown() {
	ps.cmdChan <- cmd{op: shutdown}
}

func (ps *PubSub) start() {
	reg := registry{
		topics:    make(map[string]map[chan interface{}]bool),
		revTopics: make(map[chan interface{}]map[string]bool),
	}

loop:
	for cmd := range ps.cmdChan {
		if cmd.topics == nil {
			switch cmd.op {
			case unsubAll:
				reg.removeChannel(cmd.ch)

			case shutdown:
				break loop
			}

			continue loop
		}

		for _, topic := range cmd.topics {
			switch cmd.op {

			case sub:
				reg.add(topic, cmd.ch, false)

			case subOnce:
				reg.add(topic, cmd.ch, true)

			case pub:
				reg.send(topic, cmd.msg)

			case unsub:
				reg.remove(topic, cmd.ch)

			case closeTopic:
				reg.removeTopic(topic)
			}
		}
	}

	for topic, chans := range reg.topics {
		for ch := range chans {
			reg.remove(topic, ch)
		}
	}
}

// registry maintains the current subscription state. It's not
// safe to access a registry from multiple goroutines simultaneously.
type registry struct {
	topics    map[string]map[chan interface{}]bool
	revTopics map[chan interface{}]map[string]bool
}

func (reg *registry) add(topic string, ch chan interface{}, once bool) {
	if reg.topics[topic] == nil {
		reg.topics[topic] = make(map[chan interface{}]bool)
	}
	reg.topics[topic][ch] = once

	if reg.revTopics[ch] == nil {
		reg.revTopics[ch] = make(map[string]bool)
	}
	reg.revTopics[ch][topic] = true
}

func (reg *registry) send(topic string, msg interface{}) {
	for ch, once := range reg.topics[topic] {
		ch <- msg
		if once {
			for topic := range reg.revTopics[ch] {
				reg.remove(topic, ch)
			}
		}
	}
}

func (reg *registry) removeTopic(topic string) {
	for ch := range reg.topics[topic] {
		reg.remove(topic, ch)
	}
}

func (reg *registry) removeChannel(ch chan interface{}) {
	for topic := range reg.revTopics[ch] {
		reg.remove(topic, ch)
	}
}

func (reg *registry) remove(topic string, ch chan interface{}) {
	if _, ok := reg.topics[topic]; !ok {
		return
	}

	if _, ok := reg.topics[topic][ch]; !ok {
		return
	}

	delete(reg.topics[topic], ch)
	delete(reg.revTopics[ch], topic)

	if len(reg.topics[topic]) == 0 {
		delete(reg.topics, topic)
	}

	if len(reg.revTopics[ch]) == 0 {
		close(ch)
		delete(reg.revTopics, ch)
	}
}
