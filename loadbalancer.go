package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
)

type roundRobinLb struct {
	mu         sync.RWMutex
	ind        int
	pods       []*pod
	alive      map[string]bool
	httpServer *http.Server
	stopCb     func()
}

func newLb(httpAddr string, stopCb func()) *roundRobinLb {
	l := &roundRobinLb{
		pods:   make([]*pod, 0),
		alive:  make(map[string]bool),
		stopCb: stopCb,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		po := l.iter()
		if po == nil {
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		_, err := fmt.Fprintf(
			writer,
			"host=%s, known=(%d) %v, state=%v",
			po.name,
			po.ml.NumMembers(),
			po.ml.Members(),
			po.gmap.State(),
		)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
	mux.HandleFunc("/inc/{key}", func(writer http.ResponseWriter, request *http.Request) {
		key := request.PathValue("key")
		po := l.iter()
		po.Inc(key)
		_, err := fmt.Fprintf(
			writer,
			"host=%s, key(%v) = %v",
			po.name,
			key,
			po.gmap.Value(key),
		)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/get/{key}", func(writer http.ResponseWriter, request *http.Request) {
		key := request.PathValue("key")
		po := l.iter()
		_, err := fmt.Fprintf(
			writer,
			"host=%s, key(%v) = %v",
			po.name,
			key,
			po.gmap.Value(key),
		)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
		}
	})
	l.httpServer = &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}
	return l
}

func (l *roundRobinLb) iter() *pod {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.pods) == 0 {
		return nil
	}
	l.ind = (l.ind + 1) % len(l.pods)
	for i := 0; i < len(l.pods); i++ {
		po := l.pods[l.ind+i]
		if l.alive[po.name] {
			return po
		}
	}
	return nil
}

func (l *roundRobinLb) Start() {
	log.Println("starting lb...", l.httpServer.Addr)
	if err := l.httpServer.ListenAndServe(); err != nil {
		log.Println(err)
	}
}

func (l *roundRobinLb) Shutdown() {
	defer l.stopCb()
	if err := l.httpServer.Shutdown(context.Background()); err != nil {
		log.Println(err)
	}
}

func (l *roundRobinLb) GossipAddr() string {
	if po := l.iter(); po != nil {
		return po.gossipAddr
	}
	return ""
}

func (l *roundRobinLb) Register(po *pod) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.alive[po.name]; !ok {
		l.pods = append(l.pods, po)
		l.alive[po.name] = true
	}
}
