package database

import (
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

func Connect(url string) *sqlx.DB {
	db, err := sqlx.Open("pgx", url)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	log.Println("database connected")
	return db
}
