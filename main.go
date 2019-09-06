package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"sync/atomic"
	"time"
)

type response struct {
	Index uint32 `json:"index"`
	Value uint64 `json:"value"`
}

type server struct {
	index uint32
	store []uint64
}

func makeServer() *server {
	return &server{0, []uint64{0, 1}}
}

func (s *server) extendStore() {
	newLen := len(s.store) * 2
	newStore := make([]uint64, newLen)
	// copy over old data
	for i := 0; i < len(s.store); i += 1 {
		newStore[i] = s.store[i]
	}
	// calculate new values
	for i := len(s.store); i < newLen; i += 1 {
		newStore[i] = newStore[i-1] + newStore[i-2]
	}

	s.store = newStore
}

func (s *server) withInitializedStore(nInit int) *server {
	nInit = int(math.Max(2, float64(nInit)))
	for i := 2; i < nInit; i++ {
		s.store = append(s.store, s.store[i-1]+s.store[i-2])
	}

	return s
}

func (s *server) handleGetCurrent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := response{s.index, s.store[s.index]}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(response); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) handleSetNext() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddUint32(&s.index, 1)
		response := response{s.index, s.store[s.index]}
		// FIXME: race condition: too many concurrent requests come in before we finish goroutine
		// FIXME: waste of resources: multiple goroutines could be kicked off
		if s.index+1 >= uint32(len(s.store)/2) {
			go s.extendStore()
		}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(response); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) handleSetPrevious() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.index > 0 {
			atomic.AddUint32(&s.index, ^uint32(0))
		}
		response := response{s.index, s.store[s.index]}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(response); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) makeRouter() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("/current", s.handleGetCurrent())
	router.HandleFunc("/next", s.handleSetNext())
	router.HandleFunc("/previous", s.handleSetPrevious())

	return router
}

func main() {
	var port *uint = flag.Uint("port", 80, "port on which to expose the API")
	var backup *string = flag.String("backup", "fibapi_backup.txt", "file to backup to")
	flag.Parse()

	fmt.Println(backup)

	// occasionally call backup in a goroutine (journaling/transaction log)
	fibServer := makeServer().withInitializedStore(3)
	router := fibServer.makeRouter()
	address := fmt.Sprintf(":%d", *port)

	httpServer := &http.Server{
		Addr:         address,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      router,
	}

	fmt.Printf("Serving at %v\n", address)

	log.Fatal(httpServer.ListenAndServe())
}
