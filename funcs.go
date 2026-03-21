package sessionstore

import (
	"github.com/dracory/str"
)

func generateSessionKey(keyLength int) string {
	gamma := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	key, err := str.RandomFromGamma(keyLength, gamma)
	if err != nil {
		key = str.Random(32)
	}
	return key
}
