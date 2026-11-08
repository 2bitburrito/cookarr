// Run from the repository root: go run ./scripts/reset-password.go
// Uses the same database and logging environment variables as the server.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/hpalmer/cookarr/internal/auth"
	"github.com/hpalmer/cookarr/internal/config"
	"github.com/hpalmer/cookarr/internal/store"
	"github.com/hpalmer/cookarr/internal/types"
	"golang.org/x/term"
)

func main() {
	if err := resetPassword(); err != nil {
		slog.Error("password reset failed", "err", err)
		os.Exit(1)
	}
}

func resetPassword() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	slog.SetDefault(cfg.Logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := store.Open(ctx, cfg.DBDriver, cfg.DBPath, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "err", err)
		}
	}()
	ctx = context.Background()
	if _, err := db.Owner(ctx); errors.Is(err, store.ErrNotFound) {
		return errors.New("no owner yet; complete setup in the web UI first")
	} else if err != nil {
		return err
	}
	pw, err := readPassword()
	if err != nil {
		return err
	}
	if len(pw) < types.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", types.MinPasswordLength)
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	if err := db.SetPassword(ctx, hash); err != nil {
		return err
	}
	slog.Info("password updated; all sessions revoked")
	return nil
}

func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return "", err
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	fmt.Fprint(os.Stderr, "New password: ")
	a, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	fmt.Fprint(os.Stderr, "Confirm password: ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	if string(a) != string(b) {
		return "", errors.New("passwords do not match")
	}
	return string(a), nil
}
