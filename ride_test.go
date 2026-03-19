package main

import (
	"fmt"
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
	sim.onTransition = func(_ *Ride, _, to RideState) {
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

// TestConcurrentRidesAllComplete verifies that all rides reach StateCompleted
// when processed by multiple concurrent workers.
func TestConcurrentRidesAllComplete(t *testing.T) {
	const n = 10
	var mu sync.Mutex
	completed := 0
	var wg sync.WaitGroup
	wg.Add(n)

	sim := NewSimulator(3)
	sim.onTransition = func(_ *Ride, _, to RideState) {
		if to == StateCompleted {
			mu.Lock()
			completed++
			mu.Unlock()
			wg.Done()
		}
	}
	sim.Run()
	defer sim.Shutdown()

	for i := 1; i <= n; i++ {
		sim.Submit(&Ride{
			ID:        fmt.Sprintf("ride-%d", i),
			RiderID:   fmt.Sprintf("rider-%d", i),
			State:     StateRequested,
			UpdatedAt: time.Now().UTC(),
		})
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("only %d/%d rides completed within timeout", completed, n)
	}

	mu.Lock()
	defer mu.Unlock()
	if completed != n {
		t.Errorf("want %d completed rides, got %d", n, completed)
	}
}

// TestStateOrderUnderConcurrency verifies that every individual ride maintains
// the correct state order even when multiple rides are processed concurrently.
func TestStateOrderUnderConcurrency(t *testing.T) {
	const n = 5
	want := []RideState{StateMatched, StateInProgress, StateCompleted}

	sequences := make(map[string]*[]RideState)
	for i := 1; i <= n; i++ {
		s := &[]RideState{}
		sequences[fmt.Sprintf("ride-%d", i)] = s
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(n)

	sim := NewSimulator(3)
	sim.onTransition = func(ride *Ride, _, to RideState) {
		mu.Lock()
		if s, ok := sequences[ride.ID]; ok {
			*s = append(*s, to)
		}
		mu.Unlock()
		if to == StateCompleted {
			wg.Done()
		}
	}
	sim.Run()
	defer sim.Shutdown()

	for i := 1; i <= n; i++ {
		sim.Submit(&Ride{
			ID:        fmt.Sprintf("ride-%d", i),
			RiderID:   fmt.Sprintf("rider-%d", i),
			State:     StateRequested,
			UpdatedAt: time.Now().UTC(),
		})
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("rides did not complete within timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	for id, s := range sequences {
		if len(*s) != 3 {
			t.Errorf("ride %s: want 3 transitions, got %d", id, len(*s))
			continue
		}
		for i, state := range want {
			if (*s)[i] != state {
				t.Errorf("ride %s transition[%d]: want %s, got %s", id, i, state, (*s)[i])
			}
		}
	}
}
