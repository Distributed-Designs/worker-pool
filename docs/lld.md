
# Worker Pool — Low-Level Design

## 1. Package Structure

```text
internal/
├── config/
│   └── config.go
├── job/
│   └── job.go
├── worker/
│   └── worker.go
├── pool/
│   └── pool.go
└── result/
    └── result.go
```

---

## 2. Job Package

### Responsibility

The `job` package defines the unit of work processed by the worker pool.

### Structure

```go
type Job struct {
    ID   int
    Task func(context.Context) (any, error)
}
```

### Fields

| Field | Type | Purpose |
|---|---|---|
| `ID` | `int` | Identifies the job |
| `Task` | `func(context.Context) (any, error)` | Function containing the work to execute |

The `Job` does not know:

- Which worker will execute it
- How many workers exist
- How the worker pool is implemented
- How results are consumed

---

## 3. Result Package

### Responsibility

The `result` package represents the outcome of executing a job.

### Structure

```go
type Result struct {
    JobID int
    Value any
    Err   error
}
```

### Fields

| Field | Type | Purpose |
|---|---|---|
| `JobID` | `int` | Identifies the completed job |
| `Value` | `any` | Result returned by the task |
| `Err` | `error` | Error returned by the task |

---

## 4. Config Package

### Responsibility

The `config` package contains configuration required by the worker pool.

### Structure

```go
type Config struct {
    WorkerCount     int
    JobQueueSize    int
    ResultQueueSize int
}
```

### Fields

| Field | Purpose |
|---|---|
| `WorkerCount` | Number of worker goroutines |
| `JobQueueSize` | Capacity of the jobs channel |
| `ResultQueueSize` | Capacity of the results channel |

### Default Configuration

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

## 5. Worker Package

### Responsibility

The worker package implements an individual worker.

A worker:

1. Waits for a job.
2. Executes the job.
3. Creates a result.
4. Sends the result.
5. Continues processing.
6. Exits when the context is cancelled or the jobs channel is closed.

### Structure

```go
type Worker struct {
    ID      int
    Jobs    <-chan job.Job
    Results chan<- result.Result
    WG      *sync.WaitGroup
}
```

### Fields

| Field | Purpose |
|---|---|
| `ID` | Identifies the worker |
| `Jobs` | Receive-only jobs channel |
| `Results` | Send-only results channel |
| `WG` | Tracks worker completion |

---

## 6. Directional Channels

The worker receives jobs through:

```go
Jobs <-chan job.Job
```

This means the worker can only receive from the channel.

The worker sends results through:

```go
Results chan<- result.Result
```

This means the worker can only send to the channel.

Directional channels make the communication contract explicit at compile time.

---

## 7. Worker Constructor

The worker is created using:

```go
func New(
    id int,
    jobs <-chan job.Job,
    results chan<- result.Result,
    wg *sync.WaitGroup,
) *Worker
```

The constructor receives the channels and `WaitGroup` from the pool.

The worker does not create or own these resources.

---

## 8. Worker Execution

The worker executes through:

```go
func (w *Worker) Run(ctx context.Context)
```

The lifecycle is:

```text
Run()
 |
 v
Wait for event
 |
 +-------------------+
 |                   |
 v                   v
Context cancelled   Job received
 |                   |
 v                   v
Exit              Execute Task
                       |
                       v
                 Create Result
                       |
                       v
                 Send Result
                       |
                       v
                  Repeat
```

The worker calls:

```go
defer w.WG.Done()
```

so the pool can determine when all workers have completed.

---

## 9. Worker Execution Logic

Conceptually:

```go
func (w *Worker) Run(ctx context.Context) {
    defer w.WG.Done()

    for {
        select {
        case <-ctx.Done():
            return

        case j, ok := <-w.Jobs:
            if !ok {
                return
            }

            value, err := j.Task(ctx)

            w.Results <- result.Result{
                JobID: j.ID,
                Value: value,
                Err:   err,
            }
        }
    }
}
```

The worker has two possible events:

1. Context cancellation
2. A new job becoming available

---

## 10. Pool Package

### Responsibility

The pool package coordinates the workers and manages their lifecycle.

### Structure

```go
type Pool struct {
    jobs        chan job.Job
    results     chan result.Result
    workerCount int

    wg sync.WaitGroup
}
```

### Fields

| Field | Purpose |
|---|---|
| `jobs` | Buffered channel containing submitted jobs |
| `results` | Buffered channel containing worker results |
| `workerCount` | Number of workers |
| `wg` | Tracks active workers |

---

## 11. Pool Constructor

The pool is created using:

```go
func New(cfg config.Config) *Pool
```

The constructor creates the channels using the configured capacities:

```go
jobs := make(chan job.Job, cfg.JobQueueSize)

results := make(
    chan result.Result,
    cfg.ResultQueueSize,
)
```

The pool stores the configured worker count.

---

## 12. Starting the Pool

Workers are started using:

```go
func (p *Pool) Start(ctx context.Context)
```

For each configured worker:

```text
WorkerCount = 4

Pool
 |
 +-- Worker 1
 +-- Worker 2
 +-- Worker 3
 +-- Worker 4
```

Each worker is started as a goroutine:

```go
go w.Run(ctx)
```

Before starting each worker:

```go
p.wg.Add(1)
```

is called.

---

## 13. Job Submission

Jobs are submitted through:

```go
func (p *Pool) Submit(j job.Job)
```

Internally:

```go
p.jobs <- j
```

The producer does not select a specific worker.

All workers consume from the same jobs channel.

---

## 14. Result Access

Results are exposed through:

```go
func (p *Pool) Results() <-chan result.Result
```

The return type is receive-only:

```go
<-chan result.Result
```

This means consumers can receive results but cannot close or send to the results channel.

Example:

```go
for r := range p.Results() {
    // process result
}
```

---

## 15. Pool Shutdown

The pool exposes:

```go
func (p *Pool) Stop()
```

The current implementation closes the jobs channel:

```go
close(p.jobs)
```

Closing the jobs channel communicates that no additional jobs should be submitted.

Workers finish processing jobs already received before exiting.

---

## 16. Result Channel Lifecycle

The results channel must remain open while workers can still produce results.

Therefore, the pool waits for all workers:

```go
go func() {
    p.wg.Wait()
    close(p.results)
}()
```

The lifecycle is:

```text
Jobs Channel Closed
        |
        v
Workers finish
        |
        v
WaitGroup.Done()
        |
        v
WaitGroup reaches zero
        |
        v
Results Channel Closed
```

This prevents workers from attempting to send to a closed results channel.

---

## 17. Context Cancellation

The caller creates the context:

```go
ctx := context.Background()
```

and starts the pool:

```go
p.Start(ctx)
```

The same context is passed to every worker:

```text
Caller
   |
   v
Context
   |
   +--------+--------+--------+
   |        |        |        |
   v        v        v        v
Worker 1 Worker 2 Worker 3 Worker N
```

The worker passes the context to the job:

```go
j.Task(ctx)
```

This allows the task to implement cooperative cancellation.

---

## 18. Cooperative Cancellation

A task can monitor:

```go
ctx.Done()
```

For example:

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

Context cancellation does not forcibly terminate a running function.

The task must cooperate with the cancellation mechanism.

---

## 19. Synchronization

The worker pool uses:

```go
sync.WaitGroup
```

to track workers.

Worker startup:

```go
p.wg.Add(1)
go w.Run(ctx)
```

Worker completion:

```go
defer w.WG.Done()
```

Pool synchronization:

```go
p.wg.Wait()
```

The `WaitGroup` reaches zero only after all workers have exited.

---

## 20. Channel Ownership

The pool owns the lifecycle of:

```text
jobs
results
```

Workers do not close either channel.

### Jobs Channel

```text
Pool
 |
 +-- creates jobs channel
 |
 +-- sends jobs
 |
 +-- closes jobs
```

### Results Channel

```text
Pool
 |
 +-- creates results channel
 |
 +-- workers send results
 |
 +-- closes results after workers finish
```

This keeps channel ownership centralized.

---

## 21. Package Dependency Flow

```text
cmd/workerpool
       |
       v
     pool
       |
       +--------> config
       |
       +--------> job
       |
       +--------> result
       |
       +--------> worker
                      |
                      +----> job
                      |
                      +----> result
```

The `worker` package does not depend on the `pool` package.

This avoids a circular dependency and keeps worker responsibilities isolated.

---

## 22. Concurrency Safety

The implementation relies primarily on:

- Go channels
- `sync.WaitGroup`
- `context.Context`

Shared mutable state is minimized.

Concurrency correctness is verified using:

```bash
go test -race ./...
```

---

## 23. Testing

Integration tests verify:

- Jobs are processed.
- Results are returned.
- Job IDs are preserved.
- Errors are propagated.
- Multiple workers execute jobs concurrently.
- Context cancellation works.
- Results channel eventually closes.

---

## 24. Benchmarking

The benchmark evaluates different worker counts:

```text
1 worker
2 workers
4 workers
8 workers
16 workers
```

The benchmark measures:

- Execution time
- Bytes allocated per operation
- Allocations per operation

Example command:

```bash
go test -bench=. -benchmem ./benchmarks
```

The benchmark is used to study how increasing concurrency affects throughput and resource usage.
