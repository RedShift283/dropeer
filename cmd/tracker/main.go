package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"dropeer/internal/common"
	"dropeer/internal/discovery"
)

// Tracker holds the state of the tracker.
type Tracker struct {
	mu    sync.RWMutex
	files map[string]map[string]common.PeerInfo // fileHash -> peerID -> PeerInfo
}

// NewTracker creates a new tracker instance.
func NewTracker() *Tracker {
	return &Tracker{
		files: make(map[string]map[string]common.PeerInfo),
	}
}

func (t *Tracker) announceHandler(w http.ResponseWriter, r *http.Request) {
	var req common.AnnounceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.files[req.FileHash]; !ok {
		t.files[req.FileHash] = make(map[string]common.PeerInfo)
	}
	req.PeerInfo.LastSeen = time.Now()
	t.files[req.FileHash][req.PeerInfo.ID] = req.PeerInfo

	log.Printf("Announce: Peer %s has file %s", req.PeerInfo.ID, req.FileHash[:10])
	w.WriteHeader(http.StatusOK)
}

func (t *Tracker) wantHandler(w http.ResponseWriter, r *http.Request) {
	var req common.WantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	peersMap, ok := t.files[req.FileHash]
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	var peers []common.PeerInfo
	for _, peer := range peersMap {
		peers = append(peers, peer)
	}

	resp := common.WantResponse{Peers: peers}
	json.NewEncoder(w).Encode(resp)
	log.Printf("Want: Sent %d peers for file %s", len(peers), req.FileHash[:10])
}

func (t *Tracker) cleanupStalePeers() {
	for {
		time.Sleep(1 * time.Minute)
		t.mu.Lock()
		for fileHash, peers := range t.files {
			for peerID, peerInfo := range peers {
				if time.Since(peerInfo.LastSeen) > 5*time.Minute {
					log.Printf("Cleanup: Removing stale peer %s for file %s", peerID, fileHash[:10])
					delete(peers, peerID)
				}
			}
			if len(peers) == 0 {
				delete(t.files, fileHash)
			}
		}
		t.mu.Unlock()
	}
}

func main() {
	port := flag.Int("port", 8080, "Port for the tracker to listen on")
	flag.Parse()

	tracker := NewTracker()

	// Start mDNS service publisher
	server, err := discovery.PublishService(*port)
	if err != nil {
		log.Fatalf("Failed to publish mDNS service: %v", err)
	}
	defer server.Shutdown()
	log.Printf("Published mDNS service '%s' on port %d", common.ServiceName, *port)

	go tracker.cleanupStalePeers()

	http.HandleFunc("/announce", tracker.announceHandler)
	http.HandleFunc("/want", tracker.wantHandler)
	// Heartbeat is handled by re-announcing, simplifying the logic.

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Tracker listening on %s", addr)

	// Use a standard HTTP server for the tracker API. Simpler than QUIC for this part.
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
