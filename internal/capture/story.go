package capture

import (
	"bytes"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

var storyPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowAttrs("alt", "src", "srcset", "width", "height").OnElements("img")
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()

// recipeCardMarkers are class fragments used by the common recipe-card
// plugins. The card holds the ingredients and steps, which have their own
// panels, so it is removed before readability so the story stays story.
var recipeCardMarkers = []string{
	"wprm-recipe-container", "wprm-recipe-jump", "tasty-recipes", "mv-create-card",
	"mv-create-wrapper", "recipe-card", "recipe-callout", "jump-to-recipe", "print-recipe",
}

type story struct {
	HTML     string
	SiteName string
	Byline   string
	Images   []string
}

// extractStory runs readability over the page with the recipe card removed
// and sanitises the result.
func extractStory(page []byte, pageURL string) (*story, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	removeRecipeCards(root)

	u, _ := url.Parse(pageURL)
	parser := readability.NewParser()
	art, err := parser.ParseDocument(root, u)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if art.Node != nil {
		if err := art.RenderHTML(&buf); err != nil {
			return nil, err
		}
	}
	clean := strings.TrimSpace(storyPolicy.Sanitize(buf.String()))

	st := &story{HTML: clean, SiteName: art.SiteName(), Byline: art.Byline()}
	if art.Node != nil {
		collectImages(art.Node, u, &st.Images)
	}
	if img := art.ImageURL(); img != "" {
		st.Images = append([]string{img}, st.Images...)
	}
	return st, nil
}

func removeRecipeCards(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && isRecipeCard(c) {
			n.RemoveChild(c)
		} else {
			removeRecipeCards(c)
		}
		c = next
	}
}

func isRecipeCard(n *html.Node) bool {
	class := strings.ToLower(attr(n, "class"))
	if class == "" {
		return false
	}
	for _, m := range recipeCardMarkers {
		if strings.Contains(class, m) {
			return true
		}
	}
	return false
}

func collectImages(n *html.Node, base *url.URL, out *[]string) {
	if n.Type == html.ElementNode && n.Data == "img" {
		if src := attr(n, "src"); src != "" && !strings.HasPrefix(src, "data:") {
			*out = append(*out, absURL(base, src))
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		collectImages(c, base, out)
	}
}

func absURL(base *url.URL, ref string) string {
	r, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ref
	}
	if base != nil {
		r = base.ResolveReference(r)
	}
	return r.String()
}
