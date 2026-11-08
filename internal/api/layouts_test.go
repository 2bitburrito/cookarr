package api_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

func quoteJSON(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestLayoutEndpoints(t *testing.T) {
	c := newClient(t, true, nil)

	// No preset saved yet.
	code, body := c.do("GET", "/api/v1/layout/presets?device_class=desktop", "")
	expect(t, code, 200, "list presets empty")
	items, ok := body["items"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("expected empty preset list: %v", body)
	}

	code, _ = c.do("PUT", "/api/v1/layout/presets/classic?device_class=desktop", `{"layout":{"grid":"a"}}`)
	expect(t, code, 204, "save preset")

	code, _ = c.do("PUT", "/api/v1/layout/presets/cook?device_class=desktop", `{"layout":{"grid":"b"}}`)
	expect(t, code, 204, "save second preset")

	code, body = c.do("GET", "/api/v1/layout/presets?device_class=desktop", "")
	expect(t, code, 200, "list presets")
	items, ok = body["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("expected 2 presets: %v", body)
	}

	code, _ = c.do("GET", "/api/v1/layout/presets?device_class=bogus", "")
	expect(t, code, 400, "bad device class")

	code, body = c.do("GET", "/api/v1/layout/active?device_class=desktop", "")
	expect(t, code, 200, "active preset default")
	if body["preset"] != "" {
		t.Fatalf("expected no active preset yet: %v", body)
	}

	code, _ = c.do("PUT", "/api/v1/layout/active?device_class=desktop", `{"preset":"cook"}`)
	expect(t, code, 204, "set active preset")
	code, body = c.do("GET", "/api/v1/layout/active?device_class=desktop", "")
	expect(t, code, 200, "active preset set")
	if body["preset"] != "cook" {
		t.Fatalf("active preset not persisted: %v", body)
	}

	code, _ = c.do("DELETE", "/api/v1/layout/presets/classic?device_class=desktop", "")
	expect(t, code, 204, "delete preset")
	code, _ = c.do("DELETE", "/api/v1/layout/presets/classic?device_class=desktop", "")
	expect(t, code, 404, "delete missing preset")

	// Per-recipe layout: resolves to the active preset ("cook") since there's
	// no override, then to an explicit override once one is saved. Needs a
	// real recipe row (recipe_layout has an FK to recipe).
	const minimalRecipeHTML = `<html><head><script type="application/ld+json">
		{"@context":"https://schema.org","@type":"Recipe","name":"Test Soup",
		"recipeIngredient":["water"],"recipeInstructions":["Boil it."]}
		</script></head><body></body></html>`
	code, recipe := c.do("POST", "/api/v1/recipes/import", `{"url":"https://example.com/soup","html":`+quoteJSON(minimalRecipeHTML)+`}`)
	expect(t, code, 201, "import minimal recipe")
	recipeID, ok := recipe["id"].(float64)
	if !ok {
		t.Fatalf("unexpected import response: %v", recipe)
	}
	path := fmt.Sprintf("/api/v1/recipes/%d/layout?device_class=desktop", int(recipeID))

	code, _ = c.do("PUT", path, `{"layout":{"grid":"override"}}`)
	expect(t, code, 204, "save recipe layout override")

	code, body = c.do("GET", path, "")
	expect(t, code, 200, "get recipe layout override")
	if body["source"] != "override" {
		t.Fatalf("expected override source: %v", body)
	}

	code, _ = c.do("DELETE", path, "")
	expect(t, code, 204, "clear recipe layout override")

	code, body = c.do("GET", path, "")
	expect(t, code, 200, "get recipe layout falls back to active preset")
	if body["source"] != "preset" {
		t.Fatalf("expected preset source: %v", body)
	}

	code, _ = c.do("DELETE", path, "")
	expect(t, code, 404, "clear with nothing to clear")
}
