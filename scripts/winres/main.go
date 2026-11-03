// Command winres writes the Windows resources of the GUI (icon, manifest,
// version information) as a .syso object that go build links into the
// executable.
//
// Adapted from MyGo cmd/mygo/winres.go and icon.go
// (third_party/mygo, commit 1f039303abe1bf8b8f6a94ae15c4d6c5ff5d739f, MIT
// license, Copyright (c) the MyGo authors). Changes: a standalone command
// with fixed options instead of the MyGo app configuration; one language;
// no MyGoIdentifier, so the app keeps setting its own name.
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24
	langZhCN    = 0x0804
)

var iconSizes = []int{16, 24, 32, 48, 64, 128, 256}

func main() {
	icon := flag.String("icon", "", "square PNG icon")
	out := flag.String("o", "", "output .syso")
	name := flag.String("name", "BunnyBox", "product name")
	version := flag.String("version", "0.1.0", "product version")
	exe := flag.String("exe", "sing-box.exe", "executable file name")
	arch := flag.String("arch", "amd64", "amd64, arm64 or 386")
	flag.Parse()
	if *icon == "" || *out == "" {
		log.Fatal("usage: winres -icon icon.png -o rsrc_windows_amd64.syso")
	}
	src, err := os.ReadFile(*icon)
	if err != nil {
		log.Fatal(err)
	}
	syso, err := build(src, *name, *version, *exe, *arch)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, syso, 0o644); err != nil {
		log.Fatal(err)
	}
}

func build(iconPNG []byte, name, version, exe, goarch string) ([]byte, error) {
	arch, ok := map[string]struct{ machine, relocation uint16 }{
		"amd64": {0x8664, 0x0003},
		"arm64": {0xAA64, 0x0002},
		"386":   {0x014C, 0x0007},
	}[goarch]
	if !ok {
		return nil, fmt.Errorf("no Windows resources for %s", goarch)
	}
	images, err := iconImages(iconPNG)
	if err != nil {
		return nil, fmt.Errorf("icon: %w", err)
	}
	res := map[uint16]map[uint16][]byte{
		rtVersion:  {1: versionInfo(name, version, exe)},
		rtManifest: {1: []byte(manifest(version))},
		rtIcon:     {},
	}
	var group bytes.Buffer
	_ = binary.Write(&group, binary.LittleEndian, [3]uint16{0, 1, uint16(len(images))})
	for i, img := range images {
		id := uint16(i + 1)
		res[rtIcon][id] = img
		dim := byte(iconSizes[i])
		if iconSizes[i] >= 256 {
			dim = 0
		}
		group.Write([]byte{dim, dim, 0, 0})
		_ = binary.Write(&group, binary.LittleEndian, [2]uint16{1, 32})
		_ = binary.Write(&group, binary.LittleEndian, uint32(len(img)))
		_ = binary.Write(&group, binary.LittleEndian, id)
	}
	res[rtGroupIcon] = map[uint16][]byte{1: group.Bytes()}
	return coff(res, arch.machine, arch.relocation), nil
}

// coff lays out the resource section (type, id and language directories,
// data entries, data) in a COFF object with one .rsrc section.
func coff(res map[uint16]map[uint16][]byte, machine, relocation uint16) []byte {
	types := sortedKeys(res)
	dirSize := func(n int) int { return 16 + 8*n }
	size := dirSize(len(types))
	count := 0
	for _, t := range types {
		size += dirSize(len(res[t])) + len(res[t])*dirSize(1)
		count += len(res[t])
	}
	entriesAt := size
	dataAt := entriesAt + 16*count

	raw := make([]byte, dataAt)
	le := binary.LittleEndian
	dir := func(at, n int) { le.PutUint16(raw[at+14:], uint16(n)) }
	var relocs []uint32
	var data bytes.Buffer
	dir(0, len(types))
	next := dirSize(len(types))
	entry := entriesAt
	for i, t := range types {
		le.PutUint32(raw[16+8*i:], uint32(t))
		le.PutUint32(raw[16+8*i+4:], uint32(next)|0x80000000)
		ids := sortedKeys(res[t])
		idDir := next
		dir(idDir, len(ids))
		next += dirSize(len(ids))
		for j, id := range ids {
			le.PutUint32(raw[idDir+16+8*j:], uint32(id))
			le.PutUint32(raw[idDir+16+8*j+4:], uint32(next)|0x80000000)
			dir(next, 1)
			le.PutUint32(raw[next+16:], langZhCN)
			le.PutUint32(raw[next+20:], uint32(entry))
			next += dirSize(1)
			for data.Len()%8 != 0 {
				data.WriteByte(0)
			}
			le.PutUint32(raw[entry:], uint32(dataAt+data.Len()))
			le.PutUint32(raw[entry+4:], uint32(len(res[t][id])))
			relocs = append(relocs, uint32(entry))
			entry += 16
			data.Write(res[t][id])
		}
	}
	raw = append(raw, data.Bytes()...)
	for len(raw)%4 != 0 {
		raw = append(raw, 0)
	}

	var out bytes.Buffer
	const headerSize, sectionHeaderSize = 20, 40
	rawAt := headerSize + sectionHeaderSize
	relocAt := rawAt + len(raw)
	symbolsAt := relocAt + 10*len(relocs)
	_ = binary.Write(&out, le, struct {
		Machine, Sections                   uint16
		Time, Symbols, SymbolCount          uint32
		OptionalHeaderSize, Characteristics uint16
	}{machine, 1, 0, uint32(symbolsAt), 1, 0, 0})
	_ = binary.Write(&out, le, struct {
		Name                                        [8]byte
		VirtualSize, VirtualAddress, RawSize, RawAt uint32
		RelocAt, LineNumbersAt                      uint32
		Relocs, LineNumbers                         uint16
		Characteristics                             uint32
	}{[8]byte{'.', 'r', 's', 'r', 'c'}, 0, 0, uint32(len(raw)), uint32(rawAt), uint32(relocAt), 0, uint16(len(relocs)), 0, 0x40000040})
	out.Write(raw)
	for _, r := range relocs {
		_ = binary.Write(&out, le, r)
		_ = binary.Write(&out, le, uint32(0))
		_ = binary.Write(&out, le, relocation)
	}
	out.Write([]byte{'.', 'r', 's', 'r', 'c', 0, 0, 0})
	_ = binary.Write(&out, le, uint32(0))
	_ = binary.Write(&out, le, uint16(1))
	_ = binary.Write(&out, le, uint16(0))
	out.Write([]byte{3, 0})
	_ = binary.Write(&out, le, uint32(4))
	return out.Bytes()
}

func sortedKeys[V any](m map[uint16]V) []uint16 {
	keys := make([]uint16, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func windowsVersion(v string) [4]uint16 {
	var out [4]uint16
	v, _, _ = strings.Cut(v, "-")
	for i, part := range strings.SplitN(v, ".", 4) {
		n, _ := strconv.Atoi(part)
		out[i] = uint16(n)
	}
	return out
}

// manifest: common controls 6, Windows 10 and 11, per-monitor DPI
// awareness. The code page stays the system's, as before.
func manifest(version string) string {
	v := windowsVersion(version)
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="SingBox.Windows" version="%d.%d.%d.%d" processorArchitecture="*"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2</dpiAwareness>
    </windowsSettings>
  </application>
</assembly>
`, v[0], v[1], v[2], v[3])
}

func versionInfo(name, version, exe string) []byte {
	v := windowsVersion(version)
	fixed := make([]byte, 52)
	le := binary.LittleEndian
	le.PutUint32(fixed[0:], 0xFEEF04BD)
	le.PutUint32(fixed[4:], 0x00010000)
	le.PutUint32(fixed[8:], uint32(v[0])<<16|uint32(v[1]))
	le.PutUint32(fixed[12:], uint32(v[2])<<16|uint32(v[3]))
	copy(fixed[16:24], fixed[8:16])
	le.PutUint32(fixed[24:], 0x3F)
	le.PutUint32(fixed[32:], 0x00040004)
	le.PutUint32(fixed[36:], 1)

	var table []verNode
	for _, s := range [][2]string{
		{"FileDescription", name},
		{"FileVersion", version},
		{"InternalName", exe},
		{"OriginalFilename", exe},
		{"ProductName", name},
		{"ProductVersion", version},
	} {
		text := utf16.Encode([]rune(s[1] + "\x00"))
		value := make([]byte, 2*len(text))
		for i, u := range text {
			le.PutUint16(value[2*i:], u)
		}
		table = append(table, verNode{key: s[0], text: true, value: value, valueLen: len(text)})
	}
	translation := []byte{byte(langZhCN & 0xFF), byte(langZhCN >> 8), 0xB0, 0x04}
	root := verNode{key: "VS_VERSION_INFO", value: fixed, valueLen: len(fixed), children: []verNode{
		{key: "StringFileInfo", text: true, children: []verNode{{key: fmt.Sprintf("%04x%04x", langZhCN, 1200), text: true, children: table}}},
		{key: "VarFileInfo", text: true, children: []verNode{{key: "Translation", value: translation, valueLen: len(translation)}}},
	}}
	return root.encode()
}

type verNode struct {
	key      string
	text     bool
	value    []byte
	valueLen int
	children []verNode
}

func (n verNode) encode() []byte {
	b := make([]byte, 6)
	for _, u := range utf16.Encode([]rune(n.key + "\x00")) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	pad := func() {
		for len(b)%4 != 0 {
			b = append(b, 0)
		}
	}
	pad()
	b = append(b, n.value...)
	for _, child := range n.children {
		pad()
		b = append(b, child.encode()...)
	}
	binary.LittleEndian.PutUint16(b[0:], uint16(len(b)))
	binary.LittleEndian.PutUint16(b[2:], uint16(n.valueLen))
	if n.text {
		binary.LittleEndian.PutUint16(b[4:], 1)
	}
	return b
}

// iconImages renders src at each of iconSizes as PNG.
func iconImages(src []byte) ([][]byte, error) {
	img, err := png.Decode(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() != b.Dy() {
		return nil, fmt.Errorf("icon must be square, got %dx%d", b.Dx(), b.Dy())
	}
	images := make([][]byte, len(iconSizes))
	for i, s := range iconSizes {
		var p bytes.Buffer
		if err := png.Encode(&p, resize(img, s)); err != nil {
			return nil, err
		}
		images[i] = p.Bytes()
	}
	return images, nil
}

// resize scales img to a size x size square: area averaging when shrinking,
// bilinear sampling when enlarging.
func resize(img image.Image, size int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	if w == 0 || h == 0 {
		return dst
	}
	sx, sy := float64(w)/float64(size), float64(h)/float64(size)
	at := func(x, y int) (float64, float64, float64, float64) {
		c := color.NRGBAModel.Convert(img.At(b.Min.X+min(max(x, 0), w-1), b.Min.Y+min(max(y, 0), h-1))).(color.NRGBA)
		a := float64(c.A)
		return float64(c.R) * a, float64(c.G) * a, float64(c.B) * a, a
	}
	for y := range size {
		for x := range size {
			var r, g, bl, a, n float64
			if sx >= 1 {
				x0, x1 := int(float64(x)*sx), max(int(float64(x+1)*sx), int(float64(x)*sx)+1)
				y0, y1 := int(float64(y)*sy), max(int(float64(y+1)*sy), int(float64(y)*sy)+1)
				for yy := y0; yy < y1; yy++ {
					for xx := x0; xx < x1; xx++ {
						cr, cg, cb, ca := at(xx, yy)
						r, g, bl, a, n = r+cr, g+cg, bl+cb, a+ca, n+1
					}
				}
			} else {
				fx, fy := (float64(x)+0.5)*sx-0.5, (float64(y)+0.5)*sy-0.5
				x0, y0 := int(fx), int(fy)
				dx, dy := fx-float64(x0), fy-float64(y0)
				for _, s := range [4]struct {
					x, y int
					wt   float64
				}{{x0, y0, (1 - dx) * (1 - dy)}, {x0 + 1, y0, dx * (1 - dy)}, {x0, y0 + 1, (1 - dx) * dy}, {x0 + 1, y0 + 1, dx * dy}} {
					cr, cg, cb, ca := at(s.x, s.y)
					r, g, bl, a, n = r+cr*s.wt, g+cg*s.wt, bl+cb*s.wt, a+ca*s.wt, n+s.wt
				}
			}
			if a == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{R: uint8(r/a + 0.5), G: uint8(g/a + 0.5), B: uint8(bl/a + 0.5), A: uint8(a/n + 0.5)})
		}
	}
	return dst
}
