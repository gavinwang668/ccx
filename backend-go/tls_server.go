package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"time"

	"github.com/BenedictKing/ccx/internal/config"
)

const localTLSCertValidity = 365 * 24 * time.Hour

type serverEndpoint struct {
	Scheme string
	Host   string
}

func endpointForEnv(envCfg *config.EnvConfig) serverEndpoint {
	scheme := "http"
	if envCfg.EnableHTTPS {
		scheme = "https"
	}
	return serverEndpoint{
		Scheme: scheme,
		Host:   fmt.Sprintf("localhost:%d", envCfg.Port),
	}
}

func (e serverEndpoint) URL(path string) string {
	return fmt.Sprintf("%s://%s%s", e.Scheme, e.Host, path)
}

func configureServerTLS(srv *http.Server, envCfg *config.EnvConfig) error {
	if !envCfg.EnableHTTPS {
		return nil
	}
	srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	if envCfg.TLSCertFile != "" || envCfg.TLSKeyFile != "" {
		if envCfg.TLSCertFile == "" || envCfg.TLSKeyFile == "" {
			return fmt.Errorf("TLS_CERT_FILE 和 TLS_KEY_FILE 必须同时设置")
		}
		return nil
	}
	if !envCfg.TLSAutoCert {
		return fmt.Errorf("ENABLE_HTTPS=true requires TLS_CERT_FILE/TLS_KEY_FILE, or TLS_AUTO_CERT=true for local self-signed TLS")
	}
	cert, err := generateLocalhostCertificate(time.Now())
	if err != nil {
		return fmt.Errorf("生成本地 HTTPS 自签名证书失败: %w", err)
	}
	srv.TLSConfig.Certificates = []tls.Certificate{cert}
	return nil
}

func startHTTPServer(srv *http.Server, envCfg *config.EnvConfig) error {
	if !envCfg.EnableHTTPS {
		return srv.ListenAndServe()
	}
	if envCfg.TLSCertFile != "" || envCfg.TLSKeyFile != "" {
		if envCfg.TLSCertFile == "" || envCfg.TLSKeyFile == "" {
			return fmt.Errorf("TLS_CERT_FILE 和 TLS_KEY_FILE 必须同时设置")
		}
		return srv.ListenAndServeTLS(envCfg.TLSCertFile, envCfg.TLSKeyFile)
	}
	return srv.ListenAndServeTLS("", "")
}

func generateLocalhostCertificate(now time.Time) (tls.Certificate, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return tls.Certificate{}, err
	}
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(localTLSCertValidity),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return tls.X509KeyPair(certPEM, keyPEM)
}
