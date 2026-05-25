package binary

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/cc-box/core/config"
)

func withInstallTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(ClaudePathEnv, "")
	return home
}

func resetInstallHooks(t *testing.T) {
	t.Helper()
	oldOfficialRunner := officialInstallRunner
	oldResolve := resolveClaudeBinaryForInstall
	oldRedetect := redetectClaudeBinaryForInstall
	oldBackup := backupExistingClaudeForInstall
	oldDetect := detectVersionForInstall
	oldRemember := rememberClaudeBinarySourceForInstall
	oldConfigurePath := configureClaudePathForInstall
	oldCommandState := commandStateForInstall
	oldConfigureUserPath := configureUserPathDirForInstall
	oldGitHubAPIURL := githubClaudeReleasesAPIURL
	oldGitHubDownload := githubDownloadURL
	oldGitHubNow := githubNowUTC
	t.Cleanup(func() {
		officialInstallRunner = oldOfficialRunner
		resolveClaudeBinaryForInstall = oldResolve
		redetectClaudeBinaryForInstall = oldRedetect
		backupExistingClaudeForInstall = oldBackup
		detectVersionForInstall = oldDetect
		rememberClaudeBinarySourceForInstall = oldRemember
		configureClaudePathForInstall = oldConfigurePath
		commandStateForInstall = oldCommandState
		configureUserPathDirForInstall = oldConfigureUserPath
		githubClaudeReleasesAPIURL = oldGitHubAPIURL
		githubDownloadURL = oldGitHubDownload
		githubNowUTC = oldGitHubNow
	})
}

func TestRefreshGitHubClaudeReleasesUsesInjectedDownloaderAndCache(t *testing.T) {
	resetInstallHooks(t)
	withInstallTempHome(t)
	assetName, supported := githubAssetNameForPlatform(config.Platform())
	if !supported {
		t.Skipf("当前平台不支持 GitHub Release 安装: %s", config.Platform())
	}

	githubClaudeReleasesAPIURL = "https://example.invalid/claude-code/releases"
	githubNowUTC = func() time.Time { return time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC) }
	var urls []string
	githubDownloadURL = func(ctx context.Context, url string) ([]byte, error) {
		urls = append(urls, url)
		if !strings.HasPrefix(url, githubClaudeReleasesAPIURL) {
			return nil, fmt.Errorf("unexpected external URL: %s", url)
		}
		return []byte(fmt.Sprintf(`[
			{
				"tag_name": "v2.0.0",
				"name": "Claude 2.0.0",
				"published_at": "2026-05-24T10:00:00Z",
				"assets": [
					{"name": %q, "size": 123, "browser_download_url": "fake://asset"},
					{"name": "SHASUMS256.txt", "size": 64, "browser_download_url": "fake://shasums"}
				]
			},
			{
				"tag_name": "v1.0.0",
				"name": "Claude 1.0.0",
				"published_at": "2026-05-23T10:00:00Z",
				"assets": [
					{"name": "other-platform.tar.gz", "size": 456, "browser_download_url": "fake://wrong"},
					{"name": "SHASUMS256.txt", "size": 64, "browser_download_url": "fake://wrong-shasums"}
				]
			}
		]`, assetName)), nil
	}

	list, err := RefreshGitHubClaudeReleases(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Releases) != 1 {
		t.Fatalf("Releases length = %d, want 1", len(list.Releases))
	}
	if list.Releases[0].Version != "2.0.0" || list.Releases[0].AssetName != assetName {
		t.Fatalf("unexpected release: %+v", list.Releases[0])
	}
	if list.FetchedAt != "2026-05-24T12:00:00Z" {
		t.Fatalf("FetchedAt = %q", list.FetchedAt)
	}
	if len(urls) != 1 || !strings.Contains(urls[0], "page=1") {
		t.Fatalf("download URLs = %v, want one injected release page", urls)
	}

	cached, err := CachedGitHubClaudeReleases(30)
	if err != nil {
		t.Fatal(err)
	}
	if !cached.FromCache || len(cached.Releases) != 1 || cached.Releases[0].Version != "2.0.0" {
		t.Fatalf("cached releases = %+v", cached)
	}
}

func TestInstallGitHubClaudeUsesInjectedDownloadsAndTempTarget(t *testing.T) {
	resetInstallHooks(t)
	home := withInstallTempHome(t)
	assetName, supported := githubAssetNameForPlatform(config.Platform())
	if !supported {
		t.Skipf("当前平台不支持 GitHub Release 安装: %s", config.Platform())
	}

	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "bin")
	cfg.Binary.VersionsDir = filepath.Join(home, "versions")
	cfg.Binary.AutoConfigurePath = true
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	targetPath := GetBinaryPath("claude")
	binaryData := []byte("fake claude binary")
	archiveData := buildClaudeArchive(t, assetName, binaryData)
	archiveHash := sha256.Sum256(archiveData)
	shasums := []byte(hex.EncodeToString(archiveHash[:]) + "  " + assetName + "\n")
	if err := writeGitHubReleaseCache(githubReleaseCache{
		Platform:  config.Platform(),
		FetchedAt: "2026-05-24T12:00:00Z",
		Releases: []GitHubClaudeRelease{{
			Version:            "2.0.0",
			Tag:                "v2.0.0",
			Name:               "Claude 2.0.0",
			PublishedAt:        "2026-05-24T10:00:00Z",
			AssetName:          assetName,
			AssetDownloadURL:   "fake://asset",
			ShasumsDownloadURL: "fake://shasums",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	var downloaded []string
	githubDownloadURL = func(ctx context.Context, url string) ([]byte, error) {
		downloaded = append(downloaded, url)
		switch url {
		case "fake://asset":
			return archiveData, nil
		case "fake://shasums":
			return shasums, nil
		default:
			return nil, fmt.Errorf("unexpected external URL: %s", url)
		}
	}
	var backupPath string
	backupExistingClaudeForInstall = func(path string) error {
		backupPath = path
		return nil
	}
	var detectedPath string
	detectVersionForInstall = func(path string) (string, error) {
		detectedPath = path
		return "2.0.0", nil
	}
	var configuredPath bool
	configureClaudePathForInstall = func() (*PathConfigureResult, error) {
		configuredPath = true
		return &PathConfigureResult{Enabled: true, Changed: true, ConfigPath: filepath.Join(home, "profile")}, nil
	}
	rememberClaudeBinarySourceForInstall = func(path, source, version string) error {
		if path != targetPath || source != "github" || version != "2.0.0" {
			t.Fatalf("remember source got path=%q source=%q version=%q", path, source, version)
		}
		return nil
	}
	commandStateForInstall = func(path string) ClaudeCommandStatus {
		return ClaudeCommandStatus{Status: "installed_not_activated", TargetPath: path, TargetExists: true}
	}

	result, err := InstallGitHubClaude(context.Background(), "v2.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Source != "github" || result.Version != "2.0.0" || result.Path != targetPath {
		t.Fatalf("InstallGitHubClaude result = %+v", result)
	}
	if !strings.HasPrefix(targetPath, home) {
		t.Fatalf("target path %q is outside temp home %q", targetPath, home)
	}
	installed, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, binaryData) {
		t.Fatalf("installed data = %q, want %q", installed, binaryData)
	}
	if strings.Join(downloaded, ",") != "fake://asset,fake://shasums" {
		t.Fatalf("downloaded URLs = %v", downloaded)
	}
	if backupPath != targetPath || detectedPath == "" || detectedPath == targetPath || !strings.HasPrefix(detectedPath, filepath.Dir(targetPath)) || !configuredPath {
		t.Fatalf("backup=%q detect=%q configured=%v, want backup target and temp validation", backupPath, detectedPath, configuredPath)
	}
	if _, err := os.Stat(detectedPath); !os.IsNotExist(err) {
		t.Fatalf("validation temp path still exists: %v", err)
	}
}

func TestInstallGitHubClaudeReportsPathConfigureWarningWithoutFailing(t *testing.T) {
	resetInstallHooks(t)
	home := withInstallTempHome(t)
	assetName, supported := githubAssetNameForPlatform(config.Platform())
	if !supported {
		t.Skipf("当前平台不支持 GitHub Release 安装: %s", config.Platform())
	}

	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "bin")
	cfg.Binary.AutoConfigurePath = true
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	binaryData := []byte("fake claude binary")
	archiveData := buildClaudeArchive(t, assetName, binaryData)
	archiveHash := sha256.Sum256(archiveData)
	shasums := []byte(hex.EncodeToString(archiveHash[:]) + "  " + assetName + "\n")
	if err := writeGitHubReleaseCache(githubReleaseCache{
		Platform: config.Platform(),
		Releases: []GitHubClaudeRelease{{
			Version:            "2.0.0",
			AssetName:          assetName,
			AssetDownloadURL:   "fake://asset",
			ShasumsDownloadURL: "fake://shasums",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	githubDownloadURL = func(ctx context.Context, url string) ([]byte, error) {
		switch url {
		case "fake://asset":
			return archiveData, nil
		case "fake://shasums":
			return shasums, nil
		default:
			return nil, fmt.Errorf("unexpected external URL: %s", url)
		}
	}
	detectVersionForInstall = func(path string) (string, error) { return "2.0.0", nil }
	configureClaudePathForInstall = func() (*PathConfigureResult, error) {
		return nil, fmt.Errorf("profile locked")
	}

	result, err := InstallGitHubClaude(context.Background(), "2.0.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PathConfig == nil || result.PathConfig.Error == "" || !strings.Contains(result.PathConfig.Message, "profile locked") {
		t.Fatalf("PathConfig = %+v, want warning", result.PathConfig)
	}
	installed, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(installed, binaryData) {
		t.Fatalf("installed data = %q, want %q", installed, binaryData)
	}
}

func TestInstallClaudeBinaryDataDoesNotReplaceOnVersionMismatch(t *testing.T) {
	resetInstallHooks(t)
	home := withInstallTempHome(t)
	targetPath := filepath.Join(home, "bin", managedBinaryName("claude"))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	oldData := []byte("old claude binary")
	if err := os.WriteFile(targetPath, oldData, 0755); err != nil {
		t.Fatal(err)
	}

	backupCalled := false
	backupExistingClaudeForInstall = func(path string) error {
		backupCalled = true
		return nil
	}
	detectVersionForInstall = func(path string) (string, error) {
		if path == targetPath {
			t.Fatalf("version detection should use temp path before replacement")
		}
		return "9.9.9", nil
	}

	if _, err := InstallClaudeBinaryData(targetPath, []byte("new claude binary"), "2.0.0"); err == nil {
		t.Fatal("InstallClaudeBinaryData succeeded with mismatched version")
	}
	if backupCalled {
		t.Fatal("backup should not run before temp binary version is validated")
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldData) {
		t.Fatalf("target was replaced on mismatch: got %q want %q", got, oldData)
	}
}

func TestInstallOfficialClaudeUsesInjectedRunner(t *testing.T) {
	resetInstallHooks(t)
	home := withInstallTempHome(t)
	currentPath := filepath.Join(home, "real", managedBinaryName("claude"))
	installedPath := filepath.Join(home, "installed", managedBinaryName("claude"))

	var backedUp string
	resolveClaudeBinaryForInstall = func() ClaudeResolution {
		return ClaudeResolution{CurrentPath: currentPath, Version: "1.0.0", Valid: true}
	}
	backupExistingClaudeForInstall = func(path string) error {
		backedUp = path
		return nil
	}
	var runnerName string
	var runnerArgs []string
	officialInstallRunner = func(ctx context.Context, name string, args []string) ([]byte, error) {
		runnerName = name
		runnerArgs = append([]string(nil), args...)
		return []byte("installed"), nil
	}
	redetectClaudeBinaryForInstall = func() ClaudeResolution {
		return ClaudeResolution{CurrentPath: installedPath, Version: "2.0.0", Valid: true}
	}
	rememberClaudeBinarySourceForInstall = func(path, source, version string) error {
		if path != installedPath || source != "official" || version != "2.0.0" {
			t.Fatalf("remember source got path=%q source=%q version=%q", path, source, version)
		}
		return nil
	}
	commandStateForInstall = func(path string) ClaudeCommandStatus {
		return ClaudeCommandStatus{Status: "activated", TargetPath: path, TargetExists: true}
	}

	result, err := InstallOfficialClaude(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if backedUp != currentPath {
		t.Fatalf("backed up %q, want %q", backedUp, currentPath)
	}
	if runnerName == "" || len(runnerArgs) == 0 {
		t.Fatalf("official runner was not called: name=%q args=%v", runnerName, runnerArgs)
	}
	if result.Source != "official" || result.Version != "2.0.0" || result.Path != installedPath || result.Output != "installed" {
		t.Fatalf("InstallOfficialClaude result = %+v", result)
	}
}

func TestConfigureClaudePathUsesInjectedPathWriter(t *testing.T) {
	resetInstallHooks(t)
	home := withInstallTempHome(t)
	cfg := config.DefaultConfig()
	cfg.Binary.BinDir = filepath.Join(home, "bin")
	if err := config.Save(cfg); err != nil {
		t.Fatal(err)
	}
	targetPath := GetBinaryPath("claude")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte{'M', 'Z', 0, 1, 2, 3}, 0755); err != nil {
		t.Fatal(err)
	}

	var wroteDir string
	configureUserPathDirForInstall = func(dir string) (string, bool, error) {
		wroteDir = dir
		return filepath.Join(home, "profile"), true, nil
	}

	result, err := ConfigureClaudePath()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Changed || result.ConfigPath != filepath.Join(home, "profile") {
		t.Fatalf("ConfigureClaudePath result = %+v", result)
	}
	if wroteDir != filepath.Dir(targetPath) {
		t.Fatalf("path writer dir = %q, want %q", wroteDir, filepath.Dir(targetPath))
	}
}

func buildClaudeArchive(t *testing.T, assetName string, binaryData []byte) []byte {
	t.Helper()
	if strings.HasSuffix(assetName, ".zip") {
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		entry, err := zw.Create("claude.exe")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(binaryData); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "claude", Mode: 0755, Size: int64(len(binaryData))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(binaryData); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
