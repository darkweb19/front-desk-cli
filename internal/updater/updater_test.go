package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestUpgradeReplacesUnixExecutable(t *testing.T) {
	t.Parallel()

	binary := []byte("new executable")
	executablePath := filepath.Join(t.TempDir(), "tm")
	if err := os.WriteFile(executablePath, []byte("old executable"), 0o700); err != nil {
		t.Fatal(err)
	}

	server := releaseServer(t, "v1.2.0", "tm_linux_amd64", binary, false)
	defer server.Close()
	u := testUpdater(server, executablePath, "linux", "amd64")

	result, err := u.Upgrade(context.Background(), "v1.1.0")
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Updated || result.Deferred || result.ToVersion != "v1.2.0" {
		t.Fatalf("Upgrade() result = %+v", result)
	}
	got, err := os.ReadFile(executablePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binary) {
		t.Fatalf("installed executable = %q, want %q", got, binary)
	}
}

func TestUpgradeSkipsCurrentVersion(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprintf(w, `{"tag_name":"v1.2.0","assets":[]}`)
	}))
	defer server.Close()
	u := testUpdater(server, filepath.Join(t.TempDir(), "tm"), "linux", "amd64")

	result, err := u.Upgrade(context.Background(), "1.2.0")
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if result.Updated || result.ToVersion != "v1.2.0" {
		t.Fatalf("Upgrade() result = %+v", result)
	}
	if requests != 1 {
		t.Fatalf("request count = %d, want 1", requests)
	}
}

func TestUpgradeRejectsChecksumMismatchAndPreservesExecutable(t *testing.T) {
	t.Parallel()

	executablePath := filepath.Join(t.TempDir(), "tm")
	if err := os.WriteFile(executablePath, []byte("old executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	server := releaseServer(t, "v2.0.0", "tm_linux_amd64", []byte("new executable"), true)
	defer server.Close()
	u := testUpdater(server, executablePath, "linux", "amd64")

	_, err := u.Upgrade(context.Background(), "v1.0.0")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Upgrade() error = %v, want checksum mismatch", err)
	}
	got, readErr := os.ReadFile(executablePath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "old executable" {
		t.Fatalf("executable changed to %q", got)
	}
}

func TestUpgradeDefersWindowsReplacement(t *testing.T) {
	t.Parallel()

	binary := []byte("new windows executable")
	executablePath := filepath.Join(t.TempDir(), "tm.exe")
	if err := os.WriteFile(executablePath, []byte("old executable"), 0o700); err != nil {
		t.Fatal(err)
	}
	server := releaseServer(t, "v3.0.0", "tm_windows_amd64.exe", binary, false)
	defer server.Close()
	u := testUpdater(server, executablePath, "windows", "amd64")
	var staged string
	u.deferWin = func(stagedPath, targetPath string) error {
		staged = stagedPath
		if targetPath != executablePath {
			t.Fatalf("target path = %q, want %q", targetPath, executablePath)
		}
		return nil
	}

	result, err := u.Upgrade(context.Background(), "v2.0.0")
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if !result.Updated || !result.Deferred {
		t.Fatalf("Upgrade() result = %+v", result)
	}
	got, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged executable: %v", err)
	}
	if string(got) != string(binary) {
		t.Fatalf("staged executable = %q, want %q", got, binary)
	}
	if err := os.Remove(staged); err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeReportsMissingPlatformAsset(t *testing.T) {
	t.Parallel()

	server := releaseServer(t, "v1.0.0", "tm_linux_amd64", []byte("binary"), false)
	defer server.Close()
	u := testUpdater(server, filepath.Join(t.TempDir(), "tm"), "plan9", "amd64")

	_, err := u.Upgrade(context.Background(), "dev")
	if err == nil || !strings.Contains(err.Error(), "tm_plan9_amd64") {
		t.Fatalf("Upgrade() error = %v, want missing platform asset", err)
	}
}

func TestChecksumFor(t *testing.T) {
	t.Parallel()

	hash := strings.Repeat("a", 64)
	got, err := checksumFor([]byte(hash+"  *tm_linux_amd64\n"), "tm_linux_amd64")
	if err != nil {
		t.Fatalf("checksumFor() error = %v", err)
	}
	if got != hash {
		t.Fatalf("checksumFor() = %q, want %q", got, hash)
	}
	if _, err := checksumFor([]byte("bad  tm_linux_amd64\n"), "tm_linux_amd64"); err == nil {
		t.Fatal("checksumFor() accepted invalid checksum")
	}
}

func TestWindowsReplacementScriptUsesStdinIndependentDelay(t *testing.T) {
	t.Parallel()

	script := windowsReplacementScript(`C:\tmp\tm-new.exe`, `C:\bin\tm.exe`)
	if strings.Contains(strings.ToLower(script), "timeout") {
		t.Fatalf("replacement script uses timeout:\n%s", script)
	}
	if !strings.Contains(script, "ping -n 2 127.0.0.1 >nul") {
		t.Fatalf("replacement script has no loopback delay:\n%s", script)
	}
}

func TestDeferredWindowsReplacementRunsWithNoStdin(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("requires cmd.exe")
	}

	directory := t.TempDir()
	stagedPath := filepath.Join(directory, "tm-new.exe")
	executablePath := filepath.Join(directory, "tm.exe")
	if err := os.WriteFile(stagedPath, []byte("new"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executablePath, []byte("old"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := deferWindowsReplacement(stagedPath, executablePath); err != nil {
		t.Fatalf("deferWindowsReplacement() error = %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := os.ReadFile(executablePath)
		if err == nil && string(got) == "new" {
			if _, err := os.Stat(stagedPath); !os.IsNotExist(err) {
				t.Fatalf("staged executable still exists: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	got, err := os.ReadFile(executablePath)
	t.Fatalf("replacement did not complete: content=%q error=%v", got, err)
}

func testUpdater(server *httptest.Server, executablePath, goos, goarch string) *Updater {
	u := New("darkweb19", "front-desk-cli")
	u.Client = server.Client()
	u.apiBaseURL = server.URL
	u.goos = goos
	u.goarch = goarch
	u.executable = func() (string, error) { return executablePath, nil }
	return u
}

func releaseServer(t *testing.T, tag, binaryName string, binary []byte, corruptChecksum bool) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(nil)
	digest := sha256.Sum256(binary)
	checksum := hex.EncodeToString(digest[:])
	if corruptChecksum {
		checksum = strings.Repeat("0", 64)
	}
	checksums := checksum + "  " + binaryName + "\n"
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/darkweb19/front-desk-cli/releases/latest":
			fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":"SHA256SUMS","browser_download_url":%q}]}`,
				tag, binaryName, server.URL+"/binary", server.URL+"/checksums")
		case "/binary":
			_, _ = w.Write(binary)
		case "/checksums":
			_, _ = w.Write([]byte(checksums))
		default:
			http.NotFound(w, r)
		}
	})
	return server
}
