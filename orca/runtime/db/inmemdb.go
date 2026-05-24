package db

import "sync"

// Implementation of the DB, InMemoryDB is a simple in-memory database that is used for testing.
type InMemoryDB struct {
	mu         sync.RWMutex
	executions map[string]any
	events     map[string][]*Event
}

func NewInMemoryDB() *InMemoryDB {
	return &InMemoryDB{
		executions: make(map[string]any),
		events:     make(map[string][]*Event),
	}
}

func (db *InMemoryDB) GetRunningExecutionIDs() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()
	ids := make([]string, 0, len(db.executions))
	for id := range db.executions {
		ids = append(ids, id)
	}
	return ids
}

func (db *InMemoryDB) EnsureExecution(runID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.executions[runID] = struct{}{}
	return nil
}

func (db *InMemoryDB) CompleteExecution(runID string) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	delete(db.executions, runID)
	return nil
}

func (db *InMemoryDB) AddEvent(runID string, event *Event) error {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.events[runID] = append(db.events[runID], event)
	return nil
}

func (db *InMemoryDB) GetEvents(runID string) []*Event {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.events[runID]
}
