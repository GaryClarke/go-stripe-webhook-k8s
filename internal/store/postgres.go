package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	// Register pgx as a database/sql driver named "pgx".
	// The blank import runs init() in the driver package; we never call it directly.
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Postgres implements Store against processed_events in PostgreSQL.
type Postgres struct {
	db *sql.DB
}

// NewPostgres opens a connection pool to the database at dsn.
// dsn is a Postgres URL, e.g. postgres://webhook:webhook@localhost:5433/stripe_webhook_dev?sslmode=disable
//
// sql.Open does not connect immediately; Ping verifies the database is reachable.
func NewPostgres(dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open postgres: %w", err)
	}

	// Fail fast at startup if DATABASE_URL is wrong or Postgres is down.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping postgres: %w", err)
	}

	return &Postgres{db: db}, nil
}

// Ping checks that the database accepts connections (used by /readyz in Phase 7).
func (p *Postgres) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p *Postgres) Status(ctx context.Context, eventID string) (*EventStatus, error) {
	var status string
	err := p.db.QueryRowContext(ctx,
		`SELECT status FROM processed_events WHERE event_id = $1`,
		eventID,
	).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return &EventStatus{Found: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: select status: %w", err)
	}
	return &EventStatus{
		Status: status,
		Found:  true,
	}, nil
}

// ProcessEvent claims eventID in one transaction: insert processing, run fn, mark processed.
// Returns claimed=true only when this caller inserted the row and committed.
// Returns claimed=false when event_id already exists (duplicate delivery — not an error).
func (p *Postgres) ProcessEvent(
	ctx context.Context,
	eventID, eventType string,
	fn func(ctx context.Context) error,
) (bool, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin tx: %w", err)
	}
	// Rollback is a no-op after successful Commit; cleans up if we return early on error.
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO processed_events (event_id, event_type, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
	`, eventID, eventType, StatusProcessing)
	if err != nil {
		return false, fmt.Errorf("store: insert claim: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: rows affected: %w", err)
	}
	if n == 0 {
		// Another request or Pod already claimed this event_id.
		return false, nil
	}

	// M8: fn is trivial (log only). Runs inside the same TX before we mark processed.
	if fn != nil {
		if err := fn(ctx); err != nil {
			return false, fmt.Errorf("store: process fn: %w", err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE processed_events
		SET status = $1, processed_at = now()
		WHERE event_id = $2
	`, StatusProcessed, eventID)
	if err != nil {
		return false, fmt.Errorf("store: mark processed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit: %w", err)
	}
	return true, nil
}

// AcceptEvent claims eventID in one transaction: ledger accepted + outbox pending.
// Returns claimed=true only when this caller inserted both rows and committed.
// Returns claimed=false when event_id already exists (duplicate delivery — not an error).
func (p *Postgres) AcceptEvent(
	ctx context.Context,
	eventID, eventType string,
	jobPayload []byte,
) (bool, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin tx: %w", err)
	}
	// Rollback is a no-op after successful Commit;
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO processed_events (event_id, event_type, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
		`, eventID, eventType, StatusAccepted)
	if err != nil {
		return false, fmt.Errorf("store: insert ledger: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("store: rows affected: %w", err)
	}
	if n == 0 {
		return false, nil
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO outbox_events (event_id, event_type, payload, status)
		VALUES ($1, $2, $3::jsonb, $4)
	`, eventID, eventType, jobPayload, OutboxPending)
	if err != nil {
		return false, fmt.Errorf("store: insert outbox: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit: %w", err)
	}
	return true, nil
}

func (p *Postgres) OutboxStatus(ctx context.Context, eventID string) (*OutboxStatus, error) {
	var status string
	var payload []byte
	err := p.db.QueryRowContext(ctx, `
		SELECT status, payload FROM outbox_events WHERE event_id = $1`, eventID).Scan(&status, &payload)
	if errors.Is(err, sql.ErrNoRows) {
		return &OutboxStatus{Found: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: select outbox: %w", err)
	}
	return &OutboxStatus{
		Found:   true,
		Status:  status,
		Payload: payload,
	}, nil
}

// TruncateLedger removes all rows from processed_events and outbox_events (integration tests).
func (p *Postgres) TruncateLedger(ctx context.Context) error {
	// Both tables in one statement: outbox_events FK references processed_events.
	_, err := p.db.ExecContext(ctx, `TRUNCATE TABLE outbox_events, processed_events`)
	return err
}

// Compile-time check: *Postgres must implement Store.
// If ProcessEvent or Status are missing, the build fails here.
var _ Store = (*Postgres)(nil)
