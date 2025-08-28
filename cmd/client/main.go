package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dropeer/internal/common"
	"dropeer/internal/discovery"
	"dropeer/internal/p2p"

	"github.com/google/uuid"
)

// TrackerClient communicates with the tracker server.
type TrackerClient struct {
	baseURL  string
	client   *http.Client
	peerInfo common.PeerInfo
}

func getLocalIP() (string, error) {
	// Use Google's DNS; no packets are actually sent for UDP dial.
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected local address type")
	}
	return localAddr.IP.String(), nil
}

func NewTrackerClient(trackerURL string, peerPort int) *TrackerClient {

	localIP, err := getLocalIP()
	if err != nil {
		log.Fatalf("Could not determine local IP address: %v", err)
	}
	log.Printf("Using local IP address: %s", localIP)

	return &TrackerClient{
		baseURL: trackerURL,
		client:  &http.Client{Timeout: 10 * time.Second},
		peerInfo: common.PeerInfo{
			ID:   uuid.New().String(),
			IP:   localIP, // IMPORTANT: Change this to your actual LAN IP
			Port: peerPort,
		},
	}
}

func (c *TrackerClient) Announce(fileHash string) error {
	reqBody := common.AnnounceRequest{
		FileHash: fileHash,
		PeerInfo: c.peerInfo,
	}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := c.client.Post(c.baseURL+"/announce", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("announce failed with status: %s", resp.Status)
	}
	log.Printf("Announced file %s to tracker", fileHash[:10])
	return nil
}

func (c *TrackerClient) Want(fileHash string) ([]common.PeerInfo, error) {
	reqBody := common.WantRequest{FileHash: fileHash}
	jsonData, _ := json.Marshal(reqBody)
	resp, err := c.client.Post(c.baseURL+"/want", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var wantResp common.WantResponse
	if err := json.NewDecoder(resp.Body).Decode(&wantResp); err != nil {
		return nil, err
	}
	return wantResp.Peers, nil
}

func (c *TrackerClient) StartHeartbeat(fileHashes []string) {
	ticker := time.NewTicker(2 * time.Minute)
	go func() {
		for range ticker.C {
			for _, hash := range fileHashes {
				c.Announce(hash) // Re-announcing acts as a heartbeat
			}
		}
	}()
}

func main() {
	// Sub-commands
	shareCmd := flag.NewFlagSet("share", flag.ExitOnError)
	sharePort := shareCmd.Int("p", 4040, "Port for P2P communication")

	getCmd := flag.NewFlagSet("get", flag.ExitOnError)
	getPort := getCmd.Int("p", 4041, "Port for P2P communication")
	getOutput := getCmd.String("o", "", "Output file name (required)")

	if len(os.Args) < 2 {
		fmt.Println("Usage: client <share|get> [options]")
		return
	}

	// Discover tracker first
	log.Println("Discovering tracker on the network...")
	trackerURL, err := discovery.DiscoverTracker()
	if err != nil {
		log.Fatalf("Could not find tracker: %v", err)
	}
	log.Printf("Tracker found at: %s", trackerURL)

	switch os.Args[1] {
	case "share":
		shareCmd.Parse(os.Args[2:])
		filePath := shareCmd.Arg(0)
		if filePath == "" {
			log.Fatal("share command requires a file path")
		}

		handleShare(trackerURL, filePath, *sharePort)

	case "get":
		getCmd.Parse(os.Args[2:])
		fileHash := getCmd.Arg(0)
		*getOutput = getCmd.Arg(1)

		if fileHash == "" {
			log.Fatal("get command requires a file hash")
		}
		if *getOutput == "" {
			log.Fatal("-o (output file name) is required")
		}

		handleGet(trackerURL, fileHash, *getOutput, *getPort)

	default:
		fmt.Println("Unknown command. Use 'share' or 'get'.")
	}
}

func handleShare(trackerURL, filePath string, peerPort int) {
	fileManager := p2p.NewFileManager()
	hash, err := fileManager.AddFile(filePath)
	if err != nil {
		log.Fatalf("Could not process file: %v", err)
	}

	trackerClient := NewTrackerClient(trackerURL, peerPort)
	if err := trackerClient.Announce(hash); err != nil {
		log.Fatalf("Could not announce file to tracker: %v", err)
	}

	// Start P2P server to seed the file
	p2pServer := p2p.NewP2PServer(fileManager, fmt.Sprintf(":%d", peerPort))
	go func() {
		if err := p2pServer.Start(); err != nil {
			log.Fatalf("P2P server failed: %v", err)
		}
	}()

	trackerClient.StartHeartbeat([]string{hash})

	log.Printf("Sharing '%s' with hash: %s", filePath, hash)
	log.Println("Client is running. Press Ctrl+C to exit.")

	// Wait for shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}

func handleGet(trackerURL, fileHash, outputPath string, peerPort int) {
	trackerClient := NewTrackerClient(trackerURL, peerPort)

	peers, err := trackerClient.Want(fileHash)
	if err != nil {
		log.Fatalf("Could not get peer list from tracker: %v", err)
	}
	log.Printf("Found %d peers for the file.", len(peers))

	fileManager := p2p.NewFileManager()
	err = p2p.DownloadFile(fileHash, outputPath, peers, fileManager)
	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}

	// Once downloaded, start seeding it
	log.Println("Download successful. Now announcing and seeding the file...")
	if err := trackerClient.Announce(fileHash); err != nil {
		log.Printf("Could not announce newly downloaded file: %v", err)
	}

	p2pServer := p2p.NewP2PServer(fileManager, fmt.Sprintf(":%d", peerPort))
	go p2pServer.Start()
	trackerClient.StartHeartbeat([]string{fileHash})

	log.Println("Client is now seeding. Press Ctrl+C to exit.")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down...")
}
