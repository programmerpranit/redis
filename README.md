# Small-Redis

A minimal Redis server implementation written in Go. This project demonstrates how to build a simple key-value store with a Redis-compatible RESP (REdis Serialization Protocol) interface.

## Features

- TCP server handling multiple concurrent client connections
- RESP protocol implementation for Redis client compatibility
- Thread-safe key-value storage using Go's `sync.RWMutex`
- Support for basic Redis commands

## Supported Commands

| Command | Arguments | Description |
|---------|-----------|-------------|
| `PING` | None | Returns "PONG" - connection test |
| `ECHO` | message | Echoes back the provided message |
| `SET` | key value | Stores a key-value pair |
| `GET` | key | Retrieves value for a key (returns nil if not found) |

## Installation

```bash
git clone https://github.com/yourusername/small-redis.git
cd small-redis
go build
```

## Usage

Start the server:

```bash
./small-redis
```

The server listens on port **6380**.

Connect using `redis-cli`:

```bash
redis-cli -p 6380
```

Example commands:

```
127.0.0.1:6380> PING
PONG
127.0.0.1:6380> SET foo bar
OK
127.0.0.1:6380> GET foo
"bar"
127.0.0.1:6380> ECHO "Hello World"
"Hello World"
```

## Project Structure

```
small-redis/
├── main.go     # Server entry point and command handler
├── resp.go     # RESP protocol parser
├── store.go    # Thread-safe key-value store
└── go.mod      # Go module definition
```

## Architecture

### Components

- **main.go**: Initializes the TCP server, handles client connections via goroutines, and executes commands
- **resp.go**: Parses RESP protocol messages (arrays and bulk strings)
- **store.go**: Implements a concurrent-safe `map[string]string` with read-write mutex protection

### How It Works

1. Server starts and listens on TCP port 6380
2. Each client connection spawns a new goroutine
3. Commands are parsed from RESP format into string slices
4. Commands execute against the thread-safe store
5. Responses are formatted as RESP and sent back to the client

## Requirements

- Go 1.21 or later

## License

MIT
