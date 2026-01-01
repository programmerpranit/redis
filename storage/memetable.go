package storage

import (
	"sort"
	"sync"
	"time"
)

const (
	DefaultMemTableSize = 4 * 1024 * 1024 // 4MB
)

// Entry represents a key-value pair with metadata
type Entry struct {
	Key       string
	Value     []byte
	Timestamp int64
	Deleted   bool // Tombstone for deletions
}

// MemTable represents an in-memory sorted buffer
// Using a simple slice - easy to understand!
type MemTable struct {
	entries   []*Entry
	sizeBytes int64
	maxSize   int64
	mu        sync.RWMutex
	immutable bool
}

func NewMemTable(maxSize int64) *MemTable {
	if maxSize <= 0 {
		maxSize = DefaultMemTableSize
	}
	return &MemTable{
		entries:   make([]*Entry, 0, 100),
		sizeBytes: 0,
		maxSize:   maxSize,
		immutable: false,
	}
}

// Set adds or updates a key-value pair
func (mt *MemTable) Set(key string, value []byte) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if mt.immutable {
		return ErrMemTableImmutable
	}

	// Find position using binary search
	idx := sort.Search(len(mt.entries), func(i int) bool {
		return mt.entries[i].Key >= key
	})

	// Calculate entry size
	entrySize := int64(len(key) + len(value) + 24) // 24 bytes for metadata

	// Key already exists - update it
	if idx < len(mt.entries) && mt.entries[idx].Key == key {
		oldSize := int64(len(mt.entries[idx].Value))
		mt.entries[idx].Value = value
		mt.entries[idx].Timestamp = time.Now().UnixNano()
		mt.entries[idx].Deleted = false
		mt.sizeBytes = mt.sizeBytes - oldSize + int64(len(value))
		return nil
	}

	// Insert new entry at correct position
	entry := &Entry{
		Key:       key,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
		Deleted:   false,
	}

	// Insert at idx to keep sorted order
	mt.entries = append(mt.entries, nil)
	copy(mt.entries[idx+1:], mt.entries[idx:])
	mt.entries[idx] = entry
	mt.sizeBytes += entrySize

	return nil
}

// Delete marks a key as deleted (tombstone)
func (mt *MemTable) Delete(key string) error {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if mt.immutable {
		return ErrMemTableImmutable
	}

	// Find position
	idx := sort.Search(len(mt.entries), func(i int) bool {
		return mt.entries[i].Key >= key
	})

	entrySize := int64(len(key) + 24) // No value, just metadata

	// Key exists - mark as deleted
	if idx < len(mt.entries) && mt.entries[idx].Key == key {
		mt.entries[idx].Deleted = true
		mt.entries[idx].Timestamp = time.Now().UnixNano()
		return nil
	}

	// Key doesn't exist - still insert tombstone
	entry := &Entry{
		Key:       key,
		Value:     nil,
		Timestamp: time.Now().UnixNano(),
		Deleted:   true,
	}

	mt.entries = append(mt.entries, nil)
	copy(mt.entries[idx+1:], mt.entries[idx:])
	mt.entries[idx] = entry
	mt.sizeBytes += entrySize

	return nil
}

// Get retrieves a value by key
func (mt *MemTable) Get(key string) ([]byte, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	// Binary search
	idx := sort.Search(len(mt.entries), func(i int) bool {
		return mt.entries[i].Key >= key
	})

	if idx < len(mt.entries) && mt.entries[idx].Key == key {
		if mt.entries[idx].Deleted {
			return nil, false
		}
		return mt.entries[idx].Value, true
	}

	return nil, false
}

// ShouldFlush checks if MemTable has reached size limit
func (mt *MemTable) ShouldFlush() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.sizeBytes >= mt.maxSize
}

// MakeImmutable marks the MemTable as read-only for flushing
func (mt *MemTable) MakeImmutable() {
	mt.mu.Lock()
	defer mt.mu.Unlock()
	mt.immutable = true
}

// GetAllEntries returns all entries in sorted order
func (mt *MemTable) GetAllEntries() []*Entry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	// Already sorted!
	result := make([]*Entry, len(mt.entries))
	copy(result, mt.entries)
	return result
}

// Size returns the approximate size in bytes
func (mt *MemTable) Size() int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.sizeBytes
}

// Count returns number of entries
func (mt *MemTable) Count() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return len(mt.entries)
}

// IsImmutable checks if MemTable is read-only
func (mt *MemTable) IsImmutable() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.immutable
}

// Errors
var (
	ErrMemTableImmutable = &StorageError{Message: "memtable is immutable"}
)

type StorageError struct {
	Message string
}

func (e *StorageError) Error() string {
	return e.Message
}
