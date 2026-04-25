package db

import (
	"database/sql"
	"time"
	"wappiz/pkg/assert"
	"wappiz/pkg/fault"
	"wappiz/pkg/logger"

	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config defines the parameters needed to establish database connections.
type Config struct {
	// The primary DSN for your database. This must support both reads and writes.
	PrimaryDSN string
}

// database implements the Database interface, providing access to database replicas
// and handling connection lifecycle.
type database struct {
	primary *Replica // Primary database connection used for read and write operations
}

func open(dns string) (*sql.DB, error) {
	// sql.Open only validates the DSN, it doesn't actually connect.
	// We need to call Ping() to verify connectivity.
	db, err := sql.Open("pgx", dns)
	if err != nil {
		return nil, fault.Wrap(err, fault.Internal("failed to open database"))
	}

	// Configure connection pool for better performance
	// These settings prevent cold-start latency by maintaining warm connections
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute) // Refresh connections every 5 min (PlanetScale recommendation)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fault.Wrap(err, fault.Internal("failed to ping database"))
	}

	logger.Info("database connection pool initialized successfully")

	return db, nil
}

// New creates a new database instance with the provided configuration.
// It establishes connections to the primary database.
// Returns an error if connections cannot be established or if DSNs are misconfigured.
func New(config Config) (*database, error) {
	err := assert.All(
		assert.NotEmpty(config.PrimaryDSN),
	)
	if err != nil {
		return nil, fault.Wrap(err, fault.Internal("invalid configuration"))
	}

	primary, err := open(config.PrimaryDSN)
	if err != nil {
		return nil, fault.Wrap(err, fault.Internal("cannot open primary replica"))
	}

	primaryReplica := &Replica{
		db:   primary,
		name: "primary",
	}

	return &database{
		primary: primaryReplica,
	}, nil
}

// Primary returns the primary database replica for read and write operations.
func (d *database) Primary() *Replica {
	return d.primary
}

// Close properly closes all database connections.
// This should be called when the application is shutting down.
func (d *database) Close() error {
	return d.primary.db.Close()
}
