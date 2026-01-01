package storage

import (
	"encoding/binary"
	"fmt"
	"os"
)

const (
	MagicNumber = 0xBABECAFE
	Version     = 1
)

type IndexEntry struct {
	Key    string
	Offset int64
}

func WriteKey(file *os.File, key string) (int64, error) {
	var bytesWritten int64 = 0

	keyLength := uint32(len(key))
	err := binary.Write(file, binary.LittleEndian, keyLength)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write key length: %v", err)
	}
	bytesWritten += 4

	// Write KeyBytes
	n, err := file.Write([]byte(key))
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write key bytes: %v", err)
	}
	bytesWritten += int64(n)

	return bytesWritten, nil
}

func WriteValue(file *os.File, value []byte) (int64, error) {
	var bytesWritten int64 = 0

	valueLength := uint32(len(value))
	err := binary.Write(file, binary.LittleEndian, valueLength)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write value length: %v", err)
	}
	bytesWritten += 4

	// Write ValueBytes
	n, err := file.Write(value)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write value bytes: %v", err)
	}
	bytesWritten += int64(n)

	return bytesWritten, nil
}

func WriteMetadata(file *os.File, timestamp int64, isDeleted bool) (int64, error) {

	var bytesWritten int64 = 0

	err := binary.Write(file, binary.LittleEndian, timestamp)

	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write timestamp: %v", err)
	}
	bytesWritten += 8

	var deletedByte byte = 0

	if isDeleted {
		deletedByte = 1
	}

	err = binary.Write(file, binary.LittleEndian, deletedByte)

	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write deleted byte: %v", err)
	}
	bytesWritten += 1

	return bytesWritten, nil
}

func WriteEntry(file *os.File, key string, value []byte, timestamp int64, isDeleted bool) (int64, error) {

	var bytesWritten int64 = 0

	n, err := WriteKey(file, key)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write key: %v", err)
	}
	bytesWritten += n

	n, err = WriteValue(file, value)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write value: %v", err)
	}
	bytesWritten += n

	n, err = WriteMetadata(file, timestamp, isDeleted)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write metadata: %v", err)
	}
	bytesWritten += n

	return bytesWritten, nil
}

// Returns: indexEntries, indexBytesWritten, error
func WriteEntries(file *os.File, entries []*Entry) ([]IndexEntry, int64, error) {

	var currentOffset int64 = 0

	var indexEntries []IndexEntry = make([]IndexEntry, 0, len(entries))

	for _, entry := range entries {

		bytesWritten, err := WriteEntry(file, entry.Key, entry.Value, entry.Timestamp, entry.Deleted)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to write entry: %v", err)
		}

		indexEntries = append(indexEntries, IndexEntry{
			Key:    entry.Key,
			Offset: currentOffset,
		})

		currentOffset += bytesWritten

	}

	return indexEntries, currentOffset, nil
}

// returns the index start offset, and the number of bytes written, error
func WriteIndex(file *os.File, indexEntries []IndexEntry) (int64, int64, error) {

	indexStartOffset := int64(0)
	info, err := file.Stat()
	if err != nil {
		return 0, 0, err
	}
	indexStartOffset = info.Size() // Current file size = where we'll start writing

	var bytesWritten int64 = 0

	for _, indexEntry := range indexEntries {

		// Write KeyLength
		keyLen := uint32(len(indexEntry.Key))
		err := binary.Write(file, binary.LittleEndian, keyLen)
		if err != nil {
			return indexStartOffset, bytesWritten, fmt.Errorf("failed to write key length: %v", err)
		}
		bytesWritten += 4

		// Write KeyBytes
		n, err := file.Write([]byte(indexEntry.Key))
		if err != nil {
			return indexStartOffset, bytesWritten, fmt.Errorf("failed to write key: %v", err)
		}
		bytesWritten += int64(n)

		// Write Offset
		offset := indexEntry.Offset
		err = binary.Write(file, binary.LittleEndian, offset)
		if err != nil {
			return indexStartOffset, bytesWritten, fmt.Errorf("failed to write offset: %v", err)
		}
		bytesWritten += 8
	}

	return indexStartOffset, bytesWritten, nil
}

func WriteFooter(file *os.File, indexStartOffset int64, numberOfEntries int64) (int64, error) {

	var bytesWritten int64 = 0

	err := binary.Write(file, binary.LittleEndian, uint64(indexStartOffset))
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write index start offset: %v", err)
	}
	bytesWritten += 8

	err = binary.Write(file, binary.LittleEndian, uint32(numberOfEntries))
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write index bytes written: %v", err)
	}
	bytesWritten += 8

	err = binary.Write(file, binary.LittleEndian, Version)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write version: %v", err)
	}
	bytesWritten += 4

	err = binary.Write(file, binary.LittleEndian, MagicNumber)
	if err != nil {
		return bytesWritten, fmt.Errorf("failed to write bytes written: %v", err)
	}
	bytesWritten += 8

	return bytesWritten, nil

}

func CreateSSTable(path string, entries []*Entry) error {

	file, err := os.Create(path)

	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	indexEntries, _, err := WriteEntries(file, entries)
	if err != nil {
		return fmt.Errorf("failed to write entries: %v", err)
	}

	indexStartOffset, _, err := WriteIndex(file, indexEntries)
	if err != nil {
		return fmt.Errorf("failed to write index: %v", err)
	}

	_, err = WriteFooter(file, indexStartOffset, int64(len(entries)))
	if err != nil {
		return fmt.Errorf("failed to write footer: %v", err)
	}

	err = file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file: %v", err)
	}

	return nil
}

func FlushMemTableToSSTable(memTable *MemTable, path string) error {

	entries := memTable.GetAllEntries()

	if len(entries) == 0 {
		return fmt.Errorf("memtable is empty nothing to flush")
	}

	// memTable.MakeImmutable()

	return CreateSSTable(path, entries)
}
