package benchmarks

import (
	"context"
	"testing"

	"github.com/Distributed-Designs/worker-pool/internal/config"
	"github.com/Distributed-Designs/worker-pool/internal/job"
	"github.com/Distributed-Designs/worker-pool/internal/pool"
)

func BenchmarkWorkerPool(b *testing.B) {
	workerCounts := []int{
		1,
		2,
		4,
		8,
		16,
	}

	for _, workerCount := range workerCounts {
		b.Run(
			workersName(workerCount),
			func(b *testing.B) {
				cfg := config.Config{
					WorkerCount:     workerCount,
					JobQueueSize:    100,
					ResultQueueSize: 100,
				}

				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					ctx := context.Background()

					p := pool.New(cfg)
					p.Start(ctx)

					for j := 0; j < 100; j++ {
						p.Submit(job.Job{
							ID: j,
							Task: func(ctx context.Context) (any, error) {
								var value int

								for k := 0; k < 1000; k++ {
									value += k
								}

								return value, nil
							},
						})
					}

					p.Stop()

					for range p.Results() {
					}
				}
			},
		)
	}
}

func workersName(count int) string {
	switch count {
	case 1:
		return "1Worker"
	case 2:
		return "2Workers"
	case 4:
		return "4Workers"
	case 8:
		return "8Workers"
	case 16:
		return "16Workers"
	default:
		return "Workers"
	}
}

func BenchmarkWorkerPoolCPUIntensive(b *testing.B) {
	workerCounts := []int{
		1,
		2,
		4,
		8,
		16,
	}

	for _, workerCount := range workerCounts {
		b.Run(
			workersName(workerCount),
			func(b *testing.B) {
				cfg := config.Config{
					WorkerCount:     workerCount,
					JobQueueSize:    100,
					ResultQueueSize: 100,
				}

				b.ReportAllocs()

				for i := 0; i < b.N; i++ {
					ctx := context.Background()

					p := pool.New(cfg)
					p.Start(ctx)

					for j := 0; j < 100; j++ {
						p.Submit(job.Job{
							ID: j,
							Task: func(ctx context.Context) (any, error) {
								var value uint64

								for k := uint64(0); k < 1_000_000; k++ {
									value += k
								}

								return value, nil
							},
						})
					}

					p.Stop()

					for range p.Results() {
					}
				}
			},
		)
	}
}
