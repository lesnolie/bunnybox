//go:build ignore

// Run explicitly: go run ./scripts/sign/check-cert.go build/cert/codesign.cer
package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		fail(fmt.Errorf("usage: check-cert.go certificate.cer (DER)"))
	}
	der, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		fail(err)
	}
	if !bytes.Equal(c.RawSubject, c.RawIssuer) {
		fail(fmt.Errorf("certificate is not self-issued"))
	}
	if err = c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature); err != nil {
		fail(err)
	}
	if c.IsCA || c.KeyUsage != x509.KeyUsageDigitalSignature {
		fail(fmt.Errorf("require CA:FALSE and digitalSignature-only key usage"))
	}
	if len(c.ExtKeyUsage) != 1 || c.ExtKeyUsage[0] != x509.ExtKeyUsageCodeSigning || len(c.UnknownExtKeyUsage) != 0 {
		fail(fmt.Errorf("require codeSigning-only extended key usage"))
	}
	switch key := c.PublicKey.(type) {
	case *rsa.PublicKey:
		if key.N.BitLen() < 3072 || c.SignatureAlgorithm != x509.SHA256WithRSA {
			fail(fmt.Errorf("require RSA >=3072 and SHA256WithRSA"))
		}
	case *ecdsa.PublicKey:
		if key.Curve != elliptic.P256() || c.SignatureAlgorithm != x509.ECDSAWithSHA256 {
			fail(fmt.Errorf("require ECDSA P-256 and ECDSAWithSHA256"))
		}
	default:
		fail(fmt.Errorf("unsupported public key"))
	}
	now := time.Now()
	if now.Before(c.NotBefore) || c.NotAfter.Before(now.AddDate(0, 18, 0)) {
		fail(fmt.Errorf("certificate must be valid now with >=18 months remaining"))
	}
	if c.SerialNumber.Sign() <= 0 || c.SerialNumber.BitLen() > 128 {
		fail(fmt.Errorf("require positive serial number of at most 128 bits"))
	}
	fmt.Printf("Certificate OK: SHA256=%X; expires=%s\n", sha256.Sum256(der), c.NotAfter.UTC().Format(time.RFC3339))
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
