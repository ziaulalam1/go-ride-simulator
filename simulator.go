package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"
)

type Simulator struct {
	requests     chan *Ride
	rides        sync.Map
	wg           sync.WaitGroup
	workers      int
	onTransition func(from, to RideState) // optional; called after each state change (used in tests)
}

func NewSimulator(workers int) *Simulator {
	return &Simulator{
		requests: make(chan *Ride, 100),
		workers:  workers,
	}
}

func (s *Simulator) Run() {
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

// Submit enqueues a ride for processing. Blocks if the buffer is full.
func (s *Simulator) Submit(ride *Ride) {
	s.rides.Store(ride.ID, ride)
	s.requests <- ride
}

// Shutdown closes the request channel and waits for all workers to drain.
func (s *Simulator) Shutdown() {
	close(s.requests)
	s.wg.Wait()
}

func (s *Simulator) worker() {
	defer s.wg.Done()
	for ride := range s.requests {
		s.process(ride)
	}
}

func (s *Simulator) process(ride *Ride) {
	for _, next := range []RideState{StateMatched, StateInProgress, StateCompleted} {
		// Simulate real-world latency between state changes (0.5–1.5s)
		time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
		s.transition(ride, next)
	}
}

func (s *Simulator) transition(ride *Ride, to RideState) {
	entry := LogEntry{
		Timestamp: time.Now().UTC(),
		RideID:    ride.ID,
		RiderID:   ride.RiderID,
		From:      ride.State,
		To:        to,
	}
	from := ride.State
	ride.State = to
	ride.UpdatedAt = entry.Timestamp
	s.rides.Store(ride.ID, ride)
	logJSON(entry)
	if s.onTransition != nil {
		s.onTransition(from, to)
	}
}

func (s *Simulator) GetRide(id string) (*Ride, bool) {
	val, ok := s.rides.Load(id)
	if !ok {
		return nil, false
	}
	return val.(*Ride), true
}

func (s *Simulator) AllRides() []*Ride {
	var rides []*Ride
	s.rides.Range(func(_, val any) bool {
		rides = append(rides, val.(*Ride))
		return true
	})
	return rides
}

func logJSON(v any) {
	b, _ := json.Marshal(v)
	log.Println(string(b))
}
