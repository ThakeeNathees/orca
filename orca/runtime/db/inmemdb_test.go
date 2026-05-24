package db

import (
	"fmt"
	"sync"
	"testing"
)

func TestInMemoryDBAddAndGetEvents(t *testing.T) {
	d := NewInMemoryDB()
	ev1 := NewRunNodeEvent("run-a", "n1")
	ev2 := NewRunNodeEvent("run-a", "n2")

	if err := d.AddEvent("run-a", ev1); err != nil {
		t.Fatal(err)
	}
	if err := d.AddEvent("run-a", ev2); err != nil {
		t.Fatal(err)
	}

	got := d.GetEvents("run-a")
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].NodeID != "n1" || got[1].NodeID != "n2" {
		t.Fatal(got[0].NodeID, got[1].NodeID)
	}

	if d.GetEvents("missing") != nil {
		t.Fatal("missing run should return nil slice")
	}
}

func TestInMemoryDBGetRunningExecutionIDs(t *testing.T) {
	d := NewInMemoryDB()
	if err := d.EnsureExecution("e1"); err != nil {
		t.Fatal(err)
	}
	if err := d.EnsureExecution("e2"); err != nil {
		t.Fatal(err)
	}
	ids := d.GetRunningExecutionIDs()
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	if !seen["e1"] || !seen["e2"] {
		t.Fatalf("ids = %v", ids)
	}

	if err := d.CompleteExecution("e1"); err != nil {
		t.Fatal(err)
	}
	ids = d.GetRunningExecutionIDs()
	if len(ids) != 1 || ids[0] != "e2" {
		t.Fatalf("ids after complete = %v", ids)
	}
}

func TestInMemoryDBConcurrentAccess(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, d *InMemoryDB)
	}{
		{
			name: "concurrent_add_and_get_events",
			run: func(t *testing.T, d *InMemoryDB) {
				const writers = 8
				const readers = 8
				const iterations = 150
				const runID = "run-concurrent"

				var writerWG sync.WaitGroup
				var readerWG sync.WaitGroup
				done := make(chan struct{})

				for i := range writers {
					writerWG.Add(1)
					go func(writerID int) {
						defer writerWG.Done()
						for j := range iterations {
							if err := d.AddEvent(runID, NewRunNodeEvent(runID, fmt.Sprintf("n-%d-%d", writerID, j))); err != nil {
								t.Errorf("AddEvent() error = %v", err)
								return
							}
						}
					}(i)
				}

				for range readers {
					readerWG.Add(1)
					go func() {
						defer readerWG.Done()
						for {
							select {
							case <-done:
								return
							default:
								_ = d.GetEvents(runID)
							}
						}
					}()
				}

				writerWG.Wait()
				close(done)
				readerWG.Wait()

				got := d.GetEvents(runID)
				want := writers * iterations
				if len(got) != want {
					t.Fatalf("len(events) = %d, want %d", len(got), want)
				}
			},
		},
		{
			name: "concurrent_execution_lifecycle_operations",
			run: func(t *testing.T, d *InMemoryDB) {
				const workers = 10
				const iterations = 120
				var wg sync.WaitGroup

				for i := range workers {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()
						runID := fmt.Sprintf("exec-%d", workerID%4)
						for j := range iterations {
							if j%2 == 0 {
								if err := d.EnsureExecution(runID); err != nil {
									t.Errorf("EnsureExecution() error = %v", err)
									return
								}
							} else {
								if err := d.CompleteExecution(runID); err != nil {
									t.Errorf("CompleteExecution() error = %v", err)
									return
								}
							}
							_ = d.GetRunningExecutionIDs()
						}
					}(i)
				}
				wg.Wait()

				ids := d.GetRunningExecutionIDs()
				seen := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					if _, ok := seen[id]; ok {
						t.Fatalf("duplicate execution id %q in %v", id, ids)
					}
					seen[id] = struct{}{}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, NewInMemoryDB())
		})
	}
}
