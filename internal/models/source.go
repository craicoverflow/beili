package models

import (
	"net/url"
	"strings"
	"time"
)

// SourceType identifies what kind of reference a source is.
type SourceType string

const (
	SourceTypeURL       SourceType = "url"
	SourceTypeBook      SourceType = "book"
	SourceTypeYouTube   SourceType = "youtube"
	SourceTypeInstagram SourceType = "instagram"
	SourceTypeOther     SourceType = "other"
)

// AllSourceTypes is the ordered list used in UI rendering.
var AllSourceTypes = []SourceType{
	SourceTypeURL,
	SourceTypeYouTube,
	SourceTypeInstagram,
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
	case SourceTypeInstagram:
		return "Instagram"
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

// ParseInstagramShortcode extracts the shortcode from an Instagram post, reel,
// or IGTV URL (/p/<code>, /reel/<code>, /tv/<code>). Returns "" if rawURL is
// not an Instagram URL.
func ParseInstagramShortcode(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "instagram.com" && host != "www.instagram.com" && host != "m.instagram.com" {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "p" || p == "reel" || p == "reels" || p == "tv") && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1]
		}
	}
	return ""
}

// IsInstagramURL reports whether rawURL points to an Instagram post or reel.
func IsInstagramURL(rawURL string) bool {
	return ParseInstagramShortcode(rawURL) != ""
}

// IsVideo reports whether the source represents a video that can be embedded.
func (s Source) IsVideo() bool {
	return s.VideoEmbedURL() != ""
}

// VideoEmbedURL returns an iframe-safe embed URL for the source's video, or
// "" if the source is not a recognised video.
func (s Source) VideoEmbedURL() string {
	if id := ParseYouTubeVideoID(s.URL); id != "" {
		return "https://www.youtube.com/embed/" + id
	}
	if code := ParseInstagramShortcode(s.URL); code != "" {
		return "https://www.instagram.com/p/" + code + "/embed"
	}
	return ""
}

// VideoThumbnailURL returns a thumbnail image URL for the source's video, or
// "" if no public thumbnail is available (e.g. Instagram, which requires
// authenticated API access for media thumbnails).
func (s Source) VideoThumbnailURL() string {
	if id := ParseYouTubeVideoID(s.URL); id != "" {
		return "https://img.youtube.com/vi/" + id + "/hqdefault.jpg"
	}
	return ""
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
