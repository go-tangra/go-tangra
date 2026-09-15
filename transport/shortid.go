package transport

import (
	"crypto/rand"
	"encoding/hex"
)

func shortID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
