// Package secret seals the passwords that profiles ask to keep.
//
// What it protects against, and what it does not:
//
//   - The key is a 32 byte random value in its own file (`secret.key`, mode
//     0600) next to the profiles, and the payload is AES-256-GCM. A profile
//     file that is copied, synced to a cloud folder, attached to an issue or
//     read out of a backup therefore carries no readable password, and a
//     hand-edited ciphertext no longer authenticates.
//   - It is not a defence against someone who can read both files in your
//     profile directory: they can unseal it too. Keeping the key in the OS
//     keychain (DPAPI, Keychain, libsecret) is the next step up, and is not
//     something this package does.
//
// The key file is created on first use, so installs that never store a
// password never have one. Everything in here is safe for concurrent use.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	// keyName is the key file, next to connections.json. KeyFileName is the
	// exported spelling the data-directory move needs: the key is part of the
	// data, and a move that left it behind would strand every saved password.
	keyName     = "secret.key"
	KeyFileName = keyName
	// keySize is what AES-256 wants.
	keySize = 32
	// keyMode keeps the key to the owner, like the profile file itself.
	keyMode = 0o600

	// TokenPrefix marks a value as sealed. It carries the scheme version so a
	// future change of algorithm can be recognised instead of mis-decrypted.
	TokenPrefix = "enc:v1:"

	// aad binds a ciphertext to this application: a token lifted into another
	// program using the same key would not authenticate.
	aad = "db-manager/connection-password/v1"
)

// Cipher seals and unseals values with the key in Dir. The zero value is not
// usable; call Open.
type Cipher struct {
	dir string

	mu  sync.Mutex
	key []byte
}

// Open returns a Cipher backed by the key file in dir. No I/O happens here: the
// key is read (or created) the first time it is needed, so a store that never
// touches a password never touches the key.
func Open(dir string) *Cipher { return &Cipher{dir: dir} }

// IsSealed reports whether value is a token produced by Seal rather than a
// plain value written by an older build.
func IsSealed(value string) bool { return strings.HasPrefix(value, TokenPrefix) }

// Seal encrypts plain into a self-describing token. Sealing an empty string is
// a no-op, so callers do not have to special-case "no password".
func (c *Cipher) Seal(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := c.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, block.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secret: read nonce: %w", err)
	}
	// The nonce travels in front of the ciphertext; GCM authenticates both.
	sealed := block.Seal(nonce, nonce, []byte(plain), []byte(aad))
	return TokenPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Unseal reverses Seal. Values that are not sealed (an empty string, or a
// plain-text password written by an older build) come back unchanged, and a
// token that does not authenticate is reported as an error rather than as an
// empty password, so the caller can tell "no secret" from "cannot read it".
func (c *Cipher) Unseal(value string) (string, error) {
	if !IsSealed(value) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, TokenPrefix))
	if err != nil {
		return "", fmt.Errorf("secret: %q is not a valid token: %w", value, err)
	}
	block, err := c.aead()
	if err != nil {
		return "", err
	}
	if len(raw) < block.NonceSize() {
		return "", errors.New("secret: token is too short to hold a nonce")
	}
	nonce, body := raw[:block.NonceSize()], raw[block.NonceSize():]
	plain, err := block.Open(nil, nonce, body, []byte(aad))
	if err != nil {
		return "", fmt.Errorf("secret: token did not authenticate: %w", err)
	}
	return string(plain), nil
}

// aead returns the GCM instance, loading the key on first use.
func (c *Cipher) aead() (cipher.AEAD, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.key == nil {
		key, err := c.loadKeyLocked()
		if err != nil {
			return nil, err
		}
		c.key = key
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("secret: %w", err)
	}
	return cipher.NewGCM(block)
}

func (c *Cipher) keyPath() string { return filepath.Join(c.dir, keyName) }

func (c *Cipher) loadKeyLocked() ([]byte, error) {
	raw, err := os.ReadFile(c.keyPath())
	switch {
	case err == nil && len(strings.TrimSpace(string(raw))) > 0:
		key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("secret: %s is not valid base64: %w", c.keyPath(), err)
		}
		if len(key) != keySize {
			return nil, fmt.Errorf("secret: %s holds %d bytes, want %d", c.keyPath(), len(key), keySize)
		}
		return key, nil
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("secret: read key: %w", err)
	}

	// No key yet (or an empty file left behind by a crash): make one. An
	// existing but empty file cannot have sealed anything, so overwriting it
	// loses nothing.
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secret: generate key: %w", err)
	}
	if err := os.WriteFile(c.keyPath(), []byte(base64.StdEncoding.EncodeToString(key)+"\n"), keyMode); err != nil {
		return nil, fmt.Errorf("secret: write key: %w", err)
	}
	return key, nil
}
