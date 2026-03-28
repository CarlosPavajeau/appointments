package db

import (
	"context"
	"database/sql"
)

// Replica wraps a standard SQL database connection and implements the gen.DBTX interface
// to enable interaction with the generated database code.
type Replica struct {
	db *sql.DB // Underlying database connection
}

// Ensure Replica implements the gen.DBTX interface
var _ DBTX = (*Replica)(nil)

// ExecContext executes a SQL statement and returns a result summary.
// It's used for INSERT, UPDATE, DELETE statements that don't return rows.
func (r *Replica) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := r.db.ExecContext(ctx, query, args...)

	return result, err
}

// PrepareContext prepares a SQL statement for later execution.
func (r *Replica) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	stmt, err := r.db.PrepareContext(ctx, query)

	return stmt, err
}

// QueryContext executes a SQL query that returns rows.
func (r *Replica) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)

	return rows, err
}

// QueryRowContext executes a SQL query that returns a single row.
func (r *Replica) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	row := r.db.QueryRowContext(ctx, query, args...)

	return row
}

// Begin starts a transaction and returns it.
// This method provides a way to use the Replica in transaction-based operations.
func (r *Replica) Begin(ctx context.Context) (DBTx, error) {
	tx, err := r.db.BeginTx(ctx, nil)

	return tx, err
}
