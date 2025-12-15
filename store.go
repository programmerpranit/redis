package main

import "sync"

// Store holds our key-value data
type Store struct {
	mu   sync.RWMutex      // Mutex for thread-safety
	data map[string]string // The actual data storage
}

// NewStore creates a new store
func NewStore() *Store {
	return &Store{
		data: make(map[string]string),
	}
}

// Set stores a key-value pair
func (s *Store) Set(key, value string) {
	s.mu.Lock()         // Lock for writing
	defer s.mu.Unlock() // Unlock when function returns
	s.data[key] = value
}

// Get retrieves a value by key
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()         // Read lock (multiple readers allowed)
	defer s.mu.RUnlock() // Unlock when function returns

	value, exists := s.data[key]
	return value, exists
}
