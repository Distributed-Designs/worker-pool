
# Worker Pool

A production-style concurrent worker pool implemented in Go.

This project is part of the **Distributed-Designs** learning lab, where distributed systems and backend infrastructure are built from first principles using Go.

The goal of this project is to understand how worker pools are designed, how goroutines and channels coordinate concurrent work, and how to reason about concurrency, backpressure, cancellation, lifecycle management, and performance.

---

## Overview

A worker pool maintains a fixed number of worker goroutines that continuously consume jobs from a shared queue and execute them concurrently.

```text
                  +----------------+
                  |    Producer    |
                  +-------+--------+
                          |
                          v
                  +---------------+
                  |   Job Queue   |
                  |   (Channel)   |
                  +-------+-------+
                          |
              +-----------+-----------+
              |           |           |
              v           v           v
          +-------+   +-------+   +-------+
          |Worker |   |Worker |   |Worker |
          |   1   |   |   2   |   |   N   |
          +---+---+   +---+---+   +---+---+
              |           |           |
              +-----------+-----------+
                          |
                          v
                  +---------------+
                  | Result Queue  |
                  |   (Channel)   |
                  +-------+-------+
                          |
                          v
                  +---------------+
                  |   Consumer    |
                  +---------------+
```

The pool provides:

- Fixed-size worker concurrency
- Buffered job queue
- Buffered result queue
- Context-based cancellation
- Job error propagation
- Graceful worker shutdown
- Backpressure through bounded queues
- Benchmarking across different worker counts
- Race detection and static analysis

---

## Why a Worker Pool?

Creating a new goroutine for every incoming task can result in an uncontrolled number of goroutines.

For example:

```go
for _, task := range tasks {
    go task()
}
```

With a large workload, this can create excessive:

- CPU scheduling overhead
- Memory usage
- Goroutine count
- Resource contention

A worker pool limits concurrency by maintaining a fixed number of workers.

```text
1000 Jobs
    |
    v
+----------------+
|   Job Queue    |
+----------------+
    |
    v
+-------------------------------+
| 4 Worker Goroutines           |
|                               |
| Worker 1                      |
| Worker 2                      |
| Worker 3                      |
| Worker 4                      |
+-------------------------------+
```

Only four jobs can execute concurrently in this configuration.

---

## Features

### Fixed Worker Count

The number of workers is configured when creating the pool.

```go
type Config struct {
    WorkerCount     int
    JobQueueSize    int
    ResultQueueSize int
}
```

---

### Buffered Job Queue

Jobs are submitted through a bounded channel.

```go
jobs chan job.Job
```

The queue prevents producers from immediately requiring a worker for every submitted job.

The queue is bounded to provide basic backpressure.

---

### Result Queue

Workers publish execution results through a result channel.

```go
results chan result.Result
```

Each result contains:

```go
type Result struct {
    JobID int
    Value any
    Err   error
}
```

---

### Context Cancellation

Workers receive a `context.Context`.

```go
func (w *Worker) Run(ctx context.Context)
```

Tasks also receive the context:

```go
Task func(context.Context) (any, error)
```

This allows tasks to cooperate with cancellation.

---

### Graceful Shutdown

Calling:

```go
p.Stop()
```

closes the job queue.

Workers finish processing jobs already present in the queue before exiting.

The result channel is closed only after all workers have completed.

---

### Error Propagation

Workers do not decide how an error should be handled.

Instead, the error is returned through the result:

```go
result.Result{
    JobID: j.ID,
    Value: value,
    Err:   err,
}
```

The consumer can decide whether to log, retry, ignore, or propagate the error.

---

## Project Structure

```text
worker-pool/
│
├── cmd/
│   └── workerpool/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── job/
│   │   └── job.go
│   │
│   ├── worker/
│   │   └── worker.go
│   │
│   ├── pool/
│   │   └── pool.go
│   │
│   └── result/
│       └── result.go
│
├── tests/
│   └── integration/
│       └── worker_pool_test.go
│
├── benchmarks/
│   └── worker_pool_test.go
│
├── docs/
│   ├── hld.md
│   ├── lld.md
│   ├── architecture.md
│   └── design-decisions.md
│
├── scripts/
│
├── .github/
│   └── workflows/
│       └── ci.yml
│
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## Core Components

### Job

Represents a unit of work.

```go
type Job struct {
    ID   int
    Task func(context.Context) (any, error)
}
```

The task receives a context and returns:

- A result value
- An error

---

### Worker

A worker consumes jobs from the shared job channel.

```text
Worker
   |
   v
Receive Job
   |
   v
Execute Task
   |
   v
Create Result
   |
   v
Send Result
```

Workers continue processing jobs until:

- The context is cancelled.
- The jobs channel is closed.

---

### Pool

The pool is responsible for:

- Creating workers
- Starting workers
- Managing the job queue
- Managing the result queue
- Tracking worker completion
- Closing the result channel

---

### Configuration

Default configuration:

```go
func Default() Config {
    return Config{
        WorkerCount:     4,
        JobQueueSize:    100,
        ResultQueueSize: 100,
    }
}
```

---

## Basic Usage

Create a configuration:

```go
cfg := config.Default()
```

Create a pool:

```go
p := pool.New(cfg)
```

Start the workers:

```go
ctx := context.Background()

p.Start(ctx)
```

Submit jobs:

```go
for i := 1; i <= 10; i++ {
    id := i

    p.Submit(job.Job{
        ID: id,
        Task: func(ctx context.Context) (any, error) {
            return fmt.Sprintf("job %d completed", id), nil
        },
    })
}
```

Stop accepting jobs:

```go
p.Stop()
```

Consume results:

```go
for r := range p.Results() {
    if r.Err != nil {
        fmt.Printf("Job %d failed: %v\n", r.JobID, r.Err)
        continue
    }

    fmt.Printf("Job %d: %v\n", r.JobID, r.Value)
}
```

---

## Concurrency Model

The worker pool uses Go's concurrency primitives:

```text
                 Context
                    |
                    v
              +-----------+
              |   Pool    |
              +-----------+
                    |
                    v
             +-------------+
             | Jobs Channel|
             +-------------+
              /     |     \
             /      |      \
            v       v       v
        Worker 1 Worker 2 Worker N
            |       |       |
            +-------+-------+
                    |
                    v
             Results Channel
```

Workers independently consume from the same jobs channel.

Go's channel synchronization ensures that a job received by one worker is not simultaneously received by another worker.

---

## Backpressure

The job queue is bounded.

For example:

```go
make(chan job.Job, 100)
```

If the queue is full, a call to:

```go
p.Submit(job)
```

can block until a worker consumes a job.

This creates a simple backpressure mechanism.

```text
Producer
   |
   v
+------------------+
| Job Queue        |
| Capacity = 100   |
+------------------+
   |
   | queue full
   v
Producer blocks
   |
   v
Worker consumes job
   |
   v
Queue capacity available
```

This prevents an unbounded in-memory queue from continuously growing.

---

## Cancellation

Cancellation is cooperative.

The worker checks the context while waiting for jobs:

```go
select {
case <-ctx.Done():
    return

case j, ok := <-w.Jobs:
    ...
}
```

The task also receives the context.

A task can explicitly observe cancellation:

```go
Task: func(ctx context.Context) (any, error) {
    select {
    case <-time.After(5 * time.Second):
        return "completed", nil

    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

Cancelling a context does not forcibly terminate arbitrary Go code.

The executing task must cooperate with the cancellation mechanism.

---

## Shutdown Model

The shutdown sequence is:

```text
Submit Jobs
    |
    v
Close Jobs Channel
    |
    v
Workers Finish Queued Jobs
    |
    v
Workers Exit
    |
    v
WaitGroup Reaches Zero
    |
    v
Close Results Channel
    |
    v
Consumers Finish Reading Results
```

The result channel is closed only after all workers have stopped.

This prevents:

```text
panic: send on closed channel
```

---

## Ordering

The worker pool does not guarantee result ordering.

For example:

```text
Job 1 -> Worker 1 -> 500ms
Job 2 -> Worker 2 -> 100ms
```

Job 2 can complete before Job 1.

Consumers should use `JobID` when they need to associate results with submitted jobs.

---

## Testing

Run the complete test suite:

```bash
go test ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

The test suite covers:

- Job processing
- Result correctness
- Error propagation
- Concurrent execution
- Context cancellation

---

## Benchmarking

The benchmark evaluates different worker counts:

```text
1
2
4
8
16
```

Run:

```bash
go test -bench=. -benchmem ./benchmarks
```

Run multiple benchmark iterations:

```bash
go test -bench=. -benchmem ./benchmarks -count=3
```

The benchmarks measure:

- Execution time
- Memory allocations
- Allocations per operation
- Effect of worker count
- Concurrency scaling

The project includes both general and CPU-intensive workloads.

---

## Race Detection

Because the worker pool relies on concurrent goroutines, race detection is an important part of validation.

Run:

```bash
go test -race ./...
```

The goal is to ensure that concurrent execution does not introduce unsafe shared-memory access.

---

## Engineering Decisions

The implementation intentionally keeps the worker pool focused.

The system uses:

- Goroutines for workers
- Channels for communication
- Buffered channels for bounded queues
- `sync.WaitGroup` for worker lifecycle tracking
- `context.Context` for cancellation
- Explicit channel ownership
- Error propagation through results

The system intentionally does not implement:

- Persistent job storage
- Automatic retries
- Dynamic worker scaling
- Distributed scheduling
- HTTP APIs
- Redis
- Kafka
- Metrics
- Dead-letter queues
- Service discovery
- Distributed coordination

These concerns belong to later projects in the Distributed-Designs learning path.

See:

- [`docs/hld.md`](docs/hld.md)
- [`docs/lld.md`](docs/lld.md)
- [`docs/architecture.md`](docs/architecture.md)
- [`docs/design-decisions.md`](docs/design-decisions.md)

---

## Design Principles

This project follows several engineering principles:

### First Principles

Understand the concurrency problem before implementing abstractions.

### Explicit Ownership

The pool owns the lifecycle of its channels.

### Bounded Resources

Queues have configurable limits.

### Correctness First

Tests and race detection are used before performance optimization.

### Measure Performance

Worker count and workload characteristics are evaluated through benchmarks.

### Simple Until Necessary

Additional complexity should be introduced only when a concrete requirement justifies it.

---

## Learning Objectives

By completing this project, the following concepts are practiced:

- Goroutines
- Channels
- Buffered channels
- Producer-consumer architecture
- Worker pools
- Synchronization
- `sync.WaitGroup`
- Context cancellation
- Graceful shutdown
- Backpressure
- Error propagation
- Concurrent testing
- Race detection
- Benchmarking
- Resource management
- Package boundaries
- Design trade-offs

---

## Future Improvements

Possible extensions include:

- Dynamic worker scaling
- Retry policies
- Exponential backoff
- Job priorities
- Job timeouts
- Metrics
- Worker utilization tracking
- Queue depth monitoring
- Panic recovery
- Graceful context-aware submission
- Persistent job queues

These are intentionally outside the scope of the initial implementation.

---

## Part of Distributed-Designs

This Worker Pool is the first project in the distributed systems learning lab.

The broader progression is:

```text
Level 1 — Go Concurrency & Foundations
        |
        +-- Worker Pool
        +-- Rate Limiter
        +-- Cache
        |
Level 2 — Backend Infrastructure
        |
        +-- Message Queue
        +-- Load Balancer
        +-- API Gateway
        +-- Circuit Breaker
        |
Level 3 — Distributed Systems
        |
        +-- Service Discovery
        +-- Distributed Lock
        +-- Consistent Hashing
        +-- Bloom Filter
        +-- Distributed Cache
        |
Level 4 — Consensus
        |
        +-- Raft
```

The objective is to progress from concurrency primitives to complete distributed-system components through implementation, testing, benchmarking, documentation, and analysis.
