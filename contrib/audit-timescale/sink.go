package timescale

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/go-freya/freya/audit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/001_audit_events.sql
var migration string

// Sink is the TimescaleDB audit sink.
type Sink struct {
	pool *pgxpool.Pool
	*Batcher
}

var _ audit.Sink = (*Sink)(nil)

// New connects with dsn (which must use sslmode=verify-full or stronger unless
// AllowInsecure) and starts the batch writer.
func New(ctx context.Context, dsn string, opts Options) (*Sink, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("audit-timescale: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("audit-timescale: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("audit-timescale: ping: %w", err)
	}
	s := &Sink{pool: pool}
	s.Batcher = NewBatcher(poolInserter{pool}, opts)
	return s, nil
}

// Migrate applies the embedded schema (requires a role that can create objects).
func Migrate(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("audit-timescale: %w", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, migration); err != nil {
		return fmt.Errorf("audit-timescale: migrate: %w", err)
	}
	return nil
}

// Close drains pending events and releases the pool.
func (s *Sink) Close() {
	s.Batcher.Close()
	s.pool.Close()
}

type poolInserter struct{ pool *pgxpool.Pool }

func (p poolInserter) InsertBatch(ctx context.Context, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	n, err := p.pool.CopyFrom(ctx, pgx.Identifier{"audit_events"}, Columns, pgx.CopyFromRows(rows))
	if err != nil {
		return err
	}
	if int(n) != len(rows) {
		return errors.New("audit-timescale: partial copy")
	}
	return nil
}
