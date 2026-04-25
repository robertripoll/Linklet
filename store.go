package main

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"
)

// URLStore handles reading and storing URLs mapped by slugs.
type URLStore struct {
	urls map[string]string
	mu   sync.RWMutex
}

// NewURLStore creates a new instance of URLStore.
func NewURLStore() *URLStore {
	return &URLStore{
		urls: make(map[string]string),
	}
}

// Load reads the JSON file and populates the store.
func (s *URLStore) Load(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	tmp := make(map[string]string)
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	s.mu.Lock()
	s.urls = tmp
	s.mu.Unlock()
	return nil
}

// Watch polls filename every interval and reloads the store when the file changes.
// Runs until ctx is cancelled.
func (s *URLStore) Watch(ctx context.Context, filename string, interval time.Duration) {
	logger := GetLogger()

	info, err := os.Stat(filename)
	var lastMod time.Time
	if err == nil {
		lastMod = info.ModTime()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			info, err := os.Stat(filename)
			if err != nil {
				logger.Warn("Could not stat URLs file", "file", filename, "error", err)
				continue
			}
			if info.ModTime().Equal(lastMod) {
				continue
			}
			lastMod = info.ModTime()
			if err := s.Load(filename); err != nil {
				logger.Error("Failed to reload URLs file", "file", filename, "error", err)
			} else {
				logger.Info("Reloaded URLs file", "file", filename)
			}
		}
	}
}

// Get retrieves a URL by its slug.
func (s *URLStore) Get(slug string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	url, ok := s.urls[slug]
	return url, ok
}
