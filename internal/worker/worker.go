package worker

import (
	"sync"

	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/result"
)

type Worker struct {
	ID int
	//<- and -> are to specify to only recieve or only send (directional channels)
	Jobs    <-chan job.Job
	Results chan<- result.Result
	WG      *sync.WaitGroup
}

func New(id int, jobs <-chan job.Job, results chan<- result.Result, wg *sync.WaitGroup) *Worker {
	return &Worker{
		ID:      id,
		Jobs:    jobs,
		Results: results,
		WG:      wg,
	}
}
func (w *Worker) Run() {
	defer w.WG.Done()

	for j := range w.Jobs {
		value, err := j.Task()
		w.Results <- result.Result{
			JobId: j.ID,
			Value: value,
			Err:   err,
		}
	}
}
