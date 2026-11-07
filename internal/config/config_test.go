package config

import "testing"

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
			t.Setenv("COOKARR_DB_DRIVER", tt.driver)
			t.Setenv("COOKARR_DATABASE_URL", tt.url)
			t.Setenv("COOKARR_DB", "")
			t.Setenv("AUTH_DISABLED", "")
			t.Setenv("COOKARR_LOG_JSON", "")
			t.Setenv("COOKARR_MAX_BODY_BYTES", "")
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
