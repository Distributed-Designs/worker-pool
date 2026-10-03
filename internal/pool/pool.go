package pool

import (
	"context"
	"sync"

	"github.com/Distributed-Designs/worker-pool/internal/config"
	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/result"
	"github.com/Distributed-Designs/worker-pool/internal/worker"
)

// Pool manages a group of workers.
type Pool struct {
	jobs        chan job.Job
	results     chan result.Result
	workerCount int

	wg sync.WaitGroup
}

// New creates a new worker pool.
func New(cfg config.Config) *Pool {
	return &Pool{
		jobs:        make(chan job.Job, cfg.JobQueueSize),
		results:     make(chan result.Result, cfg.ResultQueueSize),
		workerCount: cfg.WorkerCount,
	}
}

// Start starts all workers.
func (p *Pool) Start(ctx context.Context) {
	for i := 1; i <= p.workerCount; i++ {
		p.wg.Add(1)

		w := worker.New(
			i,
			p.jobs,
			p.results,
			&p.wg,
		)

		go w.Run(ctx)
	}

	go func() {
		p.wg.Wait()
		close(p.results)
	}()
}

// Submit adds a job to the worker pool.
func (p *Pool) Submit(j job.Job) {
	p.jobs <- j
}

// Results returns the result channel.
func (p *Pool) Results() <-chan result.Result {
	return p.results
}

// Stop stops accepting new jobs and allows workers
// to finish jobs already in the queue.
func (p *Pool) Stop() {
	close(p.jobs)
}
