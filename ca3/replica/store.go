package main

import (
	"sync"
)

// Item represents a versioned value in the key-value store.
type Item struct {
	Value     string `json:"value"`
	Version   int    `json:"version"`
	UpdatedBy string `json:"updated_by"`
}

// Store is a thread-safe in-memory key-value database.
type Store struct {
	mu sync.RWMutex
	db map[string]Item
}

// NewStore creates a new Store instance.
func NewStore() *Store {
	return &Store{
		db: make(map[string]Item),
	}
}

// Get retrieves an item by key. Returns the item and a boolean indicating if it exists.
func (s *Store) Get(key string) (Item, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.db[key]
	return item, exists
}

// Put updates a key with the incoming Item applying the Last-Write-Wins (LWW)
// and version checks. It returns true if the write was successful/applied, and
// the current active item in the store for that key.
func (s *Store) Put(key string, incoming Item) (bool, Item) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.db[key]
	if !exists {
		s.db[key] = incoming
		return true, incoming
	}

	// 1. Version check: higher version always wins
	if incoming.Version > current.Version {
		s.db[key] = incoming
		return true, incoming
	} else if incoming.Version < current.Version {
		return false, current
	}

	// 2. Equal versions: Last-Write-Wins (LWW) using alphabetical order of replica IDs
	if incoming.UpdatedBy > current.UpdatedBy {
		s.db[key] = incoming
		return true, incoming
	}

	return false, current
}

// IncrementAndPut atomically increments the version of the key and stores the new value,
// returning the newly created Item. This is used by the coordinator replica for client writes.
func (s *Store) IncrementAndPut(key string, value string, updatedBy string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.db[key]
	newVersion := 1
	if exists {
		newVersion = current.Version + 1
	}

	newItem := Item{
		Value:     value,
		Version:   newVersion,
		UpdatedBy: updatedBy,
	}
	s.db[key] = newItem
	return newItem
}
