// Package types contains shared Cookarr domain types and constants.
package types

import "time"

const (
	// Authentication and password policy.
	MinPasswordLength = 8

	// Session cookie settings.
	SessionCookieName = "cookarr_session"
	SessionTTL        = 30 * 24 * time.Hour

	// Structured recipe extraction sources.
	SourceJSONLD         = "jsonld"
	SourceHeadings       = "headings"
	SourceJSONLDHeadings = "jsonld+headings"

	// HTML and recipe-section labels used across capture stages.
	HTMLScriptElement        = "script"
	RecipeSectionIngredients = "ingredients"
	RecipeSectionSteps       = "steps"
)

type PhotoRole string

const (
	PhotoHero  PhotoRole = "hero"
	PhotoStep  PhotoRole = "step"
	PhotoStory PhotoRole = "story"
)

type AuthorLinkKind string

const (
	AuthorLinkSocial  AuthorLinkKind = "social"
	AuthorLinkSupport AuthorLinkKind = "support"
	AuthorLinkOther   AuthorLinkKind = "other"
)

// DeviceClass is the workspace layout bucket: desktop, tablet or phone.
// Layouts (built-in presets, saved presets and per-recipe overrides) are
// stored and resolved per class, per docs/PLAN.md.
type DeviceClass string

const (
	DeviceDesktop DeviceClass = "desktop"
	DeviceTablet  DeviceClass = "tablet"
	DevicePhone   DeviceClass = "phone"
)

// ValidDeviceClass reports whether s is one of the known device classes.
func ValidDeviceClass(s string) bool {
	switch DeviceClass(s) {
	case DeviceDesktop, DeviceTablet, DevicePhone:
		return true
	default:
		return false
	}
}
