package db

import (
	"context"
	"database/sql"
)

type Database interface {
	Primary() *Replica

	Close() error
}

// DBTX is an interface that abstracts database operations for both
// direct connections and transactions. It allows query methods to work
// with either a database or transaction, making transaction handling more
// flexible.
//
// This interface is implemented by both sql.DB and sql.Tx, as well as
// the custom Replica type in this package.
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	PrepareContext(context.Context, string) (*sql.Stmt, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// DBTx represents a database transaction with commit and rollback capabilities.
// It extends DBTX with transaction-specific methods.
type DBTx interface {
	DBTX
	Commit() error
	Rollback() error
}
