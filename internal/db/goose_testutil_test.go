// Copyright (c) 2025 Reliant Labs
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"testing"
)

// The goose-history tests (access-tokens renumber repair, the GitHub sender,
// provider-token sealing, workflow owner and daemon-binding migrations) pin
// what particular goose migrations do to a database in a particular state.
// The goose bridge still runs those migrations, on any database an older goose
// release left behind, so they still matter — but they need a database GOOSE
// built, with goose_db_version, which the baseline-built template is not. This
// is that template, built the way every goose release built a database:
// RunMigrations on an empty one.

// gooseTemplateKey names the goose-built template after goose's frozen
// history and the seed.
var gooseTemplateKey = sync.OnceValues(func() (string, error) {
	return testTemplateKey("reliant_goose_", FS, migrationsDir, testSeedSQL)
})

// setupGooseTestDBWithRawDB is SetupTestDBWithRawDB over a copy of a database
// goose migrated to its frozen head (plus the seeded "test-project").
func setupGooseTestDBWithRawDB(t *testing.T) (*Repo, *sql.DB, func()) {
	t.Helper()
	key, err := gooseTemplateKey()
	if err != nil {
		t.Fatalf("goose template key: %v", err)
	}
	raw, _, cleanup := cloneTestTemplate(t, key, func(_ string, build *sql.DB) error {
		if err := RunMigrations(build); err != nil {
			return fmt.Errorf("goose migrations: %w", err)
		}
		if _, err := build.Exec(testSeedSQL); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
		return nil
	})
	return repoOverTestDB(t, raw, cleanup)
}

// startUp runs what the api-server runs at boot, Migrate, against the
// database raw is open on.
func startUp(t *testing.T, raw *sql.DB) error {
	t.Helper()
	dsn, err := testDSNOf(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Migrate(context.Background(), dsn, slog.New(slog.DiscardHandler))
	return err
}
