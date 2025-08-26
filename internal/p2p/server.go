package p2p

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"dropeer/internal/common"

	"github.com/quic-go/quic-go/http3"
)

// P2PServer is the server component of a peer.
type P2PServer struct {
	fileManager *FileManager
	addr        string
}

// NewP2PServer creates a new peer server.
func NewP2PServer(fileManager *FileManager, addr string) *P2PServer {
	return &P2PServer{
		fileManager: fileManager,
		addr:        addr,
	}
}

// Start runs the P2P server.
func (s *P2PServer) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metadata/", s.metadataHandler)
	mux.HandleFunc("/chunk/", s.chunkHandler)
	mux.HandleFunc("/speedtest", s.speedTestHandler)

	tlsConfig, err := common.GenerateTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
	}

	server := http3.Server{
		Addr:      s.addr,
		Handler:   mux,
		TLSConfig: tlsConfig,
	}

	log.Printf("P2P server listening on %s (QUIC/HTTP3)", s.addr)
	return server.ListenAndServe()
}

func (s *P2PServer) metadataHandler(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/metadata/")
	filePath, ok := s.fileManager.GetFilePath(hash)
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	meta, err := GetFileMetadata(filePath)
	if err != nil {
		http.Error(w, "could not get file metadata", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(meta)
}

func (s *P2PServer) chunkHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/chunk/"), "/")
	if len(parts) != 2 {
		http.Error(w, "invalid chunk request format: /chunk/{hash}/{index}", http.StatusBadRequest)
		return
	}
	hash, indexStr := parts[0], parts[1]

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "invalid chunk index", http.StatusBadRequest)
		return
	}

	filePath, ok := s.fileManager.GetFilePath(hash)
	if !ok {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	chunk, err := ReadChunk(filePath, index)
	if err != nil {
		http.Error(w, "failed to read chunk", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(chunk)
}

func (s *P2PServer) speedTestHandler(w http.ResponseWriter, r *http.Request) {
	// Send 1MB of dummy data for a quick throughput test
	data := make([]byte, 1024*1024)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// Example method
func (fm *FileManager) SaveFile(name string, data []byte) error {
	// Implement file saving logic
	return nil
}
