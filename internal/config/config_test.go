package config

import (
	"log/slog"
	"os"
	"strings"
	"testing"
)

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for _, key := range []string{"ADDR", "DB", "DB_DRIVER", "DATABASE_URL", "AUTH_DISABLED", "MAX_BODY_BYTES", "LOG_JSON", "LOG_LEVEL"} {
		t.Setenv(key, "")
	}
}

func TestLogLevel(t *testing.T) {
	isolateConfig(t)
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
	isolateConfig(t)
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

func TestDefaults(t *testing.T) {
	isolateConfig(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":1347" || c.DBPath != "data/cookarr.db" || c.DBDriver != "sqlite" || c.DatabaseURL != "" || c.AuthDisabled || c.LogJSON || c.MaxBodyBytes != 16<<20 || c.LogLevel != slog.LevelInfo || c.Logger == nil {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestDotEnv(t *testing.T) {
	isolateConfig(t)
	contents := "ADDR=:9090\nDB=custom.db\nDB_DRIVER=postgres\nDATABASE_URL=postgres://localhost/cookarr\nAUTH_DISABLED=\"true\"\nLOG_JSON=1\nLOG_LEVEL=debug\nMAX_BODY_BYTES=1024\n"
	if err := os.WriteFile(".env", []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Addr != ":9090" || c.DBPath != "custom.db" || c.DBDriver != "postgres" || c.DatabaseURL != "postgres://localhost/cookarr" || !c.AuthDisabled || !c.LogJSON || c.LogLevel != slog.LevelDebug || c.MaxBodyBytes != 1024 {
		t.Fatalf("unexpected .env config: %+v", c)
	}

	t.Setenv("AUTH_DISABLED", "false")
	t.Setenv("LOG_JSON", "false")
	t.Setenv("ADDR", ":7070")
	t.Setenv("MAX_BODY_BYTES", "2048")
	c, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.AuthDisabled || c.LogJSON || c.Addr != ":7070" || c.MaxBodyBytes != 2048 {
		t.Fatalf("environment did not override .env: %+v", c)
	}
}

func TestInvalidSettings(t *testing.T) {
	for _, tt := range []struct{ key, value string }{
		{"AUTH_DISABLED", "maybe"},
		{"LOG_JSON", "maybe"},
		{"MAX_BODY_BYTES", "lots"},
		{"MAX_BODY_BYTES", "0"},
		{"MAX_BODY_BYTES", "-1"},
	} {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			isolateConfig(t)
			t.Setenv(tt.key, tt.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("Load() error = %v, want error naming %s", err, tt.key)
			}
		})
	}
}

func TestInvalidDotEnv(t *testing.T) {
	isolateConfig(t)
	if err := os.WriteFile(".env", []byte("AUTH_DISABLED=\"unterminated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted malformed .env")
	}
}
