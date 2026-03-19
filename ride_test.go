package main

import (
	"sync"
	"testing"
	"time"
)

// TestStateOrderInvariant proves that every ride advances through states in
// exactly the order matched → in_progress → completed, with no skips or reversals.
func TestStateOrderInvariant(t *testing.T) {
	want := []RideState{StateMatched, StateInProgress, StateCompleted}
	var got []RideState
	var mu sync.Mutex
	done := make(chan struct{})

	sim := NewSimulator(1)
	sim.onTransition = func(_, to RideState) {
		mu.Lock()
		got = append(got, to)
		if len(got) == len(want) {
			close(done)
		}
		mu.Unlock()
	}
	sim.Run()
	defer sim.Shutdown()

	ride := &Ride{
		ID:        "inv-1",
		RiderID:   "rider-test",
		State:     StateRequested,
		UpdatedAt: time.Now().UTC(),
	}
	sim.Submit(ride)

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("ride did not complete within 10s")
	}

	mu.Lock()
	defer mu.Unlock()
	for i, state := range want {
		if got[i] != state {
			t.Errorf("transition[%d]: want %s, got %s", i, state, got[i])
		}
	}
}
