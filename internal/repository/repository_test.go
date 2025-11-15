package repository

import (
	"testing"
)

func TestComputeEventHash(t *testing.T) {
	tests := []struct {
		name    string
		rawXML  string
		want    string
		wantLen int
	}{
		{
			name:    "simple XML",
			rawXML:  "<xml>test</xml>",
			wantLen: 64, // SHA-256 produces 64 hex characters
		},
		{
			name:    "empty string",
			rawXML:  "",
			wantLen: 64,
		},
		{
			name:    "complex XML",
			rawXML:  "<entry><id>yt:video:dQw4w9WgXcQ</id><title>Test Video</title></entry>",
			wantLen: 64,
		},
		{
			name:    "same input produces same hash",
			rawXML:  "<test>data</test>",
			want:    ComputeEventHash("<test>data</test>"),
			wantLen: 64,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeEventHash(tt.rawXML)

			if len(got) != tt.wantLen {
				t.Errorf("ComputeEventHash() hash length = %v, want %v", len(got), tt.wantLen)
			}

			if tt.want != "" && got != tt.want {
				t.Errorf("ComputeEventHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeEventHash_Deterministic(t *testing.T) {
	input := "<entry><id>yt:video:test123</id></entry>"

	hash1 := ComputeEventHash(input)
	hash2 := ComputeEventHash(input)

	if hash1 != hash2 {
		t.Errorf("ComputeEventHash() is not deterministic: %v != %v", hash1, hash2)
	}
}

func TestComputeEventHash_Different(t *testing.T) {
	hash1 := ComputeEventHash("<xml>test1</xml>")
	hash2 := ComputeEventHash("<xml>test2</xml>")

	if hash1 == hash2 {
		t.Error("ComputeEventHash() produced same hash for different inputs")
	}
}
