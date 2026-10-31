package recepies

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractorReadsRecipeFromJSONLDGraph(t *testing.T) {
	html := []byte(`<!doctype html><html><head>
	<script type="application/ld+json">{
	 "@context":"https://schema.org", "@graph":[
	  {"@type":"WebPage","name":"A recipe page"},
	  {"@type":"Recipe","name":"Soup", "description":"Warm soup",
	   "recipeYield":"4 servings", "prepTime":"PT15M", "cookTime":"PT1H",
	   "recipeIngredient":["2 cups stock", "1 onion"],
	   "recipeInstructions":[{"@type":"HowToSection","name":"Prepare","itemListElement":[
	    {"@type":"HowToStep","text":"Chop the onion."},
	    {"@type":"HowToStep","text":"Heat the stock."}]}],
	   "image":["https://example.test/soup.jpg"],
	   "author":{"@type":"Person","name":"A Cook","url":"https://example.test/about"},
	   "sameAs":["https://instagram.com/a-cook"]}
	 ]}</script>
	</head><body></body></html>`)

	recipe, err := (Extractor{}).Extract("https://example.test/soup", html)
	if err != nil {
		t.Fatal(err)
	}
	if recipe.Title != "Soup" || recipe.Description != "Warm soup" {
		t.Fatalf("unexpected basic fields: %+v", recipe)
	}
	if recipe.PrepTime != 15*time.Minute || recipe.CookTime != time.Hour {
		t.Errorf("unexpected durations: prep=%s cook=%s", recipe.PrepTime, recipe.CookTime)
	}
	if len(recipe.Ingredients) != 2 || recipe.Ingredients[1].Text != "1 onion" {
		t.Errorf("unexpected ingredients: %+v", recipe.Ingredients)
	}
	if len(recipe.Steps) != 2 || recipe.Steps[0].Group != "Prepare" || recipe.Steps[1].Text != "Heat the stock." {
		t.Errorf("unexpected steps: %+v", recipe.Steps)
	}
	if len(recipe.Images) != 1 || recipe.Images[0].URL != "https://example.test/soup.jpg" {
		t.Errorf("unexpected images: %+v", recipe.Images)
	}
	if recipe.AuthorName != "A Cook" || recipe.AuthorURL != "https://example.test/about" {
		t.Errorf("unexpected author: %q %q", recipe.AuthorName, recipe.AuthorURL)
	}
	if len(recipe.SameAs) != 1 || recipe.SameAs[0] != "https://instagram.com/a-cook" {
		t.Errorf("unexpected sameAs: %+v", recipe.SameAs)
	}
}

func TestExtractorRejectsArticleOnlyJSONLD(t *testing.T) {
	page := []byte(`<script type="application/ld+json">{"@type":"Article","headline":"Not a recipe"}</script>`)
	if _, err := (Extractor{}).Extract("https://example.test/article", page); err == nil {
		t.Fatal("expected no-recipe error")
	}
}

func TestExtractorUsesHeadingsWhenJSONLDSectionsAreMissing(t *testing.T) {
	page := []byte(`<!doctype html><html><head>
	<script type="application/ld+json">{"@type":"Recipe","name":"Tart"}</script>
	</head><body>
	<h2>Ingredients</h2><ul><li>1 squash</li><li>2 cloves garlic</li></ul>
	<h2>Instructions</h2>
	<div><h3>Step 1</h3><p>Roast the squash.</p></div>
	<div><h3>Step 2</h3><p>Serve warm.</p></div>
	</body></html>`)

	recipe, err := (Extractor{}).Extract("https://example.test/tart", page)
	if err != nil {
		t.Fatal(err)
	}
	if recipe.Title != "Tart" {
		t.Errorf("JSON-LD title was not preserved: %q", recipe.Title)
	}
	if len(recipe.Ingredients) != 2 || recipe.Ingredients[0].Text != "1 squash" {
		t.Errorf("unexpected fallback ingredients: %+v", recipe.Ingredients)
	}
	if len(recipe.Steps) != 2 || recipe.Steps[0].Text != "Roast the squash." || recipe.Steps[1].Text != "Serve warm." {
		t.Errorf("unexpected fallback steps: %+v", recipe.Steps)
	}
}

func TestExtractorUsesHeadingFallbackWithoutJSONLD(t *testing.T) {
	page := []byte(`<h2>Ingredients</h2><p>300 g flour</p><h2>Method</h2><ol><li>Mix the flour.</li></ol>`)
	recipe, err := (Extractor{}).Extract("https://example.test/bread", page)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipe.Ingredients) != 1 || len(recipe.Steps) != 1 {
		t.Errorf("expected heading fallback to find both sections; got %+v", recipe)
	}
}

func TestHeadingFallbackHandlesNigellaAndSaveurFixtures(t *testing.T) {
	fixtures := []struct {
		name string
		url  string
	}{
		{"nigella", "https://www.nigella.com/recipes/coffee-shortbreads"},
		{"saveur", "https://www.saveur.com/article/recipes/seasonal-squash-tart/"},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			page, err := os.ReadFile(filepath.Join("../../internal/capture/testdata/fixtures", fixture.name+".html"))
			if err != nil {
				t.Fatal(err)
			}
			recipe, err := (Extractor{}).Extract(fixture.url, page)
			if err != nil {
				t.Fatal(err)
			}
			if len(recipe.Ingredients) == 0 || len(recipe.Steps) == 0 {
				t.Fatalf("heading fallback did not find both sections: ingredients=%d steps=%d", len(recipe.Ingredients), len(recipe.Steps))
			}
		})
	}
}
