package deferred

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go-agent-harness/internal/cron"
	tools "go-agent-harness/internal/harness/tools"
)

// NewScopedCronClient confines every model-facing cron operation to the
// immutable tenant, conversation, and agent scope carried by RunMetadata.
// The wrapper belongs at the tool boundary so embedded and remote cron clients
// enforce the same ownership contract without changing their operator APIs.
func NewScopedCronClient(client tools.CronClient) tools.CronClient {
	if _, alreadyScoped := client.(*scopedCronClient); alreadyScoped {
		return client
	}
	return &scopedCronClient{client: client}
}

func scopedCronContext(ctx context.Context, metadata tools.RunMetadata) context.Context {
	return cron.WithScope(ctx, cron.Scope{TenantID: metadata.TenantID, ConversationID: metadata.ConversationID, AgentID: metadata.AgentID})
}

type scopedCronClient struct {
	client tools.CronClient
}

func cronScopeFromContext(ctx context.Context) (tools.RunMetadata, error) {
	metadata, ok := tools.RunMetadataFromContext(ctx)
	if !ok ||
		strings.TrimSpace(metadata.TenantID) == "" ||
		strings.TrimSpace(metadata.ConversationID) == "" ||
		strings.TrimSpace(metadata.AgentID) == "" {
		return tools.RunMetadata{}, fmt.Errorf("cron scope is required")
	}
	metadata.TenantID = strings.TrimSpace(metadata.TenantID)
	metadata.ConversationID = strings.TrimSpace(metadata.ConversationID)
	metadata.AgentID = strings.TrimSpace(metadata.AgentID)
	return metadata, nil
}

func cronJobInScope(job tools.CronJob, metadata tools.RunMetadata) bool {
	return job.TenantID == metadata.TenantID &&
		job.ConversationID == metadata.ConversationID &&
		job.AgentID == metadata.AgentID
}

func (c *scopedCronClient) CreateJob(ctx context.Context, req tools.CronCreateJobRequest) (tools.CronJob, error) {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return tools.CronJob{}, err
	}
	if strings.TrimSpace(req.TenantID) != metadata.TenantID ||
		strings.TrimSpace(req.ConversationID) != metadata.ConversationID ||
		strings.TrimSpace(req.AgentID) != metadata.AgentID {
		return tools.CronJob{}, fmt.Errorf("cron create scope does not match the active run")
	}
	return c.client.CreateJob(scopedCronContext(ctx, metadata), req)
}

func (c *scopedCronClient) ListJobs(ctx context.Context) ([]tools.CronJob, error) {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := c.client.ListJobs(scopedCronContext(ctx, metadata))
	if err != nil {
		return nil, err
	}
	filtered := make([]tools.CronJob, 0, len(jobs))
	for _, job := range jobs {
		if cronJobInScope(job, metadata) {
			filtered = append(filtered, job)
		}
	}
	return filtered, nil
}

func (c *scopedCronClient) GetJob(ctx context.Context, id string) (tools.CronJob, error) {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return tools.CronJob{}, err
	}
	job, err := c.client.GetJob(scopedCronContext(ctx, metadata), id)
	if err != nil {
		return tools.CronJob{}, err
	}
	if !cronJobInScope(job, metadata) {
		return tools.CronJob{}, tools.ErrCronJobNotFound
	}
	return job, nil
}

func (c *scopedCronClient) UpdateJob(ctx context.Context, id string, req tools.CronUpdateJobRequest) (tools.CronJob, error) {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return tools.CronJob{}, err
	}
	if _, err := c.GetJob(ctx, id); err != nil {
		return tools.CronJob{}, err
	}
	return c.client.UpdateJob(scopedCronContext(ctx, metadata), id, req)
}

func (c *scopedCronClient) DeleteJob(ctx context.Context, id string) error {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return err
	}
	if _, err := c.GetJob(ctx, id); err != nil {
		return err
	}
	return c.client.DeleteJob(scopedCronContext(ctx, metadata), id)
}

func (c *scopedCronClient) DeleteJobCAS(ctx context.Context, id string, expectedUpdatedAt time.Time) error {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return err
	}
	if _, err := c.GetJob(ctx, id); err != nil {
		return err
	}
	return c.client.DeleteJobCAS(scopedCronContext(ctx, metadata), id, expectedUpdatedAt)
}

func (c *scopedCronClient) ListExecutions(ctx context.Context, jobID string, limit, offset int) ([]tools.CronExecution, error) {
	metadata, err := cronScopeFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := c.GetJob(ctx, jobID); err != nil {
		return nil, err
	}
	return c.client.ListExecutions(scopedCronContext(ctx, metadata), jobID, limit, offset)
}

func (c *scopedCronClient) Health(ctx context.Context) error {
	return c.client.Health(ctx)
}
