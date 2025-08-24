// tracker/main.go (very compact, not production hardened)
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"
)

type AnnounceReq struct {
	InfoHash string `json:"infohash"`
	PeerIP   string `json:"peerip"` // "10.10.0.5:8080"
	Have     []int  `json:"have"`   // optional piece indices
}

type Peer struct {
	Addr     string
	Have     map[int]bool
	LastSeen time.Time
}

var (
	mu    sync.Mutex
	swarm = map[string]map[string]*Peer{} // infohash -> addr -> peer
)

func announceHandler(w http.ResponseWriter, r *http.Request) {
	var a AnnounceReq
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "bad", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := swarm[a.InfoHash]; !ok {
		swarm[a.InfoHash] = map[string]*Peer{}
	}
	haveMap := map[int]bool{}
	for _, i := range a.Have {
		haveMap[i] = true
	}
	swarm[a.InfoHash][a.PeerIP] = &Peer{Addr: a.PeerIP, Have: haveMap, LastSeen: time.Now()}
	w.WriteHeader(http.StatusOK)
}

func peersHandler(w http.ResponseWriter, r *http.Request) {
	info := r.URL.Query().Get("infohash")
	mu.Lock()
	defer mu.Unlock()
	peers := []Peer{}
	if m, ok := swarm[info]; ok {
		for _, p := range m {
			// prune old
			if time.Since(p.LastSeen) > 3*time.Minute {
				continue
			}
			peers = append(peers, *p)
		}
	}
	json.NewEncoder(w).Encode(peers)
}

func main() {
	go func() {
		for range time.Tick(1 * time.Minute) {
			mu.Lock()
			for ih, m := range swarm {
				for addr, p := range m {
					if time.Since(p.LastSeen) > 5*time.Minute {
						delete(m, addr)
					}
				}
				if len(m) == 0 {
					delete(swarm, ih)
				}
			}
			mu.Unlock()
		}
	}()
	http.HandleFunc("/announce", announceHandler)
	http.HandleFunc("/peers", peersHandler)
	log.Println("tracker listening :6969")
	log.Fatal(http.ListenAndServe(":6969", nil))
}
