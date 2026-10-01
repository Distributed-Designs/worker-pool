package config

type Config struct {
	WorkerCount     int
	JobQueueSize    int
	ResultQueueSize int
}

func Default() Config {
	return Config{
		WorkerCount:     4,
		JobQueueSize:    100,
		ResultQueueSize: 100,
	}
}
