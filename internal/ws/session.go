package ws

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var sessionKey = func() []byte {
	if v := os.Getenv("SESSION_SECRET"); len(v) >= 32 {
		return []byte(v)
	}
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "data"
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	path := filepath.Join(dir, "session.key")
	if key, err := os.ReadFile(path); err == nil {
		if len(key) != 32 {
			panic("invalid session key")
		}
		return key
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		panic(err)
	}
	return b
}()

type identity struct {
	UID     string `json:"uid"`
	Name    string `json:"name"`
	Expires int64  `json:"expires"`
}

func session(token, name string) (identity, string, error) {
	if token != "" {
		parts := strings.Split(token, ".")
		if len(parts) != 2 {
			return identity{}, "", errors.New("invalid session")
		}
		mac := hmac.New(sha256.New, sessionKey)
		mac.Write([]byte(parts[0]))
		sig, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
			return identity{}, "", errors.New("invalid session")
		}
		raw, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			return identity{}, "", err
		}
		var id identity
		if json.Unmarshal(raw, &id) != nil || id.UID == "" || id.Expires < time.Now().Unix() {
			return identity{}, "", errors.New("expired session")
		}
		if updated := normalizeAnyUID(name); updated != "" && updated != id.Name {
			id.Name = updated
			return signIdentity(id)
		}
		return id, token, nil
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return identity{}, "", err
	}
	id := identity{hex.EncodeToString(b), normalizeAnyUID(name), time.Now().Add(7 * 24 * time.Hour).Unix()}
	if id.Name == "" {
		id.Name = "牌友" + id.UID[:4]
	}
	return signIdentity(id)
}
func signIdentity(id identity) (identity, string, error) {
	raw, _ := json.Marshal(id)
	payload := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(payload))
	return id, payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
