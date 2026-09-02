// Package updater downloads and installs tm releases published on GitHub.
package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultAPIBaseURL = "https://api.github.com"
	checksumsAsset    = "SHA256SUMS"
	maxReleaseJSON    = 2 << 20
	maxChecksums      = 1 << 20
	maxBinary         = 100 << 20
)

// Updater describes the GitHub repository from which releases are installed.
type Updater struct {
	Owner  string
	Repo   string
	Client *http.Client

	apiBaseURL string
	goos       string
	goarch     string
	executable func() (string, error)
	deferWin   func(stagedPath, executablePath string) error
}

// Result reports the outcome of an upgrade.
type Result struct {
	FromVersion string
	ToVersion   string
	Updated     bool
	Deferred    bool
}

// New creates an updater using the current platform and executable.
func New(owner, repo string) *Updater {
	return &Updater{
		Owner:      owner,
		Repo:       repo,
		Client:     &http.Client{Timeout: 60 * time.Second},
		apiBaseURL: defaultAPIBaseURL,
		goos:       runtime.GOOS,
		goarch:     runtime.GOARCH,
		executable: os.Executable,
		deferWin:   deferWindowsReplacement,
	}
}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Upgrade installs the latest GitHub release. If currentVersion already equals
// the latest tag, it performs no download. On Windows replacement is scheduled
// for immediately after the current process exits.
func (u *Updater) Upgrade(ctx context.Context, currentVersion string) (Result, error) {
	result := Result{FromVersion: currentVersion}
	if err := u.validate(); err != nil {
		return result, err
	}

	rel, err := u.latest(ctx)
	if err != nil {
		return result, err
	}
	result.ToVersion = rel.TagName
	if sameVersion(currentVersion, rel.TagName) {
		return result, nil
	}

	binaryName := assetName(u.goos, u.goarch)
	binaryURL, err := findAsset(rel.Assets, binaryName)
	if err != nil {
		return result, err
	}
	checksumsURL, err := findAsset(rel.Assets, checksumsAsset)
	if err != nil {
		return result, err
	}

	checksums, err := u.download(ctx, checksumsURL, maxChecksums)
	if err != nil {
		return result, fmt.Errorf("download checksums: %w", err)
	}
	want, err := checksumFor(checksums, binaryName)
	if err != nil {
		return result, err
	}
	binary, err := u.download(ctx, binaryURL, maxBinary)
	if err != nil {
		return result, fmt.Errorf("download %s: %w", binaryName, err)
	}
	got := sha256.Sum256(binary)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return result, fmt.Errorf("checksum mismatch for %s", binaryName)
	}

	executablePath, err := u.executable()
	if err != nil {
		return result, fmt.Errorf("find current executable: %w", err)
	}
	executablePath, err = filepath.Abs(executablePath)
	if err != nil {
		return result, fmt.Errorf("resolve current executable: %w", err)
	}
	stagedPath, err := stageBinary(executablePath, binary)
	if err != nil {
		return result, err
	}
	removeStaged := true
	defer func() {
		if removeStaged {
			_ = os.Remove(stagedPath)
		}
	}()

	if u.goos == "windows" {
		if err := u.deferWin(stagedPath, executablePath); err != nil {
			return result, fmt.Errorf("schedule executable replacement: %w", err)
		}
		removeStaged = false
		result.Deferred = true
	} else {
		if err := replaceUnix(stagedPath, executablePath); err != nil {
			return result, fmt.Errorf("replace executable: %w", err)
		}
		removeStaged = false
	}

	result.Updated = true
	return result, nil
}

func (u *Updater) validate() error {
	if strings.TrimSpace(u.Owner) == "" || strings.TrimSpace(u.Repo) == "" {
		return errors.New("GitHub owner and repository are required")
	}
	if u.Client == nil || u.executable == nil || u.deferWin == nil {
		return errors.New("updater is not initialized; use updater.New")
	}
	if u.goos == "" || u.goarch == "" {
		return errors.New("operating system and architecture are required")
	}
	return nil
}

func (u *Updater) latest(ctx context.Context) (release, error) {
	url := strings.TrimRight(u.apiBaseURL, "/") + "/repos/" + u.Owner + "/" + u.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return release{}, fmt.Errorf("create latest-release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tm-updater")

	response, err := u.Client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("check latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("check latest release: GitHub returned %s", response.Status)
	}

	var rel release
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxReleaseJSON))
	if err := decoder.Decode(&rel); err != nil {
		return release{}, fmt.Errorf("decode latest release: %w", err)
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return release{}, errors.New("latest GitHub release has no tag")
	}
	return rel, nil
}

func (u *Updater) download(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tm-updater")
	response, err := u.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %s", response.Status)
	}
	reader := io.LimitReader(response.Body, limit+1)
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

func assetName(goos, goarch string) string {
	name := "tm_" + goos + "_" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

func findAsset(assets []asset, name string) (string, error) {
	for _, candidate := range assets {
		if candidate.Name == name && candidate.URL != "" {
			return candidate.URL, nil
		}
	}
	return "", fmt.Errorf("release does not contain %s", name)
}

func checksumFor(data []byte, filename string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name != filename {
			continue
		}
		decoded, err := hex.DecodeString(fields[0])
		if err != nil || len(decoded) != sha256.Size {
			return "", fmt.Errorf("invalid checksum for %s", filename)
		}
		return fields[0], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read checksums: %w", err)
	}
	return "", fmt.Errorf("checksums do not contain %s", filename)
}

func sameVersion(current, latest string) bool {
	return strings.TrimPrefix(strings.TrimSpace(current), "v") == strings.TrimPrefix(strings.TrimSpace(latest), "v")
}

func stageBinary(executablePath string, binary []byte) (string, error) {
	directory := filepath.Dir(executablePath)
	temporary, err := os.CreateTemp(directory, ".tm-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("create staged executable: %w", err)
	}
	path := temporary.Name()
	clean := true
	defer func() {
		_ = temporary.Close()
		if clean {
			_ = os.Remove(path)
		}
	}()
	if _, err := temporary.Write(binary); err != nil {
		return "", fmt.Errorf("write staged executable: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return "", fmt.Errorf("sync staged executable: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close staged executable: %w", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		return "", fmt.Errorf("make staged executable runnable: %w", err)
	}
	clean = false
	return path, nil
}

func replaceUnix(stagedPath, executablePath string) error {
	info, err := os.Stat(executablePath)
	if err != nil {
		return err
	}
	if err := os.Chmod(stagedPath, info.Mode().Perm()|0o100); err != nil {
		return err
	}
	return os.Rename(stagedPath, executablePath)
}

func deferWindowsReplacement(stagedPath, executablePath string) error {
	script, err := os.CreateTemp(filepath.Dir(executablePath), ".tm-upgrade-*.cmd")
	if err != nil {
		return err
	}
	scriptPath := script.Name()
	contents := windowsReplacementScript(stagedPath, executablePath)
	if _, err := io.WriteString(script, contents); err != nil {
		_ = script.Close()
		_ = os.Remove(scriptPath)
		return err
	}
	if err := script.Close(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	command := exec.Command("cmd.exe", "/C", scriptPath)
	if err := command.Start(); err != nil {
		_ = os.Remove(scriptPath)
		return err
	}
	_ = command.Process.Release()
	return nil
}

func windowsReplacementScript(stagedPath, executablePath string) string {
	return "@echo off\r\n" +
		"set attempts=0\r\n" +
		":retry\r\n" +
		"move /Y \"" + escapeBatch(stagedPath) + "\" \"" + escapeBatch(executablePath) + "\" >nul 2>&1\r\n" +
		"if not errorlevel 1 goto done\r\n" +
		"set /a attempts+=1\r\n" +
		"if %attempts% GEQ 120 goto failed\r\n" +
		"ping -n 2 127.0.0.1 >nul\r\n" +
		"goto retry\r\n" +
		":failed\r\n" +
		"del \"" + escapeBatch(stagedPath) + "\" >nul 2>&1\r\n" +
		":done\r\n" +
		"del \"%~f0\"\r\n"
}

func escapeBatch(path string) string {
	return strings.ReplaceAll(path, "%", "%%")
}
