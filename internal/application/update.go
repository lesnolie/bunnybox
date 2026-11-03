package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	desktop "singboxwin.local/client/internal/rpc/gen/desktop"
	"singboxwin.local/client/internal/ui/shell"
)

// UpdateRepository publishes this application's releases. Its GitHub Actions
// workflow builds one whenever the official sing-box core has a new stable
// version; the release version is that core version. Only this repository is
// checked: the official releases are signed by SagerNet, and the installed
// core accepts updates only from its own signer.
const UpdateRepository = "lesnolie/bunnybox"

const (
	updateAssetSuffix = "-windows-amd64-setup.exe"
	updateCheckURL    = "https://api.github.com/repos/" + UpdateRepository + "/releases/latest"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Draft   bool   `json:"draft"`
	Assets  []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// coreVersion is the running core's version, which is the installed
// product's version.
func (b *Backend) coreVersion(ctx context.Context) (string, error) {
	conn, err := b.rpc.Daemon(ctx)
	if err != nil {
		return "", err
	}
	info, err := desktop.NewDesktopServiceClient(conn).GetDaemonInfo(ctx, &emptypb.Empty{})
	if err != nil {
		return "", err
	}
	return info.GetVersion(), nil
}

// checkUpdate reads the latest release of UpdateRepository.
func (b *Backend) checkUpdate(ctx context.Context) (shell.UpdateInfo, error) {
	current, err := b.coreVersion(ctx)
	if err != nil {
		return shell.UpdateInfo{}, fmt.Errorf("read core version: %w", err)
	}
	info := shell.UpdateInfo{Current: current}
	url := updateCheckURL
	if override := os.Getenv("SINGBOX_WIN_UPDATE_URL"); override != "" {
		// Development: a local release description. The core still checks
		// the installer's signer and version.
		url = override
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return info, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	response, err := updateHTTP.Do(request)
	if err != nil {
		return info, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return info, nil // no release yet
	}
	if response.StatusCode != http.StatusOK {
		return info, fmt.Errorf("GitHub: HTTP %d", response.StatusCode)
	}
	var release githubRelease
	if err = json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&release); err != nil {
		return info, err
	}
	version := strings.TrimPrefix(release.TagName, "v")
	if release.Draft || version == "" {
		return info, nil
	}
	info.Latest, info.Notes, info.Page = version, release.Body, release.HTMLURL
	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, updateAssetSuffix) {
			info.AssetName, info.AssetURL, info.Size = asset.Name, asset.URL, asset.Size
			info.SHA256 = strings.TrimPrefix(asset.Digest, "sha256:")
		}
	}
	if info.AssetURL != "" && info.SHA256 == "" {
		for _, asset := range release.Assets {
			if asset.Name == "SHA256SUMS" {
				info.SHA256 = sumFor(ctx, asset.URL, info.AssetName)
			}
		}
	}
	info.Available = info.AssetURL != "" && len(info.SHA256) == 64 && newerVersion(version, current)
	return info, nil
}

// installUpdate downloads the installer, checks its digest and hands it to
// the official core, which verifies the signer and version and runs it.
func (b *Backend) installUpdate(ctx context.Context, info shell.UpdateInfo, progress func(float64)) error {
	if !info.Available || info.AssetURL == "" || len(info.SHA256) != 64 || strings.ContainsAny(info.AssetName, `/\`) {
		return errors.New("更新信息不完整 / Incomplete update information")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(cache, "BunnyBox", "updates")
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, info.AssetName)
	if err = download(ctx, info, path, progress); err != nil {
		return err
	}
	conn, err := b.rpc.Daemon(ctx)
	if err != nil {
		return err
	}
	response, err := desktop.NewDesktopServiceClient(conn).InstallUpdate(ctx, &desktop.InstallUpdateRequest{InstallerPath: path})
	if err != nil {
		return err
	}
	switch response.GetResult() {
	case desktop.InstallUpdateResult_INSTALL_UPDATE_RESULT_STARTED:
		return relaunchAfter(strings.TrimSuffix(info.AssetName, ".exe"))
	case desktop.InstallUpdateResult_INSTALL_UPDATE_RESULT_SIGNER_MISMATCH:
		return errors.New("安装包的签名与当前核心不一致，核心拒绝安装 / The installer's signer differs from the core's")
	case desktop.InstallUpdateResult_INSTALL_UPDATE_RESULT_NOT_NEWER:
		return errors.New("安装包的版本不比当前核心新 / The installer is not newer than the core")
	}
	return fmt.Errorf("核心未开始安装（%v）/ The core did not start the update", response.GetResult())
}

// sumFor reads name's digest from a SHA256SUMS file.
func sumFor(ctx context.Context, url, name string) string {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	response, err := updateHTTP.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil || response.StatusCode != http.StatusOK {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == 64 {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

func download(ctx context.Context, info shell.UpdateInfo, path string, progress func(float64)) error {
	if sum, err := fileSHA256(path); err == nil && sum == info.SHA256 {
		progress(1)
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, info.AssetURL, nil)
	if err != nil {
		return err
	}
	response, err := downloadHTTP.Do(request) // no API timeout: the download may take longer
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败 / Download failed: HTTP %d", response.StatusCode)
	}
	temporary := path + ".download"
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	hash := sha256.New()
	total := max(info.Size, response.ContentLength)
	var written int64
	buffer := make([]byte, 256<<10)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err = file.Write(buffer[:n]); err != nil {
				break
			}
			hash.Write(buffer[:n])
			written += int64(n)
			if total > 0 {
				progress(float64(written) / float64(total))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			err = readErr
			break
		}
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil && hex.EncodeToString(hash.Sum(nil)) != info.SHA256 {
		err = errors.New("安装包校验失败 / Installer digest mismatch")
	}
	if err != nil {
		os.Remove(temporary)
		return err
	}
	os.Remove(path)
	return os.Rename(temporary, path)
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// newerVersion compares sing-box versions (1.14.5, 1.15.0-beta.2): a release
// is newer than its prereleases.
func newerVersion(candidate, current string) bool {
	a, b := parseVersion(candidate), parseVersion(current)
	if a == nil || b == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if a.core[i] != b.core[i] {
			return a.core[i] > b.core[i]
		}
	}
	switch {
	case a.pre == "" && b.pre == "":
		return false
	case a.pre == "":
		return true
	case b.pre == "":
		return false
	}
	ap, bp := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		an, aErr := strconv.Atoi(ap[i])
		bn, bErr := strconv.Atoi(bp[i])
		switch {
		case aErr == nil && bErr == nil && an != bn:
			return an > bn
		case (aErr != nil || bErr != nil) && ap[i] != bp[i]:
			return ap[i] > bp[i]
		}
	}
	return len(ap) > len(bp)
}

type version struct {
	core [3]int
	pre  string
}

func parseVersion(s string) *version {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	main, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return nil
	}
	v := &version{pre: pre}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil
		}
		v.core[i] = n
	}
	return v
}
