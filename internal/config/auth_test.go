package config

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func authHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(hash)
}

func TestBcryptContract(t *testing.T) {
	hash := authHash(t)
	for _, family := range []string{"$2a$", "$2b$", "$2y$"} {
		for _, cost := range []string{"04", "12", "31"} {
			value := family + cost + hash[6:]
			cfg := mustLoad(t, configData(t, map[string]any{"auth.enabled": true, "auth.basic.username": "admin", "auth.basic.password_hash": value}), emptyEnv)
			if cfg.Auth.PasswordHash != value {
				t.Fatal("hash modified")
			}
		}
	}
	for _, value := range []string{
		"plaintext-SECRET", "{SHA256}" + strings.Repeat("A", 44), "{SHA512}" + strings.Repeat("A", 88),
		strings.Repeat("a", 64), "$2x$" + hash[4:], "$2$" + hash[4:],
		hash[:len(hash)-1], hash + "a", hash[:len(hash)-1] + "!", " " + hash, hash + "\n",
		"$2a$03$" + hash[7:], "$2a$32$" + hash[7:], "$2a$xx$" + hash[7:],
		"${NESTED}",
	} {
		for _, input := range []string{value, "${HASH}"} {
			_, err := load(configData(t, map[string]any{"auth.enabled": true, "auth.basic.username": "private-user", "auth.basic.password_hash": input}), env(map[string]string{"HASH": value, "NESTED": "invalid-SECRET"}))
			if err == nil || !strings.Contains(err.Error(), "auth.basic.password_hash") || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), value) || strings.Contains(err.Error(), "private-user") {
				t.Fatalf("missing or unsafe bcrypt error: %v", err)
			}
		}
	}
}

func TestBasicAuthUsername(t *testing.T) {
	hash := authHash(t)
	for _, tc := range []struct{ input, want string }{
		{"admin", "admin"}, {"operator@example", "operator@example"}, {"运维", "运维"}, {"ope\u0301rateur", "opérateur"},
	} {
		for _, input := range []string{tc.input, "${USER}"} {
			cfg := mustLoad(t, configData(t, map[string]any{"auth.enabled": true, "auth.basic.username": input, "auth.basic.password_hash": hash}), env(map[string]string{"USER": tc.input}))
			if cfg.Auth.Username != tc.want {
				t.Fatal("username not NFC normalized")
			}
		}
	}
	for _, value := range []string{"", "private:SECRET", "private\nSECRET", "private\rSECRET", "private\tSECRET", "private\x00SECRET", "private\x7fSECRET", "private\u0085SECRET"} {
		for _, input := range []string{value, "${USER}"} {
			_, err := load(configData(t, map[string]any{"auth.enabled": true, "auth.basic.username": input, "auth.basic.password_hash": hash}), env(map[string]string{"USER": value}))
			if err == nil || !strings.Contains(err.Error(), "auth.basic.username") || strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), hash) {
				t.Fatalf("missing or unsafe username error: %v", err)
			}
		}
	}
	for _, values := range [][2]string{{"bad:username", "plaintext"}, {"${MISSING}", "${INVALID:-expression}"}} {
		cfg := mustLoad(t, configData(t, map[string]any{"auth.enabled": false, "auth.basic.username": values[0], "auth.basic.password_hash": values[1]}), func(string) (string, bool) { t.Fatal("inactive auth resolved env"); return "", false })
		if cfg.Auth != nil {
			t.Fatal("inactive auth retained")
		}
	}
}
