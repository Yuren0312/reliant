// Copyright (c) 2025 Reliant Labs
package store_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/reliant-labs/forge/pkg/orm"
	"github.com/reliant-labs/forge/pkg/pgtest"

	migrations "github.com/reliant-labs/reliant/db"
	"github.com/reliant-labs/reliant/internal/db"
)

// spikeEnv is one test's private database, reachable through BOTH data
// layers over ONE pgx pool: the sqlc/hand Repo the product uses today and the
// forge ORM client the P5b spike ports onto. Sharing the pool keeps the
// benchmarks honest — the two paths differ only in the code that builds and
// scans the query.
type spikeEnv struct {
	Repo *db.Repo
	SQL  *sql.DB
	ORM  *orm.Client
}

// newSpikeEnv clones a migrated template (forge pgtest.NewFromTemplate, built
// once per migration set by reliant's own db.Migrate — goose bridge then
// migratekit, the production path) and opens it with pgx.
//
// It needs a postgres server: FORGE_TEST_POSTGRES_URL (a maintenance DSN,
// database "postgres"). Skipped under -short, like every DB-backed test here.
func newSpikeEnv(tb testing.TB) *spikeEnv {
	tb.Helper()
	if testing.Short() {
		tb.Skip("P5b spike: DB-backed; skipped under -short")
	}
	base := os.Getenv(pgtest.EnvBaseURL)
	if base == "" {
		tb.Skipf("SKIPPING DB-BACKED SPIKE TEST — set %s to a postgres maintenance DSN", pgtest.EnvBaseURL)
	}

	key := "reliant_p5b_" + migrationsHash(tb)
	clone, cleanup, err := pgtest.NewFromTemplate(key, func(build *sql.DB) error {
		var name string
		if err := build.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
			return err
		}
		dsn := withDatabase(tb, base, name)
		if _, err := db.Migrate(context.Background(), dsn, slog.New(slog.DiscardHandler)); err != nil {
			return err
		}
		_, err := build.Exec(`INSERT INTO projects (id, user_id, name, path, created_at, updated_at, last_active)
			VALUES ('test-project', 'test-user', 'Test Project', '/tmp/reliant-test-seed-project', NOW(), NOW(), NOW())`)
		return err
	})
	if err != nil {
		tb.Fatalf("pgtest template: %v", err)
	}
	tb.Cleanup(cleanup)

	// pgtest hands back a lib/pq handle; production (and serverkit.OpenDB)
	// is pgx. Reopen the same clone on pgx so both layers are measured on
	// the driver they ship with.
	var name string
	if err := clone.QueryRow(`SELECT current_database()`).Scan(&name); err != nil {
		tb.Fatalf("clone name: %v", err)
	}
	sqlDB, err := sql.Open("pgx", withDatabase(tb, base, name))
	if err != nil {
		tb.Fatalf("open pgx: %v", err)
	}
	tb.Cleanup(func() { _ = sqlDB.Close() })

	client, err := orm.NewClientWithDB(sqlDB, "postgres")
	if err != nil {
		tb.Fatalf("orm client: %v", err)
	}
	return &spikeEnv{Repo: db.NewRepoWithDriver(sqlDB, db.DriverPostgres), SQL: sqlDB, ORM: client}
}

func withDatabase(tb testing.TB, base, name string) string {
	tb.Helper()
	u, err := url.Parse(base)
	if err != nil {
		tb.Fatalf("parse %s: %v", pgtest.EnvBaseURL, err)
	}
	u.Path = "/" + name
	return u.String()
}

// migrationsHash keys the template on the migration set, so an edited or
// added migration never gets a stale template.
func migrationsHash(tb testing.TB) string {
	tb.Helper()
	src := migrations.Source()
	var names []string
	if err := fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, path)
		}
		return err
	}); err != nil {
		tb.Fatalf("walk migrations: %v", err)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		body, err := fs.ReadFile(src, n)
		if err != nil {
			tb.Fatalf("read %s: %v", n, err)
		}
		h.Write([]byte(n))
		h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

// seedChat creates a chat in the seeded test project plus its root thread
// (id == chat id, the production invariant), through the existing Repo.
func seedChat(tb testing.TB, env *spikeEnv, chatID string) {
	tb.Helper()
	ctx := context.Background()
	now := time.Now()
	if err := env.Repo.CreateChat(ctx, &db.Chat{
		ID: chatID, Title: "spike chat", ProjectID: "test-project", UserID: "test-user",
		State: 2, CreatedAt: now, UpdatedAt: now, LastActive: now,
	}); err != nil {
		tb.Fatalf("seed chat %s: %v", chatID, err)
	}
	if _, err := env.Repo.CreateThread(ctx, &db.Thread{ID: chatID, ChatID: chatID, CreatedAt: now}); err != nil {
		tb.Fatalf("seed root thread %s: %v", chatID, err)
	}
}

// TestSpikeHarness proves the harness: both layers see one database.
func TestSpikeHarness(t *testing.T) {
	env := newSpikeEnv(t)
	seedChat(t, env, "chat-harness")
	n, err := env.ORM.Bun().NewSelect().Table("chats").Where("id = ?", "chat-harness").Count(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("ORM sees %d chats (err %v), want 1", n, err)
	}
}
