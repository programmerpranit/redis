# Code Analysis - Issues Found

## 🔴 CRITICAL ISSUES

### 1. **Race Condition in `Get()` Method** (storage/lsm_store.go:99-101)
**Location:** `storage/lsm_store.go:93-135`
**Issue:** The mutex locks are commented out in the `Get()` method, making it not thread-safe.
```go
// store.mu.RLock()
// defer store.mu.RUnlock()
```
**Impact:** Multiple goroutines can read `store.memTable`, `store.immutableMemTable`, and `store.sstables` concurrently without synchronization, leading to data races and potential crashes.
**Fix:** Uncomment the mutex locks.

### 2. **Race Condition During WAL Recovery** (storage/wal.go:91)
**Location:** `storage/wal.go:60-106`
**Issue:** `store.Set()` and `store.Delete()` are called during WAL recovery without holding the store's mutex lock.
**Impact:** If the store is accessed during recovery (unlikely but possible), it can cause race conditions.
**Fix:** Ensure recovery happens before the store is made available, or acquire the lock during recovery.

### 3. **Footer Size Calculation Error** (storage/sstable.go)
**Location:** `storage/sstable.go:178-208`
**Issue:** The `WriteFooter()` function incorrectly counts bytes written:
- Line 182: Writes `uint64` (8 bytes) ✓
- Line 188: Writes `uint32` (4 bytes) but counts as 8 bytes ✗
- Line 194: Writes `uint32` (4 bytes) ✓
- Line 200: Writes `uint32` (4 bytes) but counts as 8 bytes ✗
- Actual bytes: 8 + 4 + 4 + 4 = 20 bytes
- Code counts: 8 + 8 + 4 + 8 = 28 bytes
**Impact:** The return value is incorrect, which could cause issues if used elsewhere. The actual file will have 20 bytes (correct), but the function reports 28 bytes.
**Fix:** Change line 192 from `bytesWritten += 8` to `bytesWritten += 4`, and line 204 from `bytesWritten += 8` to `bytesWritten += 4`.

### 4. **Incomplete Resource Cleanup** (storage/lsm_store.go:80, main.go:26-27)
**Location:** `storage/lsm_store.go:80-91`, `main.go:26-27`
**Issue:** `store.Close()` only closes SSTables but not the WAL. The WAL must be closed separately in `main.go`, which is error-prone.
**Impact:** Resource leak if WAL close is forgotten, or inconsistent cleanup pattern.
**Fix:** Make `store.Close()` also close the WAL to ensure all resources are properly cleaned up in one place.

## 🟠 HIGH PRIORITY ISSUES

### 5. **Ignored Error in RESP Parser** (resp.go:90)
**Location:** `resp.go:90`
**Issue:** Error from `reader.ReadString('\n')` is ignored when reading trailing `\r\n`.
```go
reader.ReadString('\n')  // Error ignored
```
**Impact:** If the trailing newline is missing, parsing will continue incorrectly.
**Fix:** Check and handle the error.

### 6. **No Error Handling for Network Write** (main.go:75)
**Location:** `main.go:75`
**Issue:** `conn.Write()` error is not checked.
```go
conn.Write([]byte(response))  // Error ignored
```
**Impact:** Network errors are silently ignored, clients may not receive responses.
**Fix:** Check error and handle appropriately.

### 7. **Hacky Synchronization in `rotateMemTable()`** (storage/lsm_store.go:184-188)
**Location:** `storage/lsm_store.go:178-198`
**Issue:** Uses `time.Sleep(100 * time.Millisecond)` as a synchronization mechanism.
```go
time.Sleep(100 * time.Millisecond)
if store.immutableMemTable.IsImmutable() {
    return fmt.Errorf("still flushing the immutable memtable, cannot rotate")
}
```
**Impact:** Unreliable, can fail under load, wastes time.
**Fix:** Use proper synchronization primitives (channels, sync.WaitGroup, or condition variables).

### 8. **Race Condition in `maybeCompact()`** (storage/lsm_store.go:386-395)
**Location:** `storage/lsm_store.go:386-395`
**Issue:** Lock is released before calling `Compact()`, which needs the lock.
```go
store.mu.RLock()
numSSTables := len(store.sstables)
store.mu.RUnlock()

if numSSTables >= CompactionThreshold {
    go store.Compact()  // Compact() needs the lock
}
```
**Impact:** `sstables` slice can be modified between the check and compaction, leading to incorrect compaction or race conditions.
**Fix:** Keep lock held or use atomic operations, or restructure to avoid the race.

### 9. **Incorrect Comment in `loadSSTables()`** (storage/lsm_store.go:280)
**Location:** `storage/lsm_store.go:280`
**Issue:** Comment says "Decending order" but code sorts in ascending order.
```go
return idI < idJ // Decending order  // Actually ascending!
```
**Impact:** Misleading documentation.
**Fix:** Correct the comment or the sort order.

### 10. **Error Handling in `Delete()`** (main.go:143)
**Location:** `main.go:143`
**Issue:** `store.Delete()` returns an error but it's ignored.
```go
store.Delete(key)  // Error ignored
```
**Impact:** Delete failures are silently ignored.
**Fix:** Check and handle the error.

## 🟡 MEDIUM PRIORITY ISSUES

### 11. **Potential Index Out of Bounds** (resp.go:71)
**Location:** `resp.go:71`
**Issue:** `line[0]` is accessed without checking if line is empty (though there's a check at line 22).
**Impact:** If the check at line 22 fails, this will panic.
**Fix:** Add explicit length check before accessing `line[0]`.

### 12. **Memory Inefficiency in Compaction** (storage/compaction.go:93-102)
**Location:** `storage/compaction.go:82-111`
**Issue:** All entries from all SSTables are loaded into memory at once.
**Impact:** For large datasets, this can cause OOM errors.
**Fix:** Implement streaming merge to process entries incrementally.

### 13. **No Validation of WAL Entry Format** (storage/wal.go:80-84)
**Location:** `storage/wal.go:80-84`
**Issue:** WAL entry parsing doesn't validate that values contain the expected number of parts.
**Impact:** Malformed WAL entries can cause recovery to fail silently or corrupt data.
**Fix:** Add stricter validation and error handling.

### 14. **Debug Print Statements Left in Production Code**
**Locations:** Multiple files
**Issue:** Many `fmt.Println()` statements for debugging are left in the code.
- `storage/lsm_store.go:95, 97, 103, 108`
- `storage/memetable.go:129`
- `storage/wal.go:38`
**Impact:** Performance degradation and log pollution.
**Fix:** Remove or use proper logging.

### 15. **SSTable Ordering Issue** (storage/lsm_store.go:234)
**Location:** `storage/lsm_store.go:234`
**Issue:** New SSTables are prepended to the slice, but older SSTables should be checked first (newer entries override older ones).
**Impact:** Actually correct behavior for LSM trees (newer SSTables first), but the comment in `loadSSTables()` suggests ascending order which is inconsistent.
**Fix:** Ensure consistent ordering logic and documentation.

### 16. **WAL Path Hardcoded** (storage/lsm_store.go:47, storage/wal.go:18)
**Location:** Multiple
**Issue:** WAL path is hardcoded as `"wal.log"` instead of being configurable.
**Impact:** Multiple instances can't run in the same directory, and path can't be customized.
**Fix:** Make WAL path configurable via `NewLSMStore()`.

### 17. **No File Sync in WAL** (storage/wal.go:48)
**Location:** `storage/wal.go:48`
**Issue:** Only `Flush()` is called, but `file.Sync()` is not called to ensure durability.
**Impact:** Data may be lost on system crash even though it's "written" to WAL.
**Fix:** Add `file.Sync()` after flush for critical durability.

### 18. **Empty MemTable Check Missing** (storage/sstable.go:246-248)
**Location:** `storage/sstable.go:246-248`
**Issue:** Returns error if memtable is empty, but this might be a valid state.
**Impact:** Unnecessary error for legitimate empty memtables.
**Fix:** Consider allowing empty memtables or handle this case more gracefully.

## 🔵 LOW PRIORITY / CODE QUALITY ISSUES

### 19. **Inconsistent Error Messages**
**Location:** Throughout codebase
**Issue:** Error messages use different formats (some use `%v`, some use `%s`, some include context).
**Impact:** Inconsistent user experience.
**Fix:** Standardize error message format.

### 20. **Magic Numbers**
**Location:** Multiple files
**Issue:** Magic numbers like `24` (metadata size in memetable.go:59), `500` (memtable size in main.go:17).
**Impact:** Code is less maintainable.
**Fix:** Extract to named constants.

### 21. **No Context Support**
**Location:** All methods
**Issue:** No `context.Context` support for cancellation/timeouts.
**Impact:** Operations can't be cancelled, leading to resource leaks.
**Fix:** Add context support to long-running operations.

### 22. **Missing Input Validation**
**Location:** Multiple methods
**Issue:** No validation for empty keys, extremely long keys/values, etc.
**Impact:** Potential DoS attacks or data corruption.
**Fix:** Add input validation and limits.

### 23. **Commented Out Code** (store.go)
**Location:** `store.go`
**Issue:** Entire file is commented out dead code.
**Impact:** Code clutter.
**Fix:** Remove or uncomment if needed.

### 24. **No Metrics/Telemetry**
**Location:** Throughout
**Issue:** No metrics collection for monitoring.
**Impact:** Difficult to debug production issues.
**Fix:** Add metrics for operations, latencies, etc.

### 25. **Footer Write Size Calculation Error** (storage/sstable.go:192)
**Location:** `storage/sstable.go:192`
**Issue:** Comment says "failed to write bytes written" but it's actually writing the magic number.
**Impact:** Misleading error message.
**Fix:** Correct the error message.

---

## Summary

**Critical Issues:** 4
**High Priority:** 6  
**Medium Priority:** 5
**Low Priority:** 6

**Total Issues Found:** 21

The most critical issues are the race conditions in `Get()` and WAL recovery, and the footer size mismatch that will cause SSTable corruption.

