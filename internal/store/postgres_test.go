package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
)

func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_URL to run Postgres integration tests")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatal("TEST_POSTGRES_URL must be a postgres:// or postgresql:// URL")
	}
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })

	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("cookarr_test_%x", b)
	quoted := pgx.Identifier{schema}.Sanitize()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(ctx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("clean up test schema: %v", err)
		}
	})
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	s, err := OpenPostgres(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestPostgresMigrations(t *testing.T) {
	s := openTestPostgres(t)
	ctx := context.Background()
	if err := s.CreateOwner(ctx, "saved-owner", "saved-hash"); err != nil {
		t.Fatal(err)
	}
	// Startup migration is idempotent and preserves data.
	if err := s.migrate(ctx); err != nil {
		t.Fatal(err)
	}
	o, err := s.Owner(ctx)
	if err != nil || o.Username != "saved-owner" {
		t.Fatalf("lost owner after migrate: %+v %v", o, err)
	}
	f, err := fs.Sub(postgresMigrations, "migrations/postgres")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectPostgres, s.db, f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if exists, err := s.OwnerExists(ctx); err != nil || exists {
		t.Fatalf("fresh schema after rollback: %v %v", exists, err)
	}
}
