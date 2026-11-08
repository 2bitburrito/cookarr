package capture

import (
	"bytes"
	"strings"

	"github.com/hpalmer/cookarr/internal/types"
	"golang.org/x/net/html"
)

// Strip mirrors what the browser extension does before upload: it removes
// <script> and <style> elements, keeping only application/ld+json scripts.
// Stripping keeps session tokens and tracking IDs out of storage.
func Strip(raw []byte) ([]byte, error) {
	root, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	stripNode(root)
	var buf bytes.Buffer
	if err := html.Render(&buf, root); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func stripNode(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && dropElement(c) {
			n.RemoveChild(c)
		} else {
			stripNode(c)
		}
		c = next
	}
}

func dropElement(n *html.Node) bool {
	switch n.Data {
	case "style":
		return true
	case types.HTMLScriptElement:
		return !strings.Contains(strings.ToLower(attr(n, "type")), "ld+json")
	}
	return false
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
