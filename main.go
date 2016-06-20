package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/ngaut/log"
)

var addr = flag.String("addr", "localhost:2234", "http service address")
var homeTemplate = template.Must(template.ParseFiles("./templates/index.html"))

type MsgType int

const (
	MsgSplit MsgType = iota + 1
	MsgAddReplica
	MsgTransLeadership
)

type LogEvent struct {
	Typ  MsgType `json:"type"`
	Body string  `json:"msg"`
}

var eventCh = make(chan LogEvent)

var upgrader = websocket.Upgrader{}

// TODO: just for test/debug, remove it when production ready
func postEventHandler(w http.ResponseWriter, r *http.Request) {
	event := r.URL.Query().Get("event")
	if len(event) == 0 {
		http.Error(w, "parameter 'event' is required", 500)
		return
	}
	var evt LogEvent
	err := json.Unmarshal([]byte(event), &evt)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	eventCh <- evt
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	for event := range eventCh {
		logMsg, err := json.Marshal(event)
		if err != nil {
			log.Fatal(err)
		}
		log.Info("receive event:", event)
		err = c.WriteMessage(websocket.TextMessage, logMsg)
		if err != nil {
			log.Error("write:", err)
			break
		}
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTemplate.Execute(w, r.Host)
}

func main() {
	flag.Parse()
	r := mux.NewRouter()
	r.HandleFunc("/ws", wsHandler)
	r.HandleFunc("/post", postEventHandler)
	r.HandleFunc("/", homeHandler)
	// for static resources
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./templates/static/")))
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
