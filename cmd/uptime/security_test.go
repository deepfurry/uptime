package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestSecurityConfigCheckAndCanceledServe(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cert, key := serveKeypair(t, dir)
	hash, err := bcrypt.GenerateFromPassword([]byte("PRIVATE"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tlsEnabled := range []bool{false, true} {
		for _, authEnabled := range []bool{false, true} {
			writeConfig(t, "uptime.yaml", validConfig+fmt.Sprintf("server: {tls: {enabled: %t, cert_file: %q, key_file: %q}}\nauth: {enabled: %t, basic: {username: '运维', password_hash: %q}}\n", tlsEnabled, cert, key, authEnabled, string(hash)))
			for _, args := range [][]string{{"config", "check"}, {"serve"}} {
				var stdout, stderr bytes.Buffer
				code := run(ctx, args, &stdout, &stderr)
				want := ""
				if args[0] == "config" {
					want = "configuration is valid\n"
				}
				if code != 0 || stdout.String() != want || stderr.Len() != 0 {
					t.Fatalf("tls=%v auth=%v args=%v: code=%d stderr=%s", tlsEnabled, authEnabled, args, code, &stderr)
				}
			}
		}
	}
	for _, data := range []string{
		"auth: {enabled: true, basic: {username: 'PRIVATE:USER', password_hash: PRIVATE-HASH}}",
		"auth: {enabled: true, basic: {username: PRIVATE-USER, password_hash: PRIVATE-HASH}}",
		"server: {tls: {enabled: true, cert_file: PRIVATE-MISSING, key_file: PRIVATE-MISSING}}",
	} {
		writeConfig(t, "uptime.yaml", validConfig+data)
		for _, args := range [][]string{{"config", "check"}, {"serve"}} {
			var stdout, stderr bytes.Buffer
			if code := run(ctx, args, &stdout, &stderr); code != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "PRIVATE") {
				t.Fatal("missing or unsafe config diagnostic")
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatal("offline validation/canceled serve created runtime files")
	}
}
