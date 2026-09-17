package config

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func keypair(t *testing.T, dir, name string) (string, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "example.invalid"},
		NotBefore: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), NotAfter: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	key, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certPath, keyPath := filepath.Join(dir, name+".crt"), filepath.Join(dir, name+".key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func TestTLS(t *testing.T) {
	dir := t.TempDir()
	cert, key := keypair(t, dir, "valid")
	_, otherKey := keypair(t, dir, "other")
	bad := filepath.Join(dir, "malformed.pem")
	if err := os.WriteFile(bad, []byte("not a PEM key or certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, cert, key string }{
		{"empty cert", "", key}, {"empty key", cert, ""},
		{"missing cert", filepath.Join(dir, "missing"), key}, {"missing key", cert, filepath.Join(dir, "missing")},
		{"unreadable cert", dir, key}, {"unreadable key", cert, dir},
		{"malformed cert", bad, key}, {"malformed key", cert, bad}, {"mismatched", cert, otherKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantInvalid(t, configData(t, map[string]any{"server.tls.enabled": true, "server.tls.cert_file": tc.cert, "server.tls.key_file": tc.key}), emptyEnv, "server.tls")
		})
	}
	wantInvalid(t, configData(t, map[string]any{"server.tls.enabled": true}), emptyEnv, "cert_file")
	cfg := mustLoad(t, configData(t, map[string]any{"server.tls.enabled": true, "server.tls.cert_file": "${CERT}", "server.tls.key_file": "${KEY}"}), env(map[string]string{"CERT": cert, "KEY": key}))
	if cfg.Server.TLS == nil || cfg.Server.TLS.CertFile != cert || cfg.Server.TLS.KeyFile != key || cfg.Server.TLS.Certificate.PrivateKey == nil || len(cfg.Server.TLS.Certificate.Certificate) == 0 {
		t.Fatal("TLS not fully normalized")
	}
	// Resolve relative TLS paths against CWD, even when YAML is elsewhere.
	t.Chdir(dir)
	if err := os.Mkdir("config-dir", 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join("config-dir", "uptime.yaml")
	if err := os.WriteFile(file, configData(t, map[string]any{"server.tls.enabled": true, "server.tls.cert_file": "valid.crt", "server.tls.key_file": "valid.key"}), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.TLS.CertFile != "valid.crt" || cfg.Server.TLS.KeyFile != "valid.key" {
		t.Fatal("relative TLS path changed")
	}
}
