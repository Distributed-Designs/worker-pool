package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Distributed-Designs/worker-pool/internal/config"
	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/pool"
)

func TestWorkerPoolProcessesJobs(t *testing.T) {
	cfg := config.Config{
		WorkerCount:     3,
		JobQueueSize:    10,
		ResultQueueSize: 10,
	}

	p := pool.New(cfg)
	p.Start(context.Background())

	const jobCount = 6

	for i := 1; i <= jobCount; i++ {
		id := i

		p.Submit(job.Job{
			ID: id,
			Task: func(ctx context.Context) (any, error) {
				return id * 2, nil
			},
		})
	}

	p.Stop()

	results := make(map[int]int)

	for r := range p.Results() {
		if r.Err != nil {
			t.Fatalf("job %d returned an unexpected error: %v", r.JobId, r.Err)
		}

		value, ok := r.Value.(int)
		if !ok {
			t.Fatalf("job %d returned unexpected value type", r.JobId)
		}

		results[r.JobId] = value
	}

	if len(results) != jobCount {
		t.Fatalf(
			"expected %d results, got %d",
			jobCount,
			len(results),
		)
	}

	for id := 1; id <= jobCount; id++ {
		expected := id * 2

		if results[id] != expected {
			t.Fatalf(
				"job %d: expected %d, got %d",
				id,
				expected,
				results[id],
			)
		}
	}
}

func TestWorkerPoolReturnsJobErrors(t *testing.T) {
	cfg := config.Config{
		WorkerCount:     2,
		JobQueueSize:    10,
		ResultQueueSize: 10,
	}

	p := pool.New(cfg)
	p.Start(context.Background())

	expectedErr := errors.New("job failed")

	p.Submit(job.Job{
		ID: 1,
		Task: func(ctx context.Context) (any, error) {
			return nil, expectedErr
		},
	})

	p.Stop()

	var received bool

	for r := range p.Results() {
		if r.JobId != 1 {
			continue
		}

		received = true

		if !errors.Is(r.Err, expectedErr) {
			t.Fatalf(
				"expected error %v, got %v",
				expectedErr,
				r.Err,
			)
		}
	}

	if !received {
		t.Fatal("expected result for job 1")
	}
}

func TestWorkerPoolRunsJobsConcurrently(t *testing.T) {
	cfg := config.Config{
		WorkerCount:     4,
		JobQueueSize:    10,
		ResultQueueSize: 10,
	}

	p := pool.New(cfg)
	p.Start(context.Background())

	const jobCount = 4
	const jobDuration = 100 * time.Millisecond

	start := time.Now()

	for i := 1; i <= jobCount; i++ {
		p.Submit(job.Job{
			ID: i,
			Task: func(ctx context.Context) (any, error) {
				time.Sleep(jobDuration)
				return nil, nil
			},
		})
	}

	p.Stop()

	for range p.Results() {
	}

	elapsed := time.Since(start)

	if elapsed >= 300*time.Millisecond {
		t.Fatalf(
			"jobs appear to be running sequentially: elapsed=%v",
			elapsed,
		)
	}
}
func TestWorkerPoolCancellation(t *testing.T) {
	cfg := config.Config{
		WorkerCount:     2,
		JobQueueSize:    10,
		ResultQueueSize: 10,
	}

	ctx, cancel := context.WithCancel(context.Background())

	p := pool.New(cfg)
	p.Start(ctx)

	p.Submit(job.Job{
		ID: 1,
		Task: func(ctx context.Context) (any, error) {
			select {
			case <-time.After(5 * time.Second):
				return "completed", nil

			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})

	cancel()

	p.Stop()

	for range p.Results() {
	}
}
