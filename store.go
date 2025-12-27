package main

import (
	"fmt"
	"sync"
)

// Store holds our key-value data
type Store struct {
	mu   sync.RWMutex      // Mutex for thread-safety
	data map[string]string // The actual data storage
	wal  *WAL              // The write-ahead log
}

// NewStore creates a new store
func NewStore() *Store {

	wal, err := NewWAL("wal.log")
	if err != nil {
		fmt.Println("Error creating WAL:", err)
		return nil
	}
	defer wal.Close()

	store := &Store{
		data: make(map[string]string),
		wal:  wal,
	}

	// Recover from WAL
	err = wal.Recover(store)
	if err != nil {
		fmt.Println("Error recovering WAL:", err)
		return nil
	}

	return store
}

// Set stores a key-value pair
func (s *Store) Set(key, value string) error {
	s.mu.Lock()         // Lock for writing
	defer s.mu.Unlock() // Unlock when function returns

	s.data[key] = value
	return nil
}

// Get retrieves a value by key
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()         // Read lock (multiple readers allowed)
	defer s.mu.RUnlock() // Unlock when function returns

	value, exists := s.data[key]
	return value, exists
}

func (s *Store) Delete(key string) {
	s.mu.Lock()         // Lock for writing
	defer s.mu.Unlock() // Unlock when function returns
	delete(s.data, key)
}
