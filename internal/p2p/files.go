package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sync"

	"dropeer/internal/common"
)

// NewFileManager creates a new file manager.
func NewFileManager() *FileManager {
	return &FileManager{
		files:     make(map[string]string),
		downloads: &sync.Map{},
	}
}

// FileManager keeps track of local files being shared.
type FileManager struct {
	mu        sync.RWMutex
	files     map[string]string // fileHash -> filePath
	downloads *sync.Map         // fileHash -> DownloadState
}

func (fm *FileManager) AddFile(filePath string) (string, error) {
	hash, err := HashFile(filePath)
	if err != nil {
		return "", err
	}
	fm.mu.Lock()
	fm.files[hash] = filePath
	fm.mu.Unlock()
	return hash, nil
}

func (fm *FileManager) GetFilePath(hash string) (string, bool) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	path, ok := fm.files[hash]
	return path, ok
}

// HashFile computes the SHA256 hash of a file.
func HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// GetFileMetadata generates metadata for a given file.
func GetFileMetadata(filePath string) (*common.FileMetadata, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	hash, err := HashFile(filePath)
	if err != nil {
		return nil, err
	}

	return &common.FileMetadata{
		FileName:  filepath.Base(filePath),
		FileSize:  stat.Size(),
		FileHash:  hash,
		ChunkSize: common.ChunkSize,
		NumChunks: int((stat.Size() + int64(common.ChunkSize) - 1) / int64(common.ChunkSize)),
	}, nil
}

// ReadChunk reads a specific chunk from a file.
func ReadChunk(filePath string, chunkIndex int) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	offset := int64(chunkIndex * common.ChunkSize)
	_, err = file.Seek(offset, 0)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, common.ChunkSize)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}

	return buffer[:n], nil
}
