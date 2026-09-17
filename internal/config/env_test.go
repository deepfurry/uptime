package config

import (
	"strings"
	"testing"
	"time"
)

func TestInterpolation(t *testing.T) {
	lookup := env(map[string]string{"HOST": "example.invalid", "TOKEN": "secret", "EMPTY": "", "NESTED": "${NOT_SET}"})
	for _, tc := range []struct{ value, want string }{
		{"plain", "plain"}, {"$HOST", "$HOST"}, {"$", "$"},
		{"$(command)", "$(command)"}, {"$2b$12$bcrypt-looking", "$2b$12$bcrypt-looking"},
		{"https://${HOST}/?token=${TOKEN}", "https://example.invalid/?token=secret"},
		{"${HOST}${HOST}", "example.invalidexample.invalid"}, {"${EMPTY}", ""},
		{"${NESTED}", "${NOT_SET}"},
	} {
		got, err := interpolate(tc.value, lookup)
		if err != nil || got != tc.want {
			t.Fatalf("interpolation result/error mismatch: %v", err)
		}
	}
	for _, value := range []string{"${MISSING}", "${}", "${1BAD}", "${BAD-NAME}", "${BAD NAME}", "${HOST:-fallback}", "${HOST-default}", "${HOST?error}", "${HOST", "${${HOST}}"} {
		if _, err := interpolate(value, lookup); err == nil {
			t.Fatal("invalid/missing expression accepted")
		}
	}
}

func TestActiveEnvironmentAndInactiveDiscard(t *testing.T) {
	changes := map[string]any{
		"server.address": "${ADDRESS}", "server.shutdown_timeout": "${SHUTDOWN}",
		"server.tls.enabled": false, "server.tls.cert_file": "${BAD:-ignored}", "server.tls.key_file": "${MISSING}",
		"storage.bbolt.path": "${DIR}/uptime.db", "storage.redis.url": "${BAD:-ignored}", "storage.redis.key_prefix": "",
		"auth.enabled": false, "auth.basic.username": "${MISSING}", "auth.basic.password_hash": "${bad expression}",
		"uptime.interval": "${INTERVAL}", "uptime.retention": "${RETENTION}", "uptime.window": "${WINDOW}", "uptime.timezone": "${ZONE}",
		"ui.path": "${UI_PATH}", "ui.title": "${TITLE}", "ui.description": "${EMPTY}", "ui.footer": "${FOOTER}", "ui.favicon_url": "${ICON}",
		"endpoints.0.name": "${NAME}", "endpoints.0.description": "${DESCRIPTION}", "endpoints.0.url": "${URL}", "endpoints.0.timeout": "${TIMEOUT}",
		"endpoints.0.headers": map[string]string{"authorization": "Bearer ${TOKEN}"},
	}
	values := map[string]string{
		"ADDRESS": "localhost:8081", "SHUTDOWN": "2s", "DIR": "./somewhere", "INTERVAL": "3s", "RETENTION": "60d", "WINDOW": "10d", "ZONE": "Asia/Shanghai",
		"UI_PATH": "/status", "TITLE": "Status", "EMPTY": "", "FOOTER": "footer", "ICON": "/icon.svg", "NAME": "Website", "DESCRIPTION": "description", "URL": "https://example.invalid/", "TIMEOUT": "1s", "TOKEN": "injected-secret",
	}
	cfg := mustLoad(t, configData(t, changes), env(values))
	if cfg.Server.TLS != nil || cfg.Storage.Redis != nil || cfg.Auth != nil {
		t.Fatal("inactive data survived")
	}
	if cfg.Server.Address != values["ADDRESS"] || cfg.Server.ShutdownTimeout != 2*time.Second || cfg.Storage.Bbolt.Path != "./somewhere/uptime.db" || cfg.Uptime.Interval != 3*time.Second || cfg.Uptime.Retention != 60 || cfg.Uptime.Window != 10 || cfg.Uptime.Timezone.String() != values["ZONE"] {
		t.Fatal("active server/storage/uptime resolution")
	}
	if cfg.UI.Title != "Status" || cfg.UI.Description != "" || cfg.UI.Footer != "footer" || cfg.UI.FaviconURL.Path != "/icon.svg" {
		t.Fatal("active UI resolution")
	}
	e := cfg.Endpoints[0]
	if e.Name != "Website" || e.Description != "description" || e.Interval != 3*time.Second || e.Timeout != time.Second || e.Headers["Authorization"] != "Bearer injected-secret" {
		t.Fatal("active endpoint resolution")
	}
	wantInvalid(t, configData(t, map[string]any{"endpoints.0.url": "${MISSING}"}), emptyEnv, "MISSING")
	wantInvalid(t, configData(t, map[string]any{"ui.title": "${EMPTY}"}), env(values), "value is required")
	// Literal fields cannot be enabled by environment even when values exist.
	for field, value := range map[string]string{"storage.type": "bbolt", "endpoints.0.id": "web", "endpoints.0.method": "GET"} {
		wantInvalid(t, configData(t, map[string]any{field: "${LITERAL}"}), func(string) (string, bool) { t.Fatal("looked up literal field"); return value, true }, strings.ReplaceAll(field, ".0.", "[0]."))
	}
}

func TestAuth(t *testing.T) {
	for _, hash := range []string{"$2b$12$literal-bcrypt-looking-hash", "not-parsed-by-config", "${HASH}"} {
		cfg := mustLoad(t, configData(t, map[string]any{"auth.enabled": true, "auth.basic.username": "${USER}", "auth.basic.password_hash": hash}), env(map[string]string{"USER": "admin", "HASH": "${UNRESOLVED_IS_LITERAL}"}))
		if cfg.Auth == nil || cfg.Auth.Username != "admin" {
			t.Fatal("missing normalized auth")
		}
		want := hash
		if hash == "${HASH}" {
			want = "${UNRESOLVED_IS_LITERAL}"
		}
		if cfg.Auth.PasswordHash != want {
			t.Fatal("hash was parsed or recursively expanded")
		}
	}
	for _, field := range []string{"username", "password_hash"} {
		changes := map[string]any{"auth.enabled": true, "auth.basic.username": "admin", "auth.basic.password_hash": "hash"}
		changes["auth.basic."+field] = ""
		wantInvalid(t, configData(t, changes), emptyEnv, "auth.basic."+field)
		delete(changes, "auth.basic."+field)
		wantInvalid(t, configData(t, changes), emptyEnv, "auth.basic."+field)
	}
}

func TestHeaders(t *testing.T) {
	cfg := mustLoad(t, configData(t, map[string]any{"endpoints.0.headers": map[string]string{"authorization": "Bearer ${TOKEN}", "user-agent": "agent", "ACCEPT": "application/json", "X-empty": ""}}), env(map[string]string{"TOKEN": "value"}))
	h := cfg.Endpoints[0].Headers
	if len(h) != 4 || h["Authorization"] != "Bearer value" || h["User-Agent"] != "agent" || h["Accept"] != "application/json" {
		t.Fatal("headers not canonicalized")
	}
	for _, headers := range []map[string]string{
		{"Accept": "a", "accept": "b"}, {"Host": "example.invalid"}, {"hOsT": "example.invalid"},
		{"": "secret"}, {"has space": "secret"}, {"Bad:Name": "secret"}, {"非ASCII": "secret"}, {"${HEADER}": "secret"},
		{"X-Secret": "secret\rdata"}, {"X-Secret": "secret\ndata"}, {"X-Secret": "${CRLF}"},
	} {
		wantInvalid(t, configData(t, map[string]any{"endpoints.0.headers": headers}), env(map[string]string{"CRLF": "secret\r\nInjected: value"}), "headers")
	}
}

func TestSecretSafeErrors(t *testing.T) {
	const secret = "SENSITIVE-unique-token"
	for _, changes := range []map[string]any{
		{"storage.type": "redis", "storage.redis.url": "redis://user:" + secret + "@example.invalid:bad/0"},
		{"endpoints.0.url": "https://example.invalid/%zz?token=" + secret},
		{"endpoints.0.headers": map[string]string{"Authorization": "Bearer " + secret + "\n"}},
		{"auth.enabled": true, "auth.basic.username": "", "auth.basic.password_hash": secret},
		{"uptime.interval": "${VALUE}"},
		{"ui.thresholds.green": secret},
		{"server.tls.enabled": true, "server.tls.cert_file": "${VALUE}", "server.tls.key_file": "${VALUE}"},
	} {
		_, err := load(configData(t, changes), env(map[string]string{"VALUE": secret}))
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("unsafe or missing error: %v", err)
		}
	}
	for _, data := range []string{minimal + "ui: {title: [" + secret, minimal + secret + ": value\n", minimal + "ui: {title: '${" + secret + ":-default}'}"} {
		_, err := load([]byte(data), emptyEnv)
		if err == nil || strings.Contains(err.Error(), secret) {
			t.Fatalf("unsafe YAML/env diagnostic: %v", err)
		}
	}
}
