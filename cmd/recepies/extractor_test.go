package recepies

import (
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
