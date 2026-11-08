// Package store persists the owner, sessions and recipes through interchangeable
// SQLite and Postgres implementations.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
)

// Open selects a database backend. Connection strings are never logged.
func Open(ctx context.Context, driver, path, databaseURL string) (Store, error) {
	slog.DebugContext(ctx, "opening database", "driver", driver)
	switch driver {
	case "sqlite":
		return OpenSQLite(path)
	case "postgres":
		return OpenPostgres(ctx, databaseURL)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

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

// LayoutPreset is a named, reusable Dockview layout for one device class.
type LayoutPreset struct {
	Name        string    `json:"name"`
	DeviceClass string    `json:"device_class"`
	Layout      string    `json:"layout"` // Dockview JSON, opaque to the server.
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store is the backend-neutral persistence contract shared by SQLite and Postgres.
// The methods form one cohesive application boundary: owner setup, sessions,
// and recipe persistence are consumed together by the server and tested together
// against each backend. Keep this combined interface rather than splitting it
// solely to meet an arbitrary method-count threshold.
//
//nolint:interfacebloat // Deliberately cohesive cross-backend persistence contract.
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

	// ListLayoutPresets returns saved presets for one device class.
	ListLayoutPresets(ctx context.Context, deviceClass string) ([]LayoutPreset, error)
	// GetLayoutPreset returns ErrNotFound if no such preset exists.
	GetLayoutPreset(ctx context.Context, name, deviceClass string) (LayoutPreset, error)
	// SaveLayoutPreset creates or overwrites a named preset for a device class.
	SaveLayoutPreset(ctx context.Context, name, deviceClass, layoutJSON string) error
	// DeleteLayoutPreset returns ErrNotFound if no such preset exists.
	DeleteLayoutPreset(ctx context.Context, name, deviceClass string) error

	// GetRecipeLayout returns the per-recipe layout override, if any, for a
	// device class. Returns ErrNotFound when there is no override.
	GetRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) (string, error)
	SaveRecipeLayout(ctx context.Context, recipeID int64, deviceClass, layoutJSON string) error
	// DeleteRecipeLayout returns ErrNotFound if there was no override.
	DeleteRecipeLayout(ctx context.Context, recipeID int64, deviceClass string) error

	// GetPref returns ErrNotFound when the key has never been set.
	GetPref(ctx context.Context, key string) (string, error)
	SetPref(ctx context.Context, key, value string) error

	Close() error
}
