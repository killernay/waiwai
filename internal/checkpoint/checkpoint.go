// Package checkpoint manages transfer state for resume-on-reconnect.
// State is written atomically to a JSON file alongside the destination file.
package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// State tracks received chunks for a single file transfer session.
type State struct {
	SessionID   string         `json:"session_id"`
	Filename    string         `json:"filename"`
	TotalChunks int64          `json:"total_chunks"`
	FileSize    int64          `json:"file_size"`
	Received    map[int64]bool `json:"received"` // chunkIdx → true
	mu          sync.Mutex
	path        string
}

// Load reads an existing checkpoint or returns a fresh one.
func Load(dir, sessionID, filename string, totalChunks, fileSize int64) (*State, error) {
	path := checkpointPath(dir, sessionID, filename)
	s := &State{
		SessionID:   sessionID,
		Filename:    filename,
		TotalChunks: totalChunks,
		FileSize:    fileSize,
		Received:    make(map[int64]bool),
		path:        path,
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil // fresh start
	}
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	s.path = path
	return s, nil
}

// Mark records a chunk as received and flushes to disk.
func (s *State) Mark(chunkIdx int64) error {
	s.mu.Lock()
	s.Received[chunkIdx] = true
	s.mu.Unlock()
	return s.flush()
}

// ResumeChunk returns the first chunk index not yet received.
// Receivers send this to the sender so it can skip already-delivered chunks.
func (s *State) ResumeChunk() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var i int64
	for i = 0; i < s.TotalChunks; i++ {
		if !s.Received[i] {
			return i
		}
	}
	return s.TotalChunks // complete
}

// Missing returns all chunk indices not yet received.
func (s *State) Missing() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []int64
	for i := int64(0); i < s.TotalChunks; i++ {
		if !s.Received[i] {
			out = append(out, i)
		}
	}
	return out
}

// Complete returns true when all chunks are received.
func (s *State) Complete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.Received)) == s.TotalChunks
}

// Delete removes the checkpoint file (call after successful transfer).
func (s *State) Delete() error {
	return os.Remove(s.path)
}

func (s *State) flush() error {
	s.mu.Lock()
	data, err := json.Marshal(s)
	s.mu.Unlock()
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // atomic on POSIX
}

func checkpointPath(dir, sessionID, filename string) string {
	safe := filepath.Base(filename)
	return filepath.Join(dir, ".waiwai_checkpoint_"+sessionID+"_"+safe+".json")
}
