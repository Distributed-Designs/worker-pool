
# Worker Pool — Design Decisions

## 1. Use Goroutines for Workers

### Decision

Each worker runs as a Go goroutine.

### Reason

Goroutines provide lightweight concurrency primitives for executing multiple jobs concurrently.

A fixed number of worker goroutines prevents the system from creating an unbounded number of goroutines.

### Trade-off

A fixed worker count limits concurrency.

However, it provides predictable resource usage and makes the concurrency model easier to reason about.

---

## 2. Use Channels for Job Distribution

### Decision

Use a Go channel as the job queue.

```go
jobs chan job.Job
```

### Reason

Channels provide a natural producer-consumer model.

The producer can submit jobs without knowing which worker will execute them.

All workers consume from the same jobs channel.

### Trade-off

The queue is entirely in memory.

Jobs are lost if the process terminates before they are processed.

Persistent queues are intentionally outside the scope of this project.

---

## 3. Use a Buffered Jobs Channel

### Decision

Use a buffered jobs channel.

```go
make(chan job.Job, queueSize)
```

### Reason

A buffered channel allows producers to submit multiple jobs without requiring an immediate worker receive.

The queue is bounded, which provides basic backpressure.

### Trade-off

When the queue reaches capacity, `Submit()` can block until a worker consumes a job.

This is preferable to allowing an unbounded queue to consume unlimited memory.

---

## 4. Use a Fixed Worker Count

### Decision

The number of workers is configured through:

```go
WorkerCount int
```

### Reason

A fixed worker count provides predictable resource consumption.

It also makes benchmarking and understanding concurrency behavior easier.

### Trade-off

The pool does not dynamically increase or decrease the number of workers based on workload.

Dynamic worker scaling is intentionally deferred.

---

## 5. Use Directional Channels

### Decision

Workers receive jobs using:

```go
<-chan job.Job
```

and send results using:

```go
chan<- result.Result
```

### Reason

Directional channels explicitly define how a component is allowed to interact with a channel.

The worker should only:

- Receive from the jobs channel.
- Send to the results channel.

### Trade-off

The types are slightly more verbose, but the communication contract becomes clearer and incorrect channel operations can be caught by the compiler.

---

## 6. Use `sync.WaitGroup`

### Decision

Use `sync.WaitGroup` to track worker completion.

### Reason

The pool needs to know when all workers have finished before closing the results channel.

The lifecycle is:

```text
Start Workers
     |
     v
WaitGroup.Add()
     |
     v
Workers Execute
     |
     v
WaitGroup.Done()
     |
     v
WaitGroup Reaches Zero
     |
     v
Close Results
```

### Trade-off

The implementation requires explicit synchronization.

However, `WaitGroup` directly represents the lifecycle relationship between the pool and its workers.

---

## 7. Pool Owns Channel Lifecycle

### Decision

The pool owns the jobs and results channels.

### Reason

The pool creates and manages the workers, so it is also responsible for coordinating channel lifecycle.

Workers only consume jobs and produce results.

### Trade-off

Channel management remains centralized instead of being distributed across workers.

This reduces the possibility of multiple workers attempting to close the same channel.

---

## 8. Workers Do Not Close Channels

### Decision

Workers never close the jobs or results channels.

### Reason

Multiple workers share the same channels.

If individual workers attempted to close a shared channel, another worker could still be using it.

The pool has the necessary lifecycle knowledge to determine when channels should be closed.

---

## 9. Close Results Only After Workers Finish

### Decision

The results channel is closed only after all workers have exited.

### Reason

Workers can send results while they are processing jobs.

Closing the results channel too early could cause:

```text
panic: send on closed channel
```

Therefore:

```text
Workers Running
      |
      v
Workers Finish
      |
      v
WaitGroup = 0
      |
      v
Close Results
```

This establishes the invariant:

> No worker can send to the results channel after the results channel has been closed.

---

## 10. Use Context for Cancellation

### Decision

Pass `context.Context` to workers.

### Reason

Context provides a standard mechanism for propagating cancellation through Go applications.

Workers can stop waiting for new jobs when the context is cancelled.

### Important Property

Context cancellation is cooperative.

It does not forcibly terminate arbitrary running Go code.

A running task must explicitly observe the context if it needs to terminate early.

---

## 11. Pass Context to Job Tasks

### Decision

The task function has the signature:

```go
func(context.Context) (any, error)
```

### Reason

A task may need to react to cancellation.

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

This allows cancellation to propagate from the application to the worker and finally to the executing task.

---

## 12. Return Errors Through Results

### Decision

Job errors are returned through the `Result` structure.

```go
type Result struct {
    JobID int
    Value any
    Err   error
}
```

### Reason

The worker should report the outcome of the task rather than deciding how the application should handle the error.

The consumer can decide whether to:

- Log the error.
- Retry the job.
- Ignore the failure.
- Return the error to another service.
- Store the failure.

### Trade-off

The current Worker Pool does not automatically retry failed jobs.

---

## 13. No Automatic Retries

### Decision

Failed jobs are not automatically retried.

### Reason

Retries introduce additional design decisions:

- Maximum retry count
- Retryable versus non-retryable errors
- Retry delay
- Exponential backoff
- Jitter
- Dead-letter handling

Adding these policies would increase the scope of the Worker Pool.

Retry mechanisms will be explored in later systems.

---

## 14. No Persistent Job Storage

### Decision

Jobs are stored only in memory.

### Reason

The primary purpose of this project is to learn concurrent job execution using Go.

Persistent queues introduce additional distributed-system concepts such as:

- Durability
- Acknowledgements
- Delivery guarantees
- Recovery
- Persistence
- Consumer state

These concerns belong to the Message Queue project.

---

## 15. No Dynamic Worker Scaling

### Decision

The worker count remains fixed after the pool is created.

### Reason

The initial implementation focuses on understanding the worker-pool pattern and concurrency primitives.

A fixed worker count also makes performance experiments easier to reproduce.

### Future Possibilities

A future implementation could dynamically scale workers based on:

- Queue depth
- CPU utilization
- Request rate
- Worker utilization
- Minimum worker count
- Maximum worker count

---

## 16. No Ordering Guarantee

### Decision

The Worker Pool does not guarantee that results are returned in job-submission order.

### Reason

Multiple workers execute jobs concurrently.

For example:

```text
Job 1 ──> Worker 1 ──> 500 ms
Job 2 ──> Worker 2 ──> 100 ms
```

Job 2 may complete before Job 1.

Therefore, consumers must use `JobID` to associate results with submitted jobs.

### Trade-off

Not guaranteeing ordering allows workers to execute independently and avoids additional coordination overhead.

---

## 17. Bounded Queue for Backpressure

### Decision

The jobs queue has a configurable capacity.

### Reason

An unbounded queue could allow producers to continuously create jobs faster than workers can process them.

This could eventually consume excessive memory.

A bounded queue provides a natural backpressure mechanism:

```text
Producer
   |
   v
Jobs Queue
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

---

## 18. Benchmark Different Worker Counts

### Decision

Benchmark multiple worker counts:

```text
1
2
4
8
16
```

### Reason

There is no universally optimal worker count.

Performance depends on:

- CPU availability
- Number of jobs
- Job characteristics
- Queue size
- Synchronization overhead
- External I/O

Therefore, worker count should be evaluated empirically.

---

## 19. Use Race Detection

### Decision

Use Go's race detector:

```bash
go test -race ./...
```

### Reason

The Worker Pool contains concurrent goroutines and shared communication channels.

The race detector helps identify unsafe concurrent memory access that may not appear during normal test execution.

Race detection is therefore part of the correctness validation process.

---

## 20. Use Benchmarks

### Decision

Benchmark the worker pool using:

```bash
go test -bench=. -benchmem ./benchmarks
```

### Reason

Functional tests establish correctness but do not establish performance.

Benchmarks allow us to measure:

- Execution time
- Bytes allocated per operation
- Allocations per operation
- Effect of worker count
- Scaling behavior

---

## 21. Keep the Worker Pool Focused

### Decision

The Worker Pool does not implement:

- HTTP APIs
- Redis
- Kafka
- Metrics
- Persistent storage
- Distributed scheduling
- Service discovery
- Dead-letter queues
- Distributed coordination

### Reason

The purpose of this project is to establish the concurrency and worker-pool fundamentals first.

Additional distributed-system capabilities will be implemented as separate projects so that each system introduces a distinct concept.

---

## 22. Engineering Principle

The implementation favors:

- Simple concurrency primitives.
- Explicit ownership.
- Bounded resources.
- Clear package responsibilities.
- Measurable performance.
- Explicit lifecycle management.

The design should remain simple until a concrete requirement justifies additional complexity.
