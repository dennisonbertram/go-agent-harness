package observationalmemory

import (
	"context"
	"time"
)

type Store interface {
	Close() error
	Migrate(ctx context.Context) error
	ResetStaleOperations(ctx context.Context, olderThan time.Time) error
	GetOrCreateRecord(ctx context.Context, key ScopeKey, defaultEnabled bool, defaultConfig Config, now time.Time) (Record, error)
	UpdateRecord(ctx context.Context, rec Record) error
	CreateOperation(ctx context.Context, op Operation) (Operation, error)
	UpdateOperationStatus(ctx context.Context, operationID, status, errorText string, now time.Time) error
	InsertMarker(ctx context.Context, marker Marker) error
}
