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
