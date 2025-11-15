package db

import (
	"crypto/sha256"
	"encoding/hex"
)

// GenerateContentHash generates a SHA-256 hash of the given content.
// This is used to create unique content hashes for webhook events to prevent duplicates.
func GenerateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
