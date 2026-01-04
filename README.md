# Small-Redis

A minimal Redis-compatible key-value database server written in Go, implementing an LSM (Log-Structured Merge) tree storage engine with write-ahead logging (WAL) for durability.

## Features

- **Redis-Compatible Protocol**: Implements RESP (REdis Serialization Protocol) for compatibility with `redis-cli` and other Redis clients
- **LSM Tree Storage**: Efficient on-disk storage using Log-Structured Merge trees
- **Write-Ahead Logging (WAL)**: Ensures data durability by logging all writes before applying them
- **Automatic Compaction**: Merges SSTables to maintain read performance and reduce disk usage
- **Concurrent Access**: Thread-safe operations supporting multiple concurrent clients
- **Persistent Storage**: Data survives server restarts through WAL recovery

## Supported Commands

| Command | Arguments | Description |
|---------|-----------|-------------|
| `PING` | None | Returns "PONG" - connection test |
| `ECHO` | message | Echoes back the provided message |
| `SET` | key value | Stores a key-value pair (persisted to disk) |
| `GET` | key | Retrieves value for a key (returns nil if not found) |
| `DEL` | key | Deletes a key (marks as deleted with tombstone) |

## Installation

### Prerequisites

- Go 1.21 or later
- `redis-cli` (optional, for testing with Redis client)

### Build

```bash
# Clone the repository
git clone https://github.com/yourusername/small-redis.git
cd small-redis

# Build the server
go build -o small-redis

# Or run directly
go run main.go
```

## Usage

### Starting the Server

```bash
./small-redis
```

The server will:
- Listen on port **6380** (to avoid conflicts with default Redis on 6379)
- Create a `./data` directory for SSTable storage
- Create a `wal.log` file for write-ahead logging
- Load existing SSTables from disk on startup
- Recover from WAL if the server was not cleanly shut down

Example output:
```
Creating LSM Store...
✓ Loaded 3 SSTables from disk
WAL recovery complete: replayed 42 entries
Redis server listening on :6380
Waiting for connection...
```

### Connecting with redis-cli

```bash
# Connect to the server
redis-cli -p 6380

# Or specify host explicitly
redis-cli -h 127.0.0.1 -p 6380
```

### Example Session

```bash
$ redis-cli -p 6380

127.0.0.1:6380> PING
PONG

127.0.0.1:6380> SET user:1 "Alice"
OK

127.0.0.1:6380> GET user:1
"Alice"

127.0.0.1:6380> SET user:2 "Bob"
OK

127.0.0.1:6380> GET user:2
"Bob"

127.0.0.1:6380> DEL user:1
OK

127.0.0.1:6380> GET user:1
(nil)

127.0.0.1:6380> ECHO "Hello, Small-Redis!"
"Hello, Small-Redis!"

127.0.0.1:6380> QUIT
```

### Using with Other Redis Clients

Since Small-Redis implements the RESP protocol, you can use any Redis client library:

**Python (redis-py):**
```python
import redis
r = redis.Redis(host='localhost', port=6380, decode_responses=True)
r.set('key', 'value')
print(r.get('key'))
```

**Node.js (ioredis):**
```javascript
const Redis = require('ioredis');
const redis = new Redis(6380, 'localhost');
redis.set('key', 'value');
redis.get('key').then(console.log);
```

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                      Client (redis-cli)                      │
└───────────────────────────┬─────────────────────────────────┘
                            │ RESP Protocol
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                    TCP Server (main.go)                      │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Connection Handler (goroutines)              │   │
│  │  - Parse RESP commands                               │   │
│  │  - Execute commands                                  │   │
│  │  - Send RESP responses                               │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────────┬─────────────────────────────────┘
                            │
┌───────────────────────────▼─────────────────────────────────┐
│                   LSM Store (storage/lsm_store.go)            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  MemTable    │  │ Immutable    │  │   SSTables   │      │
│  │  (Active)    │  │ MemTable     │  │  (On Disk)  │      │
│  │              │  │ (Flushing)   │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└───────────────────────────┬─────────────────────────────────┘
                            │
        ┌───────────────────┴───────────────────┐
        │                                       │
┌───────▼────────┐                    ┌────────▼────────┐
│  WAL (wal.log) │                    │  SSTable Files  │
│  Write-Ahead   │                    │  ./data/        │
│  Log           │                    │  sstable-*.db   │
└────────────────┘                    └─────────────────┘
```

### LSM Tree Structure

```
┌─────────────────────────────────────────────────────────────┐
│                         Write Path                           │
┌─────────────────────────────────────────────────────────────┐
│                                                               │
│  1. Client sends SET key value                               │
│     │                                                         │
│     ▼                                                         │
│  2. Write to WAL (wal.log)                                   │
│     │                                                         │
│     ▼                                                         │
│  3. Write to MemTable (in-memory, sorted)                    │
│     │                                                         │
│     ▼                                                         │
│  4. If MemTable full (>500 bytes default):                   │
│     │                                                         │
│     ├─► Mark MemTable as Immutable                           │
│     │                                                         │
│     ├─► Create new MemTable                                  │
│     │                                                         │
│     └─► Flush Immutable MemTable to SSTable (async)          │
│         │                                                     │
│         └─► Write to ./data/sstable-N.db                     │
│                                                               │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                          Read Path                            │
┌─────────────────────────────────────────────────────────────┐
│                                                               │
│  1. Client sends GET key                                     │
│     │                                                         │
│     ▼                                                         │
│  2. Check MemTable (newest data)                             │
│     │  ┌─► Found? Return value                               │
│     │  └─► Not found? Continue                               │
│     │                                                         │
│     ▼                                                         │
│  3. Check Immutable MemTable                                 │
│     │  ┌─► Found? Return value                               │
│     │  └─► Not found? Continue                               │
│     │                                                         │
│     ▼                                                         │
│  4. Check SSTables (newest to oldest)                        │
│     │  ┌─► Found? Return value                               │
│     │  └─► Not found? Return nil                             │
│     │                                                         │
│     └─► SSTable-0 (newest)                                   │
│         SSTable-1                                            │
│         SSTable-2                                            │
│         ...                                                   │
│         SSTable-N (oldest)                                   │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Data Structures

#### MemTable
- **Purpose**: Fast in-memory write buffer
- **Structure**: Sorted slice of entries (by key)
- **Operations**: O(log n) for Get/Set using binary search
- **Size Limit**: 500 bytes by default (configurable)
- **When Full**: Marked as immutable and flushed to disk

```
MemTable Structure:
┌─────────────────────────────────────┐
│ Entry 1: key="a", value="1"         │
│ Entry 2: key="b", value="2"         │
│ Entry 3: key="c", value="3"         │
│ ...                                 │
│ Entry N: key="z", value="26"        │
└─────────────────────────────────────┘
(Sorted by key for efficient lookup)
```

#### SSTable (Sorted String Table)
- **Purpose**: Persistent on-disk storage
- **Structure**: 
  - Data section: Key-value entries
  - Index section: Key → offset mapping
  - Footer: Metadata (index offset, entry count, version, magic number)

```
SSTable File Structure:
┌─────────────────────────────────────┐
│         Data Section                │
│  ┌──────────────────────────────┐   │
│  │ Entry 1 (key, value, meta) │   │
│  │ Entry 2 (key, value, meta) │   │
│  │ ...                         │   │
│  │ Entry N (key, value, meta) │   │
│  └──────────────────────────────┘   │
│         Index Section                │
│  ┌──────────────────────────────┐   │
│  │ Key1 → Offset1              │   │
│  │ Key2 → Offset2              │   │
│  │ ...                         │   │
│  │ KeyN → OffsetN              │   │
│  └──────────────────────────────┘   │
│         Footer                      │
│  ┌──────────────────────────────┐   │
│  │ Index Start Offset (8 bytes)│   │
│  │ Number of Entries (4 bytes)│   │
│  │ Version (4 bytes)           │   │
│  │ Magic Number (4 bytes)       │   │
│  └──────────────────────────────┘   │
└─────────────────────────────────────┘
```

#### Write-Ahead Log (WAL)
- **Purpose**: Ensure durability - all writes are logged before being applied
- **Format**: `timestamp|operation|key|value\n`
- **Recovery**: On startup, WAL is replayed to restore state

```
WAL Format Example:
1699123456|SET|user:1|Alice
1699123457|SET|user:2|Bob
1699123458|DEL|user:1|
1699123459|SET|user:3|Charlie
```

### Compaction Process

When the number of SSTables exceeds the threshold (default: 5), compaction is triggered:

```
Before Compaction:
┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐
│ SSTable0 │  │ SSTable1 │  │ SSTable2 │  │ SSTable3 │  │ SSTable4 │
│ (newest) │  │          │  │          │  │          │  │ (oldest) │
└──────────┘  └──────────┘  └──────────┘  └──────────┘  └──────────┘

Compaction Triggered (5 SSTables ≥ threshold)

┌─────────────────────────────────────────────────────────────┐
│ 1. Read all entries from all SSTables                       │
│ 2. Merge entries (keep newest version for duplicate keys)   │
│ 3. Remove tombstones (deleted entries)                      │
│ 4. Write merged entries to new SSTable                      │
└─────────────────────────────────────────────────────────────┘

After Compaction:
┌──────────┐
│ SSTable5 │  (merged, contains all unique entries)
│ (newest) │
└──────────┘

Old SSTables (0-4) are deleted
```

**Compaction Benefits:**
- Reduces number of files to check during reads
- Removes duplicate/deleted entries
- Improves read performance
- Reduces disk space usage

## How It Works

### Write Operation Flow

```
┌─────────────┐
│ Client: SET │
│ key="foo"   │
│ val="bar"   │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│ 1. Parse RESP    │
│    command       │
└──────┬───────────┘
       │
       ▼
┌─────────────────┐
│ 2. Write to WAL  │
│    (wal.log)     │
│    For durability│
└──────┬───────────┘
       │
       ▼
┌─────────────────┐
│ 3. Write to     │
│    MemTable     │
│    (in-memory)  │
└──────┬───────────┘
       │
       ▼
┌─────────────────┐
│ 4. Check if     │
│    MemTable     │
│    full?        │
└──────┬───────────┘
       │
       ├─► No ──► Return OK
       │
       ▼ Yes
┌─────────────────┐
│ 5. Rotate       │
│    MemTable     │
│    (make        │
│    immutable)   │
└──────┬───────────┘
       │
       ▼
┌─────────────────┐
│ 6. Flush to     │
│    SSTable      │
│    (async)      │
└─────────────────┘
```

### Read Operation Flow

```
┌─────────────┐
│ Client: GET │
│ key="foo"   │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│ 1. Check        │
│    MemTable     │
│    (newest)     │
└──────┬───────────┘
       │
       ├─► Found ──► Return value
       │
       ▼ Not found
┌─────────────────┐
│ 2. Check        │
│    Immutable    │
│    MemTable     │
└──────┬───────────┘
       │
       ├─► Found ──► Return value
       │
       ▼ Not found
┌─────────────────┐
│ 3. Check        │
│    SSTables     │
│    (newest to   │
│    oldest)      │
└──────┬───────────┘
       │
       ├─► Found ──► Return value
       │
       ▼ Not found
┌─────────────────┐
│ 4. Return nil   │
│    (key doesn't │
│    exist)       │
└─────────────────┘
```

### Startup and Recovery

```
┌─────────────────────────────────────────┐
│ Server Startup                           │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ 1. Create LSM Store                     │
│    - Initialize MemTable                │
│    - Initialize Immutable MemTable      │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ 2. Load Existing SSTables               │
│    - Scan ./data/ for sstable-*.db      │
│    - Open each SSTable                  │
│    - Load index into memory             │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ 3. Recover from WAL                      │
│    - Open wal.log                       │
│    - Read each entry                    │
│    - Replay SET/DEL operations          │
│    - Restore state                      │
└──────────────┬──────────────────────────┘
               │
               ▼
┌─────────────────────────────────────────┐
│ 4. Start TCP Server                     │
│    - Listen on port 6380                │
│    - Accept connections                 │
│    - Ready to serve requests            │
└─────────────────────────────────────────┘
```

## Project Structure

```
small-redis/
├── main.go                 # Server entry point, TCP handling, RESP parsing
├── resp.go                 # RESP protocol parser (arrays, bulk strings)
├── store.go                # (Legacy - commented out)
├── go.mod                  # Go module definition
├── wal.log                 # Write-ahead log file (created at runtime)
├── data/                   # SSTable storage directory (created at runtime)
│   ├── sstable-0.db
│   ├── sstable-1.db
│   └── ...
└── storage/
    ├── lsm_store.go        # Main LSM store implementation
    ├── memetable.go        # In-memory sorted table
    ├── wal.go              # Write-ahead log implementation
    ├── sstable.go          # SSTable writing functions
    ├── sstable_read.go     # SSTable reading functions
    └── compaction.go       # SSTable compaction logic
```

## Configuration

You can modify these constants in the code:

- **Port**: Change `:6380` in `main.go:30`
- **MemTable Size**: Change `500` in `main.go:17` (bytes)
- **Compaction Threshold**: Change `5` in `storage/lsm_store.go:15` (number of SSTables)
- **Data Directory**: Change `"./data"` in `main.go:17`
- **WAL Path**: Change `"wal.log"` in `storage/lsm_store.go:47`

## Performance Characteristics

- **Write Performance**: O(log n) - writes go to in-memory MemTable
- **Read Performance**: 
  - O(log n) if key in MemTable (best case)
  - O(k * log n) if key in SSTables, where k = number of SSTables (worst case)
- **Space Efficiency**: Compaction reduces duplicate entries and tombstones
- **Durability**: All writes are logged to WAL before being applied

## Limitations

- No expiration/TTL support
- No transaction support
- No pub/sub functionality
- No replication
- Simple compaction strategy (merges all SSTables at once)
- No bloom filters for SSTable lookups
- Limited error handling in some edge cases

## Troubleshooting

### Server won't start
- Check if port 6380 is already in use: `lsof -i :6380`
- Ensure you have write permissions in the current directory

### Data not persisting
- Check that `wal.log` and `./data/` directory exist
- Verify file permissions
- Check disk space

### Connection refused
- Ensure server is running
- Check firewall settings
- Verify you're connecting to the correct port (6380, not 6379)

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...
```

### Building for Production

```bash
# Build optimized binary
go build -ldflags="-s -w" -o small-redis

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o small-redis-linux
```

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
