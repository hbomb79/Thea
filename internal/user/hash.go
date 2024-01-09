package user

import (
	"bytes"
	"crypto/rand"
	"errors"

	"golang.org/x/crypto/argon2"
)

type (
	argonHasher struct {
		time    uint32
		memory  uint32
		threads uint8
		keyLen  uint32
		saltLen uint32
	}

	hashAndSalt struct {
		hash []byte
		salt []byte
	}
)

func newArgon2IdHasher(time, saltLen uint32, memory uint32, threads uint8, keyLen uint32) *argonHasher {
	return &argonHasher{time: time, saltLen: saltLen, memory: memory, threads: threads, keyLen: keyLen}
}

// GenerateHash using the password and provided salt.
// If not salt value provided fallback to random value
// generated of a given length.
func (a *argonHasher) GenerateHash(password, salt []byte) (*hashAndSalt, error) {
	var err error
	// If salt is not provided generate a salt of
	// the configured salt length.
	if len(salt) == 0 {
		salt, err = randomSecret(a.saltLen)
	}
	if err != nil {
		return nil, err
	}
	hash := argon2.IDKey(password, salt, a.time, a.memory, a.threads, a.keyLen)
	return &hashAndSalt{hash, salt}, nil
}

// Compare generated hash with store hash.
func (a *argonHasher) Compare(hash, salt, password []byte) error {
	hashSalt, err := a.GenerateHash(password, salt)
	if err != nil {
		return err
	}

	if !bytes.Equal(hash, hashSalt.hash) {
		return errors.New("hash doesn't match")
	}
	return nil
}

// randomSecret generates a random byte slice of the
// requested length. This is used to create random
// salts for the hashing of passwords.
func randomSecret(length uint32) ([]byte, error) {
	secret := make([]byte, length)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}
