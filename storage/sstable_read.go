package storage

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

type SSTableFooter struct {
	IndexStartOffset int64
	NumberOfEntries  uint32
	Version          uint32
	MagicNumber      uint32
}

type SSTable struct {
	filePath string
	file     *os.File
	index    map[string]int64 // key → offset mapping
	footer   *SSTableFooter
}

func ReadFooter(file *os.File) (*SSTableFooter, error) {

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := info.Size()
	footerSize := int64(20)

	if fileSize < footerSize {
		return nil, fmt.Errorf("file is too small to contain a footer")
	}

	footerOffset := fileSize - footerSize
	_, err = file.Seek(footerOffset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to footer: %v", err)
	}

	footer := &SSTableFooter{}

	err = binary.Read(file, binary.LittleEndian, &footer.IndexStartOffset)
	if err != nil {
		return nil, fmt.Errorf("failed to read footer: %v", err)
	}

	err = binary.Read(file, binary.LittleEndian, &footer.NumberOfEntries)
	if err != nil {
		return nil, fmt.Errorf("failed to read number of entries: %v", err)
	}

	err = binary.Read(file, binary.LittleEndian, &footer.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %v", err)
	}

	err = binary.Read(file, binary.LittleEndian, &footer.MagicNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to read magic number: %v", err)
	}

	if footer.MagicNumber != MagicNumber {
		return nil, fmt.Errorf("invalid magic number: %v", footer.MagicNumber)
	}

	return footer, nil
}

func ReadIndex(file *os.File, footer *SSTableFooter) (map[string]int64, error) {

	_, err := file.Seek(footer.IndexStartOffset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to index: %v", err)
	}

	index := make(map[string]int64, footer.NumberOfEntries)

	for i := 0; i < int(footer.NumberOfEntries); i++ {
		var keyLength uint32
		err = binary.Read(file, binary.LittleEndian, &keyLength)
		if err != nil {
			return nil, fmt.Errorf("failed to read key length: %v", err)
		}

		key := make([]byte, keyLength)
		_, err = io.ReadFull(file, key)
		if err != nil {
			return nil, fmt.Errorf("failed to read key: %v", err)
		}

		var offset int64
		err = binary.Read(file, binary.LittleEndian, &offset)
		if err != nil {
			return nil, fmt.Errorf("failed to read offset: %v", err)
		}

		index[string(key)] = offset
	}

	return index, nil
}

func ReadEntryAtOffset(file *os.File, offset int64) (*Entry, error) {

	_, err := file.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to seek to entry: %v", err)
	}

	keyLength := uint32(0)
	err = binary.Read(file, binary.LittleEndian, &keyLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read key length: %v", err)
	}

	keyBytes := make([]byte, keyLength)
	_, err = io.ReadFull(file, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %v", err)
	}

	valueLength := uint32(0)
	err = binary.Read(file, binary.LittleEndian, &valueLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read value length: %v", err)
	}

	valueBytes := make([]byte, valueLength)
	_, err = io.ReadFull(file, valueBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read value: %v", err)
	}

	var timestamp int64
	err = binary.Read(file, binary.LittleEndian, &timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %v", err)
	}

	var deleted byte
	err = binary.Read(file, binary.LittleEndian, &deleted)
	if err != nil {
		return nil, fmt.Errorf("failed to read deleted: %v", err)
	}

	return &Entry{
		Key:       string(keyBytes),
		Value:     valueBytes,
		Timestamp: timestamp,
		Deleted:   deleted != 0,
	}, nil
}

func OpenSSTable(filePath string) (*SSTable, error) {
	// Open File
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}

	// Read Footer
	footer, err := ReadFooter(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read footer: %v", err)
	}

	// Read Index
	index, err := ReadIndex(file, footer)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to read index: %v", err)
	}

	return &SSTable{
		filePath: filePath,
		file:     file,
		index:    index,
		footer:   footer,
	}, nil
}

// Close the SSTable (file)
func (s *SSTable) Close() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// Returns: value, found, error
func (s *SSTable) Get(key string) ([]byte, bool, error) {
	// Check Index
	offset, exists := s.index[key]
	if !exists {
		return nil, false, nil
	}

	// Read Entry at Offset
	entry, err := ReadEntryAtOffset(s.file, offset)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read entry: %v", err)
	}

	// Check Key (should be the same)
	if entry.Key != key {
		return nil, false, fmt.Errorf("Index Corruption: %s != %s", entry.Key, key)
	}

	if entry.Deleted {
		return nil, false, nil
	}

	return entry.Value, true, nil
}

// NumEntries returns the number of entries in the SSTable
func (sst *SSTable) NumEntries() int {
	return len(sst.index)
}

// FilePath returns the file path
func (sst *SSTable) FilePath() string {
	return sst.filePath
}

// ContainsKey checks if a key exists (without reading the value)
func (sst *SSTable) ContainsKey(key string) bool {
	_, exists := sst.index[key]
	return exists
}
