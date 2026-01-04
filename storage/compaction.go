package storage

import "fmt"

func mergeEntries(e1, e2 *Entry) *Entry {
	if e1.Timestamp > e2.Timestamp {
		return e1
	}
	return e2
}

// mergeTwoSSTables merges two SSTable files into a new one
// Returns: path to new SSTable, error
func MergeTwoSSTables(sst1, sst2 *SSTable, outputPath string) (string, error) {

	// Read entries from both SSTables
	entries1 := getAllEntriesFromSSTable(sst1)
	entries2 := getAllEntriesFromSSTable(sst2)

	// Step 4: Merge the two sorted lists
	mergedEntries := mergeSortedEntries(entries1, entries2)

	// Write merged entries to new SSTable
	err := CreateSSTable(outputPath, mergedEntries)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func getAllEntriesFromSSTable(sst *SSTable) []*Entry {
	entries := make([]*Entry, 0)

	for key, offset := range sst.index {
		entry, err := ReadEntryAtOffset(sst.file, offset)
		if err != nil {
			continue // skip this entry
		}

		// Sanity check
		if entry.Key != key {
			continue
		}

		entries = append(entries, entry)
	}

	return entries
}

func mergeSortedEntries(entries1, entries2 []*Entry) []*Entry {
	mergedEntries := make([]*Entry, 0, len(entries1)+len(entries2))
	i, j := 0, 0
	for i < len(entries1) && j < len(entries2) {
		if entries1[i].Key < entries2[j].Key {
			mergedEntries = append(mergedEntries, entries1[i])
			i++
		} else if entries1[i].Key > entries2[j].Key {
			mergedEntries = append(mergedEntries, entries2[j])
			j++
		} else {
			merged := mergeEntries(entries1[i], entries2[j])
			if !merged.Deleted {
				mergedEntries = append(mergedEntries, merged)
			}
			i++
			j++
		}
	}

	if i < len(entries1) {
		mergedEntries = append(mergedEntries, entries1[i:]...)
	}
	if j < len(entries2) {
		mergedEntries = append(mergedEntries, entries2[j:]...)
	}

	return mergedEntries
}

func CompactSSTables(sstables []*SSTable, outputPath string) (string, error) {
	if len(sstables) == 0 {
		return "", fmt.Errorf("no sstables to compact")
	}

	if len(sstables) == 1 {
		// Only one SSTable, nothing to compact
		return sstables[0].FilePath(), nil
	}

	// Collect all entries from all SSTables
	allEntries := make([][]*Entry, len(sstables))
	for i, sst := range sstables {
		allEntries[i] = getAllEntriesFromSSTable(sst)
	}

	// Merge all entries together
	merged := allEntries[0]
	for i := 1; i < len(allEntries); i++ {
		merged = mergeSortedEntries(merged, allEntries[i])
	}

	// Write to new SSTable
	err := CreateSSTable(outputPath, merged)
	if err != nil {
		return "", err
	}

	return outputPath, nil
}
