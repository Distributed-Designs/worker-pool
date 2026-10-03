
# Worker Pool — Architecture

## 1. System Overview

The Worker Pool is an in-memory concurrent job-processing system.

The system consists of:

- Producer
- Jobs channel
- Worker goroutines
- Results channel
- Consumer
- WaitGroup
- Context

The producer submits jobs to the worker pool. The pool distributes jobs among multiple workers, and workers return execution results through the results channel.

---

## 2. System Architecture

```text
                         +----------------+
                         |    Producer    |
                         +-------+--------+
                                 |
                                 | Submit(Job)
                                 v
                         +---------------+
                         |  Jobs Channel |
                         |   Buffered    |
                         +-------+-------+
                                 |
                +----------------+----------------+
                |                |                |
                v                v                v
          +-----------+    +-----------+    +-----------+
          |  Worker 1 |    |  Worker 2 |    |  Worker N |
          +-----+-----+    +-----+-----+    +-----+-----+
                |                |                |
                +----------------+----------------+
                                 |
                                 v
                         +---------------+
                         | Results       |
                         | Channel       |
                         +-------+-------+
                                 |
                                 v
                         +---------------+
                         |   Consumer    |
                         +---------------+
```

---

## 3. Components

### 3.1 Producer

The producer creates jobs and submits them to the pool.

```go
p.Submit(job)
```

The producer does not need to know which worker will execute the job.

---

### 3.2 Jobs Channel

The jobs channel acts as the in-memory work queue.

```go
jobs chan job.Job
```

It is buffered according to:

```go
JobQueueSize
```

The channel provides communication between producers and workers.

---

### 3.3 Workers

Each worker runs as a separate goroutine.

```text
Worker 1
Worker 2
Worker 3
...
Worker N
```

All workers consume from the same jobs channel.

A worker:

1. Receives a job.
2. Executes the task.
3. Creates a result.
4. Sends the result to the results channel.
5. Waits for another job.

---

### 3.4 Results Channel

The results channel transports completed job results from workers to the consumer.

```go
results chan result.Result
```

It is buffered according to:

```go
ResultQueueSize
```

Consumers access it through:

```go
p.Results()
```

---

### 3.5 Consumer

The consumer reads results from the results channel.

```go
for result := range p.Results() {
    // process result
}
```

The consumer is responsible for deciding what to do with successful results or errors.

---

## 4. Worker Communication

All workers share the same jobs channel.

```text
                    +----------------+
                    |  Jobs Channel  |
                    +-------+--------+
                            |
              +-------------+-------------+
              |             |             |
              v             v             v
         +---------+   +---------+   +---------+
         | Worker 1|   | Worker 2|   | Worker N|
         +---------+   +---------+   +---------+
```

The pool does not explicitly assign individual jobs to individual workers.

Workers receive available jobs from the shared channel.

---

## 5. Job Data Flow

```text
Producer
    |
    | Job
    v
+-----------+
| Jobs      |
| Channel   |
+-----+-----+
      |
      +--------> Worker 1
      |
      +--------> Worker 2
      |
      +--------> Worker N
```

Each job contains:

```go
type Job struct {
    ID   int
    Task func(context.Context) (any, error)
}
```

---

## 6. Result Data Flow

```text
Worker 1 --------+
                 |
Worker 2 --------+----> Results Channel ----> Consumer
                 |
Worker N --------+
```

Each result contains:

```go
type Result struct {
    JobID int
    Value any
    Err   error
}
```

The result retains the original `JobID`, allowing the consumer to associate the result with the submitted job.

---

## 7. Worker Lifecycle

```text
                     +----------------+
                     |     Worker     |
                     +-------+--------+
                             |
                             v
                      Wait for Event
                             |
                 +-----------+-----------+
                 |                       |
                 v                       v
        Context Cancelled           Job Received
                 |                       |
                 v                       v
               Exit                 Execute Task
                                         |
                                         v
                                  Create Result
                                         |
                                         v
                                  Send Result
                                         |
                                         v
                                   Wait Again
```

A worker exits when:

- The context is cancelled.
- The jobs channel is closed.

---

## 8. Pool Lifecycle

```text
                    New()
                      |
                      v
              Create Pool
                      |
                      v
                    Start()
                      |
                      v
              Start N Workers
                      |
                      v
               Submit Jobs
                      |
                      v
              Process Jobs
                      |
                      v
                   Stop()
                      |
                      v
             Close Jobs Channel
                      |
                      v
              Workers Finish
                      |
                      v
            WaitGroup = Zero
                      |
                      v
            Close Results Channel
```

---

## 9. Startup Architecture

When `Start()` is called:

```text
                         Pool
                          |
              +-----------+-----------+
              |           |           |
              v           v           v
          Worker 1    Worker 2    Worker N
              |           |           |
              +-----------+-----------+
                          |
                    Shared Jobs Channel
```

Each worker is launched as a goroutine.

The pool tracks every worker using a `sync.WaitGroup`.

---

## 10. Shutdown Architecture

Shutdown consists of two mechanisms.

### 10.1 Jobs Channel Closure

The pool closes the jobs channel:

```go
close(p.jobs)
```

This indicates that no additional jobs should be submitted.

Workers finish processing jobs already received and then exit when the channel is exhausted.

```text
close(jobs)
     |
     v
Workers finish queued work
     |
     v
Workers exit
```

### 10.2 Context Cancellation

The context provides a cancellation signal:

```text
cancel()
   |
   v
context.Done()
   |
   +--------+--------+--------+
   |        |        |        |
   v        v        v        v
Worker 1 Worker 2 Worker 3 Worker N
```

A worker can stop waiting for additional work when the context is cancelled.

---

## 11. WaitGroup Coordination

The pool uses a `sync.WaitGroup` to track worker completion.

```text
                 WaitGroup
                     |
       +-------------+-------------+
       |             |             |
       v             v             v
    Worker 1      Worker 2      Worker N
       |             |             |
     Done()        Done()        Done()
       |             |             |
       +-------------+-------------+
                     |
                     v
               Counter = 0
                     |
                     v
             Close Results
```

The results channel is closed only after all workers have completed.

---

## 12. Channel Ownership

The pool owns both channels:

```text
Pool
 |
 +---- Jobs Channel
 |
 +---- Results Channel
```

### Jobs Channel

```text
Pool
 |
 +-- creates
 +-- provides to workers
 +-- closes
```

### Results Channel

```text
Pool
 |
 +-- creates
 +-- provides to workers
 +-- closes after all workers finish
```

Workers do not close either channel.

This centralizes channel lifecycle management.

---

## 13. Backpressure

The jobs channel is bounded.

Example:

```go
jobs := make(chan job.Job, 100)
```

Architecture:

```text
Producer
    |
    v
+-----------------------+
|     Jobs Queue        |
|                       |
| Job Job Job Job ...   |
|       capacity = 100  |
+-----------+-----------+
            |
            v
         Workers
```

When the queue is full, a producer submitting another job can block until a worker consumes a job.

This provides basic backpressure.

---

## 14. Context Propagation

The context flows through the system:

```text
Application
     |
     v
  Context
     |
     +-------------+-------------+
     |             |             |
     v             v             v
 Worker 1       Worker 2       Worker N
     |             |             |
     v             v             v
   Task          Task          Task
```

The task receives the same context:

```go
j.Task(ctx)
```

Tasks can use the context to implement cooperative cancellation.

---

## 15. Package Architecture

```text
cmd/workerpool
       |
       v
     pool
       |
       +----------> config
       |
       +----------> job
       |
       +----------> result
       |
       +----------> worker
                       |
                       +----> job
                       |
                       +----> result
```

### Package Responsibilities

| Package | Responsibility |
|---|---|
| `cmd/workerpool` | Application entry point and wiring |
| `internal/config` | Worker pool configuration |
| `internal/job` | Job definition |
| `internal/result` | Result definition |
| `internal/worker` | Individual worker execution |
| `internal/pool` | Pool coordination and lifecycle |

The worker package does not depend on the pool package, preventing circular dependencies.

---

## 16. Concurrency Model

The system uses a fixed number of worker goroutines.

For example:

```text
WorkerCount = 4
```

results in:

```text
                    Jobs Channel
                         |
          +--------------+--------------+
          |              |              |
          v              v              v
       Worker 1       Worker 2       Worker 3
                                         |
                                      Worker 4
```

Workers execute independently and concurrently.

The shared jobs channel provides synchronization between producers and workers.

---

## 17. Performance Model

Increasing worker count can increase throughput when jobs can execute concurrently.

However, additional workers also introduce:

- Goroutine scheduling overhead
- Channel synchronization
- CPU contention
- Context switching
- Potential CPU saturation

Therefore:

```text
More Workers
     |
     v
More Parallelism
     |
     v
Higher Throughput
     |
     v
Saturation Point
     |
     v
Additional Workers
     |
     v
Possible Performance Degradation
```

Worker count should therefore be evaluated using benchmarks.

---

## 18. Failure Model

A task can return:

```go
(value, error)
```

The worker converts the task outcome into a `Result`.

```text
Task
 |
 +---- Success ----> Result.Value
 |
 +---- Failure ----> Result.Err
```

The worker does not retry failed jobs.

Retry and dead-letter handling are outside the current architecture.

---

## 19. System Boundary

The Worker Pool is an in-memory concurrency component.

```text
+--------------------------------------+
|            Worker Pool               |
|                                      |
| Producer                             |
|    |                                 |
|    v                                 |
| Jobs Channel                         |
|    |                                 |
|    +--> Workers                      |
|             |                        |
|             v                        |
|       Results Channel                |
|             |                        |
|             v                        |
|          Consumer                    |
+--------------------------------------+
```

The system does not currently provide:

- Persistent job storage
- Distributed workers
- Network communication
- Job durability
- Retry queues
- Dead-letter queues
- Service discovery
- Distributed coordination

These concerns are intentionally handled by later projects in the distributed-systems roadmap.
