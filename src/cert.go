// Reference: https://go.dev/src/crypto/tls/generate_cert.go
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

func generateCert() {
	// Generate RSA 2048-bit private key
	const rsaBits = 2048
	priv, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	// Certificate valid for 10 year
	const validFor = 10 * 365 * 24 * time.Hour

	// KeyUsage
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set.
	// In the context of TLS this KeyUsage is particular to RSA key exchange and authentication.
	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	// Serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("Failed to generate serial number: %v", err)
	}

	// Certificate template
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(validFor),
		KeyUsage:     keyUsage,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, priv.Public(), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %v", err)
	}

	// Create cert temp directory
	dir := ".certs"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0755)
	} else if err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}

	// cert.pem
	certOut, err := os.Create(filepath.Join(dir, "cert.pem"))
	if err != nil {
		log.Fatalf("Failed to open cert.pem for writing: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		log.Fatalf("Failed to write data to cert.pem: %v", err)
	}
	if err := certOut.Close(); err != nil {
		log.Fatalf("Error closing cert.pem: %v", err)
	}
	log.Print("wrote cert.pem\n")

	// key.pem
	keyOut, err := os.OpenFile(filepath.Join(dir, "key.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Failed to open key.pem for writing: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		log.Fatalf("Unable to marshal private key: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		log.Fatalf("Failed to write data to key.pem: %v", err)
	}
	if err := keyOut.Close(); err != nil {
		log.Fatalf("Error closing key.pem: %v", err)
	}
	log.Print("wrote key.pem\n")

	setHiddenAttribute(dir)
}
