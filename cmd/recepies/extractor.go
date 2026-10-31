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

	// Prefer the JSON-LD candidate with the most useful recipe content. Pages
	// can contain several Recipe objects, including incomplete carousel cards.
	var result *capture.Structured
	for _, candidate := range recipes {
		candidateResult := fromRecipe(candidate)
		if result == nil || recipeScore(candidateResult) > recipeScore(result) {
			result = candidateResult
		}
	}
	if result == nil {
		result = &capture.Structured{}
	}

	// If JSON-LD is absent or incomplete, make a second pass over headings and
	// use matching ingredient/instruction sections to fill missing fields.
	if len(result.Ingredients) == 0 || len(result.Steps) == 0 {
		sections := extractHeadingSections(root)
		if len(result.Ingredients) == 0 {
			result.Ingredients = sections.ingredients
		}
		if len(result.Steps) == 0 {
			result.Steps = sections.steps
		}
	}
	if hasRecipeData(result) {
		return result, nil
	}
	return nil, errors.New("no recipe data found in JSON-LD or recognized HTML sections")
}

// recipeScore prefers JSON-LD candidates with ingredients and instructions;
// title-only objects should not hide a more complete recipe object.
func recipeScore(r *capture.Structured) int {
	score := 0
	if r.Title != "" {
		score++
	}
	if len(r.Ingredients) > 0 {
		score += 2
	}
	if len(r.Steps) > 0 {
		score += 2
	}
	return score
}

type headingSections struct {
	ingredients []capture.Ingredient
	steps       []capture.Step
}

type textBlock struct {
	text string
}

// extractHeadingSections finds h2/h3 headings for ingredients and directions,
// then reads list items (preferred) or paragraphs up to the next h2/h3.
func extractHeadingSections(root *html.Node) headingSections {
	var nodes []*html.Node
	walk(root, func(n *html.Node) { nodes = append(nodes, n) })
	var result headingSections
	for i, n := range nodes {
		if n.Type != html.ElementNode || !isSectionHeading(n) {
			continue
		}
		kind := classifyHeading(text(n))
		if kind == "" {
			continue
		}
		end := len(nodes)
		for j := i + 1; j < len(nodes); j++ {
			if nodes[j].Type == html.ElementNode && isSectionHeading(nodes[j]) {
				end = j
				break
			}
		}
		contentNodes := nodes[i+1 : end]
		if kind == "steps" && isNumberedStepHeading(text(n)) {
			if localNodes := followingSiblingNodes(n); len(localNodes) > 0 {
				contentNodes = localNodes
			}
		}
		for _, block := range sectionBlocks(contentNodes) {
			switch kind {
			case "ingredients":
				appendUniqueIngredient(&result.ingredients, block.text)
			case "steps":
				appendUniqueStep(&result.steps, block.text)
			}
		}
	}
	return result
}

// isSectionHeading limits the fallback to the h2/h3 section hierarchy, where
// recipe sections are commonly marked without machine-readable metadata.
func isSectionHeading(n *html.Node) bool {
	return n.Data == "h2" || n.Data == "h3"
}

// classifyHeading maps common recipe section labels to their content type.
func classifyHeading(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, " \t\r\n:.-–—")
	if strings.HasPrefix(value, "step ") {
		return "steps"
	}
	for _, keyword := range []string{"ingredients", "ingredient", "ingredient list", "what you need", "what you'll need", "what you will need"} {
		if value == keyword || strings.HasPrefix(value, keyword+" ") || strings.HasPrefix(value, keyword+":") {
			return "ingredients"
		}
	}
	for _, keyword := range []string{"method", "steps", "directions", "instructions", "preparation", "how to make"} {
		if value == keyword || strings.HasPrefix(value, keyword+" ") || strings.HasPrefix(value, keyword+":") {
			return "steps"
		}
	}
	return ""
}

// isNumberedStepHeading identifies headings such as "Step 1". Their content
// is limited to the same parent container so unrelated lists after the recipe
// are not accidentally treated as cooking steps.
func isNumberedStepHeading(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(value, "step ") {
		return false
	}
	return len(value) > len("step ") && value[len("step ")] >= '0' && value[len("step ")] <= '9'
}

// followingSiblingNodes returns the descendants of siblings after a heading
// within the same parent, stopping at another h2/h3. Recipe markup commonly
// wraps each numbered step in its own container with a paragraph after h3.
func followingSiblingNodes(heading *html.Node) []*html.Node {
	if heading.Parent == nil {
		return nil
	}
	var nodes []*html.Node
	for sibling := heading.NextSibling; sibling != nil; sibling = sibling.NextSibling {
		if sibling.Type == html.ElementNode && isSectionHeading(sibling) {
			break
		}
		walk(sibling, func(n *html.Node) { nodes = append(nodes, n) })
	}
	return nodes
}

// appendUniqueIngredient adds a section-derived ingredient unless the same
// normalized line was already found in another repeated page section.
func appendUniqueIngredient(items *[]capture.Ingredient, text string) {
	key := normalizeBlock(text)
	for _, item := range *items {
		if normalizeBlock(item.Text) == key {
			return
		}
	}
	*items = append(*items, capture.Ingredient{Text: text})
}

// appendUniqueStep adds a section-derived step unless it duplicates one found
// in another repeated page section (for example desktop/mobile markup).
func appendUniqueStep(items *[]capture.Step, text string) {
	key := normalizeBlock(text)
	for _, item := range *items {
		if normalizeBlock(item.Text) == key {
			return
		}
	}
	*items = append(*items, capture.Step{Text: text})
}

// normalizeBlock creates a stable comparison key for duplicate extracted text.
func normalizeBlock(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// sectionBlocks selects one text block per list item, falling back to
// paragraphs when a section is not represented as a list. It avoids counting
// nested list items twice and removes duplicate/empty text.
func sectionBlocks(nodes []*html.Node) []textBlock {
	var listItems, paragraphs, textContainers []textBlock
	for _, n := range nodes {
		if n.Type != html.ElementNode {
			continue
		}
		switch n.Data {
		case "li":
			if hasAncestorElement(n, "li") {
				continue
			}
			if value := cleanBlockText(n, true); value != "" {
				listItems = append(listItems, textBlock{text: value})
			}
		case "p":
			if value := cleanBlockText(n, false); value != "" {
				paragraphs = append(paragraphs, textBlock{text: value})
			}
		case "div":
			if !hasContentBlockDescendant(n) {
				if value := cleanBlockText(n, false); value != "" {
					textContainers = append(textContainers, textBlock{text: value})
				}
			}
		}
	}
	blocks := listItems
	if len(blocks) == 0 {
		blocks = paragraphs
	}
	if len(blocks) == 0 {
		blocks = textContainers
	}
	seen := make(map[string]bool, len(blocks))
	out := blocks[:0]
	for _, block := range blocks {
		key := normalizeBlock(block.text)
		if !seen[key] {
			seen[key] = true
			out = append(out, block)
		}
	}
	return out
}

// hasContentBlockDescendant reports whether a div contains a nested block
// whose text should be collected separately instead of as one large container.
func hasContentBlockDescendant(n *html.Node) bool {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "div", "p", "li", "ul", "ol", "h2", "h3":
			return true
		}
		if hasContentBlockDescendant(child) {
			return true
		}
	}
	return false
}

// hasAncestorElement reports whether n is nested inside an element of tag.
func hasAncestorElement(n *html.Node, tag string) bool {
	for parent := n.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == html.ElementNode && parent.Data == tag {
			return true
		}
	}
	return false
}

// cleanBlockText extracts readable text and skips nested lists when collecting
// a list item, preventing nested items from being repeated in the parent.
func cleanBlockText(n *html.Node, skipLists bool) string {
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(node *html.Node) {
		if node.Type == html.ElementNode {
			if node.Data == "script" || node.Data == "style" || (skipLists && (node.Data == "ul" || node.Data == "ol")) {
				return
			}
		}
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			collect(child)
		}
	}
	collect(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// collectRecipes recursively finds Recipe objects in SON-LD arrays and
// "@graph" containers. Other objects are traversed only when they contain @graph.
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
