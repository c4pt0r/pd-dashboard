package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/ngaut/log"
)

var addr = flag.String("addr", "localhost:2234", "http service address")
var pdAddr = flag.String("pd", "localhost:9090", "pd restful api addr, without http://")
var homeTemplate = template.Must(template.New("index.html").Delims("[[", "]]").ParseFiles("./templates/index.html"))

type statusType byte

const (
	evtStart statusType = iota + 1
	evtEnd
)

type msgType byte

const (
	msgSplit msgType = iota + 1
	msgTransferLeader
	msgAddReplica
	msgRemoveReplica
)

// LogEvent is operator log event.
type LogEvent struct {
	ID     uint64     `json:"id"`
	Code   msgType    `json:"code"`
	Status statusType `json:"status"`

	SplitEvent struct {
		Region uint64 `json:"region"`
		Left   uint64 `json:"left"`
		Right  uint64 `json:"right"`
	} `json:"split_event,omitempty"`

	AddReplicaEvent struct {
		Region uint64 `json:"region"`
	} `json:"add_replica_event,omitempty"`

	RemoveReplicaEvent struct {
		Region uint64 `json:"region"`
	} `json:"remove_replica_event,omitempty"`

	TransferLeaderEvent struct {
		Region    uint64 `json:"region"`
		StoreFrom uint64 `json:"store_from"`
		StoreTo   uint64 `json:"store_to"`
	} `json:"transfer_leader_event,omitempty"`
}

var mu sync.RWMutex
var chs = make(map[*http.Request]chan LogEvent)

var eventCh = make(chan LogEvent)

var upgrader = websocket.Upgrader{}

// TODO: just for test/debug, remove it when production ready
func postEventHandler(w http.ResponseWriter, r *http.Request) {
	event, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if len(event) == 0 {
		http.Error(w, "parameter 'event' is required", 500)
		return
	}
	var evt LogEvent
	err = json.Unmarshal(event, &evt)
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

func fetchEventFeed() {
	var offset uint64 = 0
	for {
		time.Sleep(1 * time.Second)
		// fetch the feeds
		url := fmt.Sprintf("http://%s/api/v1/feed?offset=%d", *pdAddr, offset)
		resp, err := http.Get(url)
		if err != nil {
			log.Error(err)
			continue
		}
		defer resp.Body.Close()

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Error(err)
			continue
		}

		log.Info(string(b))
		var events []LogEvent
		if err := json.Unmarshal(b, &events); err != nil {
			log.Error(err)
			continue
		}

		for _, event := range events {
			log.Info(event)
			if offset < event.ID {
				offset = event.ID
			}
			eventCh <- event
		}
	}
}

func fetchRecentEvents() []LogEvent {
	url := fmt.Sprintf("http://%s/api/v1/events", *pdAddr)
	resp, err := http.Get(url)
	if err != nil {
		log.Error(err)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	var events []LogEvent
	if err := dec.Decode(&events); err != nil {
		log.Error(err)
		return nil
	}
	return events
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
	go fetchEventFeed()
	r := mux.NewRouter()
	r.HandleFunc("/ws", wsHandler)
	r.HandleFunc("/post", postEventHandler)
	r.HandleFunc("/", homeHandler)
	// for static resources
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./templates/static/")))
	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
