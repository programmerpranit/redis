package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LSMStore struct {
	// In-Memory MemTable
	memTable          *MemTable
	immutableMemTable *MemTable

	// On-Disk SSTables
	sstables []*SSTable

	memtableSize  int64
	dataDir       string
	nextSSTableID int

	mu sync.RWMutex
}

func NewLSMStore(memtableSize int64, dataDir string) (*LSMStore, error) {

	if memtableSize <= 0 {
		memtableSize = DefaultMemTableSize
	}

	if dataDir == "" {
		dataDir = "database"
	}

	store := &LSMStore{
		memTable:          NewMemTable(memtableSize),
		immutableMemTable: NewMemTable(memtableSize),
		sstables:          make([]*SSTable, 0),
		memtableSize:      memtableSize,
		dataDir:           dataDir,
		nextSSTableID:     0,
	}

	// Load existing SSTables from disk
	err := store.loadSSTables()
	if err != nil {
		return nil, fmt.Errorf("failed to load sstables: %w", err)
	}

	return store, nil

}

// Close all SSTables
func (store *LSMStore) Close() error {
	store.mu.Lock()
	defer store.mu.Unlock()

	for _, sst := range store.sstables {
		if err := sst.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (store *LSMStore) Get(key string) ([]byte, bool) {

	store.mu.RLock()
	defer store.mu.RUnlock()

	// Check MemTable
	value, found := store.memTable.Get(key)
	if found {
		return value, true
	}

	// Check Immutable MemTable
	if store.immutableMemTable != nil {
		value, found := store.immutableMemTable.Get(key)
		if found {
			return value, true
		}
	}

	// check SSTables
	for _, sst := range store.sstables {
		value, found, err := sst.Get(key)
		if err != nil {
			fmt.Printf("failed to get value from sstable: %v\n", err)
			continue
		}
		if found {
			return value, true
		}
	}

	return nil, false
}

func (store *LSMStore) Set(key string, value []byte) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	// Check MemTable
	err := store.memTable.Set(key, value)
	if err != nil {
		return fmt.Errorf("failed to set value in memtable: %w", err)
	}

	// Check Immutable MemTable
	if store.memTable.ShouldFlush() {
		err = store.rotateMemTable()
		if err != nil {
			return fmt.Errorf("failed to rotate memtable: %w", err)
		}
	}

	return nil
}

func (store *LSMStore) Delete(key string) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	// Check MemTable
	err := store.memTable.Delete(key)
	if err != nil {
		return fmt.Errorf("failed to delete value in memtable: %w", err)
	}

	// Check Immutable MemTable
	if store.memTable.ShouldFlush() {
		err = store.rotateMemTable()
		if err != nil {
			return fmt.Errorf("failed to rotate memtable: %w", err)
		}
	}
	return nil
}

func (store *LSMStore) rotateMemTable() error {
	store.memTable.MakeImmutable()

	if store.immutableMemTable != nil {
		// Still Flushing the Immutable MemTable
		// wait for 0.1 seconds
		// Hacky Fix (ideally queue this task)
		time.Sleep(100 * time.Millisecond)
		if store.immutableMemTable.IsImmutable() {
			return fmt.Errorf("still flushing the immutable memtable, cannot rotate")
		}
	}

	store.immutableMemTable = store.memTable

	store.memTable = NewMemTable(store.memtableSize)

	go store.flushImmutableMemTable()

	return nil
}

func (store *LSMStore) flushImmutableMemTable() {

	store.mu.Lock()

	if store.immutableMemTable == nil {
		store.mu.Unlock()
		return
	}
	// get name for new sstable
	memtableToFlush := store.immutableMemTable

	sstableID := store.nextSSTableID
	store.nextSSTableID++

	store.mu.Unlock()

	// flush the memetable

	path := fmt.Sprintf("%s/sstable-%d.sst", store.dataDir, sstableID)

	err := FlushMemTableToSSTable(memtableToFlush, path)
	if err != nil {
		fmt.Printf("failed to flush memtable to sstable: %v\n", err)
		return
	}
	sstable, err := OpenSSTable(path)
	if err != nil {
		fmt.Printf("failed to open sstable: %v\n", err)
		return
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	store.sstables = append([]*SSTable{sstable}, store.sstables...)

	store.immutableMemTable = nil

	fmt.Printf("flushed immutable memtable to sstable: %s\n", path)
}

func extractSSTableId(fileName string) int {

	base := filepath.Base(fileName)

	idStr := strings.TrimPrefix(base, "sstable-")
	idStr = strings.TrimSuffix(idStr, ".db")

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0
	}
	return id

}

func (store *LSMStore) loadSSTables() error {

	// Create directory if it doesn't exist
	err := os.MkdirAll(store.dataDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// Find all sstable files in the directory
	files, err := filepath.Glob(filepath.Join(store.dataDir, "sstable-*.db"))
	if err != nil {
		return fmt.Errorf("failed to read directory: %v", err)
	}

	if len(files) == 0 {
		return nil
	}

	// sort files by id
	sort.Slice(files, func(i, j int) bool {
		idI := extractSSTableId(files[i])
		idJ := extractSSTableId(files[j])
		return idI < idJ // Decending order
	})

	// Load SSTables Index from disk
	for _, file := range files {
		sstable, err := OpenSSTable(file)
		if err != nil {
			return fmt.Errorf("failed to open sstable: %v", err)
		}
		store.sstables = append(store.sstables, sstable)

		id := extractSSTableId(file)
		if id >= store.nextSSTableID {
			store.nextSSTableID = id + 1
		}
	}

	fmt.Printf("✓ Loaded %d SSTables from disk\n", len(store.sstables))
	return nil
}

// Stats returns storage statistics
func (store *LSMStore) Stats() map[string]interface{} {
	store.mu.RLock()
	defer store.mu.RUnlock()

	stats := map[string]interface{}{
		"memtable_size":      store.memTable.Size(),
		"memtable_entries":   store.memTable.Count(),
		"immutable_memtable": store.immutableMemTable != nil,
		"num_sstables":       len(store.sstables),
		"next_sstable_id":    store.nextSSTableID,
	}

	// Count total entries in SSTables
	totalSSTableEntries := 0
	for _, sst := range store.sstables {
		totalSSTableEntries += sst.NumEntries()
	}
	stats["sstable_total_entries"] = totalSSTableEntries

	return stats
}

// PrintStats prints storage statistics
func (store *LSMStore) PrintStats() {
	stats := store.Stats()
	fmt.Println("=== LSM Store Stats ===")
	fmt.Printf("MemTable Size: %d bytes\n", stats["memtable_size"])
	fmt.Printf("MemTable Entries: %d\n", stats["memtable_entries"])
	fmt.Printf("Immutable MemTable: %v\n", stats["immutable_memtable"])
	fmt.Printf("Number of SSTables: %d\n", stats["num_sstables"])
	fmt.Printf("Total SSTable Entries: %d\n", stats["sstable_total_entries"])
	fmt.Println("======================")
}
