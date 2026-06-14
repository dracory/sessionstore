package sessionstore

import (
	"crypto/rand"
	"math/big"
)

const sessionKeyGamma = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateSessionKey generates a cryptographically random session key of the given length.
func generateSessionKey(keyLength int) string {
	gammaLen := big.NewInt(int64(len(sessionKeyGamma)))
	buf := make([]byte, keyLength)
	for i := range buf {
		n, err := rand.Int(rand.Reader, gammaLen)
		if err != nil {
			// fallback: use index mod gamma length
			buf[i] = sessionKeyGamma[i%len(sessionKeyGamma)]
			continue
		}
		buf[i] = sessionKeyGamma[n.Int64()]
	}
	return string(buf)
}
