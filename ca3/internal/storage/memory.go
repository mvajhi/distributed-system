package storage

import (
	"fmt"
	"sync"
	"time"
)

// ValueEntry represents a versioned value stored for a key.
type ValueEntry struct {
	Value     string
	Version   int64
	UpdatedBy string
	Timestamp int64 // UnixNano
}

// Clone returns a deep copy of the entry.
func (v *ValueEntry) Clone() *ValueEntry {
	return &ValueEntry{
		Value:     v.Value,
		Version:   v.Version,
		UpdatedBy: v.UpdatedBy,
		Timestamp: v.Timestamp,
	}
}

// MemoryStore is a thread-safe, in-memory key-value store with versioning.
type MemoryStore struct {
	mu        sync.RWMutex
	data      map[string]*ValueEntry
	replicaID string
}

// NewMemoryStore creates a new empty store for the given replica.
func NewMemoryStore(replicaID string) *MemoryStore {
	return &MemoryStore{
		data:      make(map[string]*ValueEntry),
		replicaID: replicaID,
	}
}

// Put writes a key-value pair, incrementing the version if the key already exists.
// Returns the resulting entry.
func (m *MemoryStore) Put(key, value string) *ValueEntry {
	m.mu.Lock()
	defer m.mu.Unlock()

	var newVersion int64 = 1
	if existing, ok := m.data[key]; ok {
		newVersion = existing.Version + 1
	}

	entry := &ValueEntry{
		Value:     value,
		Version:   newVersion,
		UpdatedBy: m.replicaID,
		Timestamp: time.Now().UnixNano(),
	}
	m.data[key] = entry
	return entry.Clone()
}

// Get reads a key. Returns the entry and true if found, nil and false otherwise.
func (m *MemoryStore) Get(key string) (*ValueEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.data[key]
	if !ok {
		return nil, false
	}
	return entry.Clone(), true
}

// ApplyReplicationResult describes the outcome of an ApplyReplication call.
type ApplyReplicationResult struct {
	Accepted       bool
	CurrentVersion int64
	Reason         string
}

// ApplyReplication attempts to apply an incoming replication entry.
//
// Conflict resolution rules:
//
//	Rule 1: incoming.Version < local.Version → REJECT (stale)
//	Rule 2: incoming.Version > local.Version → ACCEPT (newer)
//	Rule 3: incoming.Version == local.Version:
//	  3a: incoming.Timestamp > local.Timestamp → ACCEPT (LWW)
//	  3b: incoming.Timestamp == local.Timestamp → higher ReplicaID wins
//	  3c: incoming.Timestamp < local.Timestamp → REJECT
func (m *MemoryStore) ApplyReplication(key string, incoming *ValueEntry) *ApplyReplicationResult {
	m.mu.Lock()
	defer m.mu.Unlock()

	local, exists := m.data[key]

	// Key doesn't exist locally — always accept.
	if !exists {
		m.data[key] = incoming.Clone()
		return &ApplyReplicationResult{
			Accepted:       true,
			CurrentVersion: incoming.Version,
			Reason:         "new_key",
		}
	}

	// Rule 1: incoming version is strictly older.
	if incoming.Version < local.Version {
		return &ApplyReplicationResult{
			Accepted:       false,
			CurrentVersion: local.Version,
			Reason:         fmt.Sprintf("stale_version: incoming=%d < local=%d", incoming.Version, local.Version),
		}
	}

	// Rule 2: incoming version is strictly newer.
	if incoming.Version > local.Version {
		m.data[key] = incoming.Clone()
		return &ApplyReplicationResult{
			Accepted:       true,
			CurrentVersion: incoming.Version,
			Reason:         "newer_version",
		}
	}

	// Rule 3: same version — apply LWW tiebreaker.
	if incoming.Timestamp > local.Timestamp {
		// 3a: incoming is more recent.
		m.data[key] = incoming.Clone()
		return &ApplyReplicationResult{
			Accepted:       true,
			CurrentVersion: incoming.Version,
			Reason:         "lww_newer_timestamp",
		}
	}

	if incoming.Timestamp == local.Timestamp {
		// 3b: same timestamp — higher replica ID wins.
		if incoming.UpdatedBy > local.UpdatedBy {
			m.data[key] = incoming.Clone()
			return &ApplyReplicationResult{
				Accepted:       true,
				CurrentVersion: incoming.Version,
				Reason:         "lww_higher_replica_id",
			}
		}
		return &ApplyReplicationResult{
			Accepted:       false,
			CurrentVersion: local.Version,
			Reason:         fmt.Sprintf("lww_lower_replica_id: incoming=%s <= local=%s", incoming.UpdatedBy, local.UpdatedBy),
		}
	}

	// 3c: incoming timestamp is older.
	return &ApplyReplicationResult{
		Accepted:       false,
		CurrentVersion: local.Version,
		Reason:         fmt.Sprintf("lww_older_timestamp: incoming=%d < local=%d", incoming.Timestamp, local.Timestamp),
	}
}
