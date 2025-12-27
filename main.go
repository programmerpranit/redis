package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

// Global store instance
var store *Store

func main() {

	// Create store
	store = NewStore()

	// Listen on TCP port 6379 (Redis default port)
	listener, err := net.Listen("tcp", ":6380")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Redis server listening on :6380")

	// Accept connections in a loop
	for {
		fmt.Println("Waiting for connection...")
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Handle each connection in a goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// This should print immediately
	fmt.Printf("New client connected: %s\n", conn.RemoteAddr())

	reader := bufio.NewReader(conn)

	for {
		// Parse the incoming RESP command
		command, err := parseRESP(reader)
		if err != nil {
			fmt.Println("Error parsing:", err)
			return
		}

		fmt.Printf("Parsed command: %v\n", command)

		// Execute the command and get response
		response := executeCommand(command)

		// Send response back to client
		conn.Write([]byte(response))

	}

}

// executeCommand processes commands and returns RESP responses
func executeCommand(args []string) string {
	if len(args) == 0 {
		return "-ERR empty command\r\n"
	}

	// Convert command to uppercase (Redis is case-insensitive)
	command := strings.ToUpper(args[0])

	switch command {
	case "PING":
		return "+PONG\r\n"

	case "ECHO":
		if len(args) < 2 {
			return "-ERR wrong number of arguments for 'echo' command\r\n"
		}
		message := args[1]
		return fmt.Sprintf("$%d\r\n%s\r\n", len(message), message)

	case "SET":
		if len(args) < 3 {
			return "-ERR wrong number of arguments for 'set' command\r\n"
		}
		key := args[1]
		value := args[2]

		var err error

		err = store.wal.WriteEntry("SET", key, value)
		if err != nil {
			return fmt.Sprintf("-ERR %s\r\n", err)
		}

		err = store.Set(key, value)
		if err != nil {
			return fmt.Sprintf("-ERR %s\r\n", err)
		}
		return "+OK\r\n"

	case "GET":
		if len(args) < 2 {
			return "-ERR wrong number of arguments for 'get' command\r\n"
		}
		key := args[1]
		value, exists := store.Get(key)
		if !exists {
			return "$-1\r\n" // Null bulk string in RESP
		}
		return fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)

	case "DEL":
		if len(args) < 2 {
			return "-ERR wrong number of arguments for 'del' command\r\n"
		}
		key := args[1]

		err := store.wal.WriteEntry("DEL", key, "")
		if err != nil {
			return fmt.Sprintf("-ERR %s\r\n", err)
		}

		store.Delete(key)

		return "+OK\r\n"

	default:
		return fmt.Sprintf("-ERR unknown command '%s'\r\n", command)
	}
}
