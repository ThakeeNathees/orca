// The database layer for the orca runtime.
package db

// DB interface is the interface for the database layer. It is used to store and retrieve checkpoints.
type DB interface {

	// Returns the list of running execution IDs.
	GetRunningExecutionIDs() []string

	// Adds an event to the database.
	AddEvent(runID string, event *Event) error

	// Returns nil if no events found.
	GetEvents(runID string) []*Event
}
