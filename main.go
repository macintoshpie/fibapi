package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type response struct {
	Index uint32 `json:"index"`
	Value string `json:"value"`
}

type server struct {
	currentIndex uint32
	fib          *FibTracker
}

func makeServer(fib *FibTracker) *server {
	s := &server{0, fib}
	return s
}

func (s *server) handleGetCurrent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := s.currentIndex
		resp := response{idx, s.fib.Get(idx).String()}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(resp); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) handleSetNext() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := atomic.AddUint32(&s.currentIndex, 1)
		resp := response{idx, s.fib.Get(idx).String()}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(resp); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

func (s *server) handleSetPrevious() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idx := uint32(0)
		if s.currentIndex > 0 {
			idx = atomic.AddUint32(&s.currentIndex, ^uint32(0))
		}
		resp := response{idx, s.fib.Get(idx).String()}
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(resp); err != nil {
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

	// TODO: occasionally call backup in a goroutine (journaling/transaction log)
	fib := MakeFibTracker().WithInitializedStore(100000)
	fibServer := makeServer(fib)
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
