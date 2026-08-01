package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go-agent-harness/internal/checkpoints"
	htools "go-agent-harness/internal/harness/tools"
)

type checkpointApprovalBroker struct {
	service *checkpoints.Service
}

func NewCheckpointApprovalBroker(service *checkpoints.Service) ApprovalBroker {
	return &checkpointApprovalBroker{service: service}
}

func (b *checkpointApprovalBroker) Register(ctx context.Context, req ApprovalRequest) (ApprovalWaiter, error) {
	if req.Timeout <= 0 {
		req.Timeout = 5 * time.Minute
	}
	// Options presented to the operator (plan approach options) ride in the
	// record's Questions field, which is otherwise unused for KindApproval.
	var options string
	if len(req.Options) > 0 {
		raw, err := json.Marshal(req.Options)
		if err != nil {
			return nil, fmt.Errorf("marshal approval options: %w", err)
		}
		options = string(raw)
	}
	record, err := b.service.Create(ctx, checkpoints.CreateRequest{
		Kind:       checkpoints.KindApproval,
		RunID:      req.RunID,
		CallID:     req.CallID,
		Tool:       req.Tool,
		Args:       req.Args,
		Questions:  options,
		DeadlineAt: time.Now().UTC().Add(req.Timeout),
	})
	if err != nil {
		return nil, err
	}
	return &checkpointApprovalWaiter{broker: b, req: req, record: record}, nil
}

// Ask retains the direct register-and-wait lifecycle for callers that do not
// need to publish an approval event between those phases.
func (b *checkpointApprovalBroker) Ask(ctx context.Context, req ApprovalRequest) (bool, string, error) {
	waiter, err := b.Register(ctx, req)
	if err != nil {
		return false, "", err
	}
	return waiter.Wait(ctx)
}

type checkpointApprovalWaiter struct {
	broker *checkpointApprovalBroker
	req    ApprovalRequest
	record checkpoints.Record
}

func (w *checkpointApprovalWaiter) Pending() PendingApproval {
	var options []PlanApproachOption
	if w.record.Questions != "" {
		if err := json.Unmarshal([]byte(w.record.Questions), &options); err != nil {
			options = nil
		}
	}
	return PendingApproval{
		RunID:      w.record.RunID,
		CallID:     w.record.CallID,
		Tool:       w.record.Tool,
		Args:       w.record.Args,
		DeadlineAt: w.record.DeadlineAt,
		Options:    options,
	}
}

func (w *checkpointApprovalWaiter) Wait(ctx context.Context) (bool, string, error) {
	remaining := time.Until(w.record.DeadlineAt)
	if remaining <= 0 {
		return w.resolveOrExpire()
	}

	waitCtx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()

	result, err := w.broker.service.Wait(waitCtx, w.record.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return w.resolveOrExpire()
		}
		return false, "", err
	}
	return w.waitResult(result)
}

// resolveOrExpire uses the checkpoint service's atomic pending-resolution
// primitive. Exactly one of approval/denial or expiry wins; a committed
// decision is loaded and returned instead of being overwritten by expiry.
func (w *checkpointApprovalWaiter) resolveOrExpire() (bool, string, error) {
	expired, err := w.broker.service.ExpirePending(context.Background(), w.record.ID)
	if err != nil {
		return false, "", err
	}
	if expired {
		return false, "", w.timeoutError()
	}
	result, err := w.broker.service.Wait(context.Background(), w.record.ID)
	if err != nil {
		return false, "", err
	}
	return w.waitResult(result)
}

func (w *checkpointApprovalWaiter) waitResult(result checkpoints.WaitResult) (bool, string, error) {
	if result.Status == checkpoints.StatusExpired {
		return false, "", w.timeoutError()
	}
	var option string
	if result.Status == checkpoints.StatusApproved {
		option, _ = result.Payload["option"].(string)
	}
	return result.Status == checkpoints.StatusApproved, option, nil
}

func (w *checkpointApprovalWaiter) timeoutError() error {
	return &ApprovalTimeoutError{
		RunID:      w.req.RunID,
		CallID:     w.req.CallID,
		DeadlineAt: w.record.DeadlineAt,
	}
}

func (b *checkpointApprovalBroker) Pending(runID string) (PendingApproval, bool) {
	record, ok, err := b.service.PendingByRun(context.Background(), runID)
	if err != nil || !ok || record.Kind != checkpoints.KindApproval {
		return PendingApproval{}, false
	}
	var options []PlanApproachOption
	if record.Questions != "" {
		if err := json.Unmarshal([]byte(record.Questions), &options); err != nil {
			options = nil
		}
	}
	return PendingApproval{
		RunID:      record.RunID,
		CallID:     record.CallID,
		Tool:       record.Tool,
		Args:       record.Args,
		DeadlineAt: record.DeadlineAt,
		Options:    options,
	}, true
}

func (b *checkpointApprovalBroker) Approve(runID string) error {
	return b.ApproveWithOption(runID, "")
}

func (b *checkpointApprovalBroker) ApproveWithOption(runID, option string) error {
	record, ok, err := b.service.PendingByRun(context.Background(), runID)
	if err != nil {
		return err
	}
	if !ok || record.Kind != checkpoints.KindApproval {
		return ErrNoPendingApproval
	}
	if option == "" {
		return mapCheckpointResolutionError(
			b.service.Approve(context.Background(), record.ID),
			ErrNoPendingApproval,
		)
	}
	return mapCheckpointResolutionError(
		b.service.ApproveWithPayload(context.Background(), record.ID, map[string]any{"option": option}),
		ErrNoPendingApproval,
	)
}

func (b *checkpointApprovalBroker) Deny(runID string) error {
	record, ok, err := b.service.PendingByRun(context.Background(), runID)
	if err != nil {
		return err
	}
	if !ok || record.Kind != checkpoints.KindApproval {
		return ErrNoPendingApproval
	}
	return mapCheckpointResolutionError(
		b.service.Deny(context.Background(), record.ID),
		ErrNoPendingApproval,
	)
}

func mapCheckpointResolutionError(err, noPending error) error {
	if errors.Is(err, checkpoints.ErrAlreadyResolved) {
		return noPending
	}
	return err
}

type checkpointAskUserQuestionBroker struct {
	service *checkpoints.Service
	now     func() time.Time
}

func NewCheckpointAskUserQuestionBroker(service *checkpoints.Service, now func() time.Time) htools.AskUserQuestionBroker {
	if now == nil {
		now = time.Now
	}
	return &checkpointAskUserQuestionBroker{service: service, now: now}
}

func (b *checkpointAskUserQuestionBroker) Ask(ctx context.Context, req htools.AskUserQuestionRequest) (map[string]string, time.Time, error) {
	if err := htools.ValidateAskUserQuestions(req.Questions); err != nil {
		return nil, time.Time{}, err
	}
	if req.Timeout <= 0 {
		req.Timeout = 5 * time.Minute
	}
	rawQuestions, err := json.Marshal(req.Questions)
	if err != nil {
		return nil, time.Time{}, err
	}
	record, err := b.service.Create(ctx, checkpoints.CreateRequest{
		Kind:       checkpoints.KindUserInput,
		RunID:      req.RunID,
		CallID:     req.CallID,
		Questions:  string(rawQuestions),
		DeadlineAt: b.now().UTC().Add(req.Timeout),
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	waitCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	if req.OnPending != nil {
		notified := make(chan struct{})
		go func() {
			defer close(notified)
			req.OnPending(
				waitCtx,
				htools.AskUserQuestionPending{
					RunID:      record.RunID,
					CallID:     record.CallID,
					Tool:       htools.AskUserQuestionToolName,
					Questions:  req.Questions,
					DeadlineAt: record.DeadlineAt,
				},
			)
		}()
		select {
		case <-notified:
		case <-waitCtx.Done():
			answers, answeredAt, err := b.finishAskWait(ctx, req, record)
			if err != nil {
				return nil, time.Time{}, err
			}
			if err := waitForPendingPublication(ctx, notified); err != nil {
				return nil, time.Time{}, err
			}
			return answers, answeredAt, nil
		}
	}

	result, err := b.service.Wait(waitCtx, record.ID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return b.finishAskWait(ctx, req, record)
		}
		return nil, time.Time{}, err
	}
	return askUserAnswers(result), b.now().UTC(), nil
}

func (b *checkpointAskUserQuestionBroker) finishAskWait(
	ctx context.Context,
	req htools.AskUserQuestionRequest,
	record checkpoints.Record,
) (map[string]string, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	expired, err := b.service.ExpirePending(context.Background(), record.ID)
	if err != nil {
		return nil, time.Time{}, err
	}
	if !expired {
		result, waitErr := b.service.Wait(context.Background(), record.ID)
		if waitErr != nil {
			return nil, time.Time{}, waitErr
		}
		if result.Status == checkpoints.StatusResumed {
			return askUserAnswers(result), b.now().UTC(), nil
		}
	}
	return nil, time.Time{}, &htools.AskUserQuestionTimeoutError{
		RunID:      req.RunID,
		CallID:     req.CallID,
		DeadlineAt: record.DeadlineAt,
	}
}

func askUserAnswers(result checkpoints.WaitResult) map[string]string {
	answers := make(map[string]string, len(result.Payload))
	for key, value := range result.Payload {
		if str, ok := value.(string); ok {
			answers[key] = str
		}
	}
	return answers
}

func (b *checkpointAskUserQuestionBroker) Pending(runID string) (htools.AskUserQuestionPending, bool) {
	record, ok, err := b.service.PendingByRun(context.Background(), runID)
	if err != nil || !ok || record.Kind != checkpoints.KindUserInput {
		return htools.AskUserQuestionPending{}, false
	}
	questions, err := decodeQuestions(record.Questions)
	if err != nil {
		return htools.AskUserQuestionPending{}, false
	}
	return htools.AskUserQuestionPending{
		RunID:      record.RunID,
		CallID:     record.CallID,
		Tool:       htools.AskUserQuestionToolName,
		Questions:  questions,
		DeadlineAt: record.DeadlineAt,
	}, true
}

func (b *checkpointAskUserQuestionBroker) Submit(runID string, answers map[string]string) error {
	record, ok, err := b.service.PendingByRun(context.Background(), runID)
	if err != nil {
		return err
	}
	if !ok || record.Kind != checkpoints.KindUserInput {
		return ErrNoPendingUserQuestion
	}
	questions, err := decodeQuestions(record.Questions)
	if err != nil {
		return err
	}
	normalized, err := htools.NormalizeAskUserAnswers(questions, answers)
	if err != nil {
		return ErrInvalidUserQuestionInput
	}
	payload := make(map[string]any, len(normalized))
	for key, value := range normalized {
		payload[key] = value
	}
	return mapCheckpointResolutionError(
		b.service.Resume(context.Background(), record.ID, payload),
		ErrNoPendingUserQuestion,
	)
}

func decodeQuestions(raw string) ([]htools.AskUserQuestion, error) {
	var questions []htools.AskUserQuestion
	if err := json.Unmarshal([]byte(raw), &questions); err != nil {
		return nil, err
	}
	return questions, nil
}
