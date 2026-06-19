package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPut_NewKey(t *testing.T) {
	store := NewMemoryStore("r1")
	entry := store.Put("key1", "value1")

	if entry.Value != "value1" {
		t.Errorf("expected value1, got %s", entry.Value)
	}
	if entry.Version != 1 {
		t.Errorf("expected version 1, got %d", entry.Version)
	}
	if entry.UpdatedBy != "r1" {
		t.Errorf("expected r1, got %s", entry.UpdatedBy)
	}
}

func TestPut_ExistingKey_IncrementsVersion(t *testing.T) {
	store := NewMemoryStore("r1")
	store.Put("key1", "v1")
	entry := store.Put("key1", "v2")

	if entry.Version != 2 {
		t.Errorf("expected version 2, got %d", entry.Version)
	}
	if entry.Value != "v2" {
		t.Errorf("expected v2, got %s", entry.Value)
	}
}

func TestGet_NotFound(t *testing.T) {
	store := NewMemoryStore("r1")
	_, found := store.Get("nonexistent")
	if found {
		t.Error("expected not found")
	}
}

func TestGet_Found(t *testing.T) {
	store := NewMemoryStore("r1")
	store.Put("key1", "value1")
	entry, found := store.Get("key1")
	if !found {
		t.Fatal("expected found")
	}
	if entry.Value != "value1" {
		t.Errorf("expected value1, got %s", entry.Value)
	}
}

func TestGet_ReturnsClone(t *testing.T) {
	store := NewMemoryStore("r1")
	store.Put("key1", "value1")
	entry1, _ := store.Get("key1")
	entry1.Value = "modified"
	entry2, _ := store.Get("key1")
	if entry2.Value != "value1" {
		t.Error("Get should return a clone, but internal data was modified")
	}
}

// ─── Conflict Resolution Tests ──────────────────────────────────────────

func TestApplyReplication_NewKey(t *testing.T) {
	store := NewMemoryStore("r1")
	incoming := &ValueEntry{Value: "v1", Version: 1, UpdatedBy: "r2", Timestamp: time.Now().UnixNano()}
	result := store.ApplyReplication("key1", incoming)

	if !result.Accepted {
		t.Errorf("expected accepted, got rejected: %s", result.Reason)
	}
	entry, found := store.Get("key1")
	if !found || entry.Value != "v1" {
		t.Error("key should exist with value v1")
	}
}

func TestApplyReplication_Rule1_StaleVersion_Rejected(t *testing.T) {
	store := NewMemoryStore("r1")
	// Local has version 3.
	store.Put("key1", "v1")
	store.Put("key1", "v2")
	store.Put("key1", "v3")

	// Incoming has version 1 — should be rejected.
	incoming := &ValueEntry{Value: "old", Version: 1, UpdatedBy: "r2", Timestamp: time.Now().UnixNano()}
	result := store.ApplyReplication("key1", incoming)

	if result.Accepted {
		t.Error("stale version should be rejected")
	}
	entry, _ := store.Get("key1")
	if entry.Value != "v3" {
		t.Errorf("value should remain v3, got %s", entry.Value)
	}
}

func TestApplyReplication_Rule2_NewerVersion_Accepted(t *testing.T) {
	store := NewMemoryStore("r1")
	store.Put("key1", "v1") // version 1

	incoming := &ValueEntry{Value: "v5", Version: 5, UpdatedBy: "r2", Timestamp: time.Now().UnixNano()}
	result := store.ApplyReplication("key1", incoming)

	if !result.Accepted {
		t.Errorf("newer version should be accepted: %s", result.Reason)
	}
	entry, _ := store.Get("key1")
	if entry.Value != "v5" {
		t.Errorf("expected v5, got %s", entry.Value)
	}
}

func TestApplyReplication_Rule3a_SameVersion_NewerTimestamp_Accepted(t *testing.T) {
	store := NewMemoryStore("r1")

	// Set up local entry with version 1 and old timestamp.
	localEntry := &ValueEntry{Value: "local", Version: 1, UpdatedBy: "r1", Timestamp: 1000}
	store.ApplyReplication("key1", localEntry)

	// Incoming with same version but newer timestamp.
	incoming := &ValueEntry{Value: "remote", Version: 1, UpdatedBy: "r2", Timestamp: 2000}
	result := store.ApplyReplication("key1", incoming)

	if !result.Accepted {
		t.Errorf("newer timestamp should win: %s", result.Reason)
	}
	entry, _ := store.Get("key1")
	if entry.Value != "remote" {
		t.Errorf("expected remote, got %s", entry.Value)
	}
}

func TestApplyReplication_Rule3b_SameVersionSameTimestamp_HigherReplicaWins(t *testing.T) {
	store := NewMemoryStore("r1")

	localEntry := &ValueEntry{Value: "local", Version: 1, UpdatedBy: "r1", Timestamp: 1000}
	store.ApplyReplication("key1", localEntry)

	// r2 > r1 lexicographically.
	incoming := &ValueEntry{Value: "remote", Version: 1, UpdatedBy: "r2", Timestamp: 1000}
	result := store.ApplyReplication("key1", incoming)

	if !result.Accepted {
		t.Errorf("higher replica ID should win: %s", result.Reason)
	}
}

func TestApplyReplication_Rule3b_SameVersionSameTimestamp_LowerReplicaLoses(t *testing.T) {
	store := NewMemoryStore("r2")

	localEntry := &ValueEntry{Value: "local", Version: 1, UpdatedBy: "r2", Timestamp: 1000}
	store.ApplyReplication("key1", localEntry)

	// r1 < r2 lexicographically — should be rejected.
	incoming := &ValueEntry{Value: "remote", Version: 1, UpdatedBy: "r1", Timestamp: 1000}
	result := store.ApplyReplication("key1", incoming)

	if result.Accepted {
		t.Error("lower replica ID should lose")
	}
}

func TestApplyReplication_Rule3c_SameVersion_OlderTimestamp_Rejected(t *testing.T) {
	store := NewMemoryStore("r1")

	localEntry := &ValueEntry{Value: "local", Version: 1, UpdatedBy: "r1", Timestamp: 2000}
	store.ApplyReplication("key1", localEntry)

	incoming := &ValueEntry{Value: "remote", Version: 1, UpdatedBy: "r2", Timestamp: 1000}
	result := store.ApplyReplication("key1", incoming)

	if result.Accepted {
		t.Error("older timestamp should be rejected")
	}
}

// ─── Concurrency Tests ─────────────────────────────────────────────────

func TestConcurrentPuts(t *testing.T) {
	store := NewMemoryStore("r1")
	var wg sync.WaitGroup
	n := 100

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			store.Put("key1", fmt.Sprintf("v%d", i))
		}(i)
	}
	wg.Wait()

	entry, found := store.Get("key1")
	if !found {
		t.Fatal("key should exist after concurrent puts")
	}
	if entry.Version != int64(n) {
		t.Errorf("expected version %d after %d puts, got %d", n, n, entry.Version)
	}
}

func TestConcurrentReads(t *testing.T) {
	store := NewMemoryStore("r1")
	store.Put("key1", "value1")

	var wg sync.WaitGroup
	n := 100

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			entry, found := store.Get("key1")
			if !found {
				t.Error("expected found")
			}
			if entry.Value != "value1" {
				t.Errorf("expected value1, got %s", entry.Value)
			}
		}()
	}
	wg.Wait()
}
