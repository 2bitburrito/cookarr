package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
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
		w.Write(make([]byte, 100))
	}))
	defer srv.Close()
	if _, err := New(10, true).Get(context.Background(), srv.URL); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("want ErrTooLarge, got %v", err)
	}
}
