package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// RandomHex generates a random hex string of the specified length.
func RandomHex(n int) string {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand.Read should never fail on modern systems.
		// If it does, that indicates a serious system problem.
		panic(fmt.Sprintf("crypto/rand.Read failed: %v", err))
	}
	return hex.EncodeToString(bytes)[:n]
}

// NewSessionID generates a session ID in the format YYYY-MM-DD-XXXXXX.
func NewSessionID() string {
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf("%s-%s", date, RandomHex(6))
}
