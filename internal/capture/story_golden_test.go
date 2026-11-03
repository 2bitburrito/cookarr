package capture_test

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

var (
	spaceRun   = regexp.MustCompile(`\s+`)
	tagBoundry = regexp.MustCompile(`>\s*<`)
)

// normaliseStory collapses whitespace and puts each tag on its own line, so
// goldens diff cleanly and don't churn on readability's indentation.
func normaliseStory(h string) string {
	h = spaceRun.ReplaceAllString(strings.TrimSpace(h), " ")
	h = tagBoundry.ReplaceAllString(h, ">\n<")
	return h + "\n"
}

// TestStoryGolden pins the extracted story for a few representative pages:
// a long blog post, a terse site, and a page with no JSON-LD. Run with
// -update after an intended change, then review the diff.
func TestStoryGolden(t *testing.T) {
	tests := []struct {
		slug string
		url  string
		// must are phrases that belong in the story; mustNot are recipe-card
		// phrases that must not leak into it.
		must    []string
		mustNot []string
	}{
		{
			slug:    "pinchofyum",
			url:     "https://pinchofyum.com/the-best-soft-chocolate-chip-cookies",
			must:    []string{"BAKE ME IN YOUR OVEN RIGHT NOW", "What Makes These Chocolate Chip Cookies The Best"},
			mustNot: []string{"8 tablespoons of salted butter"},
		},
		{
			slug: "seriouseats",
			url:  "https://www.seriouseats.com/blistered-tomato-pasta-salad-recipe-11980610",
		},
		{
			slug: "nigella",
			url:  "https://www.nigella.com/recipes/coffee-shortbreads",
		},
	}
	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			rec := parseFixture(t, tt.slug, tt.url)
			got := normaliseStory(rec.StoryHTML)

			for _, s := range tt.must {
				if !strings.Contains(got, s) {
					t.Errorf("story is missing %q", s)
				}
			}
			for _, s := range tt.mustNot {
				if strings.Contains(got, s) {
					t.Errorf("story leaks recipe card text %q", s)
				}
			}
			for _, bad := range []string{"<script", "<style", "<iframe", "onclick="} {
				if strings.Contains(got, bad) {
					t.Errorf("story contains unsafe markup %q", bad)
				}
			}

			path := filepath.Join("testdata", "golden", tt.slug+".story.html")
			if *update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s; run go test -run TestStoryGolden -update", path)
			}
			if got != string(want) {
				t.Errorf("story differs from %s; run with -update and review the diff", path)
			}
		})
	}
}
