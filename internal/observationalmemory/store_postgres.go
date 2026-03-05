package observationalmemory

import (
	"context"
	"fmt"
	"time"
)

type PostgresStore struct {
	dsn string
}

func NewPostgresStore(dsn string) (*PostgresStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	return &PostgresStore{dsn: dsn}, nil
}

func (s *PostgresStore) Close() error {
	return nil
}

func (s *PostgresStore) Migrate(context.Context) error {
	return fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) ResetStaleOperations(context.Context, time.Time) error {
	return fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) GetOrCreateRecord(context.Context, ScopeKey, bool, Config, time.Time) (Record, error) {
	return Record{}, fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) UpdateRecord(context.Context, Record) error {
	return fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) CreateOperation(context.Context, Operation) (Operation, error) {
	return Operation{}, fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) UpdateOperationStatus(context.Context, string, string, string, time.Time) error {
	return fmt.Errorf("postgres store is not implemented in v1")
}

func (s *PostgresStore) InsertMarker(context.Context, Marker) error {
	return fmt.Errorf("postgres store is not implemented in v1")
}
