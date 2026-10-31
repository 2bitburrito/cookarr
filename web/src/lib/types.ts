// Mirrors capture.Recipe in internal/capture/recipe.go. Durations are Go
// time.Duration values, i.e. nanoseconds.
export interface Ingredient {
  group?: string
  text: string
}

export interface Step {
  group?: string
  text: string
}

export interface Photo {
  url: string
  role: 'hero' | 'step' | 'story'
  caption?: string
  width?: number
  height?: number
}

export interface Link {
  url: string
  kind: 'social' | 'support' | 'other'
  label?: string
}

export interface Author {
  name: string
  url?: string
  links?: Link[]
}

export interface Recipe {
  source_url: string
  site_name?: string
  language?: string
  title?: string
  description?: string
  yield?: string
  prep_time?: number
  cook_time?: number
  total_time?: number
  keywords?: string[]
  ingredients?: Ingredient[]
  steps?: Step[]
  story_html?: string
  photos?: Photo[]
  author?: Author
  extractor: string
  source?: string
  warnings?: string[]
}

export const NS_PER_MINUTE = 60_000_000_000
