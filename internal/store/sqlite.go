package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store/sqlitedb"
	"github.com/hpalmer/cookarr/internal/types"
	"github.com/pressly/goose/v3"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrations embed.FS

type SQLite struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

var _ Store = (*SQLite)(nil)

// OpenSQLite opens (creating if needed) the database at path and applies
// pending migrations. Use ":memory:" for tests.
func OpenSQLite(path string) (*SQLite, error) {
	dsn := path
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		dsn = "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection avoids SQLITE_BUSY between writers, and keeps :memory:
	// as a single database.
	db.SetMaxOpenConns(1)
	s := &SQLite{db: db, queries: sqlitedb.New(db)}
	if err := s.migrate(); err != nil {
		return nil, errors.Join(fmt.Errorf("migrate: %w", err), db.Close())
	}
	return s, nil
}

func (s *SQLite) migrate() error {
	migrations, err := fs.Sub(sqliteMigrations, "migrations/sqlite")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, s.db, migrations, goose.WithSlog(slog.Default()))
	if err != nil {
		return err
	}
	applied, err := provider.Up(context.Background())
	slog.Debug("database migrations completed", "driver", "sqlite", "applied", len(applied), "success", err == nil)
	return err
}

func (s *SQLite) Close() error { return s.db.Close() }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(v string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored timestamp: %w", err)
	}
	return t, nil
}

func (s *SQLite) OwnerExists(ctx context.Context) (bool, error) {
	n, err := s.queries.CountOwners(ctx)
	return n > 0, err
}

func (s *SQLite) CreateOwner(ctx context.Context, username, passwordHash string) error {
	err := s.queries.CreateOwner(ctx, sqlitedb.CreateOwnerParams{
		Username: username, PasswordHash: passwordHash, CreatedAt: ts(time.Now()),
	})
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code()&0xff == sqlite3.SQLITE_CONSTRAINT {
		return ErrOwnerExists
	}
	return err
}

func (s *SQLite) Owner(ctx context.Context) (Owner, error) {
	row, err := s.queries.GetOwner(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Owner{}, ErrNotFound
	}
	if err != nil {
		return Owner{}, err
	}
	createdAt, err := parseTS(row.CreatedAt)
	if err != nil {
		return Owner{}, err
	}
	return Owner{Username: row.Username, PasswordHash: row.PasswordHash, CreatedAt: createdAt}, nil
}

func (s *SQLite) SetPassword(ctx context.Context, passwordHash string) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback password reset: %w", rollbackErr))
		}
	}()
	q := s.queries.WithTx(tx)
	n, err := q.SetPassword(ctx, passwordHash)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := q.DeleteAllSessions(ctx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *SQLite) CreateSession(ctx context.Context, tokenHash string, expires time.Time) error {
	now := time.Now()
	// Opportunistically clear expired sessions.
	if err := s.queries.DeleteExpiredSessions(ctx, ts(now)); err != nil {
		return err
	}
	return s.queries.CreateSession(ctx, sqlitedb.CreateSessionParams{
		TokenHash: tokenHash, CreatedAt: ts(now), ExpiresAt: ts(expires),
	})
}

func (s *SQLite) SessionValid(ctx context.Context, tokenHash string) (bool, error) {
	expires, err := s.queries.GetSessionExpiry(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	expiresAt, err := parseTS(expires)
	if err != nil {
		return false, err
	}
	return expiresAt.After(time.Now()), nil
}

func (s *SQLite) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.queries.DeleteSession(ctx, tokenHash)
}

func (s *SQLite) CreateRecipe(ctx context.Context, r *capture.Recipe, strippedHTML []byte) (Recipe, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return Recipe{}, err
	}
	hero := ""
	for _, p := range r.Photos {
		if p.Role == types.PhotoHero {
			hero = p.URL
			break
		}
	}
	now := time.Now()
	id, err := s.queries.CreateRecipe(ctx, sqlitedb.CreateRecipeParams{
		Title: r.Title, Description: r.Description, SourceUrl: r.SourceURL,
		SiteName: r.SiteName, HeroUrl: hero, Data: string(data),
		StrippedHtml: strippedHTML, CapturedAt: ts(now),
	})
	if err != nil {
		return Recipe{}, err
	}
	return Recipe{ID: id, CapturedAt: now.UTC(), Recipe: *r}, nil
}

func (s *SQLite) GetRecipe(ctx context.Context, id int64) (Recipe, error) {
	row, err := s.queries.GetRecipe(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, err
	}
	capturedAt, err := parseTS(row.CapturedAt)
	if err != nil {
		return Recipe{}, err
	}
	rec := Recipe{ID: id, CapturedAt: capturedAt}
	if err := json.Unmarshal([]byte(row.Data), &rec.Recipe); err != nil {
		return Recipe{}, fmt.Errorf("decode recipe %d: %w", id, err)
	}
	return rec, nil
}

func (s *SQLite) ListRecipes(ctx context.Context, limit, offset int) ([]Summary, int, error) {
	total, err := s.queries.CountRecipes(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.queries.ListRecipes(ctx, sqlitedb.ListRecipesParams{Limit: int64(limit), Offset: int64(offset)})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Summary, 0, len(rows))
	for _, row := range rows {
		capturedAt, err := parseTS(row.CapturedAt)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, Summary{
			ID: row.ID, Title: row.Title, Description: row.Description,
			SourceURL: row.SourceUrl, SiteName: row.SiteName, HeroURL: row.HeroUrl,
			CapturedAt: capturedAt,
		})
	}
	return items, int(total), nil
}

func (s *SQLite) DeleteRecipe(ctx context.Context, id int64) error {
	n, err := s.queries.DeleteRecipe(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) ListLayoutPresets(ctx context.Context, deviceClass string) ([]LayoutPreset, error) {
	rows, err := s.queries.ListLayoutPresets(ctx, deviceClass)
	if err != nil {
		return nil, err
	}
	out := make([]LayoutPreset, 0, len(rows))
	for _, row := range rows {
		updatedAt, err := parseTS(row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, LayoutPreset{
			Name: row.Name, DeviceClass: row.DeviceClass, Layout: row.LayoutJson, UpdatedAt: updatedAt,
		})
	}
	return out, nil
}

func (s *SQLite) GetLayoutPreset(ctx context.Context, name, deviceClass string) (LayoutPreset, error) {
	row, err := s.queries.GetLayoutPreset(ctx, sqlitedb.GetLayoutPresetParams{Name: name, DeviceClass: deviceClass})
	if errors.Is(err, sql.ErrNoRows) {
		return LayoutPreset{}, ErrNotFound
	}
	if err != nil {
		return LayoutPreset{}, err
	}
	updatedAt, err := parseTS(row.UpdatedAt)
	if err != nil {
		return LayoutPreset{}, err
	}
	return LayoutPreset{Name: row.Name, DeviceClass: row.DeviceClass, Layout: row.LayoutJson, UpdatedAt: updatedAt}, nil
}

func (s *SQLite) SaveLayoutPreset(ctx context.Context, name, deviceClass, layoutJSON string) error {
	return s.queries.UpsertLayoutPreset(ctx, sqlitedb.UpsertLayoutPresetParams{
		Name: name, DeviceClass: deviceClass, LayoutJson: layoutJSON, UpdatedAt: ts(time.Now()),
	})
}

func (s *SQLite) DeleteLayoutPreset(ctx context.Context, name, deviceClass string) error {
	n, err := s.queries.DeleteLayoutPreset(ctx, sqlitedb.DeleteLayoutPresetParams{Name: name, DeviceClass: deviceClass})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) GetRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) (string, error) {
	row, err := s.queries.GetRecipeLayout(ctx, sqlitedb.GetRecipeLayoutParams{RecipeID: recipeID, DeviceClass: deviceClass})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return row.LayoutJson, nil
}

func (s *SQLite) SaveRecipeLayout(ctx context.Context, recipeID int64, deviceClass, layoutJSON string) error {
	return s.queries.UpsertRecipeLayout(ctx, sqlitedb.UpsertRecipeLayoutParams{
		RecipeID: recipeID, DeviceClass: deviceClass, LayoutJson: layoutJSON, UpdatedAt: ts(time.Now()),
	})
}

func (s *SQLite) DeleteRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) error {
	n, err := s.queries.DeleteRecipeLayout(ctx, sqlitedb.DeleteRecipeLayoutParams{RecipeID: recipeID, DeviceClass: deviceClass})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLite) GetPref(ctx context.Context, key string) (string, error) {
	value, err := s.queries.GetPref(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return value, err
}

func (s *SQLite) SetPref(ctx context.Context, key, value string) error {
	return s.queries.SetPref(ctx, sqlitedb.SetPrefParams{Key: key, Value: value, UpdatedAt: ts(time.Now())})
}
