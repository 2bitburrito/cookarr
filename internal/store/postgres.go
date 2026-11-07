package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"github.com/hpalmer/cookarr/internal/store/postgresdb"
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
		db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}
	return s, nil
}

func (s *Postgres) migrate(ctx context.Context) error {
	migrations, err := fs.Sub(postgresMigrations, "migrations/postgres")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, s.db, migrations)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
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

func (s *Postgres) SetPassword(ctx context.Context, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
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
		if p.Role == capture.PhotoHero {
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
