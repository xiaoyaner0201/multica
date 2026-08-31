package migrations

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestOIDCIdentityTableMigrationIsRetrySafeAndFailsClosed(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("integration test requires Postgres at DATABASE_URL")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire Postgres connection: %v", err)
	}
	defer conn.Release()

	schema := fmt.Sprintf("oidc_identity_migration_%d", time.Now().UnixNano())
	schemaIdent := pgx.Identifier{schema}.Sanitize()
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schemaIdent+" CASCADE")
	}
	cleanup()
	t.Cleanup(cleanup)
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+schemaIdent); err != nil {
		t.Fatalf("create isolated migration schema: %v", err)
	}
	if _, err := conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, schema); err != nil {
		t.Fatalf("set isolated migration search path: %v", err)
	}

	// Simulate a crash after DDL commit but before the migration ledger insert:
	// running the same migration again must validate and succeed.
	applyMigrationFile(t, ctx, conn.Conn(), "444_user_oidc_identity.up.sql")
	applyMigrationFile(t, ctx, conn.Conn(), "444_user_oidc_identity.up.sql")

	if _, err := conn.Exec(ctx, `DROP TABLE user_oidc_identity`); err != nil {
		t.Fatalf("drop compatible table: %v", err)
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE user_oidc_identity (issuer TEXT NOT NULL)`); err != nil {
		t.Fatalf("create incompatible table: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(realMigrationsDir(t), "444_user_oidc_identity.up.sql"))
	if err != nil {
		t.Fatalf("read OIDC migration: %v", err)
	}
	if _, err := conn.Exec(ctx, string(body)); err == nil || !strings.Contains(err.Error(), "incompatible schema") {
		t.Fatalf("incompatible table error = %v, want fail-closed schema error", err)
	}
}
