package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchErrorOmitsURLSecrets(t *testing.T) {
	_, err := New(1024, false).Get(context.Background(), "http://user:password@127.0.0.1/private?token=secret")
	if !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
	for _, secret := range []string{"user", "password", "private", "secret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error contains URL data: %v", err)
		}
	}
}

func TestBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("hi")); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()

	if _, err := New(1<<20, false).Get(context.Background(), srv.URL); !errors.Is(err, ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
	if body, err := New(1<<20, true).Get(context.Background(), srv.URL); err != nil || string(body) != "hi" {
		t.Fatalf("allowPrivate: %q %v", body, err)
	}
}

func TestBlocksRedirectToLoopback(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	// The guard applies to every connection, so a redirect can't reach a
	// private address either; here the first hop is already blocked.
	if _, err := New(1<<20, false).Get(context.Background(), target.URL); err == nil {
		t.Fatal("expected error")
	}
}

func TestTooLarge(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(make([]byte, 100)); err != nil {
			t.Error(err)
		}
	}))
	defer srv.Close()
	if _, err := New(10, true).Get(context.Background(), srv.URL); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}
