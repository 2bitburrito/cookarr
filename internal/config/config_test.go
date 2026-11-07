package config

import (
	"log/slog"
	"testing"
)

func TestLogLevel(t *testing.T) {
	for _, tt := range []struct {
		value   string
		want    slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"debug", slog.LevelDebug, false},
		{"INFO", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"ERROR", slog.LevelError, false},
		{"verbose", 0, true},
	} {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tt.value)
			t.Setenv("DB_DRIVER", "sqlite")
			cfg, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v", err)
			}
			if !tt.wantErr && cfg.LogLevel != tt.want {
				t.Fatalf("level = %v, want %v", cfg.LogLevel, tt.want)
			}
		})
	}
}

func TestDatabaseConfig(t *testing.T) {
	for _, tt := range []struct {
		name, driver, url, wantDriver string
		wantErr                       bool
	}{
		{name: "default", wantDriver: "sqlite"},
		{name: "sqlite", driver: "sqlite", wantDriver: "sqlite"},
		{name: "postgres", driver: "postgres", url: "postgres://localhost/cookarr", wantDriver: "postgres"},
		{name: "missing postgres URL", driver: "postgres", wantErr: true},
		{name: "unknown", driver: "mysql", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DB_DRIVER", tt.driver)
			t.Setenv("DATABASE_URL", tt.url)
			t.Setenv("DB", "")
			t.Setenv("AUTH_DISABLED", "")
			t.Setenv("LOG_JSON", "")
			t.Setenv("MAX_BODY_BYTES", "")
			c, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v", err)
			}
			if !tt.wantErr && (c.DBDriver != tt.wantDriver || c.DatabaseURL != tt.url || c.DBPath != "data/cookarr.db") {
				t.Fatalf("unexpected database config: %+v", c)
			}
		})
	}
}
