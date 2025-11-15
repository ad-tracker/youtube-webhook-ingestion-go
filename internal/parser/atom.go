package parser

import (
	"encoding/xml"
	"fmt"
	"time"
)

// AtomFeed represents a YouTube Atom feed notification.
// YouTube uses the Atom 1.0 format with custom YouTube namespaces.
type AtomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Entry   *AtomEntry  `xml:"entry"`
	Deleted *DeletedEntry `xml:"http://www.youtube.com/xml/schemas/2015 deleted-entry"`
}

// AtomEntry represents a video entry in the Atom feed.
type AtomEntry struct {
	VideoID     string    `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
	ChannelID   string    `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
	Title       string    `xml:"title"`
	Link        AtomLink  `xml:"link"`
	Published   time.Time `xml:"published"`
	Updated     time.Time `xml:"updated"`
}

// AtomLink represents a link element in the Atom feed.
type AtomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

// DeletedEntry represents a deleted video notification.
type DeletedEntry struct {
	Ref     string    `xml:"ref,attr"`
	When    time.Time `xml:"when,attr"`
}

// VideoData contains the parsed video information from an Atom feed.
type VideoData struct {
	VideoID       string
	ChannelID     string
	Title         string
	VideoURL      string
	PublishedAt   time.Time
	UpdatedAt     time.Time
	IsDeleted     bool
}

// ParseAtomFeed parses a YouTube Atom feed XML and extracts video information.
// It returns the parsed VideoData or an error if the XML is invalid or missing required fields.
func ParseAtomFeed(rawXML string) (*VideoData, error) {
	var feed AtomFeed
	if err := xml.Unmarshal([]byte(rawXML), &feed); err != nil {
		return nil, fmt.Errorf("unmarshal atom feed: %w", err)
	}

	// Check if this is a deleted entry
	if feed.Deleted != nil {
		return &VideoData{
			IsDeleted: true,
		}, nil
	}

	// Validate that we have an entry
	if feed.Entry == nil {
		return nil, fmt.Errorf("atom feed missing entry element")
	}

	entry := feed.Entry

	// Validate required fields
	if entry.VideoID == "" {
		return nil, fmt.Errorf("atom entry missing video ID")
	}
	if entry.ChannelID == "" {
		return nil, fmt.Errorf("atom entry missing channel ID")
	}
	if entry.Title == "" {
		return nil, fmt.Errorf("atom entry missing title")
	}

	// Extract video URL from link element
	videoURL := entry.Link.Href
	if videoURL == "" {
		// Construct URL from video ID if link is missing
		videoURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", entry.VideoID)
	}

	return &VideoData{
		VideoID:     entry.VideoID,
		ChannelID:   entry.ChannelID,
		Title:       entry.Title,
		VideoURL:    videoURL,
		PublishedAt: entry.Published,
		UpdatedAt:   entry.Updated,
		IsDeleted:   false,
	}, nil
}
