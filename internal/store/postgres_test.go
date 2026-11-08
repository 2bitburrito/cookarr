package store

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/peterldowns/pgtestdb"
	"github.com/peterldowns/pgtestdb/migrators/goosemigrator"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Store tests require Docker. One disposable server is shared by this package;
// pgtestdb gives each test its own database. No application config is read.
var testPostgresConfig pgtestdb.Config

func TestMain(m *testing.M) {
	os.Exit(runStoreTests(m))
}

func runStoreTests(m *testing.M) (code int) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	server, err := postgres.Run(ctx, "postgres:17",
		postgres.WithUsername("cookarr_test"),
		postgres.WithPassword("cookarr_test"),
		postgres.WithDatabase("cookarr_test"),
		testcontainers.WithTmpfs(map[string]string{"/var/lib/postgresql/data": "rw"}),
		// Unsafe for persistent data; this server exists only for this test run.
		testcontainers.WithCmd("postgres",
			"-c", "fsync=off",
			"-c", "synchronous_commit=off",
			"-c", "full_page_writes=off",
		),
		postgres.BasicWaitStrategies(),
	)
	if server != nil {
		defer func() {
			// Use a fresh context even if startup or the test run timed out.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := server.Terminate(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "terminate test Postgres:", err)
				code = 1
			}
		}()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "start test Postgres (Docker is required):", err)
		return 1
	}
	host, err := server.Host(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "get test Postgres host:", err)
		return 1
	}
	port, err := server.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintln(os.Stderr, "get test Postgres port:", err)
		return 1
	}
	testPostgresConfig = pgtestdb.Config{
		DriverName: "pgx",
		Host:       host,
		Port:       port.Port(),
		User:       "cookarr_test",
		Password:   "cookarr_test",
		Database:   "cookarr_test",
		Options:    "sslmode=disable",
	}
	return m.Run()
}

func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	migrator := goosemigrator.New("migrations/postgres", goosemigrator.WithFS(postgresMigrations))
	// Custom provisions an isolated, migrated database and owns its cleanup.
	// Open it through the production path so startup migrations are tested too.
	db := pgtestdb.Custom(t, testPostgresConfig, migrator)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := OpenPostgres(ctx, db.URL())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("close Postgres: %v", err)
		}
	})
	return s
}

func TestPostgresTestServerSettings(t *testing.T) {
	t.Parallel()
	s := openTestPostgres(t)
	for _, setting := range []string{"fsync", "synchronous_commit", "full_page_writes"} {
		var value string
		if err := s.db.QueryRowContext(t.Context(), "SELECT current_setting($1)", setting).Scan(&value); err != nil {
			t.Fatal(err)
		}
		if value != "off" {
			t.Errorf("%s = %q, want off for the disposable test server", setting, value)
		}
	}
}

func TestPostgresMigrations(t *testing.T) {
	t.Parallel()
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
