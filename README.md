# go-ride-simulator

A concurrent ride-dispatch simulator for demonstrating the fundamental building blocks of a full-featured production ride-hailing service:
goroutine-based concurrency; rigid state machine enforcement; and structured, observable logging.

## What it Does

Simulates a set of ride requests, each going through a 4-state lifecycle: `requested → matched → in_progress → completed`. Worker pool of goroutines are utilized to process rides concurrently. Each time a ride goes from one state to another, the new state is logged as structured JSON to standard output, which can be piped into any other log aggregator without additional processing.

In the "metric" mode, this simulates an end-to-end trip for 50 rides, and writes per-transition latency data to `reports/` as both CSV and PNG.

## Why it Matters

Any distributed system including artificial intelligence-based components — e.g., dispatch engines, routing models, real-time recommendation layers — will either survive or fail due to its ability to maintain state correctness while under concurrent load. This project is focused entirely on that problem — ensuring a system will behave consistently under concurrent load, and proving it with tests rather than simply assertions.

The invariant of the order of states is enforced in `ride_test.go` via a transition hook which logs every `(from, to)` pair during actual goroutine execution. The test then asserts that the exact sequence was executed — not merely the final state.

## Stack

Go, Goroutines, Channels, Sync.Map, HTTP (net/http), Structured Logging

## Run It

```bash
# Server mode
go run .

# Metric mode — writes reports/metrics.json, reports/metrics.csv, reports/throughput.png
go run . -metric

# Tests
go test ./...
```
