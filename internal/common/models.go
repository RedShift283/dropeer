package common

import "time"

const (
	// ServiceName is the mDNS service name for tracker discovery.
	ServiceName = "_localtorrent._tcp"
	// ServiceDomain is the mDNS service domain.
	ServiceDomain = "local."
	// ChunkSize is the size of each file chunk in bytes (1MB).
	ChunkSize = 1024 * 1024
)

// PeerInfo holds information about a peer.
type PeerInfo struct {
	ID       string    `json:"id"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"last_seen"`
}

// AnnounceRequest is sent by a client to announce it has a file.
type AnnounceRequest struct {
	FileHash string   `json:"file_hash"`
	PeerInfo PeerInfo `json:"peer_info"`
}

// WantRequest is sent by a client to ask for peers with a file.
type WantRequest struct {
	FileHash string `json:"file_hash"`
}

// WantResponse is the tracker's response with a list of peers.
type WantResponse struct {
	Peers []PeerInfo `json:"peers"`
}

// FileMetadata contains information about a file necessary for download.
type FileMetadata struct {
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
	FileHash  string `json:"file_hash"`
	ChunkSize int    `json:"chunk_size"`
	NumChunks int    `json:"num_chunks"`
}
