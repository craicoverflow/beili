package models

import "time"

// SourceType identifies what kind of reference a source is.
type SourceType string

const (
	SourceTypeURL     SourceType = "url"
	SourceTypeBook    SourceType = "book"
	SourceTypeYouTube SourceType = "youtube"
	SourceTypeOther   SourceType = "other"
)

// AllSourceTypes is the ordered list used in UI rendering.
var AllSourceTypes = []SourceType{
	SourceTypeURL,
	SourceTypeYouTube,
	SourceTypeBook,
	SourceTypeOther,
}

func (s SourceType) Label() string {
	switch s {
	case SourceTypeURL:
		return "Website"
	case SourceTypeBook:
		return "Book"
	case SourceTypeYouTube:
		return "YouTube"
	default:
		return "Other"
	}
}

// Source is a reference attached to a Meal (URL, book, YouTube video, etc.).
type Source struct {
	ID            string
	MealID        string
	Type          SourceType
	Title         string
	URL           string
	PageReference string // e.g. "p.47, The Ottolenghi Cookbook"
	Notes         string
	CreatedAt     time.Time
}
