package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Config struct {
	PrimaryDns string
}

type database struct {
	primary *Replica
}

func open(dns string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dns)

	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	log.Println("database connection pool initialized successfully")

	return db, nil
}

func New(config Config) (*database, error) {
	primary, err := open(config.PrimaryDns)

	if err != nil {
		return nil, err
	}

	primaryReplica := &Replica{
		db: primary,
	}

	return &database{
		primary: primaryReplica,
	}, nil
}

func (d *database) Primary() *Replica {
	return d.primary
}

func (d *database) Close() error {
	return d.primary.db.Close()
}
