package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpalmer/cookarr/internal/auth"
	"github.com/hpalmer/cookarr/internal/store"
)

func TestResetPasswordRevokesSessions(t *testing.T) {
	t.Setenv("DB_DRIVER", "sqlite")
	path := filepath.Join(t.TempDir(), "cookarr.db")
	t.Setenv("DB", path)
	t.Setenv("LOG_LEVEL", "ERROR")
	t.Setenv("LOG_JSON", "false")
	t.Setenv("AUTH_DISABLED", "false")
	t.Setenv("MAX_BODY_BYTES", "")
	oldLog := slog.Default()
	t.Cleanup(func() { slog.SetDefault(oldLog) })
	db, err := store.OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	hash, err := auth.HashPassword("old-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.CreateOwner(ctx, "owner", hash); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(ctx, "old-session", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	input, err := os.CreateTemp(t.TempDir(), "password")
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	if _, err := input.WriteString("new-password\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	oldStdin := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = oldStdin })
	if err := resetPassword(); err != nil {
		t.Fatal(err)
	}
	owner, err := db.Owner(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := auth.VerifyPassword("new-password", owner.PasswordHash); err != nil || !ok {
		t.Fatalf("new password was not installed: %v", err)
	}
	if valid, err := db.SessionValid(ctx, "old-session"); err != nil || valid {
		t.Fatalf("old session was not revoked: valid=%v err=%v", valid, err)
	}
}
