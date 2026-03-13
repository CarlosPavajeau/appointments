package database

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

type queryTracer struct{}

type queryCtxKey struct{}

type queryMeta struct {
	sql   string
	start time.Time
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryCtxKey{}, &queryMeta{sql: data.SQL, start: time.Now()})
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	meta, ok := ctx.Value(queryCtxKey{}).(*queryMeta)
	if !ok {
		return
	}
	elapsed := time.Since(meta.start)
	if data.Err != nil {
		log.Printf("query failed [%s] sql: %s | error: %v", elapsed, meta.sql, data.Err)
		return
	}
	log.Printf("query executed [%s] sql: %s", elapsed, meta.sql)
}

func Connect(url string) *sqlx.DB {
	config, err := pgx.ParseConfig(url)
	if err != nil {
		log.Fatalf("failed to parse database url: %v", err)
	}
	config.Tracer = &queryTracer{}

	connStr := stdlib.RegisterConnConfig(config)
	db, err := sqlx.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("failed to open database connection: %v", err)
	}

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("database connected")
	return db
}
