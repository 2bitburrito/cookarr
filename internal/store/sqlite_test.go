package store

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store/sqlitedb"
	"github.com/pressly/goose/v3"
)

func openTestStore(t *testing.T) *SQLite {
	t.Helper()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOwnerAndSessions(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.Owner(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing owner: %v", err)
	}
	if err := s.SetPassword(ctx, "hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reset without owner: %v", err)
	}
	if err := s.CreateOwner(ctx, "owner", "old-hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateOwner(ctx, "other", "hash"); !errors.Is(err, ErrOwnerExists) {
		t.Fatalf("duplicate owner: %v", err)
	}
	for _, token := range []string{"session-1", "session-2"} {
		if err := s.CreateSession(ctx, token, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if valid, err := s.SessionValid(ctx, token); err != nil || !valid {
			t.Fatalf("session invalid: %v", err)
		}
	}
	if err := s.CreateSession(ctx, "expired", time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if valid, err := s.SessionValid(ctx, "expired"); err != nil || valid {
		t.Fatalf("expired session accepted: %v", err)
	}
	if err := s.SetPassword(ctx, "new-hash"); err != nil {
		t.Fatal(err)
	}
	o, err := s.Owner(ctx)
	if err != nil || o.PasswordHash != "new-hash" {
		t.Fatalf("password not changed: %+v %v", o, err)
	}
	for _, token := range []string{"session-1", "session-2"} {
		if valid, err := s.SessionValid(ctx, token); err != nil || valid {
			t.Fatalf("session not revoked: %v", err)
		}
	}
}

func TestConcurrentSetup(t *testing.T) {
	s := openTestStore(t)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { results <- s.CreateOwner(context.Background(), "owner", "hash") })
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrOwnerExists):
			conflict++
		default:
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
}

func TestRecipeQueries(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	r := &capture.Recipe{Title: "Soup", SourceURL: "https://example.com/soup", Ingredients: []capture.Ingredient{{Text: "water"}}}
	first, err := s.CreateRecipe(ctx, r, []byte("<html></html>"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateRecipe(ctx, r, []byte("<html></html>"))
	if err != nil || first.ID == second.ID {
		t.Fatalf("duplicate import: %+v %v", second, err)
	}
	got, err := s.GetRecipe(ctx, first.ID)
	if err != nil || got.Title != "Soup" || got.Ingredients[0].Text != "water" {
		t.Fatalf("round trip: %+v %v", got, err)
	}
	items, total, err := s.ListRecipes(ctx, 1, 0)
	if err != nil || total != 2 || len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("pagination: %+v total=%d err=%v", items, total, err)
	}
	if err := s.DeleteRecipe(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRecipe(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}
	if _, err := s.GetRecipe(ctx, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing get: %v", err)
	}
	if _, err := s.queries.CreateRecipe(ctx, sqlitedb.CreateRecipeParams{Data: "not JSON", StrippedHtml: []byte{}}); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
}

// Reproduce the initial custom runner's schema and ensure goose upgrades it
// without discarding existing recipes, owner credentials or sessions.
func TestLegacyMigrationAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cookarr.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	body, err := sqliteMigrations.ReadFile("migrations/sqlite/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Only execute the up section, as the old migration had no down section.
	up, _, _ := strings.Cut(string(body), "-- +goose Down")
	if _, err := db.Exec(up); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (name TEXT PRIMARY KEY, applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations VALUES ('0001_init.sql', '2026-01-01T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	q := sqlitedb.New(db)
	ctx := context.Background()
	if err := q.CreateOwner(ctx, sqlitedb.CreateOwnerParams{Username: "legacy", PasswordHash: "saved-hash", CreatedAt: ts(time.Now())}); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateSession(ctx, sqlitedb.CreateSessionParams{TokenHash: "saved-token", CreatedAt: ts(time.Now()), ExpiresAt: ts(time.Now().Add(time.Hour))}); err != nil {
		t.Fatal(err)
	}
	id, err := q.CreateRecipe(ctx, sqlitedb.CreateRecipeParams{Title: "Saved", Data: `{"title":"Saved"}`, StrippedHtml: []byte("source")})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		s, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		o, err := s.Owner(ctx)
		if err != nil || o.Username != "legacy" || o.PasswordHash != "saved-hash" {
			t.Fatalf("lost owner: %+v %v", o, err)
		}
		if valid, err := s.SessionValid(ctx, "saved-token"); err != nil || !valid {
			t.Fatalf("lost session: %v", err)
		}
		r, err := s.GetRecipe(ctx, id)
		if err != nil || r.Title != "Saved" {
			t.Fatalf("lost recipe: %+v %v", r, err)
		}
		s.Close()
	}
}

func TestGooseDownAndUp(t *testing.T) {
	s := openTestStore(t)
	f, err := fs.Sub(sqliteMigrations, "migrations/sqlite")
	if err != nil {
		t.Fatal(err)
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, s.db, f)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
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
