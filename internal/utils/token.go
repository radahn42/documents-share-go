package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
