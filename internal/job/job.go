package job

type Job struct {
	ID   int
	Task func() (any, error)
}
