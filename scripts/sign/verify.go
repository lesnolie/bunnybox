//go:build ignore

// Windows-only standalone verifier. Policy follows the pinned official
// third_party/sing-box/experimental/boxdd/authenticode_windows.go.
// Run: go run ./scripts/sign/verify.go -cert build/cert/codesign.cer file.exe ...
package main

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var trustDLL = systemTrustDLL()
var winVerify = trustDLL.NewProc("WinVerifyTrust")
var providerData = trustDLL.NewProc("WTHelperProvDataFromStateData")
var providerSigner = trustDLL.NewProc("WTHelperGetProvSignerFromChain")
var providerCert = trustDLL.NewProc("WTHelperGetProvCertFromChain")

func systemTrustDLL() *syscall.LazyDLL {
	// Match upstream NewLazySystemDLL: never load wintrust.dll from the CWD.
	var directory [32768]uint16
	n, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetSystemDirectoryW").Call(
		uintptr(unsafe.Pointer(&directory[0])), uintptr(len(directory)))
	if n == 0 || n >= uintptr(len(directory)) {
		panic("GetSystemDirectoryW failed")
	}
	return syscall.NewLazyDLL(filepath.Join(syscall.UTF16ToString(directory[:n]), "wintrust.dll"))
}

type guid struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}
type fileInfo struct {
	Size         uint32
	Path         *uint16
	File         syscall.Handle
	KnownSubject *guid
}
type trustData struct {
	Size                                    uint32
	PolicyCallback, SIPClient               unsafe.Pointer
	UIChoice, RevocationChecks, UnionChoice uint32
	FileInfo                                unsafe.Pointer
	StateAction                             uint32
	StateData                               syscall.Handle
	URLReference                            *uint16
	ProvFlags, UIContext                    uint32
}
type certContext struct {
	Encoding    uint32
	EncodedCert *byte
	Length      uint32
	Info        unsafe.Pointer
	Store       syscall.Handle
}
type providerCertificate struct {
	Size    uint32
	Context *certContext
}

func signer(path string) (der []byte, err error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	f := fileInfo{Path: p}
	f.Size = uint32(unsafe.Sizeof(f))
	d := trustData{UIChoice: 2, UnionChoice: 1, FileInfo: unsafe.Pointer(&f), StateAction: 1,
		ProvFlags: 0x1000 | 0x10 | 0x2000, UIContext: 0}
	d.Size = uint32(unsafe.Sizeof(d))
	action := guid{0xaac56b, 0xcd44, 0x11d0, [8]byte{0x8c, 0xc2, 0, 0xc0, 0x4f, 0xc2, 0x95, 0xee}}
	status, _, _ := winVerify.Call(^uintptr(0), uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&d)))
	defer func() {
		d.StateAction = 2
		closed, _, _ := winVerify.Call(^uintptr(0), uintptr(unsafe.Pointer(&action)), uintptr(unsafe.Pointer(&d)))
		if uint32(closed) != 0 && err == nil {
			err = fmt.Errorf("close WinVerifyTrust: 0x%08X", uint32(closed))
			der = nil
		}
	}()
	result := uint32(status)
	if result != 0 && result != 0x800b0109 && result != 0x800b010a {
		return nil, fmt.Errorf("WinVerifyTrust rejected signature: 0x%08X", result)
	}
	pd, _, _ := providerData.Call(uintptr(d.StateData))
	if pd == 0 {
		return nil, fmt.Errorf("missing provider data")
	}
	ps, _, _ := providerSigner.Call(pd, 0, 0, 0)
	if ps == 0 {
		return nil, fmt.Errorf("missing provider signer")
	}
	pc, _, _ := providerCert.Call(ps, 0)
	if pc == 0 {
		return nil, fmt.Errorf("missing provider certificate")
	}
	ctx := (*providerCertificate)(unsafe.Pointer(pc)).Context
	if ctx == nil || ctx.Length == 0 || ctx.EncodedCert == nil {
		return nil, fmt.Errorf("empty signer certificate")
	}
	der = append([]byte(nil), unsafe.Slice(ctx.EncodedCert, int(ctx.Length))...)
	c, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	codeSigning := false
	for _, eku := range c.ExtKeyUsage {
		if eku == x509.ExtKeyUsageCodeSigning || eku == x509.ExtKeyUsageAny {
			codeSigning = true
		}
	}
	if !codeSigning {
		return nil, fmt.Errorf("signer has no code-signing EKU")
	}
	if result != 0 {
		if !bytes.Equal(c.RawSubject, c.RawIssuer) {
			return nil, fmt.Errorf("untrusted signer is not self-issued")
		}
		if err = c.CheckSignature(c.SignatureAlgorithm, c.RawTBSCertificate, c.Signature); err != nil {
			return nil, err
		}
		now := time.Now()
		if now.Before(c.NotBefore) || now.After(c.NotAfter) {
			return nil, fmt.Errorf("untrusted signer is outside certificate validity")
		}
	}
	return der, nil
}

func main() {
	certificate := flag.String("cert", "build/cert/codesign.cer", "expected signer DER certificate")
	flag.Parse()
	if flag.NArg() == 0 {
		fatal(fmt.Errorf("usage: verify.go -cert certificate.cer file.exe [file.exe ...]"))
	}
	expected, err := os.ReadFile(*certificate)
	if err != nil {
		fatal(err)
	}
	if _, err = x509.ParseCertificate(expected); err != nil {
		fatal(err)
	}
	for _, path := range flag.Args() {
		actual, err := signer(path)
		if err != nil {
			fatal(fmt.Errorf("%s: %w", path, err))
		}
		if !bytes.Equal(actual, expected) {
			fatal(fmt.Errorf("%s: signer DER does not match expected certificate", path))
		}
		fmt.Printf("Signature OK: %s; signer SHA256=%X\n", path, sha256.Sum256(actual))
	}
}
func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
