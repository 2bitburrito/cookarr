// Package authors finds an author's outbound links and classifies them as
// social or support. There is no standard for "support the author", so the
// domain lists here are ours to maintain.
package authors

import (
	"net/url"
	"strings"

	"github.com/hpalmer/cookarr/internal/types"
)

type Kind = types.AuthorLinkKind

var social = map[string]string{
	"instagram.com": "instagram",
	"facebook.com":  "facebook",
	"fb.com":        "facebook",
	"pinterest.com": "pinterest",
	"twitter.com":   "twitter",
	"x.com":         "x",
	"youtube.com":   "youtube",
	"tiktok.com":    "tiktok",
	"threads.net":   "threads",
	"bsky.app":      "bluesky",
	"linkedin.com":  "linkedin",
	"flipboard.com": "flipboard",
	"substack.com":  "substack",
}

var support = map[string]string{
	"ko-fi.com":           "ko-fi",
	"patreon.com":         "patreon",
	"buymeacoffee.com":    "buy-me-a-coffee",
	"github.com/sponsors": "github-sponsors",
	"paypal.me":           "paypal",
	"liberapay.com":       "liberapay",
	"opencollective.com":  "open-collective",
	"gofundme.com":        "gofundme",
}

// Classify reports the kind and service label for a link, or ok=false when
// the link is neither social nor support.
func Classify(raw string) (kind Kind, label string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", "", false
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	hostPath := host + strings.ToLower(u.Path)

	for prefix, l := range support {
		if hostPath == prefix || strings.HasPrefix(hostPath, prefix+"/") || hostMatches(host, prefix) {
			return types.AuthorLinkSupport, l, true
		}
	}
	for d, l := range social {
		if hostMatches(host, d) {
			if isShareLink(u) {
				return "", "", false
			}
			return types.AuthorLinkSocial, l, true
		}
	}
	return "", "", false
}

func hostMatches(host, domain string) bool {
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// isShareLink filters the "share this recipe" buttons that sit on every blog
// and point at a network rather than at the author.
func isShareLink(u *url.URL) bool {
	p := strings.ToLower(u.Path)
	for _, s := range []string{"/sharer", "/share", "/intent/", "/pin/create", "/dialog/"} {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}
