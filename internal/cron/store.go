package cron

import "context"

// Store is the persistence interface for cron jobs and executions.
type Store interface {
	Migrate(ctx context.Context) error

	CreateJob(ctx context.Context, job Job) (Job, error)
	GetJob(ctx context.Context, id string) (Job, error)
	GetJobByName(ctx context.Context, name string) (Job, error)
	ListJobs(ctx context.Context) ([]Job, error)
	UpdateJob(ctx context.Context, job Job) error
	DeleteJob(ctx context.Context, id string) error // soft delete

	CreateExecution(ctx context.Context, exec Execution) (Execution, error)
	UpdateExecution(ctx context.Context, exec Execution) error
	ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]Execution, error)

	Close() error
}
