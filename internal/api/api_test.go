package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/hpalmer/cookarr/internal/api"
	"github.com/hpalmer/cookarr/internal/store"
)

type stubFetcher struct{ page []byte }

func (f stubFetcher) Get(context.Context, string) ([]byte, error) { return f.page, nil }

type client struct {
	t    *testing.T
	http *http.Client
	base string
}

func newClient(t *testing.T, authDisabled bool, page []byte) *client {
	t.Helper()
	db, err := store.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(api.New(api.Config{
		Store:   db,
		Fetcher: stubFetcher{page}, AuthDisabled: authDisabled, MaxBodyBytes: 4 << 20,
	}))
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{t: t, http: &http.Client{Jar: jar}, base: srv.URL}
}

func (c *client) do(method, path, body string) (int, map[string]any) {
	c.t.Helper()
	req, err := http.NewRequestWithContext(c.t.Context(), method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.t.Error(err)
		}
	}()
	var out map[string]any
	if resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			c.t.Fatal(err)
		}
	}
	return resp.StatusCode, out
}

func expect(t *testing.T, got, want int, what string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: status %d, want %d", what, got, want)
	}
}

func TestAuthFlow(t *testing.T) {
	c := newClient(t, false, nil)

	code, body := c.do("GET", "/api/v1/setup", "")
	expect(t, code, 200, "setup status")
	if body["required"] != true {
		t.Fatalf("setup should be required: %v", body)
	}
	code, _ = c.do("GET", "/api/v1/recipes", "")
	expect(t, code, 401, "unauthenticated list")

	code, _ = c.do("POST", "/api/v1/setup", `{"username":"hugh","password":"short"}`)
	expect(t, code, 422, "short password")
	code, _ = c.do("POST", "/api/v1/setup", `{"username":"hugh","password":"a long enough pass"}`)
	expect(t, code, 201, "setup")
	code, _ = c.do("POST", "/api/v1/setup", `{"username":"evil","password":"another long pass"}`)
	expect(t, code, 409, "second setup")

	code, _ = c.do("GET", "/api/v1/recipes", "")
	expect(t, code, 200, "list after setup (session cookie)")

	code, _ = c.do("POST", "/api/v1/auth/logout", "")
	expect(t, code, 204, "logout")
	code, _ = c.do("GET", "/api/v1/recipes", "")
	expect(t, code, 401, "list after logout")

	code, _ = c.do("POST", "/api/v1/auth/login", `{"username":"hugh","password":"wrong password!"}`)
	expect(t, code, 401, "bad login")
	code, _ = c.do("POST", "/api/v1/auth/login", `{"username":"hugh","password":"a long enough pass"}`)
	expect(t, code, 200, "login")
	code, _ = c.do("GET", "/api/v1/recipes", "")
	expect(t, code, 200, "list after login")
}

func TestLoginRateLimit(t *testing.T) {
	c := newClient(t, false, nil)
	c.do("POST", "/api/v1/setup", `{"username":"hugh","password":"a long enough pass"}`)
	for i := 0; i < 5; i++ {
		code, _ := c.do("POST", "/api/v1/auth/login", `{"username":"hugh","password":"nope nope nope"}`)
		expect(t, code, 401, "failed login")
	}
	code, _ := c.do("POST", "/api/v1/auth/login", `{"username":"hugh","password":"a long enough pass"}`)
	expect(t, code, 429, "rate limited")
}

func TestAuthDisabled(t *testing.T) {
	c := newClient(t, true, nil)
	code, body := c.do("GET", "/api/v1/setup", "")
	expect(t, code, 200, "setup status")
	if body["required"] != false {
		t.Fatalf("setup should not be required: %v", body)
	}
	code, _ = c.do("GET", "/api/v1/recipes", "")
	expect(t, code, 200, "list without login")
}

func TestRecipes(t *testing.T) {
	page, err := os.ReadFile("../capture/testdata/fixtures/allrecipes.html")
	if err != nil {
		t.Skip("fixture missing; run mage fixtures")
	}
	c := newClient(t, true, page)
	const u = `"url":"https://www.allrecipes.com/recipe/1/x/"`

	code, _ := c.do("POST", "/api/v1/recipes/import", `{"url":"ftp://x"}`)
	expect(t, code, 422, "bad url")
	code, _ = c.do("POST", "/api/v1/recipes/import", `{`+u+`,"html":""}`)
	expect(t, code, 422, "empty html")

	code, first := c.do("POST", "/api/v1/recipes/import", `{`+u+`}`)
	expect(t, code, 201, "import by url")
	if first["title"] == "" || first["id"] == nil {
		t.Fatalf("unexpected recipe: %v", first)
	}

	htmlJSON, err := json.Marshal(string(page))
	if err != nil {
		t.Fatal(err)
	}
	code, second := c.do("POST", "/api/v1/recipes/import", `{`+u+`,"html":`+string(htmlJSON)+`}`)
	expect(t, code, 201, "import html")
	if first["id"] == second["id"] {
		t.Fatal("re-import should create a new entry")
	}

	code, list := c.do("GET", "/api/v1/recipes?limit=1", "")
	expect(t, code, 200, "list")
	total, totalOK := list["total"].(float64)
	items, itemsOK := list["items"].([]any)
	if !totalOK || !itemsOK || total != 2 || len(items) != 1 {
		t.Fatalf("unexpected list: %v", list)
	}

	id, ok := first["id"].(float64)
	if !ok {
		t.Fatalf("unexpected recipe ID: %v", first["id"])
	}
	path := fmt.Sprintf("/api/v1/recipes/%d", int(id))
	code, got := c.do("GET", path, "")
	expect(t, code, 200, "get")
	if got["title"] != first["title"] {
		t.Fatalf("round trip mismatch: %v vs %v", got["title"], first["title"])
	}
	code, _ = c.do("DELETE", path, "")
	expect(t, code, 204, "delete")
	code, _ = c.do("GET", path, "")
	expect(t, code, 404, "get after delete")
}

func TestCrossOriginRejected(t *testing.T) {
	c := newClient(t, true, nil)
	req, err := http.NewRequestWithContext(t.Context(), "POST", c.base+"/api/v1/recipes/import", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Error(err)
	}
	expect(t, resp.StatusCode, 403, "cross-origin")
}
