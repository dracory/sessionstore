package sessionstore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

const encryptedValuePrefix = "enc:v1:"

func newSessionEncryptor(key []byte) (*sessionEncryptor, error) {
	if len(key) == 0 {
		return nil, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session store: create encryption cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session store: initialise AEAD: %w", err)
	}

	return &sessionEncryptor{aead: aead}, nil
}

type sessionEncryptor struct {
	aead cipher.AEAD
}

func (s *sessionEncryptor) encrypt(value string) (string, error) {
	if s == nil || s.aead == nil {
		return "", errors.New("session store: encryptor is not initialised")
	}

	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("session store: generate nonce: %w", err)
	}

	ciphertext := s.aead.Seal(nil, nonce, []byte(value), nil)

	combined := make([]byte, 0, len(nonce)+len(ciphertext))
	combined = append(combined, nonce...)
	combined = append(combined, ciphertext...)

	encoded := base64.StdEncoding.EncodeToString(combined)

	return encryptedValuePrefix + encoded, nil
}

func (s *sessionEncryptor) decrypt(value string) (string, error) {
	if s == nil || s.aead == nil {
		return "", errors.New("session store: encryptor is not initialised")
	}

	if value == "" {
		return value, nil
	}

	if !strings.HasPrefix(value, encryptedValuePrefix) {
		return value, nil
	}

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedValuePrefix))
	if err != nil {
		return "", fmt.Errorf("session store: decode encrypted value: %w", err)
	}

	nonceSize := s.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("session store: encrypted value is malformed")
	}

	nonce := raw[:nonceSize]
	ciphertext := raw[nonceSize:]

	plaintext, err := s.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("session store: decrypt value: %w", err)
	}

	return string(plaintext), nil
}
