package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
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
	debug        bool
	backup       *os.File
}

// Make server for fibonacci api
func makeServer(fib *FibTracker, backup *os.File, debug bool) *server {
	s := &server{0, fib, debug, backup}
	return s
}

// handler for requests for current fibonacci value
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

// handler for requests for next fibonacci value - increments sequence index and returns value
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

// handler for requests for previous fibonacci value - decrements sequence index and returns value
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

// handler for getting cache hit/miss numbers
func (s *server) handleGetCacheStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responseEncoder := json.NewEncoder(w)
		if err := responseEncoder.Encode(s.fib.CacheStats); err != nil {
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
}

// Create router for the fibonacci server
func (s *server) makeRouter() http.Handler {
	router := http.NewServeMux()

	router.HandleFunc("/current", s.handleGetCurrent())
	router.HandleFunc("/next", s.handleSetNext())
	router.HandleFunc("/previous", s.handleSetPrevious())

	if s.debug {
		router.HandleFunc("/debug/cache", s.handleGetCacheStats())
		router.HandleFunc("/debug/pprof/", pprof.Index)
		router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		router.HandleFunc("/debug/pprof/profile", pprof.Profile)
		router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		router.HandleFunc("/debug/pprof/trace", pprof.Trace)
		router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		router.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		router.Handle("/debug/pprof/block", pprof.Handler("block"))
	}
	return router
}

// logs current index on a timer - panics if it fails multiple times
func (s *server) logCurrentIndex(seconds time.Duration) {
	ticker := time.NewTicker(seconds)
	bs := make([]byte, 4)
	remainingFails := 3
	for {
		<-ticker.C
		binary.LittleEndian.PutUint32(bs, s.currentIndex)
		_, err := s.backup.WriteAt(bs, 0)
		if err != nil {
			remainingFails -= 1
			if remainingFails == 0 {
				log.Fatalf("Failed to write backup: %v (exiting)\n", err)
			}
			log.Printf("Failed to write backup: %v (%v fails remaining)\n", err, remainingFails)
		}
	}
}

func main() {
	var port *uint = flag.Uint("port", 80, "port on which to expose the API")
	var backupPath *string = flag.String("file", "fibapi_backup", "file to journal sequence index to")
	var backupSeconds *uint = flag.Uint("seconds", 3, "seconds between each backup")
	flag.Parse()

	// create fibonacci tracker
	hc, err := MakeSliceCache(100000)
	if err != nil {
		log.Fatal(err)
	}
	fib := MakeFibTracker(10, hc)

	// setup the backup file
	var backupFile *os.File
	_, err = os.Stat(*backupPath)
	if os.IsNotExist(err) {
		backupFile, err = os.Create(*backupPath)
		defer backupFile.Close()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		backupFile, err = os.OpenFile(*backupPath, os.O_RDWR, os.ModePerm)
		defer backupFile.Close()
		if err != nil {
			log.Fatal(err)
		}
	}

	// create the server and routes
	fibServer := makeServer(fib, backupFile, true)
	router := fibServer.makeRouter()
	address := fmt.Sprintf(":%d", *port)

	httpServer := &http.Server{
		Addr:         address,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      router,
	}

	// set starting index
	bs := make([]byte, 4)
	n, err := fibServer.backup.Read(bs)
	if err != nil || n != 4 {
		log.Printf("Failed reading backup: %v\n", err)
		log.Println("Starting sequence index at zero")
	} else {
		fibServer.currentIndex = binary.LittleEndian.Uint32(bs)
		log.Printf("Starting sequence index at %v\n", fibServer.currentIndex)
	}

	// start logger and server
	go fibServer.logCurrentIndex(time.Duration(*backupSeconds) * time.Second)

	log.Printf("Serving at %v\n", address)
	log.Fatal(httpServer.ListenAndServe())
}
