package capture

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/borschtapp/krip"
	"github.com/borschtapp/krip/scraper"
	"golang.org/x/net/html"
)

// Where the structured fields came from.
const (
	SourceJSONLD         = "jsonld"
	SourceHeadings       = "headings"
	SourceJSONLDHeadings = "jsonld+headings"
)

// Structured is the common shape of what a structured-data extractor (JSON-LD,
// microdata, OpenGraph) can tell us about a page. Extractors are swappable so
// the harness can compare them on the same fixtures.
type Structured struct {
	Title       string
	Description string
	Yield       string
	Language    string
	PrepTime    time.Duration
	CookTime    time.Duration
	TotalTime   time.Duration
	Keywords    []string

	Ingredients []Ingredient
	Steps       []Step
	Images      []Photo

	AuthorName    string
	AuthorURL     string
	PublisherName string
	PublisherURL  string
	SameAs        []string

	// Source records how the fields were found (see the Source constants).
	// Extractors that don't track this leave it empty.
	Source string
}

type StructuredExtractor interface {
	Name() string
	// Extract returns an error when the page has no recipe data at all.
	Extract(pageURL string, page []byte) (*Structured, error)
}

// Extractors lists the available structured extractors by name.
var Extractors = map[string]StructuredExtractor{
	"krip":    Krip{},
	"cookarr": CookarrExtractor{},
}

// Krip adapts github.com/borschtapp/krip.
type Krip struct{}

func (Krip) Name() string { return "krip" }

func (Krip) Extract(pageURL string, page []byte) (*Structured, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	opts := krip.ScrapeOptions{SkipCustomScrapers: true}
	input, err := scraper.NodeInput(root, pageURL, opts)
	if err != nil {
		return nil, err
	}
	r, err := krip.Scrape(input, opts)
	if err != nil {
		return nil, err
	}

	s := &Structured{
		Title:       r.Name,
		Description: r.Description,
		Yield:       r.Yield,
		Language:    r.Language,
		PrepTime:    parseISODuration(r.PrepTime),
		CookTime:    parseISODuration(r.CookTime),
		TotalTime:   parseISODuration(r.TotalTime),
		Keywords:    r.Keywords,
		SameAs:      r.Links,
	}
	for _, ing := range r.Ingredients {
		if text := ingredientText(ing); text != "" {
			s.Ingredients = append(s.Ingredients, Ingredient{Text: text})
		}
	}
	for _, sec := range r.Instructions {
		if len(sec.Steps) == 0 {
			if t := firstNonEmpty(sec.Text, sec.Name); t != "" {
				s.Steps = append(s.Steps, Step{Text: t})
			}
			continue
		}
		for _, st := range sec.Steps {
			if t := firstNonEmpty(st.Text, st.Name); t != "" {
				s.Steps = append(s.Steps, Step{Group: sec.Name, Text: t})
			}
		}
	}
	for _, img := range r.Images {
		s.Images = append(s.Images, Photo{URL: img.Url, Caption: img.Caption, Width: img.Width, Height: img.Height})
	}
	if r.Author != nil {
		s.AuthorName, s.AuthorURL = r.Author.Name, r.Author.Url
	}
	if r.Publisher != nil {
		s.PublisherName, s.PublisherURL = r.Publisher.Name, r.Publisher.Url
	}
	return s, nil
}

// ingredientText flattens krip's PropertyValue. Plain string ingredients land
// in Name; structured ones carry amount and unit separately.
func ingredientText(p *krip.PropertyValue) string {
	if p == nil {
		return ""
	}
	parts := []string{p.Value, p.UnitText, p.Name}
	out := parts[:0]
	for _, v := range parts {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, " ")
}

func parseISODuration(v string) time.Duration {
	if v == "" {
		return 0
	}
	// Recipe durations are typically ISO 8601 values such as PT15M or PT1H30M.
	parts := isoDuration.FindStringSubmatch(v)
	if len(parts) != 5 {
		return 0
	}
	units := []float64{24 * 60 * 60, 60 * 60, 60, 1}
	var seconds float64
	for i, part := range parts[1:] {
		if part == "" {
			continue
		}
		value, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return 0
		}
		seconds += value * units[i]
	}
	return time.Duration(seconds * float64(time.Second))
}

var isoDuration = regexp.MustCompile(`^P(?:(\d+(?:\.\d+)?)D)?(?:T(?:(\d+(?:\.\d+)?)H)?(?:(\d+(?:\.\d+)?)M)?(?:(\d+(?:\.\d+)?)S)?)?$`)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}
