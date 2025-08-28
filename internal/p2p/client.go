package p2p

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"dropeer/internal/common"

	"github.com/quic-go/quic-go/http3"
)

type peerSpeed struct {
	peer common.PeerInfo
	mbps float64
}

// DownloadFile coordinates the download of a file from the best available peer.
func DownloadFile(fileHash, outputPath string, peers []common.PeerInfo, fileManager *FileManager) error {
	if len(peers) == 0 {
		return fmt.Errorf("no peers found for file hash %s", fileHash)
	}

	log.Println("Finding the best peer by running speed tests...")
	bestPeer, bestSpeed, err := findBestPeer(peers)
	if err != nil {
		return fmt.Errorf("could not determine best peer: %w", err)
	}
	log.Printf("Best peer found: %s (%s:%d) with %.2f Mbps", bestPeer.ID, bestPeer.IP, bestPeer.Port, bestSpeed) // Speed isn't returned here, could be added.

	client, err := createQUICClient()
	if err != nil {
		return err
	}

	// 1. Get file metadata from the chosen peer
	meta, err := getMetadataFromPeer(client, bestPeer, fileHash)
	if err != nil {
		return fmt.Errorf("failed to get metadata from peer %s: %w", bestPeer.ID, err)
	}
	log.Printf("Downloading '%s' (%d chunks)...", meta.FileName, meta.NumChunks)

	// 2. Create a temporary file
	tempOutputPath := outputPath + ".tmp"
	outFile, err := os.Create(tempOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()
	outFile.Truncate(meta.FileSize) // Pre-allocate space

	// 3. Download chunks in parallel
	var wg sync.WaitGroup
	chunks := make(chan int, meta.NumChunks)
	for i := 0; i < meta.NumChunks; i++ {
		chunks <- i
	}
	close(chunks)

	numWorkers := 10 // Concurrent downloads
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for chunkIndex := range chunks {
				data, err := downloadChunk(client, bestPeer, fileHash, chunkIndex)
				if err != nil {
					log.Printf("Error downloading chunk %d: %v. Will retry.", chunkIndex, err)
					// A real implementation would have a retry mechanism
					continue
				}
				offset := int64(chunkIndex) * int64(common.ChunkSize)
				_, err = outFile.WriteAt(data, offset)
				if err != nil {
					log.Printf("Error writing chunk %d to file: %v", chunkIndex, err)
				}
				fmt.Printf("\rDownloaded chunk %d/%d", chunkIndex+1, meta.NumChunks)
			}
		}()
	}
	wg.Wait()
	fmt.Println("\nDownload complete.")

	// 4. Rename file and add to file manager
	if err := os.Rename(tempOutputPath, outputPath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}
	fileManager.AddFile(outputPath)

	// 5. Verify final file hash
	finalHash, err := HashFile(outputPath)
	if err != nil {
		return fmt.Errorf("could not hash final file: %w", err)
	}
	if finalHash != fileHash {
		return fmt.Errorf("file hash mismatch! Expected %s, got %s", fileHash, finalHash)
	}

	log.Printf("File verified successfully. Saved to %s", outputPath)
	return nil
}

func findBestPeer(peers []common.PeerInfo) (common.PeerInfo, float64, error) {
	client, err := createQUICClient()
	if err != nil {
		return common.PeerInfo{}, 0, err
	}

	speeds := make(chan peerSpeed, len(peers))
	var wg sync.WaitGroup

	for _, p := range peers {
		wg.Add(1)
		go func(peer common.PeerInfo) {
			defer wg.Done()
			start := time.Now()
			req, _ := http.NewRequest("GET", fmt.Sprintf("https://%s:%d/speedtest", peer.IP, peer.Port), nil)
			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Speed test failed for peer %s: %v", peer.ID, err)
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Failed to read speed test response from %s: %v", peer.ID, err)
				return
			}
			duration := time.Since(start)
			mbps := (float64(len(body)) * 8) / (1024 * 1024) / duration.Seconds()
			log.Printf("Peer @ %s speed: %.2f Mbps", peer.IP, mbps)
			speeds <- peerSpeed{peer: peer, mbps: mbps}
		}(p)
	}
	wg.Wait()
	close(speeds)

	var sortedSpeeds []peerSpeed
	for s := range speeds {
		sortedSpeeds = append(sortedSpeeds, s)
	}
	if len(sortedSpeeds) == 0 {
		return common.PeerInfo{}, 0, fmt.Errorf("all peers failed the speed test")
	}

	// Sort by speed, descending
	sort.Slice(sortedSpeeds, func(i, j int) bool {
		return sortedSpeeds[i].mbps > sortedSpeeds[j].mbps
	})

	return sortedSpeeds[0].peer, sortedSpeeds[0].mbps, nil
}

func getMetadataFromPeer(client *http.Client, peer common.PeerInfo, fileHash string) (*common.FileMetadata, error) {
	url := fmt.Sprintf("https://%s:%d/metadata/%s", peer.IP, peer.Port, fileHash)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var meta common.FileMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func downloadChunk(client *http.Client, peer common.PeerInfo, fileHash string, chunkIndex int) ([]byte, error) {
	url := fmt.Sprintf("https://%s:%d/chunk/%s/%d", peer.IP, peer.Port, fileHash, chunkIndex)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("peer returned status %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func createQUICClient() (*http.Client, error) {
	tlsConfig, err := common.GenerateTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("could not generate TLS config for client: %w", err)
	}
	return &http.Client{
		Transport: &http3.Transport{
			TLSClientConfig: tlsConfig,
		},
	}, nil
}
