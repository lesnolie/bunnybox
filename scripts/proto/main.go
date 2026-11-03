// Command proto generates only wire contracts; it never imports the sing-box runtime.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type manifest struct {
	Schema           int               `json:"schema"`
	SourceRoot       string            `json:"sourceRoot"`
	Entries          []string          `json:"entries"`
	IncludeRoot      string            `json:"includeRoot"`
	WellKnownVersion string            `json:"wellKnownVersion"`
	Output           string            `json:"output"`
	Module           string            `json:"module"`
	Protoc           string            `json:"protoc"`
	Go               string            `json:"protocGenGo"`
	GRPC             string            `json:"protocGenGoGrpc"`
	Mappings         map[string]string `json:"mappings"`
}
type dependency struct {
	Path    string   `json:"path"`
	Origin  string   `json:"origin"`
	SHA256  string   `json:"sha256"`
	Imports []string `json:"imports"`
}
type record struct {
	SourceCommit     string       `json:"sourceCommit"`
	Tools            []string     `json:"tools"`
	WellKnownVersion string       `json:"wellKnownVersion"`
	ManifestSHA256   string       `json:"manifestSHA256"`
	Entries          []string     `json:"entries"`
	Dependencies     []dependency `json:"dependencies"`
	Commands         [][]string   `json:"commands"`
}

func baseline(m manifest) (string, error) {
	b, e := os.ReadFile("upstream.lock")
	if e != nil {
		return "", e
	}
	section := regexp.MustCompile(`(?ms)^\[core\]\s*\n(.*?)(?:\n\[|\z)`).FindSubmatch(b)
	if section == nil {
		return "", errors.New("missing core lock section")
	}
	get := func(key string) string {
		match := regexp.MustCompile(`(?m)^` + key + `\s*=\s*"([^"\r\n]+)"`).FindSubmatch(section[1])
		if len(match) != 2 {
			return ""
		}
		return string(match[1])
	}
	commit, tag := get("commit"), get("tag")
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(commit) || !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-(?:alpha|beta|rc)\.[0-9]+)?$`).MatchString(tag) {
		return "", errors.New("invalid core lock")
	}
	for _, ref := range []string{"HEAD", "refs/tags/" + tag + "^{commit}"} {
		got, e := run("git", "-C", m.SourceRoot, "rev-parse", ref)
		if e != nil {
			return "", e
		}
		if strings.TrimSpace(string(got)) != commit {
			return "", fmt.Errorf("core %s does not match locked commit", ref)
		}
	}
	status, e := run("git", "-C", m.SourceRoot, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return "", e
	}
	if len(status) != 0 {
		return "", errors.New("core source tree is modified; refusing protocol generation")
	}
	return commit, nil
}

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func run(name string, args ...string) ([]byte, error) {
	c := exec.Command(name, args...)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	b, e := c.Output()
	if e != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, e, stderr.String())
	}
	return b, nil
}
func version(name, want string) error {
	b, e := run(name, "--version")
	if e != nil {
		return e
	}
	if strings.TrimSpace(string(b)) != want {
		return fmt.Errorf("%s version %q; require %q", name, strings.TrimSpace(string(b)), want)
	}
	return nil
}
func entries(m manifest) ([]string, error) {
	set := map[string]bool{}
	for _, pattern := range m.Entries {
		matches, e := filepath.Glob(filepath.Join(m.SourceRoot, filepath.FromSlash(pattern)))
		if e != nil {
			return nil, e
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("entry pattern has no files: %s", pattern)
		}
		for _, f := range matches {
			r, e := filepath.Rel(m.SourceRoot, f)
			if e != nil {
				return nil, e
			}
			set[filepath.ToSlash(r)] = true
		}
	}
	result := make([]string, 0, len(set))
	for p := range set {
		result = append(result, p)
	}
	sort.Strings(result)
	return result, nil
}
func safeName(p string) bool {
	return p != "." && !strings.Contains(p, "\\") && !strings.HasPrefix(p, "/") && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.Contains(p, ":")
}

// resolve uses protoc's parser for recursive imports, including public/weak imports.
// The returned descriptor is also the canonical input to dependency auditing.
func resolve(m manifest, entry []string, tmp string) (*descriptorpb.FileDescriptorSet, []byte, []string, error) {
	args := []string{"-I" + m.SourceRoot, "-I" + m.IncludeRoot, "--include_imports", "--descriptor_set_out=" + filepath.Join(tmp, "protocol.pb")}
	args = append(args, entry...)
	if _, e := run("protoc", args...); e != nil {
		return nil, nil, nil, e
	}
	b, e := os.ReadFile(filepath.Join(tmp, "protocol.pb"))
	if e != nil {
		return nil, nil, nil, e
	}
	ds := new(descriptorpb.FileDescriptorSet)
	if e = proto.Unmarshal(b, ds); e != nil {
		return nil, nil, nil, e
	}
	return ds, b, args, nil
}
func dependencies(m manifest, ds *descriptorpb.FileDescriptorSet) ([]dependency, error) {
	result := []dependency{}
	for _, f := range ds.File {
		p := f.GetName()
		if !safeName(p) {
			return nil, fmt.Errorf("unsafe import: %s", p)
		}
		origin := m.SourceRoot
		if strings.HasPrefix(p, "google/protobuf/") {
			origin = m.IncludeRoot
		}
		b, e := os.ReadFile(filepath.Join(origin, filepath.FromSlash(p)))
		if e != nil {
			return nil, e
		}
		// Reject shadowed Google definitions; a search-path change must not silently
		// replace the vendored, fixed-version well-known types.
		if origin == m.IncludeRoot {
			if _, e := os.Stat(filepath.Join(m.SourceRoot, filepath.FromSlash(p))); e == nil {
				return nil, fmt.Errorf("shadowed Google import: %s", p)
			}
		}
		imports := append([]string{}, f.Dependency...)
		sort.Strings(imports)
		result = append(result, dependency{p, origin, digest(b), imports})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result, nil
}
func mapping(m manifest, p string) (string, error) {
	found := ""
	for pattern, pkg := range m.Mappings {
		ok, e := path.Match(pattern, p)
		if e != nil {
			return "", e
		}
		if ok {
			if found != "" {
				return "", fmt.Errorf("ambiguous mapping: %s", p)
			}
			found = pkg
		}
	}
	if found == "" {
		return "", fmt.Errorf("no local Go package mapping: %s", p)
	}
	return found, nil
}
func generatedFiles(root string) (map[string][]byte, error) {
	result := map[string][]byte{}
	e := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			r, e := filepath.Rel(root, p)
			if e != nil {
				return e
			}
			b, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			result[filepath.ToSlash(r)] = b
		}
		return nil
	})
	if os.IsNotExist(e) {
		return result, nil
	}
	return result, e
}
func execute(mode string) error {
	raw, e := os.ReadFile("scripts/proto/manifest.json")
	if e != nil {
		return e
	}
	var m manifest
	if e = json.Unmarshal(raw, &m); e != nil {
		return e
	}
	if m.Schema != 1 {
		return errors.New("unsupported manifest schema")
	}
	commit, e := baseline(m)
	if e != nil {
		return e
	}
	if e = version("protoc", m.Protoc); e != nil {
		return e
	}
	entry, e := entries(m)
	if e != nil {
		return e
	}
	tmp, e := os.MkdirTemp("", "singboxwin-proto-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(tmp)
	ds, desc, args, e := resolve(m, entry, tmp)
	if e != nil {
		return e
	}
	deps, e := dependencies(m, ds)
	if e != nil {
		return e
	}
	rec := record{SourceCommit: commit, Tools: []string{m.Protoc, m.Go, m.GRPC}, WellKnownVersion: m.WellKnownVersion, ManifestSHA256: digest(raw), Entries: entry, Dependencies: deps}
	args[len(args)-len(entry)-1] = "--descriptor_set_out=<output>/protocol.pb"
	rec.Commands = append(rec.Commands, append([]string{"protoc"}, args...))
	if mode == "inspect" {
		b, e := json.MarshalIndent(rec, "", "  ")
		if e != nil {
			return e
		}
		fmt.Println(string(b))
		return nil
	}
	if e = version("protoc-gen-go", m.Go); e != nil {
		return e
	}
	if e = version("protoc-gen-go-grpc", m.GRPC); e != nil {
		return e
	}
	out := filepath.Join(tmp, "generated")
	if e = os.MkdirAll(out, 0755); e != nil {
		return e
	}
	args = []string{"-I" + m.SourceRoot, "-I" + m.IncludeRoot, "--go_out=" + out, "--go_opt=module=" + m.Module, "--go-grpc_out=" + out, "--go-grpc_opt=module=" + m.Module}
	// Generate all non-Google transitive imports as well as direct entries.
	inputs := []string{}
	for _, dep := range deps {
		if dep.Origin == m.IncludeRoot {
			continue
		}
		pkg, e := mapping(m, dep.Path)
		if e != nil {
			return e
		}
		args = append(args, "--go_opt=M"+dep.Path+"="+pkg, "--go-grpc_opt=M"+dep.Path+"="+pkg)
		inputs = append(inputs, dep.Path)
	}
	args = append(args, inputs...)
	if _, e = run("protoc", args...); e != nil {
		return e
	}
	for i, a := range args {
		args[i] = strings.ReplaceAll(a, out, "<output>")
	}
	rec.Commands = append(rec.Commands, append([]string{"protoc"}, args...))
	generated := filepath.Join(out, filepath.FromSlash(m.Output))
	if e = os.MkdirAll(generated, 0755); e != nil {
		return e
	}
	recJSON, e := json.MarshalIndent(rec, "", "  ")
	if e != nil {
		return e
	}
	recJSON = append(recJSON, '\n')
	if e = os.WriteFile(filepath.Join(generated, "dependencies.json"), recJSON, 0644); e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(generated, "protocol.pb"), desc, 0644); e != nil {
		return e
	}
	want, e := generatedFiles(generated)
	if e != nil {
		return e
	}
	have, e := generatedFiles(m.Output)
	if e != nil {
		return e
	}
	changed := []string{}
	for p, b := range want {
		if !bytes.Equal(b, have[p]) {
			changed = append(changed, p)
		}
	}
	for p := range have {
		if _, ok := want[p]; !ok {
			changed = append(changed, p)
		}
	}
	sort.Strings(changed)
	if mode == "check" {
		if len(changed) > 0 {
			return fmt.Errorf("protocol artifacts differ; run scripts/proto/generate.ps1: %s", strings.Join(changed, ", "))
		}
		fmt.Printf("protocol artifacts current: %d inputs, %d artifacts\n", len(deps), len(want))
		return nil
	}
	for p := range have {
		if _, ok := want[p]; !ok {
			if e = os.Remove(filepath.Join(m.Output, filepath.FromSlash(p))); e != nil {
				return e
			}
		}
	}
	for p, b := range want {
		dest := filepath.Join(m.Output, filepath.FromSlash(p))
		if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
			return e
		}
		if e = os.WriteFile(dest, b, 0644); e != nil {
			return e
		}
	}
	fmt.Printf("generated %d artifacts from %d protocol inputs\n", len(want), len(deps))
	return nil
}
func main() {
	mode := flag.String("mode", "generate", "generate, check, or inspect")
	flag.Parse()
	if *mode != "generate" && *mode != "check" && *mode != "inspect" {
		fmt.Fprintln(os.Stderr, "invalid mode")
		os.Exit(2)
	}
	if e := execute(*mode); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
