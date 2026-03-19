package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	log.SetFlags(0) // suppress log prefix — output is pure JSON

	metricMode := flag.Bool("metric", false, "run benchmark mode and write reports/ then exit")
	numRides := flag.Int("n", 50, "number of rides to simulate in metric mode")
	flag.Parse()

	if *metricMode {
		runMetric(*numRides)
		return
	}

	sim := NewSimulator(5)
	sim.Run()

	mux := http.NewServeMux()

	// POST /rides — submit a new ride request; returns the ride ID and initial state
	mux.HandleFunc("/rides", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		n := atomic.AddInt64(&riderCounter, 1)
		ride := &Ride{
			ID:        fmt.Sprintf("ride-%d", n),
			RiderID:   fmt.Sprintf("rider-%d", n),
			State:     StateRequested,
			UpdatedAt: time.Now().UTC(),
		}
		logJSON(LogEntry{Timestamp: ride.UpdatedAt, RideID: ride.ID, RiderID: ride.RiderID, To: StateRequested})
		sim.Submit(ride)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(ride)
	})

	// GET /rides/{id} — query live state of a ride
	mux.HandleFunc("/rides/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/rides/")
		ride, ok := sim.GetRide(id)
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"ride not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ride)
	})

	srv := &http.Server{Addr: ":8080", Handler: mux}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logJSON(map[string]string{"event": "server_started", "addr": ":8080"})
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-quit
	logJSON(map[string]string{"event": "shutdown_initiated", "msg": "draining in-flight requests"})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	sim.Shutdown()
	logJSON(map[string]string{"event": "shutdown_complete", "msg": "all rides drained"})
}

var riderCounter int64

func runMetric(n int) {
	type record struct {
		RideID     string    `json:"ride_id"`
		State      RideState `json:"state"`
		DurationMS int64     `json:"duration_ms"`
	}

	var mu sync.Mutex
	var records []record
	var lastTime sync.Map // rideID → time.Time

	var wg sync.WaitGroup
	wg.Add(n)

	sim := NewSimulator(5)
	sim.onTransition = func(ride *Ride, _, to RideState) {
		now := time.Now()
		var durMS int64
		if v, ok := lastTime.Load(ride.ID); ok {
			durMS = now.Sub(v.(time.Time)).Milliseconds()
		}
		lastTime.Store(ride.ID, now)
		mu.Lock()
		records = append(records, record{RideID: ride.ID, State: to, DurationMS: durMS})
		mu.Unlock()
		if to == StateCompleted {
			wg.Done()
		}
	}
	sim.Run()

	for i := 1; i <= n; i++ {
		ride := &Ride{
			ID:        fmt.Sprintf("ride-%d", i),
			RiderID:   fmt.Sprintf("rider-%d", i),
			State:     StateRequested,
			UpdatedAt: time.Now().UTC(),
		}
		lastTime.Store(ride.ID, time.Now())
		sim.Submit(ride)
	}

	wg.Wait()
	sim.Shutdown()

	if err := os.MkdirAll("reports", 0o755); err != nil {
		log.Fatal(err)
	}

	b, _ := json.MarshalIndent(records, "", "  ")
	if err := os.WriteFile("reports/metrics.json", b, 0o644); err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("reports/metrics.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ride_id", "state", "duration_ms"})
	for _, r := range records {
		_ = w.Write([]string{r.RideID, string(r.State), fmt.Sprintf("%d", r.DurationMS)})
	}
	w.Flush()

	fmt.Printf("wrote %d records to reports/\n", len(records))
}
