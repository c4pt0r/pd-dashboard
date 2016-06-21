package main

import (
	"encoding/json"
	"flag"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/ngaut/log"
)

var addr = flag.String("addr", "localhost:2234", "http service address")
var homeTemplate = template.Must(template.New("index.html").Delims("[[", "]]").ParseFiles("./templates/index.html"))

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

var mu sync.RWMutex
var chs = make(map[*http.Request]chan LogEvent)

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

// fanout events to clients
func fanout() {
	for event := range eventCh {
		mu.RLock()
		log.Infof("fanout message: %+v to %d clients", event, len(chs))
		for _, ch := range chs {
			select {
			case ch <- event:
			default:
			}
		}
		mu.RUnlock()
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()

	// make sure the client is alive
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ch := make(chan LogEvent)
	mu.Lock()
	chs[r] = ch
	mu.Unlock()

	defer func() {
		mu.Lock()
		log.Info("client is closed, removing channel")
		close(chs[r])
		delete(chs, r)
		mu.Unlock()
	}()

	for {
		select {
		case <-ticker.C:
			if err := c.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		case event := <-ch:
			logMsg, _ := json.Marshal(event)
			err = c.WriteMessage(websocket.TextMessage, logMsg)
			if err != nil {
				log.Error(err)
				return
			}
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
	homeTemplate.ExecuteTemplate(w, "index.html", r.Host)
}

func main() {
	flag.Parse()
	go fanout()
	r := mux.NewRouter()
	r.HandleFunc("/ws", wsHandler)
	r.HandleFunc("/post", postEventHandler)
	r.HandleFunc("/", homeHandler)
	// for static resources
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./templates/static/")))
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
