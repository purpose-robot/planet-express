package krypto

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	// Generated key length, 16 bytes or more should be used.
	keyLength = 32
	// Generated token length, 32 bytes or more should be used.
	tokenLength = 32
	// SecretMarker is used to see exposed secrets by accident.
	SecretMarker = "<!SECRET_REDACTED!>"
)

var (
	// ErrInvalidKey indicates that the key is invalid.
	ErrInvalidKey = errors.New("invalid key")
	// ErrInvalidToken indicates that the token is invalid.
	ErrInvalidToken = errors.New("invalid token")
)

// Key is a 32-byte encryption key which holds a byte slice.
type Key struct {
	value []byte
}

func (k Key) Format(f fmt.State, _ rune) {
	_, _ = f.Write([]byte(SecretMarker))
}

func (k Key) MarshalText() ([]byte, error) {
	return []byte(SecretMarker), nil
}

// ParseKey expects a hex encoded key of 32 bytes (64 bytes as hex).
func ParseKey(raw string) (Key, error) {
	if len(raw) != 2*keyLength {
		return Key{}, ErrInvalidKey
	}

	value := make([]byte, keyLength)

	_, err := hex.Decode(value, []byte(raw))
	if err != nil {
		return Key{}, ErrInvalidKey
	}

	return Key{
		value: value,
	}, nil
}

// Token is a random token that is sent via email.
type Token [tokenLength]byte

// Hash returns the hashed representation of the token.
func (t Token) Hash() []byte {
	hash := sha256.Sum256(t[:])
	return hash[:]
}

// String returns the string representation of the token.
func (t Token) String() string {
	return hex.EncodeToString(t[:])
}

// GenerateToken creates a new random token.
func GenerateToken() (Token, error) {
	b, err := generateRandomBytes(tokenLength)
	if err != nil {
		return [tokenLength]byte{}, err
	}

	return [tokenLength]byte(b), err
}

// ParseToken parses a token from a string.
func ParseToken(raw string) (Token, error) {
	if len(raw) != 2*tokenLength {
		return Token{}, ErrInvalidToken
	}

	b, err := hex.DecodeString(raw)
	if err != nil {
		return [tokenLength]byte{}, ErrInvalidToken
	}

	return [tokenLength]byte(b), nil
}
