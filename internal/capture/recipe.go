package capture

import (
	"time"

	"github.com/hpalmer/cookarr/internal/types"
)

// Recipe is what the capture pipeline produces. Each field group maps to a
// workspace panel: ingredients, steps, story, photos, author.
type Recipe struct {
	SourceURL string `json:"source_url"`
	SiteName  string `json:"site_name,omitempty"`
	Language  string `json:"language,omitempty"`

	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	Yield       string        `json:"yield,omitempty"`
	PrepTime    time.Duration `json:"prep_time,omitempty"`
	CookTime    time.Duration `json:"cook_time,omitempty"`
	TotalTime   time.Duration `json:"total_time,omitempty"`
	Keywords    []string      `json:"keywords,omitempty"`

	Ingredients []Ingredient `json:"ingredients,omitempty"`
	Steps       []Step       `json:"steps,omitempty"`
	StoryHTML   string       `json:"story_html,omitempty"`
	Photos      []Photo      `json:"photos,omitempty"`
	Author      *Author      `json:"author,omitempty"`

	// Extractor records which extractor produced the structured fields.
	Extractor string `json:"extractor"`
	// Source records whether the structured fields came from JSON-LD, the
	// heading fallback, or both.
	Source string `json:"source,omitempty"`
	// Warnings are non-fatal problems, e.g. a section that came back empty.
	Warnings []string `json:"warnings,omitempty"`
}

type Ingredient struct {
	Group string `json:"group,omitempty"`
	Text  string `json:"text"`
}

type Step struct {
	Group string `json:"group,omitempty"`
	Text  string `json:"text"`
}

type PhotoRole = types.PhotoRole

type Photo struct {
	URL     string    `json:"url"`
	Role    PhotoRole `json:"role"`
	Caption string    `json:"caption,omitempty"`
	Width   int       `json:"width,omitempty"`
	Height  int       `json:"height,omitempty"`
}

type LinkKind = types.AuthorLinkKind

type Link struct {
	URL  string   `json:"url"`
	Kind LinkKind `json:"kind"`
	// Label is the network or service, e.g. "instagram" or "ko-fi".
	Label string `json:"label,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Links []Link `json:"links,omitempty"`
}

// Sections reports which panels have content. Used by the test harness and by
// the API to decide which panels to offer.
type Sections struct {
	Title       bool `json:"title"`
	Ingredients bool `json:"ingredients"`
	Steps       bool `json:"steps"`
	Story       bool `json:"story"`
	Photos      bool `json:"photos"`
	Author      bool `json:"author"`
}

func (r *Recipe) Sections() Sections {
	return Sections{
		Title:       r.Title != "",
		Ingredients: len(r.Ingredients) > 0,
		Steps:       len(r.Steps) > 0,
		Story:       r.StoryHTML != "",
		Photos:      len(r.Photos) > 0,
		Author:      r.Author != nil && r.Author.Name != "",
	}
}

// HasRecipe reports whether the page yielded a usable recipe, meaning both
// ingredients and steps were found.
func (r *Recipe) HasRecipe() bool {
	s := r.Sections()
	return s.Ingredients && s.Steps
}
