package job

import "context"

// Job represents a unit of work that can be executed by a worker.
type Job struct {
	ID   int
	Task func(context.Context) (any, error)
}
