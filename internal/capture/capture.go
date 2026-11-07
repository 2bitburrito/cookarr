package capture

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/hpalmer/cookarr/internal/authors"
	"golang.org/x/net/html"
)

// Options configures Parse.
type Options struct {
	// Extractor is the structured-data extractor. Defaults to krip.
	Extractor StructuredExtractor
	// Logger defaults to the application's slog logger.
	Logger *slog.Logger
}

// Parse turns a page into a Recipe. page is the HTML as the extension would
// send it, or as the server fetched it; Parse strips scripts and styles itself
// (keeping JSON-LD), so callers can pass raw HTML.
//
// A page with no recipe data still returns a Recipe, carrying only the story,
// photos and author, along with ErrNoRecipe. Callers can decide whether that
// is worth saving.
func Parse(pageURL string, page []byte, opts Options) (*Recipe, error) {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	ex := opts.Extractor
	if ex == nil {
		ex = Krip{}
	}
	log = log.With("component", "capture", "extractor", ex.Name())
	log.Debug("parsing page", "input_bytes", len(page))
	stripped, err := Strip(page)
	if err != nil {
		return nil, fmt.Errorf("strip: %w", err)
	}

	rec := &Recipe{SourceURL: pageURL, Extractor: ex.Name()}
	base, _ := url.Parse(pageURL)

	var structErr error
	st, err := safeExtract(ex, pageURL, stripped)
	if err != nil {
		structErr = err
		log.Debug("structured extraction failed")
		rec.Warnings = append(rec.Warnings, "structured: "+err.Error())
	} else {
		applyStructured(rec, st, base)
	}

	sty, err := extractStory(stripped, pageURL)
	if err != nil {
		rec.Warnings = append(rec.Warnings, "story: "+err.Error())
		log.Error("story extraction failed")
	} else {
		rec.StoryHTML = sty.HTML
		if rec.SiteName == "" {
			rec.SiteName = sty.SiteName
		}
		addStoryPhotos(rec, sty.Images)
		if rec.Author == nil && sty.Byline != "" {
			rec.Author = &Author{Name: cleanByline(sty.Byline)}
		}
	}

	attachAuthorLinks(rec, st, stripped)

	if rec.Title == "" {
		rec.Title = pageTitle(stripped)
	}
	if !rec.HasRecipe() {
		log.Warn("no recipe data found", "warnings", len(rec.Warnings))
		if structErr == nil {
			structErr = errors.New("structured data has no ingredients and steps")
		}
		return rec, fmt.Errorf("%w: %v", ErrNoRecipe, structErr)
	}
	log.Debug("page parsed", "ingredients", len(rec.Ingredients), "steps", len(rec.Steps), "photos", len(rec.Photos), "story_bytes", len(rec.StoryHTML), "warnings", len(rec.Warnings))
	return rec, nil
}

// ErrNoRecipe is returned, alongside a partial Recipe, when no ingredients and
// steps could be found.
var ErrNoRecipe = errors.New("no recipe found")

// safeExtract keeps a panic inside a third-party extractor from taking the
// capture request down with it.
func safeExtract(ex StructuredExtractor, pageURL string, page []byte) (st *Structured, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("extractor panic: %v", r)
		}
	}()
	return ex.Extract(pageURL, page)
}

func applyStructured(rec *Recipe, st *Structured, base *url.URL) {
	rec.Title = st.Title
	rec.Description = st.Description
	rec.Yield = st.Yield
	rec.Language = st.Language
	rec.PrepTime, rec.CookTime, rec.TotalTime = st.PrepTime, st.CookTime, st.TotalTime
	rec.Keywords = st.Keywords
	rec.Ingredients = st.Ingredients
	rec.Steps = st.Steps
	rec.SiteName = st.PublisherName
	rec.Source = st.Source

	for i, img := range st.Images {
		role := PhotoStep
		if i == 0 {
			role = PhotoHero
		}
		img.URL = absURL(base, img.URL)
		img.Role = role
		rec.Photos = append(rec.Photos, img)
	}
	if st.AuthorName != "" {
		a := &Author{Name: st.AuthorName}
		if st.AuthorURL != "" {
			a.URL = absURL(base, st.AuthorURL)
		}
		rec.Author = a
	}
}

// addStoryPhotos appends photos found in the story that the structured data
// didn't already list.
func addStoryPhotos(rec *Recipe, urls []string) {
	seen := map[string]bool{}
	for _, p := range rec.Photos {
		seen[p.URL] = true
	}
	for i, u := range urls {
		if seen[u] {
			continue
		}
		seen[u] = true
		role := PhotoStory
		if i == 0 && len(rec.Photos) == 0 {
			role = PhotoHero
		}
		rec.Photos = append(rec.Photos, Photo{URL: u, Role: role})
	}
}

// attachAuthorLinks gathers social and support links from the structured
// sameAs list and from anchors on the page (rel="me" plus any link to a known
// social or support domain), and attaches them to the author.
func attachAuthorLinks(rec *Recipe, st *Structured, page []byte) {
	var candidates []string
	if st != nil {
		candidates = append(candidates, st.SameAs...)
		if st.AuthorURL != "" {
			candidates = append(candidates, st.AuthorURL)
		}
	}
	candidates = append(candidates, pageLinks(page)...)

	seen := map[string]bool{}
	var links []Link
	for _, c := range candidates {
		kind, label, ok := authors.Classify(c)
		if !ok {
			continue
		}
		key := strings.TrimRight(strings.ToLower(c), "/")
		if seen[key] {
			continue
		}
		seen[key] = true
		links = append(links, Link{URL: c, Kind: LinkKind(kind), Label: label})
	}
	if len(links) == 0 {
		return
	}
	if rec.Author == nil {
		name := rec.SiteName
		if name == "" {
			return
		}
		rec.Author = &Author{Name: name}
	}
	rec.Author.Links = links
}

func pageLinks(page []byte) []string {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil
	}
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			if href := strings.TrimSpace(attr(n, "href")); strings.HasPrefix(href, "http") {
				out = append(out, href)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return out
}

func pageTitle(page []byte) string {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return ""
	}
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = strings.TrimSpace(n.FirstChild.Data)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return title
}

func cleanByline(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"By ", "by ", "BY "} {
		s = strings.TrimPrefix(s, p)
	}
	return strings.TrimSpace(s)
}
