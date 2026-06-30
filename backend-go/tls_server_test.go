package main

import (
	"crypto/x509"
	"net/http"
	"testing"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
)

func TestEndpointForEnv(t *testing.T) {
	tests := []struct {
		name string
		env  *config.EnvConfig
		want string
	}{
		{
			name: "http by default",
			env:  &config.EnvConfig{Port: 3688},
			want: "http://localhost:3688/v1",
		},
		{
			name: "https when enabled",
			env:  &config.EnvConfig{Port: 8443, EnableHTTPS: true},
			want: "https://localhost:8443/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := endpointForEnv(tt.env).URL("/v1"); got != tt.want {
				t.Fatalf("URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigureServerTLSRequiresCertificateSource(t *testing.T) {
	srv := &http.Server{}
	envCfg := &config.EnvConfig{EnableHTTPS: true, TLSAutoCert: false}

	if err := configureServerTLS(srv, envCfg); err == nil {
		t.Fatal("configureServerTLS() error = nil, want error")
	}
}

func TestConfigureServerTLSRequiresCertAndKeyPair(t *testing.T) {
	srv := &http.Server{}
	envCfg := &config.EnvConfig{EnableHTTPS: true, TLSCertFile: "/tmp/localhost.pem"}

	if err := configureServerTLS(srv, envCfg); err == nil {
		t.Fatal("configureServerTLS() error = nil, want error")
	}
}

func TestConfigureServerTLSAutoCert(t *testing.T) {
	srv := &http.Server{}
	envCfg := &config.EnvConfig{EnableHTTPS: true, TLSAutoCert: true}

	if err := configureServerTLS(srv, envCfg); err != nil {
		t.Fatalf("configureServerTLS() error = %v", err)
	}
	if srv.TLSConfig == nil || len(srv.TLSConfig.Certificates) != 1 {
		t.Fatalf("TLSConfig certificates = %#v, want one certificate", srv.TLSConfig)
	}
}

func TestGenerateLocalhostCertificateSANs(t *testing.T) {
	cert, err := generateLocalhostCertificate(time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("generateLocalhostCertificate() error = %v", err)
	}
	parsed, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}
	if err := parsed.VerifyHostname("localhost"); err != nil {
		t.Fatalf("localhost hostname verification failed: %v", err)
	}
	if err := parsed.VerifyHostname("127.0.0.1"); err != nil {
		t.Fatalf("127.0.0.1 hostname verification failed: %v", err)
	}
}
