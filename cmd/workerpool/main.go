package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Distributed-Designs/worker-pool/internal/config"
	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/pool"
)

func main() {
	cfg := config.Default()

	ctx := context.Background()

	p := pool.New(cfg)

	p.Start(ctx)

	for i := 1; i <= 10; i++ {
		id := i

		p.Submit(job.Job{
			ID: id,
			Task: func(ctx context.Context) (any, error) {
				time.Sleep(500 * time.Millisecond)

				return fmt.Sprintf("job %d completed", id), nil
			},
		})
	}

	p.Stop()

	for r := range p.Results() {
		if r.Err != nil {
			fmt.Printf("Job %d failed: %v\n", r.JobId, r.Err)
			continue
		}

		fmt.Printf("Job %d: %v\n", r.JobId, r.Value)
	}
}
