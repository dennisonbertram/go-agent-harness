package observationalmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

type Manager interface {
	Close() error
	Mode() Mode
	Status(ctx context.Context, key ScopeKey) (Status, error)
	SetEnabled(ctx context.Context, key ScopeKey, enabled bool, cfg *Config, runID, toolCallID string) (Status, error)
	Observe(ctx context.Context, req ObserveRequest) (ObserveResult, error)
	Snippet(ctx context.Context, key ScopeKey) (string, Status, error)
	ReflectNow(ctx context.Context, key ScopeKey, runID, toolCallID string) (Status, error)
	Export(ctx context.Context, key ScopeKey, format string) (ExportResult, error)
}

type ServiceOptions struct {
	Mode                 Mode
	Store                Store
	Coordinator          Coordinator
	Observer             Observer
	Reflector            Reflector
	Estimator            TokenEstimator
	DefaultConfig        Config
	DefaultEnabled       bool
	Now                  func() time.Time
	StaleProcessingAfter time.Duration
}

type service struct {
	mode           Mode
	store          Store
	coordinator    Coordinator
	observer       Observer
	reflector      Reflector
	estimator      TokenEstimator
	defaultConfig  Config
	defaultEnabled bool
	now            func() time.Time
}

var opSeq uint64

func NewService(opts ServiceOptions) (Manager, error) {
	mode := opts.Mode
	if mode == "" {
		mode = ModeAuto
	}
	if mode == ModeOff {
		return NewDisabledManager(mode), nil
	}
	if mode == ModeAuto {
		mode = ModeLocalCoordinator
	}
	if mode != ModeLocalCoordinator {
		return nil, fmt.Errorf("unsupported memory mode %q", mode)
	}
	if opts.Store == nil {
		return nil, fmt.Errorf("memory store is required")
	}
	if opts.Coordinator == nil {
		opts.Coordinator = NewLocalCoordinator()
	}
	if opts.Estimator == nil {
		opts.Estimator = RuneTokenEstimator{}
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.DefaultConfig.ObserveMinTokens <= 0 || opts.DefaultConfig.SnippetMaxTokens <= 0 || opts.DefaultConfig.ReflectThresholdTokens <= 0 {
		opts.DefaultConfig = DefaultConfig()
	}
	if opts.StaleProcessingAfter <= 0 {
		opts.StaleProcessingAfter = 5 * time.Minute
	}

	ctx := context.Background()
	if err := opts.Store.Migrate(ctx); err != nil {
		return nil, err
	}
	if err := opts.Store.ResetStaleOperations(ctx, opts.Now().UTC().Add(-opts.StaleProcessingAfter)); err != nil {
		return nil, err
	}

	return &service{
		mode:           mode,
		store:          opts.Store,
		coordinator:    opts.Coordinator,
		observer:       opts.Observer,
		reflector:      opts.Reflector,
		estimator:      opts.Estimator,
		defaultConfig:  opts.DefaultConfig,
		defaultEnabled: opts.DefaultEnabled,
		now:            opts.Now,
	}, nil
}

func NewDisabledManager(mode Mode) Manager {
	if mode == "" {
		mode = ModeOff
	}
	return disabledManager{mode: mode}
}

func (m *service) Close() error {
	return m.store.Close()
}

func (m *service) Mode() Mode {
	return m.mode
}

func (m *service) Status(ctx context.Context, key ScopeKey) (Status, error) {
	rec, err := m.store.GetOrCreateRecord(ctx, normalizeScope(key), m.defaultEnabled, normalizeConfig(m.defaultConfig), m.now().UTC())
	if err != nil {
		return Status{}, err
	}
	rec.Config = normalizeConfig(rec.Config)
	return statusFromRecord(m.mode, rec), nil
}

func (m *service) SetEnabled(ctx context.Context, key ScopeKey, enabled bool, cfg *Config, runID, toolCallID string) (Status, error) {
	scope := normalizeScope(key)
	opType := "disable"
	if enabled {
		opType = "enable"
	}
	status, err := m.executeMutation(ctx, scope, runID, toolCallID, opType, map[string]any{"enabled": enabled}, func(ctx context.Context, rec *Record, op Operation) error {
		rec.Enabled = enabled
		if cfg != nil {
			rec.Config = mergeConfig(rec.Config, *cfg)
		}
		rec.StateVersion++
		rec.UpdatedAt = m.now().UTC()
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return status, nil
}

func (m *service) Observe(ctx context.Context, req ObserveRequest) (ObserveResult, error) {
	scope := normalizeScope(req.Scope)
	rec, err := m.store.GetOrCreateRecord(ctx, scope, m.defaultEnabled, normalizeConfig(m.defaultConfig), m.now().UTC())
	if err != nil {
		return ObserveResult{}, err
	}
	rec.Config = normalizeConfig(rec.Config)
	status := statusFromRecord(m.mode, rec)
	if !rec.Enabled {
		return ObserveResult{Status: status}, nil
	}
	if len(req.Messages) == 0 {
		return ObserveResult{Status: status}, nil
	}
	lastIndex := int64(len(req.Messages) - 1)
	if rec.LastObservedMessageIndex >= lastIndex {
		return ObserveResult{Status: status}, nil
	}
	start := rec.LastObservedMessageIndex + 1
	if start < 0 {
		start = 0
	}
	unobserved := req.Messages[start:]
	unobservedTokens := m.estimator.EstimateMessagesTokens(unobserved)
	if unobservedTokens < rec.Config.ObserveMinTokens {
		return ObserveResult{Status: status}, nil
	}

	observed := false
	reflected := false
	status, err = m.executeMutation(ctx, scope, req.RunID, req.ToolCallID, "observe", map[string]any{"messages": len(unobserved)}, func(ctx context.Context, rec *Record, op Operation) error {
		if !rec.Enabled {
			return nil
		}
		rec.Config = normalizeConfig(rec.Config)
		start := rec.LastObservedMessageIndex + 1
		if start < 0 {
			start = 0
		}
		if start >= int64(len(req.Messages)) {
			return nil
		}
		unobserved := req.Messages[start:]
		tokens := m.estimator.EstimateMessagesTokens(unobserved)
		if tokens < rec.Config.ObserveMinTokens {
			return nil
		}
		if m.observer == nil {
			return fmt.Errorf("observer is not configured")
		}
		text, err := m.observer.Observe(ctx, scope, rec.Config, unobserved, rec.ActiveObservations, rec.ActiveReflection)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return fmt.Errorf("observer returned empty output")
		}
		chunk := ObservationChunk{
			Seq:              int64(len(rec.ActiveObservations) + 1),
			Content:          text,
			TokenCount:       m.estimator.EstimateTextTokens(text),
			CreatedAt:        m.now().UTC(),
			SourceStartIndex: start,
			SourceEndIndex:   int64(len(req.Messages) - 1),
		}
		rec.ActiveObservations = append(rec.ActiveObservations, chunk)
		rec.ActiveObservationTokens += chunk.TokenCount
		rec.LastObservedMessageIndex = chunk.SourceEndIndex
		rec.StateVersion++
		rec.UpdatedAt = m.now().UTC()
		observed = true

		if err := m.insertMarker(ctx, Marker{
			MarkerID:          nextID("mk"),
			MemoryID:          rec.MemoryID,
			MarkerType:        "observation_start",
			CycleID:           op.OperationID,
			MessageIndexStart: chunk.SourceStartIndex,
			MessageIndexEnd:   chunk.SourceStartIndex,
			TokenCount:        chunk.TokenCount,
			PayloadJSON:       markerPayloadJSON(map[string]any{"seq": chunk.Seq}),
			CreatedAt:         m.now().UTC(),
		}); err != nil {
			return err
		}
		if err := m.insertMarker(ctx, Marker{
			MarkerID:          nextID("mk"),
			MemoryID:          rec.MemoryID,
			MarkerType:        "observation_end",
			CycleID:           op.OperationID,
			MessageIndexStart: chunk.SourceEndIndex,
			MessageIndexEnd:   chunk.SourceEndIndex,
			TokenCount:        chunk.TokenCount,
			PayloadJSON:       markerPayloadJSON(map[string]any{"seq": chunk.Seq}),
			CreatedAt:         m.now().UTC(),
		}); err != nil {
			return err
		}

		if rec.Config.ReflectThresholdTokens > 0 && rec.ActiveObservationTokens >= rec.Config.ReflectThresholdTokens && m.reflector != nil {
			reflection, err := m.reflector.Reflect(ctx, scope, rec.Config, rec.ActiveObservations, rec.ActiveReflection)
			if err != nil {
				return err
			}
			reflection = strings.TrimSpace(reflection)
			if reflection != "" {
				rec.ActiveReflection = reflection
				rec.ActiveReflectionTokens = m.estimator.EstimateTextTokens(reflection)
				rec.LastReflectedObservationSeq = chunk.Seq
				rec.StateVersion++
				rec.UpdatedAt = m.now().UTC()
				reflected = true
				if err := m.insertMarker(ctx, Marker{
					MarkerID:          nextID("mk"),
					MemoryID:          rec.MemoryID,
					MarkerType:        "reflection_end",
					CycleID:           op.OperationID,
					MessageIndexStart: chunk.SourceStartIndex,
					MessageIndexEnd:   chunk.SourceEndIndex,
					TokenCount:        rec.ActiveReflectionTokens,
					PayloadJSON:       markerPayloadJSON(map[string]any{"last_reflected_seq": rec.LastReflectedObservationSeq}),
					CreatedAt:         m.now().UTC(),
				}); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return ObserveResult{}, err
	}
	return ObserveResult{Status: status, Observed: observed, Reflected: reflected}, nil
}

func (m *service) Snippet(ctx context.Context, key ScopeKey) (string, Status, error) {
	rec, err := m.store.GetOrCreateRecord(ctx, normalizeScope(key), m.defaultEnabled, normalizeConfig(m.defaultConfig), m.now().UTC())
	if err != nil {
		return "", Status{}, err
	}
	rec.Config = normalizeConfig(rec.Config)
	status := statusFromRecord(m.mode, rec)
	if !rec.Enabled {
		return "", status, nil
	}
	if rec.ActiveReflection == "" && len(rec.ActiveObservations) == 0 {
		return "", status, nil
	}

	limit := rec.Config.SnippetMaxTokens
	if limit <= 0 {
		limit = DefaultConfig().SnippetMaxTokens
	}
	used := 0
	sections := make([]string, 0, 2)
	if rec.ActiveReflection != "" {
		sections = append(sections, "Reflection:\n"+strings.TrimSpace(rec.ActiveReflection))
		used += rec.ActiveReflectionTokens
	}

	selected := make([]ObservationChunk, 0, len(rec.ActiveObservations))
	for i := len(rec.ActiveObservations) - 1; i >= 0; i-- {
		chunk := rec.ActiveObservations[i]
		if used+chunk.TokenCount > limit {
			break
		}
		selected = append(selected, chunk)
		used += chunk.TokenCount
	}
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Seq < selected[j].Seq })
	if len(selected) > 0 {
		var b strings.Builder
		b.WriteString("Observations:\n")
		for _, chunk := range selected {
			b.WriteString(fmt.Sprintf("- [%d] %s\n", chunk.Seq, strings.TrimSpace(chunk.Content)))
		}
		sections = append(sections, strings.TrimSpace(b.String()))
	}
	if len(sections) == 0 {
		return "", status, nil
	}
	snippet := "<observational-memory>\n" + strings.Join(sections, "\n\n") + "\n</observational-memory>"
	return snippet, status, nil
}

func (m *service) ReflectNow(ctx context.Context, key ScopeKey, runID, toolCallID string) (Status, error) {
	scope := normalizeScope(key)
	status, err := m.executeMutation(ctx, scope, runID, toolCallID, "reflect_now", map[string]any{}, func(ctx context.Context, rec *Record, op Operation) error {
		if !rec.Enabled {
			return nil
		}
		rec.Config = normalizeConfig(rec.Config)
		if len(rec.ActiveObservations) == 0 {
			return nil
		}
		if m.reflector == nil {
			return fmt.Errorf("reflector is not configured")
		}
		reflection, err := m.reflector.Reflect(ctx, scope, rec.Config, rec.ActiveObservations, rec.ActiveReflection)
		if err != nil {
			return err
		}
		reflection = strings.TrimSpace(reflection)
		if reflection == "" {
			return fmt.Errorf("reflector returned empty output")
		}
		rec.ActiveReflection = reflection
		rec.ActiveReflectionTokens = m.estimator.EstimateTextTokens(reflection)
		rec.LastReflectedObservationSeq = int64(len(rec.ActiveObservations))
		rec.StateVersion++
		rec.UpdatedAt = m.now().UTC()
		return m.insertMarker(ctx, Marker{
			MarkerID:          nextID("mk"),
			MemoryID:          rec.MemoryID,
			MarkerType:        "reflection_end",
			CycleID:           op.OperationID,
			MessageIndexStart: rec.LastObservedMessageIndex,
			MessageIndexEnd:   rec.LastObservedMessageIndex,
			TokenCount:        rec.ActiveReflectionTokens,
			PayloadJSON:       markerPayloadJSON(map[string]any{"last_reflected_seq": rec.LastReflectedObservationSeq}),
			CreatedAt:         m.now().UTC(),
		})
	})
	if err != nil {
		return Status{}, err
	}
	return status, nil
}

func (m *service) Export(ctx context.Context, key ScopeKey, format string) (ExportResult, error) {
	rec, err := m.store.GetOrCreateRecord(ctx, normalizeScope(key), m.defaultEnabled, normalizeConfig(m.defaultConfig), m.now().UTC())
	if err != nil {
		return ExportResult{}, err
	}
	rec.Config = normalizeConfig(rec.Config)
	status := statusFromRecord(m.mode, rec)
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = "json"
	}
	if format != "json" && format != "markdown" {
		return ExportResult{}, fmt.Errorf("unsupported export format %q", format)
	}

	var content string
	if format == "json" {
		payload := map[string]any{
			"status":       status,
			"config":       rec.Config,
			"reflection":   rec.ActiveReflection,
			"observations": rec.ActiveObservations,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return ExportResult{}, fmt.Errorf("marshal export json: %w", err)
		}
		content = string(data)
	} else {
		var b strings.Builder
		b.WriteString("# Observational Memory Export\n\n")
		b.WriteString(fmt.Sprintf("- tenant_id: `%s`\n", rec.Scope.TenantID))
		b.WriteString(fmt.Sprintf("- conversation_id: `%s`\n", rec.Scope.ConversationID))
		b.WriteString(fmt.Sprintf("- agent_id: `%s`\n", rec.Scope.AgentID))
		b.WriteString(fmt.Sprintf("- enabled: `%t`\n", rec.Enabled))
		b.WriteString(fmt.Sprintf("- observations: `%d`\n", len(rec.ActiveObservations)))
		b.WriteString(fmt.Sprintf("- updated_at: `%s`\n\n", rec.UpdatedAt.UTC().Format(time.RFC3339)))
		if rec.ActiveReflection != "" {
			b.WriteString("## Reflection\n\n")
			b.WriteString(strings.TrimSpace(rec.ActiveReflection))
			b.WriteString("\n\n")
		}
		b.WriteString("## Observations\n\n")
		if len(rec.ActiveObservations) == 0 {
			b.WriteString("- (none)\n")
		} else {
			for _, chunk := range rec.ActiveObservations {
				b.WriteString(fmt.Sprintf("- [%d] %s\n", chunk.Seq, strings.TrimSpace(chunk.Content)))
			}
		}
		content = b.String()
	}

	return ExportResult{
		Format:  format,
		Content: content,
		Bytes:   len(content),
		Status:  status,
	}, nil
}

func (m *service) executeMutation(ctx context.Context, key ScopeKey, runID, toolCallID, opType string, payload map[string]any, mutate func(context.Context, *Record, Operation) error) (Status, error) {
	var status Status
	err := m.coordinator.WithinScope(ctx, key, func(ctx context.Context) error {
		rec, err := m.store.GetOrCreateRecord(ctx, key, m.defaultEnabled, normalizeConfig(m.defaultConfig), m.now().UTC())
		if err != nil {
			return err
		}
		rec.Config = normalizeConfig(rec.Config)
		op, err := m.createOperation(ctx, rec.MemoryID, runID, toolCallID, opType, payload)
		if err != nil {
			return err
		}
		if err := m.store.UpdateOperationStatus(ctx, op.OperationID, "processing", "", m.now().UTC()); err != nil {
			return err
		}
		if err := mutate(ctx, &rec, op); err != nil {
			_ = m.store.UpdateOperationStatus(ctx, op.OperationID, "failed", err.Error(), m.now().UTC())
			return err
		}
		if err := m.store.UpdateRecord(ctx, rec); err != nil {
			_ = m.store.UpdateOperationStatus(ctx, op.OperationID, "failed", err.Error(), m.now().UTC())
			return err
		}
		if err := m.store.UpdateOperationStatus(ctx, op.OperationID, "applied", "", m.now().UTC()); err != nil {
			return err
		}
		status = statusFromRecord(m.mode, rec)
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return status, nil
}

func (m *service) createOperation(ctx context.Context, memoryID, runID, toolCallID, opType string, payload map[string]any) (Operation, error) {
	if runID == "" {
		runID = "unknown"
	}
	if toolCallID == "" {
		toolCallID = "system"
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Operation{}, fmt.Errorf("marshal operation payload: %w", err)
	}
	now := m.now().UTC()
	op := Operation{
		OperationID:   nextID("op"),
		MemoryID:      memoryID,
		RunID:         runID,
		ToolCallID:    toolCallID,
		OperationType: opType,
		Status:        "queued",
		PayloadJSON:   string(payloadJSON),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return m.store.CreateOperation(ctx, op)
}

func (m *service) insertMarker(ctx context.Context, marker Marker) error {
	if marker.PayloadJSON == "" {
		marker.PayloadJSON = "{}"
	}
	if marker.CreatedAt.IsZero() {
		marker.CreatedAt = m.now().UTC()
	}
	return m.store.InsertMarker(ctx, marker)
}

type disabledManager struct {
	mode Mode
}

func (m disabledManager) Close() error {
	return nil
}

func (m disabledManager) Mode() Mode {
	return m.mode
}

func (m disabledManager) Status(_ context.Context, key ScopeKey) (Status, error) {
	key = normalizeScope(key)
	return Status{
		Mode:                     ModeOff,
		MemoryID:                 key.MemoryID(),
		Scope:                    key,
		Enabled:                  false,
		StateVersion:             0,
		ObservationCount:         0,
		ActiveObservationTokens:  0,
		ReflectionPresent:        false,
		ActiveReflectionTokens:   0,
		LastObservedMessageIndex: -1,
		UpdatedAt:                time.Now().UTC(),
	}, nil
}

func (m disabledManager) SetEnabled(ctx context.Context, key ScopeKey, _ bool, _ *Config, _ string, _ string) (Status, error) {
	status, _ := m.Status(ctx, key)
	return status, fmt.Errorf("observational memory is disabled by mode")
}

func (m disabledManager) Observe(ctx context.Context, req ObserveRequest) (ObserveResult, error) {
	status, _ := m.Status(ctx, req.Scope)
	return ObserveResult{Status: status}, nil
}

func (m disabledManager) Snippet(ctx context.Context, key ScopeKey) (string, Status, error) {
	status, _ := m.Status(ctx, key)
	return "", status, nil
}

func (m disabledManager) ReflectNow(ctx context.Context, key ScopeKey, _ string, _ string) (Status, error) {
	status, _ := m.Status(ctx, key)
	return status, fmt.Errorf("observational memory is disabled by mode")
}

func (m disabledManager) Export(ctx context.Context, key ScopeKey, _ string) (ExportResult, error) {
	status, _ := m.Status(ctx, key)
	return ExportResult{Status: status}, fmt.Errorf("observational memory is disabled by mode")
}

func statusFromRecord(mode Mode, rec Record) Status {
	return Status{
		Mode:                     mode,
		MemoryID:                 rec.MemoryID,
		Scope:                    rec.Scope,
		Enabled:                  rec.Enabled,
		StateVersion:             rec.StateVersion,
		ObservationCount:         len(rec.ActiveObservations),
		ActiveObservationTokens:  rec.ActiveObservationTokens,
		ReflectionPresent:        strings.TrimSpace(rec.ActiveReflection) != "",
		ActiveReflectionTokens:   rec.ActiveReflectionTokens,
		LastObservedMessageIndex: rec.LastObservedMessageIndex,
		UpdatedAt:                rec.UpdatedAt,
	}
}

func normalizeScope(key ScopeKey) ScopeKey {
	if strings.TrimSpace(key.TenantID) == "" {
		key.TenantID = "default"
	}
	if strings.TrimSpace(key.AgentID) == "" {
		key.AgentID = "default"
	}
	if strings.TrimSpace(key.ConversationID) == "" {
		key.ConversationID = "unknown"
	}
	return key
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.ObserveMinTokens <= 0 {
		cfg.ObserveMinTokens = defaults.ObserveMinTokens
	}
	if cfg.SnippetMaxTokens <= 0 {
		cfg.SnippetMaxTokens = defaults.SnippetMaxTokens
	}
	if cfg.ReflectThresholdTokens <= 0 {
		cfg.ReflectThresholdTokens = defaults.ReflectThresholdTokens
	}
	return cfg
}

func mergeConfig(current Config, requested Config) Config {
	current = normalizeConfig(current)
	if requested.ObserveMinTokens > 0 {
		current.ObserveMinTokens = requested.ObserveMinTokens
	}
	if requested.SnippetMaxTokens > 0 {
		current.SnippetMaxTokens = requested.SnippetMaxTokens
	}
	if requested.ReflectThresholdTokens > 0 {
		current.ReflectThresholdTokens = requested.ReflectThresholdTokens
	}
	return normalizeConfig(current)
}

func markerPayloadJSON(v map[string]any) string {
	if len(v) == 0 {
		return "{}"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func nextID(prefix string) string {
	n := atomic.AddUint64(&opSeq, 1)
	return fmt.Sprintf("%s_%d", prefix, n)
}
