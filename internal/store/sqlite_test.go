package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store/postgresdb"
	"github.com/hpalmer/cookarr/internal/store/sqlitedb"
	"github.com/hpalmer/cookarr/internal/types"
	"github.com/pressly/goose/v3"
)

func openTestStore(t *testing.T) *SQLite {
	t.Helper()
	s, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func TestOwnerAndSessions(t *testing.T) {
	forEachStore(t, testOwnerAndSessions)
}

func testOwnerAndSessions(t *testing.T, s Store) {
	ctx := context.Background()
	if exists, err := s.OwnerExists(ctx); err != nil || exists {
		t.Fatalf("unexpected owner before setup: %v %v", exists, err)
	}
	if _, err := s.Owner(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing owner: %v", err)
	}
	if err := s.SetPassword(ctx, "hash"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reset without owner: %v", err)
	}
	if err := s.CreateOwner(ctx, "owner", "old-hash"); err != nil {
		t.Fatal(err)
	}
	if exists, err := s.OwnerExists(ctx); err != nil || !exists {
		t.Fatalf("owner missing after setup: %v %v", exists, err)
	}
	if valid, err := s.SessionValid(ctx, "unknown"); err != nil || valid {
		t.Fatalf("unknown session accepted: %v", err)
	}
	if err := s.CreateSession(ctx, "logout", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSession(ctx, "logout"); err != nil {
		t.Fatal(err)
	}
	if valid, err := s.SessionValid(ctx, "logout"); err != nil || valid {
		t.Fatalf("deleted session accepted: %v", err)
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
	forEachStore(t, testConcurrentSetup)
}

func testConcurrentSetup(t *testing.T, s Store) {
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
	forEachStore(t, testRecipeQueries)
}

func testRecipeQueries(t *testing.T, s Store) {
	ctx := context.Background()
	items, total, err := s.ListRecipes(ctx, 50, 0)
	if err != nil || total != 0 || items == nil || len(items) != 0 {
		t.Fatalf("empty library: %+v total=%d err=%v", items, total, err)
	}
	r := &capture.Recipe{
		Title: "Soup", Description: "A soup", SourceURL: "https://example.com/soup", SiteName: "Recipes",
		Ingredients: []capture.Ingredient{{Text: "water"}},
		Photos:      []capture.Photo{{URL: "https://example.com/photo.jpg", Role: types.PhotoHero}},
	}
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
	items, total, err = s.ListRecipes(ctx, 1, 0)
	if err != nil || total != 2 || len(items) != 1 || items[0].ID != second.ID {
		t.Fatalf("pagination: %+v total=%d err=%v", items, total, err)
	}
	if items[0].Title != r.Title || items[0].Description != r.Description || items[0].SourceURL != r.SourceURL || items[0].SiteName != r.SiteName || items[0].HeroURL != r.Photos[0].URL || items[0].CapturedAt.IsZero() {
		t.Fatalf("summary mismatch: %+v", items[0])
	}
	items, total, err = s.ListRecipes(ctx, 1, 1)
	if err != nil || total != 2 || len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("offset: %+v total=%d err=%v", items, total, err)
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
	var jsonErr error
	switch s := s.(type) {
	case *SQLite:
		_, jsonErr = s.queries.CreateRecipe(ctx, sqlitedb.CreateRecipeParams{Data: "not JSON", StrippedHtml: []byte{}})
	case *Postgres:
		_, jsonErr = s.queries.CreateRecipe(ctx, postgresdb.CreateRecipeParams{Data: []byte("not JSON"), StrippedHtml: []byte{}, CapturedAt: time.Now()})
	}
	if jsonErr == nil {
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
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
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
	id, err := q.CreateRecipe(ctx, sqlitedb.CreateRecipeParams{
		Title: "Saved", Data: `{"title":"Saved"}`, StrippedHtml: []byte("source"), CapturedAt: ts(time.Now()),
	})
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
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSQLiteRejectsInvalidStoredTimestamps(t *testing.T) {
	s := openTestStore(t)
	ctx := t.Context()
	if err := s.CreateOwner(ctx, "owner", "hash"); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSession(ctx, "token", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	rec, err := s.CreateRecipe(ctx, &capture.Recipe{Title: "Soup"}, []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		"UPDATE owner SET created_at = 'invalid'",
		"UPDATE session SET expires_at = 'invalid'",
		"UPDATE recipe SET captured_at = 'invalid'",
	} {
		if _, err := s.db.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Owner(ctx); err == nil {
		t.Error("Owner accepted an invalid timestamp")
	}
	if _, err := s.SessionValid(ctx, "token"); err == nil {
		t.Error("SessionValid accepted an invalid timestamp")
	}
	if _, err := s.GetRecipe(ctx, rec.ID); err == nil {
		t.Error("GetRecipe accepted an invalid timestamp")
	}
	if _, _, err := s.ListRecipes(ctx, 50, 0); err == nil {
		t.Error("ListRecipes accepted an invalid timestamp")
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

// Run the identical contract against both backends. Each Postgres test gets an
// isolated database through pgtestdb on the package's disposable test container.
func forEachStore(t *testing.T, test func(*testing.T, Store)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) { test(t, openTestStore(t)) })
	t.Run("postgres", func(t *testing.T) {
		t.Parallel()
		test(t, openTestPostgres(t))
	})
}

func TestLayoutQueries(t *testing.T) {
	forEachStore(t, testLayoutQueries)
}

// layoutEqual compares stored layout JSON structurally: Postgres JSONB
// reformats on write (e.g. adds a space after ':'), so byte-for-byte
// comparison would fail on that backend even when the data round-trips
// correctly.
func layoutEqual(t *testing.T, got, want string) bool {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("got is not JSON: %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("want is not JSON: %q: %v", want, err)
	}
	return reflect.DeepEqual(g, w)
}

func testLayoutQueries(t *testing.T, s Store) {
	ctx := context.Background()

	if _, err := s.GetLayoutPreset(ctx, "classic", "desktop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing preset: %v", err)
	}
	if err := s.SaveLayoutPreset(ctx, "classic", "desktop", `{"grid":"a"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLayoutPreset(ctx, "cook", "desktop", `{"grid":"b"}`); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLayoutPreset(ctx, "classic", "phone", `{"grid":"c"}`); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetLayoutPreset(ctx, "classic", "desktop")
	if err != nil || !layoutEqual(t, got.Layout, `{"grid":"a"}`) {
		t.Fatalf("get preset: %+v %v", got, err)
	}

	list, err := s.ListLayoutPresets(ctx, "desktop")
	if err != nil || len(list) != 2 {
		t.Fatalf("list presets: %+v %v", list, err)
	}

	// Overwrite is an upsert, not a duplicate.
	if err := s.SaveLayoutPreset(ctx, "classic", "desktop", `{"grid":"updated"}`); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetLayoutPreset(ctx, "classic", "desktop")
	if err != nil || !layoutEqual(t, got.Layout, `{"grid":"updated"}`) {
		t.Fatalf("preset not updated: %+v %v", got, err)
	}

	if err := s.DeleteLayoutPreset(ctx, "classic", "desktop"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteLayoutPreset(ctx, "classic", "desktop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}

	rec, err := s.CreateRecipe(ctx, &capture.Recipe{Title: "Soup"}, []byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRecipeLayout(ctx, rec.ID, "desktop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing override: %v", err)
	}
	if err := s.SaveRecipeLayout(ctx, rec.ID, "desktop", `{"grid":"override"}`); err != nil {
		t.Fatal(err)
	}
	override, err := s.GetRecipeLayout(ctx, rec.ID, "desktop")
	if err != nil || !layoutEqual(t, override, `{"grid":"override"}`) {
		t.Fatalf("get override: %q %v", override, err)
	}
	if err := s.DeleteRecipeLayout(ctx, rec.ID, "desktop"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteRecipeLayout(ctx, rec.ID, "desktop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing override delete: %v", err)
	}

	if _, err := s.GetPref(ctx, "active_preset:desktop"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing pref: %v", err)
	}
	if err := s.SetPref(ctx, "active_preset:desktop", `"cook"`); err != nil {
		t.Fatal(err)
	}
	value, err := s.GetPref(ctx, "active_preset:desktop")
	if err != nil || value != `"cook"` {
		t.Fatalf("get pref: %q %v", value, err)
	}
	if err := s.SetPref(ctx, "active_preset:desktop", `"classic"`); err != nil {
		t.Fatal(err)
	}
	value, err = s.GetPref(ctx, "active_preset:desktop")
	if err != nil || value != `"classic"` {
		t.Fatalf("pref not updated: %q %v", value, err)
	}
}
