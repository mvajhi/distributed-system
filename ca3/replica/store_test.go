package main

import (
	"testing"
)

func TestStorePutConflictResolution(t *testing.T) {
	s := NewStore()
	key := "test-key"

	// 1. Initial write
	item1 := Item{Value: "val1", Version: 1, UpdatedBy: "replica1"}
	updated, active := s.Put(key, item1)
	if !updated || active.Value != "val1" {
		t.Errorf("Initial write failed: expected true, got %v with value %s", updated, active.Value)
	}

	// 2. Write with a higher version
	item2 := Item{Value: "val2", Version: 2, UpdatedBy: "replica1"}
	updated, active = s.Put(key, item2)
	if !updated || active.Value != "val2" || active.Version != 2 {
		t.Errorf("Higher version write failed: expected update to version 2, active: %+v", active)
	}

	// 3. Write with a lower version (should be rejected)
	item3 := Item{Value: "val3", Version: 1, UpdatedBy: "replica3"}
	updated, active = s.Put(key, item3)
	if updated || active.Value != "val2" || active.Version != 2 {
		t.Errorf("Lower version write should be rejected: updated=%v, active: %+v", updated, active)
	}

	// 4. Write with equal version but higher replica ID (should be accepted - replica2 > replica1)
	item4 := Item{Value: "val4", Version: 2, UpdatedBy: "replica2"}
	updated, active = s.Put(key, item4)
	if !updated || active.Value != "val4" || active.UpdatedBy != "replica2" {
		t.Errorf("Equal version, higher replica ID should be accepted: updated=%v, active: %+v", updated, active)
	}

	// 5. Write with equal version but lower replica ID (should be rejected - replica1 < replica2)
	item5 := Item{Value: "val5", Version: 2, UpdatedBy: "replica1"}
	updated, active = s.Put(key, item5)
	if updated || active.Value != "val4" || active.UpdatedBy != "replica2" {
		t.Errorf("Equal version, lower replica ID should be rejected: updated=%v, active: %+v", updated, active)
	}
}
