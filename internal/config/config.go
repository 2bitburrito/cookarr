// Package config reads server settings from the environment.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
)

type Config struct {
	Addr         string
	DBPath       string
	DBDriver     string
	DatabaseURL  string
	AuthDisabled bool
	// MaxBodyBytes caps import request bodies and fetched pages.
	MaxBodyBytes int64
	LogJSON      bool
	LogLevel     slog.Level
	Logger       *slog.Logger
}

func Load() (Config, error) {
	c := Config{
		Addr:         env("ADDR", ":8080"),
		DBPath:       env("DB", "data/cookarr.db"),
		DBDriver:     env("DB_DRIVER", "sqlite"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		MaxBodyBytes: 16 << 20,
	}

	switch c.DBDriver {
	case "sqlite":
	case "postgres":
		if c.DatabaseURL == "" {
			return c, fmt.Errorf("DATABASE_URL is required when DB_DRIVER=postgres")
		}
	default:
		return c, fmt.Errorf("DB_DRIVER must be sqlite or postgres")
	}
	var err error
	if err := c.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "INFO"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if c.AuthDisabled, err = envBool("AUTH_DISABLED"); err != nil {
		return c, err
	}

	if c.LogJSON, err = envBool("LOG_JSON"); err != nil {
		return c, err
	}

	if v := os.Getenv("MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("MAX_BODY_BYTES: invalid value %q", v)
		}
		c.MaxBodyBytes = n
	}
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if c.LogJSON {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	c.Logger = slog.New(handler)
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string) (bool, error) {
	v := os.Getenv(key)
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: invalid boolean %q", key, v)
	}
	return b, nil
}
