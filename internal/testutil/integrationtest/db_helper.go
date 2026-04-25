//go:build integration

package integrationtest

import (
	"context"
	"os"
	"testing"
	"wappiz/pkg/db"
)

func RequireDatabase(t *testing.T) db.Database {
	t.Helper()

	dsn, ok := os.LookupEnv("DATABASE_URL")
	if !ok || dsn == "" {
		t.Skip("DATABASE_URL is required for integration tests")
	}

	database, err := db.New(db.Config{
		PrimaryDSN: dsn,
	})
	if err != nil {
		t.Fatalf("failed to connect integration database: %v", err)
	}

	t.Cleanup(func() {
		_ = database.Close()
	})

	return database
}

func ResetPublicSchema(t *testing.T, database db.Database) {
	t.Helper()

	_, err := database.Primary().ExecContext(context.Background(), `
DO $$
DECLARE
  tab RECORD;
BEGIN
  FOR tab IN
    SELECT tablename
    FROM pg_tables
    WHERE schemaname = 'public'
  LOOP
    EXECUTE 'TRUNCATE TABLE ' || quote_ident(tab.tablename) || ' RESTART IDENTITY CASCADE';
  END LOOP;
END $$;
`)
	if err != nil {
		t.Fatalf("failed to reset public schema: %v", err)
	}
}

func InsertUser(t *testing.T, database db.Database, userID, name, email string) {
	t.Helper()

	_, err := database.Primary().ExecContext(
		context.Background(),
		`INSERT INTO users (id, name, email, email_verified) VALUES ($1, $2, $3, TRUE)`,
		userID,
		name,
		email,
	)
	if err != nil {
		t.Fatalf("failed to seed user %q: %v", userID, err)
	}
}
