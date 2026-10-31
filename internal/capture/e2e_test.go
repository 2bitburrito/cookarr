package capture_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
)

var markup = regexp.MustCompile(`</?[a-zA-Z][^>]*>|&(amp|lt|gt|quot|#\d+);`)

func parseFixture(t *testing.T, slug, pageURL string) *capture.Recipe {
	t.Helper()
	page, err := os.ReadFile(filepath.Join("testdata", "fixtures", slug+".html"))
	if err != nil {
		t.Skipf("fixture %s missing; run scripts/fetch-fixtures.sh", slug)
	}
	rec, err := capture.Parse(pageURL, page, capture.Options{Extractor: capture.CookarrExtractor{}})
	if err != nil {
		t.Fatalf("Parse(%s): %v (warnings: %v)", slug, err, rec.Warnings)
	}
	return rec
}

// TestParseFixtures runs the whole pipeline (strip, extract, story, author)
// over real pages and checks stable facts about each.
func TestParseFixtures(t *testing.T) {
	tests := []struct {
		slug        string
		url         string
		title       string
		yield       string
		author      string
		ingredients int
		steps       int
		firstIng    string
		firstStep   string
		source      string
	}{
		{
			slug: "allrecipes", source: capture.SourceJSONLD, url: "https://www.allrecipes.com/recipe/10813/best-chocolate-chip-cookies/",
			title: "Best Chocolate Chip Cookies", yield: "4 dozen cookies", author: "Dora",
			ingredients: 11, steps: 9,
			// A comma inside an ingredient must survive intact.
			firstIng:  "1 cup butter, softened",
			firstStep: "Gather your ingredients, making sure your butter is softened, and your eggs are room temperature.",
		},
		{
			slug: "seriouseats", source: capture.SourceJSONLD, url: "https://www.seriouseats.com/blistered-tomato-pasta-salad-recipe-11980610",
			title: "Blistered-Tomato Pasta Salad With Basil", yield: "8", author: "Daniel Gritzer",
			ingredients: 7, steps: 3,
			firstIng: "1 pound dried pasta, sich as fusilli bucati or regular fusilli",
		},
		{
			slug: "jamieoliver", source: capture.SourceJSONLD, url: "https://www.jamieoliver.com/recipes/pasta-recipes/gennaro-s-classic-spaghetti-carbonara/",
			title: "Gennaro's classic spaghetti carbonara", yield: "2", author: "Gennaro Contaldo",
			ingredients: 6, steps: 8,
			firstIng:  "3 large free-range egg yolks",
			firstStep: "Put the egg yolks into a bowl, finely grate in the Parmesan, season with pepper, then mix well with a fork and put to one side.",
		},
		{
			slug: "simplyrecipes", source: capture.SourceJSONLD, url: "https://www.simplyrecipes.com/recipes/perfect_guacamole/",
			title: "How to Make the Best Guacamole", yield: "4", author: "Elise Bauer",
			ingredients: 10, steps: 5,
			firstIng: "2 ripe avocados",
		},
		{
			slug: "pinchofyum", source: capture.SourceJSONLD, url: "https://pinchofyum.com/the-best-soft-chocolate-chip-cookies",
			title: "The Best Soft Chocolate Chip Cookies", yield: "12 cookies", author: "Lindsay Ostrom",
			ingredients: 9, steps: 5,
			firstIng: "8 tablespoons of salted butter",
		},
		{
			// No Recipe JSON-LD title: ingredients and steps come from the
			// heading fallback.
			slug: "nigella", source: capture.SourceHeadings, url: "https://www.nigella.com/recipes/coffee-shortbreads",
			ingredients: 12, steps: 7,
			firstIng:  "60 grams golden (or regular) caster sugar",
			firstStep: "Heat the oven to 160°C/140°C Fan/325°F, and line 2 flat baking sheets with baking parchment.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			rec := parseFixture(t, tt.slug, tt.url)

			if tt.title != "" && rec.Title != tt.title {
				t.Errorf("title = %q, want %q", rec.Title, tt.title)
			}
			if tt.yield != "" && rec.Yield != tt.yield {
				t.Errorf("yield = %q, want %q", rec.Yield, tt.yield)
			}
			if tt.author != "" && (rec.Author == nil || rec.Author.Name != tt.author) {
				t.Errorf("author = %+v, want %q", rec.Author, tt.author)
			}
			if len(rec.Ingredients) != tt.ingredients {
				t.Errorf("ingredients = %d, want %d", len(rec.Ingredients), tt.ingredients)
			}
			if len(rec.Steps) != tt.steps {
				t.Errorf("steps = %d, want %d", len(rec.Steps), tt.steps)
			}
			if tt.firstIng != "" && rec.Ingredients[0].Text != tt.firstIng {
				t.Errorf("first ingredient = %q, want %q", rec.Ingredients[0].Text, tt.firstIng)
			}
			if tt.firstStep != "" && rec.Steps[0].Text != tt.firstStep {
				t.Errorf("first step = %q, want %q", rec.Steps[0].Text, tt.firstStep)
			}
			if rec.Source != tt.source {
				t.Errorf("source = %q, want %q", rec.Source, tt.source)
			}
			if rec.Extractor != "cookarr" {
				t.Errorf("extractor = %q", rec.Extractor)
			}
			if rec.SourceURL != tt.url {
				t.Errorf("source url = %q", rec.SourceURL)
			}
		})
	}
}

func TestParseFixtureTimes(t *testing.T) {
	rec := parseFixture(t, "allrecipes", "https://www.allrecipes.com/recipe/10813/best-chocolate-chip-cookies/")
	if rec.PrepTime <= 0 || rec.CookTime <= 0 {
		t.Errorf("prep=%s cook=%s, want both positive", rec.PrepTime, rec.CookTime)
	}
	if rec.TotalTime < rec.PrepTime+rec.CookTime-time.Minute {
		t.Errorf("total %s is shorter than prep %s + cook %s", rec.TotalTime, rec.PrepTime, rec.CookTime)
	}
}

// TestParseCorpusInvariants checks properties that must hold for every
// fixture that parses, whatever its source. These catch data-quality bugs
// (leaked markup, undecoded entities, empty rows) without pinning exact output.
func TestParseCorpusInvariants(t *testing.T) {
	entries, err := loadCorpus("testdata/sources.tsv", "testdata/fixtures")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Skip("no fixtures; run scripts/fetch-fixtures.sh")
	}
	parsed := 0
	for _, e := range entries {
		t.Run(e.slug, func(t *testing.T) {
			page, err := os.ReadFile(e.path)
			if err != nil {
				t.Fatal(err)
			}
			rec, err := capture.Parse(e.url, page, capture.Options{Extractor: capture.CookarrExtractor{}})
			if err != nil {
				if _, expected := noRecipe[e.slug]; !expected {
					t.Logf("no recipe (not in noRecipe list): %v", err)
				}
				return
			}
			parsed++
			for i, ing := range rec.Ingredients {
				checkLine(t, "ingredient", i, ing.Text)
			}
			for i, st := range rec.Steps {
				checkLine(t, "step", i, st.Text)
			}
			if rec.Title != "" && markup.MatchString(rec.Title) {
				t.Errorf("title has markup or entities: %q", rec.Title)
			}
		})
	}
	if parsed < len(entries)/2 {
		t.Errorf("only %d of %d fixtures parsed; extractor regression?", parsed, len(entries))
	}
}

func checkLine(t *testing.T, kind string, i int, s string) {
	t.Helper()
	if s == "" || s != strings.TrimSpace(s) {
		t.Errorf("%s %d is empty or untrimmed: %q", kind, i, s)
	}
	if markup.MatchString(s) {
		t.Errorf("%s %d has markup or entities: %q", kind, i, s)
	}
	if strings.Contains(s, "  ") || strings.ContainsAny(s, "\n\t") {
		t.Errorf("%s %d has unnormalised whitespace: %q", kind, i, s)
	}
}

func TestParseNonRecipeFixtureReturnsErrNoRecipe(t *testing.T) {
	page, err := os.ReadFile("testdata/fixtures/bbcgoodfood_roundup.html")
	if err != nil {
		t.Skip("fixture missing")
	}
	rec, err := capture.Parse("https://www.bbcgoodfood.com/health/healthy-food-guides/15-best-healthy-cake-recipes", page, capture.Options{Extractor: capture.CookarrExtractor{}})
	if err == nil || rec == nil {
		t.Fatalf("want partial recipe and ErrNoRecipe, got rec=%v err=%v", rec, err)
	}
	if !strings.Contains(err.Error(), capture.ErrNoRecipe.Error()) {
		t.Errorf("err = %v", err)
	}
}
