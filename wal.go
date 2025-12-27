package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"
)

// WAL represents a Write-Ahead Log
type WAL struct {
	file   *os.File
	writer *bufio.Writer
	path   string
}

func NewWAL(path string) (*WAL, error) {

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &WAL{
		file:   file,
		writer: bufio.NewWriter(file),
		path:   path,
	}, nil
}

func (w *WAL) WriteEntry(operation string, key string, value string) error {

	// Format: timestamp|operation|key|value
	timestamp := time.Now().Unix()

	entry := fmt.Sprintf("%d|%s|%s|%s\n", timestamp, operation, key, value)

	fmt.Printf("writing to WAL: %s", entry)

	// Write to buffer
	_, err := w.writer.WriteString(entry)
	if err != nil {
		fmt.Printf("error writing to WAL: %v", err)
		return err
	}

	// Flush to disk immediately for durability
	return w.writer.Flush()
}

func (w *WAL) Close() error {

	// Flush any remaining data to disk
	w.writer.Flush()

	// Close the file
	return w.file.Close()
}

func (w *WAL) Recover(store *Store) error {

	// Open the file for reading
	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	// Read the file line by line
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			fmt.Printf("error parsing line %d: unexpected EOF or corrupt WAL entry\n", lineNum)
			continue
		}
		operation := parts[1]
		key := parts[2]
		value := parts[3]

		switch operation {
		case "SET":
			store.Set(key, value)
		case "DEL":
			store.Delete(key)
		default:
			fmt.Printf("unknown operation: %s", operation)
		}

	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL: %v", err)
	}

	fmt.Printf("WAL recovery complete: replayed %d entries\n", lineNum)
	return nil
}
