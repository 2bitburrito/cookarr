// Package recepies contains Cookarr's in-progress recipe structured-data
// extractor. It is intentionally small and explicit.
package recepies

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hpalmer/cookarr/internal/capture"
	"golang.org/x/net/html"
)

// Extractor reads Recipe objects from JSON-LD script elements.
// Microdata and HTML fallbacks can be added here as separately tested stages.
type Extractor struct{}

// Name identifies this extractor in benchmark output and stored metadata.
func (Extractor) Name() string { return "cookarr" }

// Extract parses the HTML and returns the first usable schema.org Recipe
// object found in application/ld+json scripts. Malformed JSON-LD blocks are
// ignored so one bad block does not prevent parsing other blocks on the page.
func (Extractor) Extract(pageURL string, page []byte) (*capture.Structured, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	var recipes []map[string]any
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "script" || !strings.Contains(strings.ToLower(attr(n, "type")), "ld+json") {
			return
		}
		var value any
		if json.Unmarshal([]byte(text(n)), &value) == nil {
			collectRecipes(value, &recipes)
		}
	})
	for _, candidate := range recipes {
		result := fromRecipe(candidate)
		if hasRecipeData(result) {
			return result, nil
		}
	}
	return nil, errors.New("no schema.org Recipe JSON-LD found")
}

// collectRecipes recursively finds Recipe objects in JSON-LD arrays and
// @graph containers. Other objects are traversed only when they contain @graph.
func collectRecipes(v any, out *[]map[string]any) {
	switch x := v.(type) {
	case []any:
		for _, item := range x {
			collectRecipes(item, out)
		}
	case map[string]any:
		if isRecipe(x["@type"]) {
			*out = append(*out, x)
		}
		// JSON-LD graph wrappers and embedded graph arrays are common.
		if graph, ok := x["@graph"]; ok {
			collectRecipes(graph, out)
		}
	}
}

// isRecipe reports whether a JSON-LD @type value names schema.org Recipe.
// It accepts both a single type and the array form allowed by JSON-LD.
func isRecipe(v any) bool {
	switch x := v.(type) {
	case string:
		return typeName(x) == "recipe"
	case []any:
		for _, item := range x {
			if isRecipe(item) {
				return true
			}
		}
	}
	return false
}

// typeName removes a schema URL prefix so schema.org/Recipe and Recipe
// compare the same way, case-insensitively.
func typeName(s string) string {
	if i := strings.LastIndexAny(s, "#/"); i >= 0 {
		s = s[i+1:]
	}
	return strings.ToLower(s)
}

// fromRecipe maps schema.org Recipe properties into Cookarr's structured
// representation. Each property is handled by a helper for its allowed shapes.
func fromRecipe(m map[string]any) *capture.Structured {
	r := &capture.Structured{
		Title:       stringValue(m["name"]),
		Description: stringValue(m["description"]),
		Yield:       stringValue(m["recipeYield"]),
		Language:    stringValue(m["inLanguage"]),
		PrepTime:    parseDuration(stringValue(m["prepTime"])),
		CookTime:    parseDuration(stringValue(m["cookTime"])),
		TotalTime:   parseDuration(stringValue(m["totalTime"])),
		Keywords:    stringList(m["keywords"]),
	}
	for _, ingredient := range stringList(m["recipeIngredient"]) {
		r.Ingredients = append(r.Ingredients, capture.Ingredient{Text: ingredient})
	}
	r.Steps = parseInstructions(m["recipeInstructions"])
	r.Images = parseImages(m["image"])
	r.AuthorName, r.AuthorURL = person(m["author"])
	r.PublisherName, r.PublisherURL = person(m["publisher"])
	r.SameAs = stringList(m["sameAs"])
	return r
}

// parseInstructions flattens Recipe instructions into steps while preserving
// HowToSection names as each step's group. It accepts text, HowToStep objects,
// arrays, and nested HowToSection objects.
func parseInstructions(v any) []capture.Step {
	var result []capture.Step
	var walkInstruction func(any, string)
	walkInstruction = func(value any, group string) {
		switch x := value.(type) {
		case string:
			if s := strings.TrimSpace(x); s != "" {
				result = append(result, capture.Step{Group: group, Text: s})
			}
		case []any:
			for _, item := range x {
				walkInstruction(item, group)
			}
		case map[string]any:
			name := stringValue(x["name"])
			if typeHas(x["@type"], "howtosection") {
				section := name
				if section == "" {
					section = group
				}
				walkInstruction(x["itemListElement"], section)
				return
			}
			text := first(name, stringValue(x["text"]))
			if text != "" {
				result = append(result, capture.Step{Group: group, Text: text})
			}
			if text == "" {
				walkInstruction(x["itemListElement"], group)
			}
		}
	}
	walkInstruction(v, "")
	return result
}

// parseImages accepts the common JSON-LD image forms: a URL string, an array
// of image values, or an ImageObject containing url/contentUrl.
func parseImages(v any) []capture.Photo {
	var out []capture.Photo
	var add func(any)
	add = func(value any) {
		switch x := value.(type) {
		case string:
			if strings.TrimSpace(x) != "" {
				out = append(out, capture.Photo{URL: strings.TrimSpace(x)})
			}
		case []any:
			for _, item := range x {
				add(item)
			}
		case map[string]any:
			add(x["url"])
			add(x["contentUrl"])
		}
	}
	add(v)
	return out
}

// person extracts a name and profile URL from a JSON-LD author/publisher
// value. Schema data may represent that value as a name string, an array of
// people, or a Person/Organization object with name and url (or @id).
// For arrays it returns the first entry containing either a name or URL.
func person(v any) (string, string) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), ""
	case []any:
		for _, item := range x {
			if name, url := person(item); name != "" || url != "" {
				return name, url
			}
		}
	case map[string]any:
		return stringValue(x["name"]), first(stringValue(x["url"]), stringValue(x["@id"]))
	}
	return "", ""
}

// stringList normalizes a JSON-LD string, list of values, or named object into
// strings. Comma-separated keywords are split into individual entries.
func stringList(v any) []string {
	var result []string
	switch x := v.(type) {
	case string:
		for _, part := range strings.Split(x, ",") {
			if s := strings.TrimSpace(part); s != "" {
				result = append(result, s)
			}
		}
	case []any:
		for _, item := range x {
			result = append(result, stringList(item)...)
		}
	case map[string]any:
		if s := stringValue(x["name"]); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// stringValue converts JSON values used for scalar recipe fields to trimmed
// strings; unsupported object shapes return an empty string.
func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return fmt.Sprint(x)
	}
	return ""
}

// typeHas checks whether a JSON-LD @type value (string or array) includes the
// requested type name.
func typeHas(v any, target string) bool {
	switch x := v.(type) {
	case string:
		return typeName(x) == target
	case []any:
		for _, item := range x {
			if typeHas(item, target) {
				return true
			}
		}
	}
	return false
}

// hasRecipeData reports whether a candidate has enough identifying recipe
// content to be useful; an empty Recipe object is ignored.
func hasRecipeData(r *capture.Structured) bool {
	return r.Title != "" || len(r.Ingredients) > 0 || len(r.Steps) > 0
}

// parseDuration converts the supported ISO 8601 day/time duration forms to a
// Go duration. Unsupported or malformed values return zero.
func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	// Duration units are interpreted by parseISODurationLocal below.
	if strings.HasPrefix(s, "P") {
		if parsed := parseISODurationLocal(s); parsed > 0 {
			return parsed
		}
	}
	return 0
}

// parseISODurationLocal parses the day/hour/minute/second subset of ISO 8601
// durations used by recipe metadata, such as P1DT2H30M or PT45S.
func parseISODurationLocal(s string) time.Duration {
	// Keep duration handling intentionally basic for the first JSON-LD pass.
	var days, hours, minutes, seconds float64
	var number strings.Builder
	inTime := false
	for _, ch := range s[1:] {
		if ch == 'T' {
			inTime = true
			continue
		}
		if (ch >= '0' && ch <= '9') || ch == '.' {
			number.WriteRune(ch)
			continue
		}
		if number.Len() == 0 {
			return 0
		}
		value, err := time.ParseDuration(number.String() + "s")
		if err != nil {
			return 0
		}
		v := value.Seconds()
		switch {
		case ch == 'D' && !inTime:
			days = v
		case ch == 'H' && inTime:
			hours = v
		case ch == 'M' && inTime:
			minutes = v
		case ch == 'S' && inTime:
			seconds = v
		default:
			return 0
		}
		number.Reset()
	}
	return time.Duration((days*86400 + hours*3600 + minutes*60 + seconds) * float64(time.Second))
}

// walk visits an HTML node and all descendants in document order.
func walk(n *html.Node, visit func(*html.Node)) {
	visit(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, visit)
	}
}

// text returns the concatenated text content of a node and its descendants.
func text(n *html.Node) string {
	var b strings.Builder
	var walkText func(*html.Node)
	walkText = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walkText(c)
		}
	}
	walkText(n)
	return b.String()
}

// attr returns the value of an HTML attribute, or an empty string if absent.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// first returns the first non-empty, trimmed string from values.
func first(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
