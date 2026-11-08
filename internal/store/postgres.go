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
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store/postgresdb"
	"github.com/hpalmer/cookarr/internal/types"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/postgres/*.sql
var postgresMigrations embed.FS

type Postgres struct {
	db      *sql.DB
	queries *postgresdb.Queries
}

var _ Store = (*Postgres)(nil)

// OpenPostgres connects to an existing database and applies pending migrations.
// The caller supplies a connection URL or pgx connection string.
func OpenPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	s := &Postgres{db: db, queries: postgresdb.New(db)}
	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("connect postgres: %w", err), db.Close())
	}
	if err := s.migrate(ctx); err != nil {
		return nil, errors.Join(fmt.Errorf("migrate postgres: %w", err), db.Close())
	}
	return s, nil
}

func (s *Postgres) migrate(ctx context.Context) error {
	migrations, err := fs.Sub(postgresMigrations, "migrations/postgres")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, s.db, migrations, goose.WithSlog(slog.Default()))
	if err != nil {
		return err
	}
	applied, err := provider.Up(ctx)
	slog.DebugContext(ctx, "database migrations completed", "driver", "postgres", "applied", len(applied), "success", err == nil)
	return err
}

func (s *Postgres) Close() error { return s.db.Close() }

func (s *Postgres) OwnerExists(ctx context.Context) (bool, error) {
	n, err := s.queries.CountOwners(ctx)
	return n > 0, err
}

func (s *Postgres) CreateOwner(ctx context.Context, username, passwordHash string) error {
	err := s.queries.CreateOwner(ctx, postgresdb.CreateOwnerParams{
		Username: username, PasswordHash: passwordHash, CreatedAt: time.Now().UTC(),
	})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" && pe.ConstraintName == "owner_pkey" {
		return ErrOwnerExists
	}
	return err
}

func (s *Postgres) Owner(ctx context.Context) (Owner, error) {
	row, err := s.queries.GetOwner(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return Owner{}, ErrNotFound
	}
	return Owner{Username: row.Username, PasswordHash: row.PasswordHash, CreatedAt: row.CreatedAt.UTC()}, err
}

func (s *Postgres) SetPassword(ctx context.Context, passwordHash string) (err error) {
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

func (s *Postgres) CreateSession(ctx context.Context, tokenHash string, expires time.Time) error {
	now := time.Now().UTC()
	if err := s.queries.DeleteExpiredSessions(ctx, now); err != nil {
		return err
	}
	return s.queries.CreateSession(ctx, postgresdb.CreateSessionParams{
		TokenHash: tokenHash, CreatedAt: now, ExpiresAt: expires.UTC(),
	})
}

func (s *Postgres) SessionValid(ctx context.Context, tokenHash string) (bool, error) {
	expires, err := s.queries.GetSessionExpiry(ctx, tokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return expires.After(time.Now()), nil
}

func (s *Postgres) DeleteSession(ctx context.Context, tokenHash string) error {
	return s.queries.DeleteSession(ctx, tokenHash)
}

func (s *Postgres) CreateRecipe(ctx context.Context, r *capture.Recipe, strippedHTML []byte) (Recipe, error) {
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
	now := time.Now().UTC()
	id, err := s.queries.CreateRecipe(ctx, postgresdb.CreateRecipeParams{
		Title: r.Title, Description: r.Description, SourceUrl: r.SourceURL,
		SiteName: r.SiteName, HeroUrl: hero, Data: data,
		StrippedHtml: strippedHTML, CapturedAt: now,
	})
	if err != nil {
		return Recipe{}, err
	}
	return Recipe{ID: id, CapturedAt: now, Recipe: *r}, nil
}

func (s *Postgres) GetRecipe(ctx context.Context, id int64) (Recipe, error) {
	row, err := s.queries.GetRecipe(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, err
	}
	rec := Recipe{ID: id, CapturedAt: row.CapturedAt.UTC()}
	if err := json.Unmarshal(row.Data, &rec.Recipe); err != nil {
		return Recipe{}, fmt.Errorf("decode recipe %d: %w", id, err)
	}
	return rec, nil
}

func (s *Postgres) ListRecipes(ctx context.Context, limit, offset int) ([]Summary, int, error) {
	total, err := s.queries.CountRecipes(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.queries.ListRecipes(ctx, postgresdb.ListRecipesParams{PageLimit: int64(limit), PageOffset: int64(offset)})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Summary, 0, len(rows))
	for _, row := range rows {
		items = append(items, Summary{
			ID: row.ID, Title: row.Title, Description: row.Description,
			SourceURL: row.SourceUrl, SiteName: row.SiteName, HeroURL: row.HeroUrl,
			CapturedAt: row.CapturedAt.UTC(),
		})
	}
	return items, int(total), nil
}

func (s *Postgres) DeleteRecipe(ctx context.Context, id int64) error {
	n, err := s.queries.DeleteRecipe(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Postgres) ListLayoutPresets(ctx context.Context, deviceClass string) ([]LayoutPreset, error) {
	rows, err := s.queries.ListLayoutPresets(ctx, deviceClass)
	if err != nil {
		return nil, err
	}
	out := make([]LayoutPreset, 0, len(rows))
	for _, row := range rows {
		out = append(out, LayoutPreset{
			Name: row.Name, DeviceClass: row.DeviceClass, Layout: string(row.LayoutJson), UpdatedAt: row.UpdatedAt.UTC(),
		})
	}
	return out, nil
}

func (s *Postgres) GetLayoutPreset(ctx context.Context, name, deviceClass string) (LayoutPreset, error) {
	row, err := s.queries.GetLayoutPreset(ctx, postgresdb.GetLayoutPresetParams{Name: name, DeviceClass: deviceClass})
	if errors.Is(err, sql.ErrNoRows) {
		return LayoutPreset{}, ErrNotFound
	}
	if err != nil {
		return LayoutPreset{}, err
	}
	return LayoutPreset{
		Name: row.Name, DeviceClass: row.DeviceClass, Layout: string(row.LayoutJson), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func (s *Postgres) SaveLayoutPreset(ctx context.Context, name, deviceClass, layoutJSON string) error {
	return s.queries.UpsertLayoutPreset(ctx, postgresdb.UpsertLayoutPresetParams{
		Name: name, DeviceClass: deviceClass, LayoutJson: json.RawMessage(layoutJSON), UpdatedAt: time.Now().UTC(),
	})
}

func (s *Postgres) DeleteLayoutPreset(ctx context.Context, name, deviceClass string) error {
	n, err := s.queries.DeleteLayoutPreset(ctx, postgresdb.DeleteLayoutPresetParams{Name: name, DeviceClass: deviceClass})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Postgres) GetRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) (string, error) {
	row, err := s.queries.GetRecipeLayout(ctx, postgresdb.GetRecipeLayoutParams{RecipeID: recipeID, DeviceClass: deviceClass})
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(row.LayoutJson), nil
}

func (s *Postgres) SaveRecipeLayout(ctx context.Context, recipeID int64, deviceClass, layoutJSON string) error {
	return s.queries.UpsertRecipeLayout(ctx, postgresdb.UpsertRecipeLayoutParams{
		RecipeID: recipeID, DeviceClass: deviceClass, LayoutJson: json.RawMessage(layoutJSON), UpdatedAt: time.Now().UTC(),
	})
}

func (s *Postgres) DeleteRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) error {
	n, err := s.queries.DeleteRecipeLayout(ctx, postgresdb.DeleteRecipeLayoutParams{RecipeID: recipeID, DeviceClass: deviceClass})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Postgres) GetPref(ctx context.Context, key string) (string, error) {
	value, err := s.queries.GetPref(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return string(value), nil
}

func (s *Postgres) SetPref(ctx context.Context, key, value string) error {
	return s.queries.SetPref(ctx, postgresdb.SetPrefParams{Key: key, Value: json.RawMessage(value), UpdatedAt: time.Now().UTC()})
}
