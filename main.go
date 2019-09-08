package main

import (
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

func makeServer(fib *FibTracker, backup *os.File, debug bool) *server {
	s := &server{0, fib, debug, backup}
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
		// TODO: RECORD idx
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

	if s.debug {
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

func main() {
	var port *uint = flag.Uint("port", 80, "port on which to expose the API")
	var backupPath *string = flag.String("backup", "fibapi_backup.txt", "file to backup to")
	flag.Parse()

	// TODO: occasionally call backup in a goroutine (journaling/transaction log)
	hc, err := MakeSliceCache(100000)
	if err != nil {
		log.Fatal(err)
	}
	fib := MakeFibTracker(10, hc)

	var backupFile *os.File
	_, err = os.Stat(*backupPath)
	if os.IsNotExist(err) {
		backupFile, err = os.Create(*backupPath)
		defer backupFile.Close()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		backupFile, err = os.Open(*backupPath)
		defer backupFile.Close()
		if err != nil {
			log.Fatal(err)
		}
	}

	fibServer := makeServer(fib, backupFile, true)
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
