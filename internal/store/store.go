// Package store persists the owner, sessions and recipes behind an interface,
// so a Postgres implementation can sit beside the SQLite one.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
)

var (
	ErrNotFound    = errors.New("store: not found")
	ErrOwnerExists = errors.New("store: owner already exists")
)

type Owner struct {
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Recipe is a stored capture.
type Recipe struct {
	ID         int64     `json:"id"`
	CapturedAt time.Time `json:"captured_at"`
	capture.Recipe
}

// Summary is the list view of a recipe.
type Summary struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	SourceURL   string    `json:"source_url"`
	SiteName    string    `json:"site_name,omitempty"`
	HeroURL     string    `json:"hero_url,omitempty"`
	CapturedAt  time.Time `json:"captured_at"`
}

type Store interface {
	OwnerExists(ctx context.Context) (bool, error)
	// CreateOwner returns ErrOwnerExists if an owner is already set.
	CreateOwner(ctx context.Context, username, passwordHash string) error
	// Owner returns ErrNotFound if setup hasn't happened.
	Owner(ctx context.Context) (Owner, error)
	// SetPassword replaces the owner's password hash and revokes all sessions.
	SetPassword(ctx context.Context, passwordHash string) error

	CreateSession(ctx context.Context, tokenHash string, expires time.Time) error
	// SessionValid reports whether an unexpired session has this token hash.
	SessionValid(ctx context.Context, tokenHash string) (bool, error)
	DeleteSession(ctx context.Context, tokenHash string) error

	// CreateRecipe always inserts a new row; duplicates by URL are allowed.
	CreateRecipe(ctx context.Context, r *capture.Recipe, strippedHTML []byte) (Recipe, error)
	GetRecipe(ctx context.Context, id int64) (Recipe, error)
	ListRecipes(ctx context.Context, limit, offset int) (items []Summary, total int, err error)
	DeleteRecipe(ctx context.Context, id int64) error

	Close() error
}
