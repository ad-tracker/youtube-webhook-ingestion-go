package parser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAtomFeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		rawXML      string
		want        *VideoData
		wantErr     bool
		errContains string
	}{
		{
			name: "valid atom feed with all fields",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>dQw4w9WgXcQ</yt:videoId>
    <yt:channelId>UCuAXFkgsw1L7xaCfnd5JJOw</yt:channelId>
    <title>Never Gonna Give You Up</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=dQw4w9WgXcQ"/>
    <published>2009-10-25T06:57:33+00:00</published>
    <updated>2022-03-15T12:00:00+00:00</updated>
  </entry>
</feed>`,
			want: &VideoData{
				VideoID:     "dQw4w9WgXcQ",
				ChannelID:   "UCuAXFkgsw1L7xaCfnd5JJOw",
				Title:       "Never Gonna Give You Up",
				VideoURL:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
				PublishedAt: mustParseTime("2009-10-25T06:57:33+00:00"),
				UpdatedAt:   mustParseTime("2022-03-15T12:00:00+00:00"),
				IsDeleted:   false,
			},
			wantErr: false,
		},
		{
			name: "valid atom feed without link element",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCchannel123</yt:channelId>
    <title>Test Video</title>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			want: &VideoData{
				VideoID:     "test123",
				ChannelID:   "UCchannel123",
				Title:       "Test Video",
				VideoURL:    "https://www.youtube.com/watch?v=test123",
				PublishedAt: mustParseTime("2025-01-15T10:00:00+00:00"),
				UpdatedAt:   mustParseTime("2025-01-15T11:00:00+00:00"),
				IsDeleted:   false,
			},
			wantErr: false,
		},
		{
			name: "title with special characters",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>abc123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test &amp; Demo &lt;Special&gt; "Characters"</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=abc123"/>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			want: &VideoData{
				VideoID:     "abc123",
				ChannelID:   "UCtest",
				Title:       `Test & Demo <Special> "Characters"`,
				VideoURL:    "https://www.youtube.com/watch?v=abc123",
				PublishedAt: mustParseTime("2025-01-15T10:00:00+00:00"),
				UpdatedAt:   mustParseTime("2025-01-15T11:00:00+00:00"),
				IsDeleted:   false,
			},
			wantErr: false,
		},
		{
			name: "deleted video entry",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <yt:deleted-entry ref="yt:video:deleted123" when="2025-01-15T12:00:00+00:00"/>
</feed>`,
			want: &VideoData{
				IsDeleted: true,
			},
			wantErr: false,
		},
		{
			name:        "invalid XML",
			rawXML:      `not valid xml at all`,
			wantErr:     true,
			errContains: "unmarshal atom feed",
		},
		{
			name: "missing entry element",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
</feed>`,
			wantErr:     true,
			errContains: "missing entry element",
		},
		{
			name: "missing video ID",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:channelId>UCtest</yt:channelId>
    <title>Test Video</title>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			wantErr:     true,
			errContains: "missing video ID",
		},
		{
			name: "missing channel ID",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <title>Test Video</title>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			wantErr:     true,
			errContains: "missing channel ID",
		},
		{
			name: "missing title",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>test123</yt:videoId>
    <yt:channelId>UCtest</yt:channelId>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			wantErr:     true,
			errContains: "missing title",
		},
		{
			name: "empty feed",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom"/>`,
			wantErr:     true,
			errContains: "missing entry element",
		},
		{
			name: "feed with whitespace in IDs",
			rawXML: `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <yt:videoId>  test123  </yt:videoId>
    <yt:channelId>  UCtest  </yt:channelId>
    <title>  Test Video  </title>
    <published>2025-01-15T10:00:00+00:00</published>
    <updated>2025-01-15T11:00:00+00:00</updated>
  </entry>
</feed>`,
			want: &VideoData{
				VideoID:     "  test123  ",
				ChannelID:   "  UCtest  ",
				Title:       "  Test Video  ",
				VideoURL:    "https://www.youtube.com/watch?v=  test123  ",
				PublishedAt: mustParseTime("2025-01-15T10:00:00+00:00"),
				UpdatedAt:   mustParseTime("2025-01-15T11:00:00+00:00"),
				IsDeleted:   false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseAtomFeed(tt.rawXML)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want.VideoID, got.VideoID)
			assert.Equal(t, tt.want.ChannelID, got.ChannelID)
			assert.Equal(t, tt.want.Title, got.Title)
			assert.Equal(t, tt.want.VideoURL, got.VideoURL)
			assert.Equal(t, tt.want.IsDeleted, got.IsDeleted)

			// For non-deleted entries, check timestamps
			if !tt.want.IsDeleted {
				assert.True(t, tt.want.PublishedAt.Equal(got.PublishedAt),
					"PublishedAt mismatch: want %v, got %v", tt.want.PublishedAt, got.PublishedAt)
				assert.True(t, tt.want.UpdatedAt.Equal(got.UpdatedAt),
					"UpdatedAt mismatch: want %v, got %v", tt.want.UpdatedAt, got.UpdatedAt)
			}
		})
	}
}

// Helper function to parse time or panic (for test fixtures only)
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
