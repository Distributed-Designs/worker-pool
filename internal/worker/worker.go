package worker

import (
	"context"
	"sync"

	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/result"
)

// Worker represents a single worker in the worker pool.
type Worker struct {
	ID      int
	Jobs    <-chan job.Job
	Results chan<- result.Result
	WG      *sync.WaitGroup
}

// New creates a new worker.
func New(
	id int,
	jobs <-chan job.Job,
	results chan<- result.Result,
	wg *sync.WaitGroup,
) *Worker {
	return &Worker{
		ID:      id,
		Jobs:    jobs,
		Results: results,
		WG:      wg,
	}
}

// Run starts the worker.
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
				JobId: j.ID,
				Value: value,
				Err:   err,
			}
		}
	}
}
