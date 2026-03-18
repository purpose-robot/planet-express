package krypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
)

const indexBytes = 4

var (
	// ErrUnknownKey indicates that the key is unknown.
	ErrUnknownKey = errors.New("unknown key")
	// ErrInvalidData indicates that the data is invalid.
	ErrInvalidData = errors.New("invalid data")
)

// Encryptor encrypts and decrypts data using AES-GCM.
//
// The encryptor uses an append only list of keys for encryption and decryption.
// The last key in the list is considered the latest key.
//
// To construct output data, the encryptor prefixes the encrypted data with the index
// of the used key. This allows the encryptor to work with multiple keys and to decrypt
// data encrypted with an older key.
//
// The index used is not considered secret.
type Encryptor struct {
	keys []Key
}

// NewEncryptor creates a new encryptor with the provided keys.
func NewEncryptor(keys []Key) (*Encryptor, error) {
	if len(keys) == 0 {
		return nil, errors.New("at least one key is required")
	}

	return &Encryptor{keys: keys}, nil
}

// Decrypt decrypts the data using the key identified by
// the first 4 bytes in the data.
// It returns the decrypted data or an error.
func (e *Encryptor) Decrypt(data []byte) ([]byte, error) {
	if len(data) < indexBytes {
		return nil, ErrInvalidData
	}

	index := binary.BigEndian.Uint32(data[:indexBytes])
	if int(index) >= len(e.keys) {
		return nil, ErrUnknownKey
	}

	cipherBlock, err := aes.NewCipher(e.keys[index].value)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(cipherBlock)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	minLen := indexBytes + nonceSize
	if len(data) <= minLen {
		return nil, ErrInvalidData
	}

	nonce := data[indexBytes:minLen]
	ciphertext := data[minLen:]

	return gcm.Open(nil, nonce, ciphertext, data[:4])
}

// Encrypt encrypts the data using the latest available key.
// It returns the encrypted data prefixed with the key identifier.
func (e *Encryptor) Encrypt(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidData
	}

	index := len(e.keys) - 1
	block, err := aes.NewCipher(e.keys[index].value)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce, err := generateRandomBytes(uint32(gcm.NonceSize()))
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, indexBytes)
	binary.BigEndian.PutUint32(buffer, uint32(index))

	encryptedOut := gcm.Seal(nil, nonce, data, buffer)
	buffer = append(buffer, nonce...)
	buffer = append(buffer, encryptedOut...)

	return buffer, nil
}
