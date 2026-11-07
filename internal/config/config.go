// Package config reads server settings from the environment.
package config

import (
	"fmt"
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
}

func Load() (Config, error) {
	c := Config{
		Addr:         env("COOKARR_ADDR", ":8080"),
		DBPath:       env("COOKARR_DB", "data/cookarr.db"),
		DBDriver:     env("COOKARR_DB_DRIVER", "sqlite"),
		DatabaseURL:  os.Getenv("COOKARR_DATABASE_URL"),
		MaxBodyBytes: 16 << 20,
	}

	switch c.DBDriver {
	case "sqlite":
	case "postgres":
		if c.DatabaseURL == "" {
			return c, fmt.Errorf("COOKARR_DATABASE_URL is required when COOKARR_DB_DRIVER=postgres")
		}
	default:
		return c, fmt.Errorf("COOKARR_DB_DRIVER must be sqlite or postgres")
	}
	var err error
	if c.AuthDisabled, err = envBool("AUTH_DISABLED"); err != nil {
		return c, err
	}

	if c.LogJSON, err = envBool("COOKARR_LOG_JSON"); err != nil {
		return c, err
	}

	if v := os.Getenv("COOKARR_MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return c, fmt.Errorf("COOKARR_MAX_BODY_BYTES: invalid value %q", v)
		}
		c.MaxBodyBytes = n
	}
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
