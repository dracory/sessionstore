package sessionstore

import (
	"strings"
	"testing"
)

func TestNewSessionEncryptor_NoKey(t *testing.T) {
	encryptor, err := newSessionEncryptor(nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if encryptor != nil {
		t.Fatalf("expected nil encryptor, got %#v", encryptor)
	}

	encryptor, err = newSessionEncryptor([]byte{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if encryptor != nil {
		t.Fatalf("expected nil encryptor for empty key, got %#v", encryptor)
	}
}

func TestNewSessionEncryptor_InvalidKey(t *testing.T) {
	if _, err := newSessionEncryptor([]byte("short")); err == nil {
		t.Fatalf("expected error for invalid key length")
	} else if !strings.Contains(err.Error(), "invalid key") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestSessionEncryptor_EncryptDecrypt(t *testing.T) {
	encryptor, err := newSessionEncryptor([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cases := []string{
		"hello world",
		"{\"key\":\"value\",\"nested\":{\"arr\":[1,2,3]}}",
		"特殊字符测试",
		"emoji 😀😇🤖",
		"newline\ncarriage\rreturn\ttab",
	}

	for _, plaintext := range cases {
		ciphertext, err := encryptor.encrypt(plaintext)
		if err != nil {
			t.Fatalf("unexpected encryption error for input %q: %v", plaintext, err)
		}
		if ciphertext == plaintext {
			t.Fatalf("ciphertext should differ from plaintext for input %q", plaintext)
		}

		decrypted, err := encryptor.decrypt(ciphertext)
		if err != nil {
			t.Fatalf("unexpected decryption error for input %q: %v", plaintext, err)
		}
		if decrypted != plaintext {
			t.Fatalf("expected decrypted value %q, got %q", plaintext, decrypted)
		}
	}
}

func TestSessionEncryptor_UninitialisedErrors(t *testing.T) {
	var encryptor *sessionEncryptor

	if _, err := encryptor.encrypt("value"); err == nil {
		t.Fatalf("expected error when encryptor is nil")
	} else if !strings.Contains(err.Error(), "encryptor is not initialised") {
		t.Fatalf("unexpected error message: %v", err)
	}

	encryptor = &sessionEncryptor{}
	if _, err := encryptor.encrypt("value"); err == nil {
		t.Fatalf("expected error when encryptor has nil AEAD")
	} else if !strings.Contains(err.Error(), "encryptor is not initialised") {
		t.Fatalf("unexpected error message: %v", err)
	}

	if _, err := encryptor.decrypt("value"); err == nil {
		t.Fatalf("expected error when decrypting with uninitialised encryptor")
	} else if !strings.Contains(err.Error(), "encryptor is not initialised") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
