package models

import (
	"net/url"
	"strings"
	"time"
)

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

// ParseYouTubeVideoID extracts a YouTube video ID from common YouTube URL
// formats (watch?v=, youtu.be/, shorts/, embed/). Returns "" if not a YouTube URL.
func ParseYouTubeVideoID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && host != "www.youtube.com" && host != "youtu.be" && host != "m.youtube.com" {
		return ""
	}
	if host == "youtu.be" {
		return strings.TrimPrefix(u.Path, "/")
	}
	// /shorts/ID, /embed/ID
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	if len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed") {
		return parts[1]
	}
	// /watch?v=ID
	if v := u.Query().Get("v"); v != "" {
		return v
	}
	return ""
}

// IsYouTubeURL reports whether rawURL points to a YouTube video.
func IsYouTubeURL(rawURL string) bool {
	return ParseYouTubeVideoID(rawURL) != ""
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
