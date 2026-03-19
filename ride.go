package main

import "time"

type RideState string

const (
	StateRequested  RideState = "requested"
	StateMatched    RideState = "matched"
	StateInProgress RideState = "in_progress"
	StateCompleted  RideState = "completed"
)

type Ride struct {
	ID        string    `json:"id"`
	RiderID   string    `json:"rider_id"`
	State     RideState `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	RideID    string    `json:"ride_id"`
	RiderID   string    `json:"rider_id"`
	From      RideState `json:"from,omitempty"`
	To        RideState `json:"to"`
}
