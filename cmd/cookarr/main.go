package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hpalmer/cookarr/internal/api"
	"github.com/hpalmer/cookarr/internal/auth"
	"github.com/hpalmer/cookarr/internal/config"
	"github.com/hpalmer/cookarr/internal/fetch"
	"github.com/hpalmer/cookarr/internal/store"
	"golang.org/x/term"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "reset-password":
		err = resetPassword()
	default:
		fmt.Fprintf(os.Stderr, "usage: cookarr [serve|reset-password]\n")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	if cfg.LogJSON {
		return slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := newLogger(cfg)
	db, err := store.OpenSQLite(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if cfg.AuthDisabled {
		log.Warn("AUTH_DISABLED is set: every request is treated as the owner")
	}
	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: api.New(api.Config{
			Store:        db,
			Fetcher:      fetch.New(cfg.MaxBodyBytes, false),
			Logger:       log,
			AuthDisabled: cfg.AuthDisabled,
			MaxBodyBytes: cfg.MaxBodyBytes,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.Addr, "db", cfg.DBPath)
		errc <- srv.ListenAndServe()
	}()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// resetPassword sets a new owner password and revokes every session. It talks
// to the database directly, so it only works where you can reach the data file
// (e.g. docker exec -it cookarr cookarr reset-password).
func resetPassword() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	db, err := store.OpenSQLite(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.Owner(ctx); errors.Is(err, store.ErrNotFound) {
		return errors.New("no owner yet; complete setup in the web UI first")
	} else if err != nil {
		return err
	}
	pw, err := readPassword()
	if err != nil {
		return err
	}
	if len(pw) < auth.MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", auth.MinPasswordLength)
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	if err := db.SetPassword(ctx, hash); err != nil {
		return err
	}
	fmt.Println("password updated; all sessions revoked")
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
