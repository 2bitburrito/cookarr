// Package config reads server settings from the environment and an optional .env file.
package config

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	Addr         string `mapstructure:"ADDR"`
	DBPath       string `mapstructure:"DB"`
	DBDriver     string `mapstructure:"DB_DRIVER"`
	DatabaseURL  string `mapstructure:"DATABASE_URL"`
	AuthDisabled bool   `mapstructure:"AUTH_DISABLED"`
	// MaxBodyBytes caps import request bodies and fetched pages.
	MaxBodyBytes int64        `mapstructure:"MAX_BODY_BYTES"`
	LogJSON      bool         `mapstructure:"LOG_JSON"`
	LogLevel     slog.Level   `mapstructure:"-"`
	Logger       *slog.Logger `mapstructure:"-"`
}

func Load() (Config, error) {
	v := viper.New()
	v.SetConfigFile(".env")
	v.SetConfigType("env")
	v.AutomaticEnv()
	v.SetDefault("ADDR", ":1347")
	v.SetDefault("DB", "data/cookarr.db")
	v.SetDefault("DB_DRIVER", "sqlite")
	v.SetDefault("DATABASE_URL", "")
	v.SetDefault("AUTH_DISABLED", false)
	v.SetDefault("MAX_BODY_BYTES", int64(16<<20))
	v.SetDefault("LOG_JSON", false)
	v.SetDefault("LOG_LEVEL", "INFO")

	var c Config
	if err := v.ReadInConfig(); err != nil && !os.IsNotExist(err) {
		return c, fmt.Errorf("read .env: %w", err)
	}
	// Decode rather than using GetBool/GetInt64, which silently return zero
	// values for malformed settings.
	if err := v.Unmarshal(&c); err != nil {
		return c, fmt.Errorf("decode configuration: %w", err)
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
	if err := c.LogLevel.UnmarshalText([]byte(v.GetString("LOG_LEVEL"))); err != nil {
		return c, fmt.Errorf("LOG_LEVEL: %w", err)
	}
	if c.MaxBodyBytes <= 0 {
		return c, fmt.Errorf("MAX_BODY_BYTES must be positive")
	}
	opts := &slog.HandlerOptions{Level: c.LogLevel}
	var handler slog.Handler = slog.NewTextHandler(os.Stderr, opts)
	if c.LogJSON {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}
	c.Logger = slog.New(handler)
	return c, nil
}
