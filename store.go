package main

import (
	"encoding/json"
	"os"
	"sync"
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
// The JSON file should contain an object where keys are slugs and values are URLs.
func (s *URLStore) Load(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// We unmarshal into a temporary map to avoid partial updates on error
	// or we can just unmarshal directly if we don't care about previous state.
	// Assuming we want to replace or merge. Let's replace for simplicity as per "reading urls from a JSON file".
	return json.Unmarshal(data, &s.urls)
}

// Get retrieves a URL by its slug.
func (s *URLStore) Get(slug string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	url, ok := s.urls[slug]
	return url, ok
}
