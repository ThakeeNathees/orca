package db

// Implementation of the DB, InMemoryDB is a simple in-memory database that is used for testing.
type InMemoryDB struct {
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
	ids := make([]string, 0, len(db.executions))
	for id := range db.executions {
		ids = append(ids, id)
	}
	return ids
}

func (db *InMemoryDB) AddEvent(runID string, event *Event) error {
	db.events[runID] = append(db.events[runID], event)
	return nil
}

func (db *InMemoryDB) GetEvents(runID string) []*Event {
	return db.events[runID]
}
