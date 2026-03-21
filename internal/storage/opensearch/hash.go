package opensearch

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
