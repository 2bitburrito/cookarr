// Package fetch downloads pages on the server's behalf, refusing to connect to
// private, loopback or link-local addresses.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

var (
	ErrTooLarge = errors.New("fetch: response too large")
	ErrBlocked  = errors.New("fetch: address not allowed")
)

type Fetcher struct {
	client   *http.Client
	maxBytes int64
}

// New returns a Fetcher that reads at most maxBytes per page. allowPrivate
// disables the address guard and exists only for tests.
func New(maxBytes int64, allowPrivate bool) *Fetcher {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if !allowPrivate {
		// Control runs after DNS resolution, on every connection including
		// redirect targets, so it can't be bypassed with a rebinding hostname.
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return err
			}
			if blocked(ap.Addr()) {
				slog.Debug("fetch connection blocked", "component", "fetch")
				return ErrBlocked
			}
			return nil
		}
	}

	return &Fetcher{
		maxBytes: maxBytes,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext:         dialer.DialContext,
				TLSHandshakeTimeout: 10 * time.Second,
				Proxy:               nil,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				slog.DebugContext(req.Context(), "following redirect", "component", "fetch", "host", req.URL.Hostname(), "redirects", len(via))
				if len(via) >= 5 {
					return errors.New("fetch: too many redirects")
				}
				return nil
			},
		},
	}
}

func blocked(a netip.Addr) bool {
	a = a.Unmap()
	return a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsLinkLocalMulticast() || a.IsMulticast() || a.IsUnspecified() ||
		a.IsInterfaceLocalMulticast()
}

// Get returns the body of an http(s) URL.
func (f *Fetcher) Get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Cookarr/0.1)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	start := time.Now()
	log := slog.Default().With("component", "fetch", "host", req.URL.Hostname())
	log.DebugContext(ctx, "fetch started", "max_bytes", f.maxBytes)
	resp, err := f.client.Do(req)
	if err != nil {
		// net/http wraps errors with the full URL, which can contain secrets.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		log.DebugContext(ctx, "fetch failed", "duration", time.Since(start))
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WarnContext(ctx, "close fetch response", "err", err)
		}
	}()
	log.DebugContext(ctx, "fetch response received", "status", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("fetch: upstream returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, f.maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > f.maxBytes {
		log.DebugContext(ctx, "fetch size limit exceeded", "max_bytes", f.maxBytes)
		return nil, ErrTooLarge
	}
	log.DebugContext(ctx, "fetch completed", "bytes", len(body), "duration", time.Since(start))
	return body, nil
}
