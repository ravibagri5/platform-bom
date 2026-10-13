package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// sealer encrypts and authenticates cookie values with AES-GCM. The cookie
// name is bound as additional data so one cookie cannot stand in for another.
type sealer struct{ aead cipher.AEAD }

func newSealer(secret []byte) (*sealer, error) {
	key := sha256.Sum256(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &sealer{aead: aead}, nil
}

type sealed[T any] struct {
	Value   T     `json:"v"`
	Expires int64 `json:"e"`
}

func seal[T any](s *sealer, name string, v T, ttl time.Duration) (string, error) {
	plain, err := json.Marshal(sealed[T]{Value: v, Expires: time.Now().Add(ttl).Unix()})
	if err != nil {
		return "", err
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain, []byte(name))), nil
}

func unseal[T any](s *sealer, name, value string) (T, error) {
	var zero T
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) < s.aead.NonceSize() {
		return zero, errors.New("malformed cookie")
	}
	n := s.aead.NonceSize()
	plain, err := s.aead.Open(nil, raw[:n], raw[n:], []byte(name))
	if err != nil {
		return zero, errors.New("invalid cookie")
	}
	var out sealed[T]
	if err := json.Unmarshal(plain, &out); err != nil {
		return zero, err
	}
	if time.Now().Unix() > out.Expires {
		return zero, errors.New("expired cookie")
	}
	return out.Value, nil
}

type cookies struct {
	sealer *sealer
	secure bool
}

func (c cookies) set(w http.ResponseWriter, name, path string, v any, ttl time.Duration) error {
	value, err := seal(c.sealer, name, v, ttl)
	if err != nil {
		return err
	}
	// Secure is off only for http:// redirect URLs, such as local testing.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // HttpOnly and SameSite are set; Secure follows the redirect URL scheme
		Name: name, Value: value, Path: path, MaxAge: int(ttl.Seconds()),
		HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (c cookies) clear(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // clears the cookie; same attributes as set
		Name: name, Value: "", Path: path, MaxAge: -1,
		HttpOnly: true, Secure: c.secure, SameSite: http.SameSiteLaxMode,
	})
}

func get[T any](c cookies, r *http.Request, name string) (T, bool) {
	var zero T
	ck, err := r.Cookie(name)
	if err != nil {
		return zero, false
	}
	v, err := unseal[T](c.sealer, name, ck.Value)
	return v, err == nil
}
