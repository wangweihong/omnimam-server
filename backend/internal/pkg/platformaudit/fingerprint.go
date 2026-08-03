package platformaudit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gowebpki/jcs"
)

// CanonicalJSONDigest returns the lowercase SHA-256 hex digest of RFC 8785 JSON.
func CanonicalJSONDigest(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := jcs.Transform(raw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
