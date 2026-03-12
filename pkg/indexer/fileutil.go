package indexer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
)

// readFile reads the entire contents of path.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// fileSHA256 returns the hex-encoded SHA-256 digest of data.
func fileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
