package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// Parse one command from the connection
func parseRESP(reader *bufio.Reader) ([]string, error) {
	// Read the first line
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	// Remove \r\n from the end
	line = strings.TrimSpace(line)

	// Check what type of data this is
	if len(line) == 0 {
		return nil, fmt.Errorf("empty line")
	}

	// RESP uses first character to identify type
	firstChar := line[0]

	switch firstChar {
	case '*':
		// Array - this is what we need for commands
		return parseArray(reader, line)
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", firstChar)
	}
}

func parseArray(reader *bufio.Reader, line string) ([]string, error) {
	// line is "*1" - extract the number
	countStr := line[1:]                 // Remove the '*', get "1"
	count, err := strconv.Atoi(countStr) // Convert string to int
	if err != nil {
		return nil, fmt.Errorf("invalid array length: %s", countStr)
	}

	// Create a slice to hold the results
	result := make([]string, count)

	// Read each element
	for i := 0; i < count; i++ {
		element, err := parseBulkString(reader)
		if err != nil {
			return nil, err
		}
		result[i] = element
	}

	return result, nil
}

func parseBulkString(reader *bufio.Reader) (string, error) {
	// Read the line that tells us the length: "$4"
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSpace(line)

	// Check it starts with $
	if line[0] != '$' {
		return "", fmt.Errorf("expected bulk string, got: %s", line)
	}

	// Extract the length: "4" from "$4"
	lengthStr := line[1:]
	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", fmt.Errorf("invalid bulk string length: %s", lengthStr)
	}

	// Read exactly 'length' bytes for the actual string
	data := make([]byte, length)
	_, err = reader.Read(data)
	if err != nil {
		return "", err
	}

	// Read the trailing \r\n
	reader.ReadString('\n')

	return string(data), nil
}
