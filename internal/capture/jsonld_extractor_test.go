package capture

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hpalmer/cookarr/internal/types"
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
			page, err := os.ReadFile(filepath.Join("testdata/fixtures", fixture.name+".html"))
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

func extractJSONLD(t *testing.T, recipeJSON string) *Structured {
	t.Helper()
	page := []byte(`<html><head><script type="application/ld+json">` + recipeJSON + `</script></head><body></body></html>`)
	recipe, err := (Extractor{}).Extract("https://example.test/r", page)
	if err != nil {
		t.Fatal(err)
	}
	return recipe
}

func TestIngredientsAreNotSplitOnCommas(t *testing.T) {
	r := extractJSONLD(t, `{"@type":"Recipe","name":"X","recipeIngredient":["1 cup flour, sifted","2 eggs, beaten"]}`)
	want := []string{"1 cup flour, sifted", "2 eggs, beaten"}
	if len(r.Ingredients) != len(want) {
		t.Fatalf("ingredients = %+v, want %v", r.Ingredients, want)
	}
	for i, w := range want {
		if r.Ingredients[i].Text != w {
			t.Errorf("ingredient %d = %q, want %q", i, r.Ingredients[i].Text, w)
		}
	}
}

func TestKeywordsAreStillSplitOnCommas(t *testing.T) {
	r := extractJSONLD(t, `{"@type":"Recipe","name":"X","keywords":"soup, winter,easy"}`)
	if len(r.Keywords) != 3 || r.Keywords[1] != "winter" {
		t.Errorf("keywords = %v", r.Keywords)
	}
}

func TestStepTextWinsOverName(t *testing.T) {
	tests := []struct {
		name string
		json string
		want []string
	}{
		{"text and name", `[{"@type":"HowToStep","name":"Mix","text":"Whisk the flour and eggs."}]`, []string{"Whisk the flour and eggs."}},
		{"name only", `[{"@type":"HowToStep","name":"Bake until golden."}]`, []string{"Bake until golden."}},
		{"text only", `[{"@type":"HowToStep","text":"Cool on a rack."}]`, []string{"Cool on a rack."}},
		{"inside section", `[{"@type":"HowToSection","name":"Dough","itemListElement":[{"@type":"HowToStep","name":"Knead","text":"Knead for 10 minutes."}]}]`, []string{"Knead for 10 minutes."}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := extractJSONLD(t, `{"@type":"Recipe","name":"X","recipeInstructions":`+tt.json+`}`)
			if len(r.Steps) != len(tt.want) {
				t.Fatalf("steps = %+v, want %v", r.Steps, tt.want)
			}
			for i, w := range tt.want {
				if r.Steps[i].Text != w {
					t.Errorf("step %d = %q, want %q", i, r.Steps[i].Text, w)
				}
			}
		})
	}
}

func TestSectionNameIsKeptAsStepGroup(t *testing.T) {
	r := extractJSONLD(t, `{"@type":"Recipe","name":"X","recipeInstructions":[{"@type":"HowToSection","name":"Dough","itemListElement":[{"@type":"HowToStep","name":"Knead","text":"Knead."}]}]}`)
	if len(r.Steps) != 1 || r.Steps[0].Group != "Dough" {
		t.Errorf("steps = %+v", r.Steps)
	}
}

func TestYieldShapes(t *testing.T) {
	tests := []struct{ name, json, want string }{
		{"string", `"4 servings"`, "4 servings"},
		{"number", `4`, "4"},
		{"array prefers words", `["4","4 servings"]`, "4 servings"},
		{"array of numbers", `["6"]`, "6"},
		{"array words first", `["12 cookies","12"]`, "12 cookies"},
		{"empty array", `[]`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := extractJSONLD(t, `{"@type":"Recipe","name":"X","recipeYield":`+tt.json+`}`)
			if r.Yield != tt.want {
				t.Errorf("yield = %q, want %q", r.Yield, tt.want)
			}
		})
	}
}

func TestInLanguageObject(t *testing.T) {
	r := extractJSONLD(t, `{"@type":"Recipe","name":"X","inLanguage":{"@type":"Language","name":"English"}}`)
	if r.Language != "English" {
		t.Errorf("language = %q", r.Language)
	}
}

func TestCleanText(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain text", "plain text"},
		{"  padded \n whitespace\t ", "padded whitespace"},
		{"2 eggs &amp; a yolk", "2 eggs & a yolk"},
		{"Don&#39;t overmix", "Don't overmix"},
		{"1&frac12; cups", "1\u00bd cups"},
		{"<p>Mix well.</p><p>Bake.</p>", "Mix well. Bake."},
		{"Stir<br>then<br/>rest", "Stir then rest"},
		{"&lt;p&gt;Escaped markup&lt;/p&gt;", "Escaped markup"},
		{"<strong>Bold</strong> step", "Bold step"},
		{"bake below 200 < 250 degrees", "bake below 200 < 250 degrees"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := cleanText(tt.in); got != tt.want {
			t.Errorf("cleanText(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractedTextIsDecodedAndStripped(t *testing.T) {
	r := extractJSONLD(t, `{"@type":"Recipe","name":"Mac &amp; Cheese",
	  "recipeIngredient":["2 eggs &amp; a yolk","1&#8260;2 cup milk"],
	  "recipeInstructions":[{"@type":"HowToStep","text":"<p>Boil the pasta.</p><p>Don&#39;t drain.</p>"},"Add <b>cheese</b>."],
	  "author":{"name":"Ben &amp; Jerry"}}`)
	if r.Title != "Mac & Cheese" {
		t.Errorf("title = %q", r.Title)
	}
	if r.Ingredients[0].Text != "2 eggs & a yolk" || r.Ingredients[1].Text != "1\u20442 cup milk" {
		t.Errorf("ingredients = %+v", r.Ingredients)
	}
	if len(r.Steps) != 2 || r.Steps[0].Text != "Boil the pasta. Don't drain." || r.Steps[1].Text != "Add cheese." {
		t.Errorf("steps = %+v", r.Steps)
	}
	if r.AuthorName != "Ben & Jerry" {
		t.Errorf("author = %q", r.AuthorName)
	}
}

func TestSourceReportsWhereFieldsCameFrom(t *testing.T) {
	tests := []struct{ name, page, want string }{
		{"jsonld", `<script type="application/ld+json">{"@type":"Recipe","name":"X","recipeIngredient":["1 egg"],"recipeInstructions":["Boil."]}</script>`, types.SourceJSONLD},
		{"jsonld plus headings", `<script type="application/ld+json">{"@type":"Recipe","name":"X"}</script><h2>Ingredients</h2><ul><li>1 egg</li></ul><h2>Method</h2><ol><li>Boil.</li></ol>`, types.SourceJSONLDHeadings},
		{"headings only", `<h2>Ingredients</h2><ul><li>1 egg</li></ul><h2>Method</h2><ol><li>Boil.</li></ol>`, types.SourceHeadings},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := (Extractor{}).Extract("https://example.test/r", []byte(tt.page))
			if err != nil {
				t.Fatal(err)
			}
			if r.Source != tt.want {
				t.Errorf("source = %q, want %q", r.Source, tt.want)
			}
		})
	}
}
