package capture_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
)

// noRecipe lists fixtures expected not to produce a structured recipe, either
// because they aren't recipes or because the extractor has a known gap.
var noRecipe = map[string]string{
	"bbcgoodfood_roundup": "not a recipe: roundup",
	"eatingwell_roundup":  "not a recipe: roundup",
	"olivemagazine_list":  "not a recipe: roundup",
	"kingarthur":          "not a recipe: blog article",
	"saveur":              "gap: Article JSON-LD only, no Recipe schema",
	"smittenkitchen":      "gap: microdata ingredients only, steps are prose",
	"smittenkitchen_2007": "gap: no structured data",
}

type corpusEntry struct {
	slug string
	url  string
	path string
}

// loadCorpus is used only by tests and the test benchmark. It pairs each TSV
// source with its downloaded fixture, skipping fixtures that aren't present.
func loadCorpus(sourcesTSV, fixturesDir string) ([]corpusEntry, error) {
	f, err := os.Open(sourcesTSV)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []corpusEntry
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		slug, pageURL, ok := strings.Cut(sc.Text(), "\t")
		if !ok || slug == "" {
			continue
		}
		path := filepath.Join(fixturesDir, slug+".html")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		out = append(out, corpusEntry{slug: slug, url: strings.TrimSpace(pageURL), path: path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].slug < out[j].slug })
	return out, sc.Err()
}

func TestCorpus(t *testing.T) {
	log.SetOutput(io.Discard)
	entries, err := loadCorpus("testdata/sources.tsv", "testdata/fixtures")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Skip("no fixtures; run scripts/fetch-fixtures.sh")
	}

	for _, e := range entries {
		t.Run(e.slug, func(t *testing.T) {
			raw, err := os.ReadFile(e.path)
			if err != nil {
				t.Fatal(err)
			}
			rec, err := capture.Parse(e.url, raw, capture.Options{})
			if rec == nil {
				t.Fatalf("no result: %v", err)
			}
			sections := rec.Sections()
			if !sections.Title || !sections.Story {
				t.Errorf("missing sections: %+v", sections)
			}

			reason, expectedGap := noRecipe[e.slug]
			switch {
			case expectedGap && err == nil:
				t.Errorf("now parses; remove from noRecipe (%s)", reason)
			case !expectedGap && errors.Is(err, capture.ErrNoRecipe):
				t.Errorf("expected a recipe: %v", err)
			case !expectedGap && err != nil:
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestParseBench is an opt-in report harness. Run it with:
//
//	go test -v -run '^TestParseBench$' ./internal/capture
//	PARSEBENCH_FLAGS='-extractor ours -v' mage parsebench
//
// By default it compares Cookarr with krip, reporting per-fixture ingredient
// and step counts and coverage summaries. Supported flags: -sources,
// -fixtures, -extractor (compare|krip|ours), -out, -file, -url and -v.
func TestParseBench(t *testing.T) {
	log.SetOutput(io.Discard)
	flags := flag.NewFlagSet("parsebench", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	sources := flags.String("sources", "testdata/sources.tsv", "slug<TAB>url list")
	fixtures := flags.String("fixtures", "testdata/fixtures", "directory of saved HTML pages")
	extractor := flags.String("extractor", "compare", "compare, krip, or ours")
	outDir := flags.String("out", "", "write one parsed JSON file per fixture here")
	file := flags.String("file", "", "parse a single HTML file (- for stdin) and print JSON")
	pageURL := flags.String("url", "", "source URL for -file")
	verbose := flags.Bool("v", false, "print warnings per fixture")
	if err := flags.Parse(strings.Fields(os.Getenv("PARSEBENCH_FLAGS"))); err != nil {
		t.Fatal(err)
	}

	if *extractor == "compare" {
		if *file != "" {
			runSinglePage(t, *file, *pageURL, capture.Options{Extractor: capture.Extractors["krip"]})
			fmt.Println("--- ours ---")
			runSinglePage(t, *file, *pageURL, capture.Options{Extractor: capture.CookarrExtractor{}})
			return
		}
		runComparison(t, *sources, *fixtures, *outDir, *verbose)
		return
	}
	ex, ok := capture.Extractors[*extractor]
	if *extractor == "ours" {
		ex, ok = capture.CookarrExtractor{}, true
	}
	if !ok {
		t.Fatalf("unknown extractor %q", *extractor)
	}
	opts := capture.Options{Extractor: ex}
	if *file != "" {
		runSinglePage(t, *file, *pageURL, opts)
		return
	}

	entries, err := loadCorpus(*sources, *fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no fixtures found; run scripts/fetch-fixtures.sh")
	}
	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	var output bytes.Buffer
	w := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "FIXTURE\tTITLE\tINGREDIENTS\tSTEPS\tSTORY\tPHOTOS\tAUTHOR\tLINKS\tMS\tRESULT")
	var totals benchSummary
	for _, e := range entries {
		raw, err := os.ReadFile(e.path)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		rec, parseErr := capture.Parse(e.url, raw, opts)
		elapsed := time.Since(start)

		result := "ok"
		switch {
		case errors.Is(parseErr, capture.ErrNoRecipe):
			result = "NO RECIPE"
		case parseErr != nil:
			result = "ERROR " + parseErr.Error()
		}
		totals.add(rec, result)
		if rec == nil {
			fmt.Fprintf(w, "%s\t-\t-\t-\t-\t-\t-\t-\t%d\t%s\n", e.slug, elapsed.Milliseconds(), result)
			continue
		}
		links := 0
		if rec.Author != nil {
			links = len(rec.Author.Links)
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\t%d\t%s\t%d\t%d\t%s\n",
			e.slug, mark(rec.Title != ""), len(rec.Ingredients), len(rec.Steps),
			storySize(rec.StoryHTML), len(rec.Photos), authorName(rec), links,
			elapsed.Milliseconds(), result)
		if *verbose {
			for _, warning := range rec.Warnings {
				fmt.Fprintf(w, "  warn\t%s\n", warning)
			}
		}
		if *outDir != "" {
			b, err := json.MarshalIndent(rec, "", "  ")
			if err != nil {
				t.Errorf("%s: encode result: %v", e.slug, err)
				continue
			}
			if err := os.WriteFile(filepath.Join(*outDir, e.slug+".json"), b, 0o644); err != nil {
				t.Errorf("%s: write result: %v", e.slug, err)
			}
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	fmt.Print(output.String())
	totals.print(ex.Name())
}

func runComparison(t *testing.T, sources, fixtures, outDir string, verbose bool) {
	t.Helper()
	entries, err := loadCorpus(sources, fixtures)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no fixtures found; run mage fixtures")
	}
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	krip := capture.Options{Extractor: capture.Extractors["krip"]}
	ours := capture.Options{Extractor: capture.CookarrExtractor{}}
	var output bytes.Buffer
	w := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "FIXTURE\tKRIP ING\tKRIP STEPS\tOURS ING\tOURS STEPS\tCOUNT MATCH\tKRIP MS\tOURS MS")
	var kripSummary, oursSummary benchSummary
	countMatches, countDiffs := 0, 0
	for _, e := range entries {
		raw, err := os.ReadFile(e.path)
		if err != nil {
			t.Fatal(err)
		}
		kStart := time.Now()
		kr, ke := capture.Parse(e.url, raw, krip)
		kTime := time.Since(kStart)
		oStart := time.Now()
		or, oe := capture.Parse(e.url, raw, ours)
		oTime := time.Since(oStart)
		kripSummary.add(kr, resultLabel(ke))
		oursSummary.add(or, resultLabel(oe))

		ki, ks := recipeCounts(kr)
		oi, os := recipeCounts(or)
		match := ki == oi && ks == os && (ke == nil) == (oe == nil)
		if match {
			countMatches++
		} else {
			countDiffs++
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%s\t%d\t%d\n", e.slug, ki, ks, oi, os,
			mark(match), kTime.Milliseconds(), oTime.Milliseconds())
		if verbose && (!match || len(warnings(kr)) > 0 || len(warnings(or)) > 0) {
			for _, warning := range warnings(kr) {
				fmt.Fprintf(w, "  krip warn\t%s\n", warning)
			}
			for _, warning := range warnings(or) {
				fmt.Fprintf(w, "  ours warn\t%s\n", warning)
			}
		}
		if outDir != "" {
			writeBenchJSON(t, filepath.Join(outDir, e.slug+".krip.json"), kr)
			writeBenchJSON(t, filepath.Join(outDir, e.slug+".ours.json"), or)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	fmt.Print(output.String())
	fmt.Printf("\ncount comparison (equal ingredient/step counts and same recipe success): %d/%d equal, %d differ\n", countMatches, len(entries), countDiffs)
	kripSummary.print("krip baseline")
	oursSummary.print("Cookarr")
	fmt.Printf("  Cookarr vs krip recipe coverage: %+d\n", oursSummary.recipes-kripSummary.recipes)
}

func resultLabel(err error) string {
	if err == nil {
		return "ok"
	}
	if errors.Is(err, capture.ErrNoRecipe) {
		return "NO RECIPE"
	}
	return "ERROR"
}

func recipeCounts(r *capture.Recipe) (int, int) {
	if r == nil {
		return 0, 0
	}
	return len(r.Ingredients), len(r.Steps)
}

func warnings(r *capture.Recipe) []string {
	if r == nil {
		return nil
	}
	return r.Warnings
}

func writeBenchJSON(t *testing.T, path string, r *capture.Recipe) {
	t.Helper()
	if r == nil {
		return
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Errorf("encode %s: %v", path, err)
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Errorf("write %s: %v", path, err)
	}
}

func runSinglePage(t *testing.T, file, pageURL string, opts capture.Options) {
	t.Helper()
	var raw []byte
	var err error
	if file == "-" {
		raw, err = io.ReadAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(file)
	}
	if err != nil {
		t.Fatal(err)
	}
	rec, parseErr := capture.Parse(pageURL, raw, opts)
	if rec != nil {
		b, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(b))
	}
	if parseErr != nil {
		t.Fatalf("parse: %v", parseErr)
	}
}

type benchSummary struct {
	total, recipes                       int
	ing, steps, story, photos, author, t int
}

func (s *benchSummary) add(r *capture.Recipe, result string) {
	s.total++
	if r == nil {
		return
	}
	sections := r.Sections()
	if result == "ok" {
		s.recipes++
	}
	for _, p := range []struct {
		on  bool
		cnt *int
	}{{sections.Title, &s.t}, {sections.Ingredients, &s.ing}, {sections.Steps, &s.steps}, {sections.Story, &s.story}, {sections.Photos, &s.photos}, {sections.Author, &s.author}} {
		if p.on {
			*p.cnt++
		}
	}
}

func (s *benchSummary) print(name string) {
	pct := func(n int) string {
		return fmt.Sprintf("%d/%d (%.0f%%)", n, s.total, 100*float64(n)/float64(s.total))
	}
	fmt.Printf("\nextractor: %s\n", name)
	fmt.Printf("  recipe (ingredients+steps): %s\n", pct(s.recipes))
	fmt.Printf("  title %s | ingredients %s | steps %s\n", pct(s.t), pct(s.ing), pct(s.steps))
	fmt.Printf("  story %s | photos %s | author %s\n", pct(s.story), pct(s.photos), pct(s.author))
}

func mark(b bool) string {
	if b {
		return "y"
	}
	return "-"
}

func storySize(h string) string {
	if h == "" {
		return "-"
	}
	return fmt.Sprintf("%dk", (len(h)+512)/1024)
}

func authorName(r *capture.Recipe) string {
	if r.Author == nil || r.Author.Name == "" {
		return "-"
	}
	n := strings.TrimSpace(r.Author.Name)
	if len(n) > 18 {
		n = n[:17] + "…"
	}
	return n
}
