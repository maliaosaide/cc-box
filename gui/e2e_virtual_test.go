package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
)

const virtualWebDAVPassword = "webdav-pass"

type virtualDevice struct {
	home        string
	claudeDir   string
	binDir      string
	versionsDir string
	app         *App
}

type virtualWebDAVServer struct {
	server *httptest.Server
	mu     sync.Mutex
	files  map[string]virtualWebDAVFile
	dirs   map[string]bool
	nextID int64
}

type virtualWebDAVFile struct {
	data     []byte
	etag     string
	modified time.Time
}

func TestVirtualGUIWorkflowWithBinaryLifecycle(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	baseURL := webdavServer.server.URL + "/dav"
	root := "/cc-box-virtual/"

	deviceA := newVirtualDevice(t)
	deviceB := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(deviceA.claudeDir, "settings.json"), `{"theme":"light"}`)
	writeFakeClaude(t, filepath.Join(deviceA.binDir, binaryName()), "1.0.0-test", "fake-claude-v1")

	activateDevice(t, deviceA)
	if err := deviceA.app.TestWebDAVConnection(baseURL, "user", virtualWebDAVPassword, root); err != nil {
		t.Fatalf("TestWebDAVConnection: %v", err)
	}
	if exists, err := deviceA.app.DetectExistingSetup(baseURL, "user", virtualWebDAVPassword, root); err != nil || exists {
		t.Fatalf("DetectExistingSetup before init = %v, %v", exists, err)
	}
	if err := deviceA.app.InitNewDevice(baseURL, "user", virtualWebDAVPassword, root, "old-secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}
	if exists, err := deviceA.app.DetectExistingSetup(baseURL, "user", virtualWebDAVPassword, root); err != nil || !exists {
		t.Fatalf("DetectExistingSetup after init = %v, %v", exists, err)
	}
	if err := deviceA.app.SetConfigField("binary", "encrypt", "true"); err != nil {
		t.Fatalf("enable binary encryption: %v", err)
	}
	if err := deviceA.app.SetConfigField("binary", "sync_enabled", "true"); err != nil {
		t.Fatalf("enable binary sync: %v", err)
	}
	if err := deviceA.app.SetConfigField("encryption", "enabled", "false"); err == nil {
		t.Fatalf("expected encryption mode switch to be rejected")
	}
	if got := runFakeClaude(t, filepath.Join(deviceA.binDir, binaryName())); got != "fake-claude-v1" {
		t.Fatalf("device A fake binary output = %q", got)
	}
	waitAsyncSuccess(t, deviceA.app.QuickPush())
	initialHead := readHead(t)

	writeFakeClaude(t, filepath.Join(deviceB.binDir, binaryName()), "0.9.0-test", "fake-claude-v0")
	activateDevice(t, deviceB)
	if err := deviceB.app.InitJoinExistingWithBinary(baseURL, "user", virtualWebDAVPassword, root, "old-secret", "device-b", true); err != nil {
		t.Fatalf("InitJoinExistingWithBinary: %v", err)
	}
	assertFileContent(t, filepath.Join(deviceB.claudeDir, "settings.json"), `{"theme":"light"}`)
	if got := runFakeClaude(t, filepath.Join(deviceB.binDir, binaryName())); got != "fake-claude-v1" {
		t.Fatalf("device B remote binary output = %q", got)
	}

	activateDevice(t, deviceA)
	writeTextFile(t, filepath.Join(deviceA.claudeDir, "settings.json"), `{"theme":"dark"}`)
	writeFakeClaude(t, filepath.Join(deviceA.binDir, binaryName()), "2.0.0-test", "fake-claude-v2")
	waitAsyncSuccess(t, deviceA.app.UploadCurrentBinary())
	waitAsyncSuccess(t, deviceA.app.QuickPush())
	secondHead := readHead(t)
	if secondHead == initialHead {
		t.Fatalf("QuickPush did not advance HEAD")
	}

	activateDevice(t, deviceB)
	waitAsyncSuccess(t, deviceB.app.QuickPull())
	assertFileContent(t, filepath.Join(deviceB.claudeDir, "settings.json"), `{"theme":"dark"}`)
	page, err := deviceB.app.GetBinaryPage()
	if err != nil {
		t.Fatalf("GetBinaryPage: %v", err)
	}
	assertBinaryPageHasVersion(t, page, "2.0.0-test")
	if got := runFakeClaude(t, filepath.Join(deviceB.binDir, binaryName())); got != "fake-claude-v2" {
		t.Fatalf("device B synced binary output = %q", got)
	}

	activateDevice(t, deviceA)
	if err := deviceA.app.RevertToSnapshot(initialHead); err != nil {
		t.Fatalf("RevertToSnapshot initial: %v", err)
	}
	assertFileContent(t, filepath.Join(deviceA.claudeDir, "settings.json"), `{"theme":"light"}`)
	if got := runFakeClaude(t, filepath.Join(deviceA.binDir, binaryName())); got != "fake-claude-v1" {
		t.Fatalf("device A reverted binary output = %q", got)
	}
	revertHead := readHead(t)
	if revertHead == secondHead || revertHead == initialHead {
		t.Fatalf("RevertToSnapshot did not create a new HEAD: %q", revertHead)
	}

	activateDevice(t, deviceB)
	writeTextFile(t, filepath.Join(deviceB.claudeDir, "settings.json"), `{"theme":"device-b-local"}`)
	pullErr := waitAsyncError(t, deviceB.app.QuickPull())
	if pullErr == nil || !strings.Contains(pullErr.Error(), "冲突") {
		t.Fatalf("expected pull conflict, got %v", pullErr)
	}
	detail, err := deviceB.app.GetConflictDetail("settings.json")
	if err != nil {
		t.Fatalf("GetConflictDetail: %v", err)
	}
	if !strings.Contains(detail.Local, "device-b-local") || !strings.Contains(detail.Remote, "light") {
		t.Fatalf("unexpected conflict detail: %+v", detail)
	}
	if err := deviceB.app.ResolveConflict("settings.json", "remote"); err != nil {
		t.Fatalf("ResolveConflict remote: %v", err)
	}
	assertFileContent(t, filepath.Join(deviceB.claudeDir, "settings.json"), `{"theme":"light"}`)

	activateDevice(t, deviceA)
	if err := deviceA.app.ChangeEncryptionPassword("old-secret", "new-secret"); err != nil {
		t.Fatalf("ChangeEncryptionPassword: %v", err)
	}
	verifyResult, err := deviceA.app.VerifyEncryptionKey()
	if err != nil || verifyResult.Status != "success" {
		t.Fatalf("VerifyEncryptionKey after rotation = %+v, %v", verifyResult, err)
	}
	if err := deviceA.app.SwitchBinaryVersion("2.0.0-test", "remote"); err != nil {
		t.Fatalf("SwitchBinaryVersion after key rotation: %v", err)
	}
	if got := runFakeClaude(t, filepath.Join(deviceA.binDir, binaryName())); got != "fake-claude-v2" {
		t.Fatalf("rotated encrypted binary output = %q", got)
	}

	activateDevice(t, deviceB)
	oldPreview, err := deviceB.app.PreviewEncryptionPassword("old-secret")
	if err != nil || oldPreview.Status != "mismatch" {
		t.Fatalf("PreviewEncryptionPassword old = %+v, %v", oldPreview, err)
	}
	newPreview, err := deviceB.app.PreviewEncryptionPassword("new-secret")
	if err != nil || newPreview.Status != "success" || newPreview.MatchesCurrent {
		t.Fatalf("PreviewEncryptionPassword new = %+v, %v", newPreview, err)
	}
	if err := deviceB.app.SaveEncryptionPassword("new-secret"); err != nil {
		t.Fatalf("SaveEncryptionPassword: %v", err)
	}
	verifyResult, err = deviceB.app.VerifyEncryptionKey()
	if err != nil || verifyResult.Status != "success" {
		t.Fatalf("VerifyEncryptionKey after saving password = %+v, %v", verifyResult, err)
	}
	if err := deviceB.app.SwitchBinaryVersion("1.0.0-test", "remote"); err != nil {
		t.Fatalf("SwitchBinaryVersion after saving password: %v", err)
	}
	if got := runFakeClaude(t, filepath.Join(deviceB.binDir, binaryName())); got != "fake-claude-v1" {
		t.Fatalf("remote binary after saving password = %q", got)
	}
}

func TestDashboardMarksMissingRemoteHeadUninitialized(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	activateDevice(t, device)
	configureVirtualDevice(t, device, webdavServer.server.URL+"/dav", "/cc-box-missing-head/")

	dashboard, err := device.app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if dashboard.SyncStatus != "remote_uninitialized" {
		t.Fatalf("SyncStatus = %q, want remote_uninitialized", dashboard.SyncStatus)
	}
	if dashboard.SyncHealth.Code != "remote_head_missing" || !dashboard.SyncHealth.CanRepair {
		t.Fatalf("SyncHealth = %+v, want repairable remote_head_missing", dashboard.SyncHealth)
	}
}

func TestGetDashboardLocalDoesNotRequireRemote(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)
	activateDevice(t, device)
	if err := device.app.InitNewDevice(webdavServer.server.URL+"/dav", "user", virtualWebDAVPassword, "/cc-box-local-dashboard/", "secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}
	webdavServer.server.Close()

	dashboard, err := device.app.GetDashboardLocal()
	if err != nil {
		t.Fatalf("GetDashboardLocal: %v", err)
	}
	if dashboard.SyncStatus != "checking" || dashboard.SyncHealth.Code != "checking_remote" {
		t.Fatalf("local dashboard health = %+v, want checking_remote", dashboard.SyncHealth)
	}
	if len(dashboard.Backups) == 0 || dashboard.Backups[0].Message == "" {
		t.Fatalf("local dashboard should include cached backups: %+v", dashboard.Backups)
	}
	if len(dashboard.Devices) != 1 || !dashboard.Devices[0].IsCurrent {
		t.Fatalf("local dashboard devices = %+v, want current device only", dashboard.Devices)
	}
}

func TestRefreshDashboardRemoteReportsSynced(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)
	activateDevice(t, device)
	if err := device.app.InitNewDevice(webdavServer.server.URL+"/dav", "user", virtualWebDAVPassword, "/cc-box-refresh-synced/", "secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}

	dashboard, err := device.app.RefreshDashboardRemote()
	if err != nil {
		t.Fatalf("RefreshDashboardRemote: %v", err)
	}
	if dashboard.SyncStatus != "synced" || dashboard.SyncHealth.Code != "synced" {
		t.Fatalf("remote dashboard health = %+v, want synced", dashboard.SyncHealth)
	}
}

func TestRefreshDashboardRemoteReportsPending(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)
	activateDevice(t, device)
	if err := device.app.InitNewDevice(webdavServer.server.URL+"/dav", "user", virtualWebDAVPassword, "/cc-box-refresh-pending/", "secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}
	initialHead := readHead(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"dark"}`)
	waitAsyncSuccess(t, device.app.QuickPush())
	remoteHead := readHead(t)
	if remoteHead == initialHead {
		t.Fatalf("QuickPush did not advance HEAD")
	}
	if err := os.WriteFile(filepath.Join(config.CCBoxDir(), "HEAD"), []byte(initialHead), 0600); err != nil {
		t.Fatalf("restore local HEAD: %v", err)
	}

	dashboard, err := device.app.RefreshDashboardRemote()
	if err != nil {
		t.Fatalf("RefreshDashboardRemote: %v", err)
	}
	if dashboard.SyncStatus != "pending" || dashboard.SyncHealth.LocalHead != initialHead || dashboard.SyncHealth.RemoteHead != remoteHead {
		t.Fatalf("remote dashboard health = %+v, want pending %s -> %s", dashboard.SyncHealth, initialHead, remoteHead)
	}
}

func TestQuickSyncDoesNotInitializeEmptyRemote(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)
	activateDevice(t, device)
	configureVirtualDevice(t, device, webdavServer.server.URL+"/dav", "/cc-box-quick-sync-init/")

	err := waitAsyncError(t, device.app.QuickSync())
	if err == nil || !strings.Contains(err.Error(), "远程尚未初始化") {
		t.Fatalf("QuickSync error = %v, want remote uninitialized", err)
	}
	client := newConfiguredWebDAVClient(mustLoadConfig(t), virtualWebDAVPassword)
	exists, err := client.Exists("HEAD")
	if err != nil {
		t.Fatalf("check remote HEAD after QuickSync: %v", err)
	}
	if exists {
		t.Fatalf("QuickSync created remote HEAD unexpectedly")
	}
	dashboard, err := device.app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard after QuickSync: %v", err)
	}
	if dashboard.SyncStatus != "remote_uninitialized" {
		t.Fatalf("SyncStatus after QuickSync = %q, want remote_uninitialized", dashboard.SyncStatus)
	}
}

func TestRepairRemoteFromLocalInitializesEmptyRemote(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)
	activateDevice(t, device)
	configureVirtualDevice(t, device, webdavServer.server.URL+"/dav", "/cc-box-repair/")

	waitAsyncSuccess(t, device.app.RepairRemoteFromLocal())
	client := newConfiguredWebDAVClient(mustLoadConfig(t), virtualWebDAVPassword)
	headData, _, err := client.GET("HEAD")
	if err != nil {
		t.Fatalf("remote HEAD after RepairRemoteFromLocal: %v", err)
	}
	head := strings.TrimSpace(string(headData))
	if head == "" {
		t.Fatalf("remote HEAD after RepairRemoteFromLocal is empty")
	}
	if _, _, err := client.GET("snapshots/" + head + ".json.enc"); err != nil {
		t.Fatalf("remote snapshot after RepairRemoteFromLocal: %v", err)
	}
	verifyResult, err := device.app.VerifyEncryptionKey()
	if err != nil || verifyResult.Status != "success" {
		t.Fatalf("VerifyEncryptionKey after RepairRemoteFromLocal = %+v, %v", verifyResult, err)
	}
	dashboard, err := device.app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard after RepairRemoteFromLocal: %v", err)
	}
	if dashboard.SyncStatus != "synced" {
		t.Fatalf("SyncStatus after RepairRemoteFromLocal = %q, want synced", dashboard.SyncStatus)
	}
}

func TestRepairRemoteFromLocalRejectsExistingHead(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	activateDevice(t, device)
	configureVirtualDevice(t, device, webdavServer.server.URL+"/dav", "/cc-box-repair-existing/")
	client := newConfiguredWebDAVClient(mustLoadConfig(t), virtualWebDAVPassword)
	if _, err := client.PUT("HEAD", []byte("foreign-head"), ""); err != nil {
		t.Fatalf("seed remote HEAD: %v", err)
	}

	err := waitAsyncError(t, device.app.RepairRemoteFromLocal())
	if err == nil || !strings.Contains(err.Error(), "远程 HEAD 已存在") {
		t.Fatalf("RepairRemoteFromLocal error = %v, want existing HEAD rejection", err)
	}
	headData, _, err := client.GET("HEAD")
	if err != nil {
		t.Fatalf("read remote HEAD after rejected repair: %v", err)
	}
	if strings.TrimSpace(string(headData)) != "foreign-head" {
		t.Fatalf("remote HEAD after rejected repair = %q, want foreign-head", strings.TrimSpace(string(headData)))
	}
}

func TestDashboardMarksMissingRemoteSnapshotIncomplete(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	device := newVirtualDevice(t)
	activateDevice(t, device)
	configureVirtualDevice(t, device, webdavServer.server.URL+"/dav", "/cc-box-missing-snapshot/")
	client := newConfiguredWebDAVClient(mustLoadConfig(t), virtualWebDAVPassword)
	if _, err := client.PUT("HEAD", []byte("missing-snapshot"), ""); err != nil {
		t.Fatalf("seed remote HEAD: %v", err)
	}

	dashboard, err := device.app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if dashboard.SyncStatus != "remote_incomplete" || dashboard.SyncHealth.Code != "remote_snapshot_missing" {
		t.Fatalf("dashboard sync health = %+v, want remote_snapshot_missing", dashboard.SyncHealth)
	}
	if dashboard.SyncHealth.CanRepair {
		t.Fatalf("missing snapshot should not be marked repairable")
	}
}

func TestDashboardMarksKeyMismatch(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	baseURL := webdavServer.server.URL + "/dav"
	root := "/cc-box-key-mismatch/"
	deviceA := newVirtualDevice(t)
	deviceB := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(deviceA.claudeDir, "settings.json"), `{"theme":"light"}`)

	activateDevice(t, deviceA)
	if err := deviceA.app.InitNewDevice(baseURL, "user", virtualWebDAVPassword, root, "right-secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}

	activateDevice(t, deviceB)
	configureVirtualDevice(t, deviceB, baseURL, root)
	dashboard, err := deviceB.app.GetDashboard()
	if err != nil {
		t.Fatalf("GetDashboard: %v", err)
	}
	if dashboard.SyncStatus != "key_mismatch" {
		t.Fatalf("SyncStatus = %q, want key_mismatch (health=%+v)", dashboard.SyncStatus, dashboard.SyncHealth)
	}
}

func newVirtualDevice(t *testing.T) *virtualDevice {
	t.Helper()
	home := t.TempDir()
	device := &virtualDevice{
		home:        home,
		claudeDir:   filepath.Join(home, ".claude"),
		binDir:      filepath.Join(home, ".local", "bin"),
		versionsDir: filepath.Join(home, ".local", "share", "claude", "versions"),
		app:         &App{},
	}
	for _, dir := range []string{device.claudeDir, device.binDir, device.versionsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}
	}
	return device
}

func activateDevice(t *testing.T, device *virtualDevice) {
	t.Helper()
	if err := os.Setenv("HOME", device.home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	if err := os.Setenv("USERPROFILE", device.home); err != nil {
		t.Fatalf("set USERPROFILE: %v", err)
	}
	if err := os.Setenv("CC_BOX_WEBDAV_PASSWORD", virtualWebDAVPassword); err != nil {
		t.Fatalf("set password env: %v", err)
	}
}

func configureVirtualDevice(t *testing.T, device *virtualDevice, baseURL, root string) {
	t.Helper()
	if err := config.InitCCBoxDir(); err != nil {
		t.Fatalf("InitCCBoxDir: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.WebDAV = config.WebDAVConfig{URL: strings.TrimRight(baseURL, "/") + "/", Username: "user", Root: root}
	cfg.Claude.Path = device.claudeDir
	cfg.Binary.BinDir = device.binDir
	cfg.Binary.VersionsDir = device.versionsDir
	if err := config.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}
	salt := []byte("0123456789abcdef")
	key := crypto.DeriveKey("secret", salt)
	if err := crypto.SaveKey(key, config.KeyPath()); err != nil {
		t.Fatalf("save key: %v", err)
	}
	if err := os.WriteFile(filepath.Join(config.CCBoxDir(), "salt.bin"), salt, 0600); err != nil {
		t.Fatalf("save salt: %v", err)
	}
}

func preserveEnv(t *testing.T, names ...string) {
	t.Helper()
	type envValue struct {
		value string
		ok    bool
	}
	originals := make(map[string]envValue, len(names))
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		originals[name] = envValue{value: value, ok: ok}
	}
	t.Cleanup(func() {
		for _, name := range names {
			original := originals[name]
			if original.ok {
				_ = os.Setenv(name, original.value)
			} else {
				_ = os.Unsetenv(name)
			}
		}
	})
}

func writeTextFile(t *testing.T, filePath, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("create parent for %s: %v", filePath, err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", filePath, err)
	}
}

func assertFileContent(t *testing.T, filePath, want string) {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read %s: %v", filePath, err)
	}
	if got := string(data); got != want {
		t.Fatalf("%s = %q, want %q", filePath, got, want)
	}
}

func binaryName() string {
	if strings.HasPrefix(config.Platform(), "windows-") {
		return "claude.exe"
	}
	return "claude"
}

func writeFakeClaude(t *testing.T, targetPath, version, marker string) {
	t.Helper()
	buildDir := t.TempDir()
	sourcePath := filepath.Join(buildDir, "fake_claude.go")
	source := fmt.Sprintf(`package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println("claude", %s)
		return
	}
	fmt.Println(%s)
}
`, strconv.Quote(version), strconv.Quote(marker))
	if err := os.WriteFile(sourcePath, []byte(source), 0600); err != nil {
		t.Fatalf("write fake claude source: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("create fake binary dir: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", targetPath, sourcePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build fake claude: %v\n%s", err, string(output))
	}
}

func runFakeClaude(t *testing.T, binPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("run %s: %v", binPath, err)
	}
	return strings.TrimSpace(string(output))
}

func waitAsyncSuccess(t *testing.T, opID int64) {
	t.Helper()
	if err := waitAsyncError(t, opID); err != nil {
		t.Fatalf("async op %d failed: %v", opID, err)
	}
}

func waitAsyncError(t *testing.T, opID int64) error {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		opCancelMu.Lock()
		_, running := opCancels[opID]
		err, done := opResults[opID]
		opCancelMu.Unlock()
		if done && !running {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for async op %d", opID)
	return nil
}

func readHead(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(config.CCBoxDir(), "HEAD"))
	if err != nil {
		t.Fatalf("read HEAD: %v", err)
	}
	return strings.TrimSpace(string(data))
}

func assertBinaryPageHasVersion(t *testing.T, page *BinaryPageData, version string) {
	t.Helper()
	for _, item := range page.AllVersions {
		if item.Version == version && item.IsRemote {
			return
		}
	}
	t.Fatalf("binary page does not contain remote version %s: %+v", version, page.AllVersions)
}

func newVirtualWebDAVServer(t *testing.T) *virtualWebDAVServer {
	t.Helper()
	server := &virtualWebDAVServer{
		files: make(map[string]virtualWebDAVFile),
		dirs:  map[string]bool{"": true},
	}
	server.server = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.server.Close)
	return server
}

func (s *virtualWebDAVServer) handle(w http.ResponseWriter, r *http.Request) {
	key := cleanDAVKey(r.URL.Path)
	s.mu.Lock()
	defer s.mu.Unlock()

	switch r.Method {
	case http.MethodGet:
		s.handleGET(w, key)
	case http.MethodHead:
		s.handleHEAD(w, key)
	case http.MethodPut:
		s.handlePUT(w, r, key)
	case "MKCOL":
		s.handleMKCOL(w, key)
	case http.MethodDelete:
		s.handleDELETE(w, key)
	case "PROPFIND":
		s.handlePROPFIND(w, r, key)
	default:
		http.Error(w, "unsupported method", http.StatusMethodNotAllowed)
	}
}

func cleanDAVKey(requestPath string) string {
	cleaned := path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	if cleaned == "/" {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	if len(parts) >= 2 && parts[0] == "dav" {
		parts = parts[2:]
	}
	return strings.Join(parts, "/")
}

func (s *virtualWebDAVServer) handleGET(w http.ResponseWriter, key string) {
	file, ok := s.files[key]
	if !ok {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("ETag", file.etag)
	w.Header().Set("Content-Length", strconv.Itoa(len(file.data)))
	_, _ = w.Write(file.data)
}

func (s *virtualWebDAVServer) handleHEAD(w http.ResponseWriter, key string) {
	if file, ok := s.files[key]; ok {
		w.Header().Set("ETag", file.etag)
		w.Header().Set("Content-Length", strconv.Itoa(len(file.data)))
		w.WriteHeader(http.StatusOK)
		return
	}
	if s.isDir(key) {
		w.Header().Set("ETag", s.dirETag(key))
		w.WriteHeader(http.StatusOK)
		return
	}
	http.NotFound(w, nil)
}

func (s *virtualWebDAVServer) handlePUT(w http.ResponseWriter, r *http.Request, key string) {
	if r.Header.Get("If-None-Match") == "*" {
		if _, ok := s.files[key]; ok {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
	}
	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		current, ok := s.files[key]
		if !ok || current.etag != ifMatch {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
	}
	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, existed := s.files[key]
	s.ensureParentDirs(key)
	s.files[key] = virtualWebDAVFile{data: append([]byte(nil), data...), etag: s.nextETag(), modified: time.Now().UTC()}
	w.Header().Set("ETag", s.files[key].etag)
	if existed {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s *virtualWebDAVServer) handleMKCOL(w http.ResponseWriter, key string) {
	if s.isDir(key) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.ensureDir(key)
	w.WriteHeader(http.StatusCreated)
}

func (s *virtualWebDAVServer) handleDELETE(w http.ResponseWriter, key string) {
	if _, ok := s.files[key]; ok {
		delete(s.files, key)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if s.isDir(key) {
		prefix := key
		if prefix != "" {
			prefix += "/"
		}
		for fileKey := range s.files {
			if strings.HasPrefix(fileKey, prefix) {
				delete(s.files, fileKey)
			}
		}
		for dirKey := range s.dirs {
			if dirKey == key || strings.HasPrefix(dirKey, prefix) {
				delete(s.dirs, dirKey)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.NotFound(w, nil)
}

func (s *virtualWebDAVServer) handlePROPFIND(w http.ResponseWriter, r *http.Request, key string) {
	depth := r.Header.Get("Depth")
	responses := make([]propfindResponse, 0)
	if file, ok := s.files[key]; ok {
		responses = append(responses, s.fileResponse(key, file))
	} else {
		responses = append(responses, s.dirResponse(key))
		if depth == "1" {
			responses = append(responses, s.childResponses(key)...)
		}
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_ = xml.NewEncoder(w).Encode(propfindMultiStatus{XMLName: xml.Name{Local: "d:multistatus"}, XMLNS: "DAV:", Responses: responses})
}

func (s *virtualWebDAVServer) childResponses(dir string) []propfindResponse {
	prefix := dir
	if prefix != "" {
		prefix += "/"
	}
	childDirs := make(map[string]bool)
	childFiles := make(map[string]virtualWebDAVFile)
	for fileKey, file := range s.files {
		if !strings.HasPrefix(fileKey, prefix) {
			continue
		}
		rest := strings.TrimPrefix(fileKey, prefix)
		if rest == "" {
			continue
		}
		if idx := strings.Index(rest, "/"); idx >= 0 {
			childDirs[prefix+rest[:idx]] = true
		} else {
			childFiles[fileKey] = file
		}
	}
	for dirKey := range s.dirs {
		if dirKey == dir || !strings.HasPrefix(dirKey, prefix) {
			continue
		}
		rest := strings.TrimPrefix(dirKey, prefix)
		if rest == "" {
			continue
		}
		if idx := strings.Index(rest, "/"); idx >= 0 {
			childDirs[prefix+rest[:idx]] = true
		} else {
			childDirs[dirKey] = true
		}
	}
	dirKeys := make([]string, 0, len(childDirs))
	for dirKey := range childDirs {
		dirKeys = append(dirKeys, dirKey)
	}
	fileKeys := make([]string, 0, len(childFiles))
	for fileKey := range childFiles {
		fileKeys = append(fileKeys, fileKey)
	}
	sort.Strings(dirKeys)
	sort.Strings(fileKeys)
	responses := make([]propfindResponse, 0, len(dirKeys)+len(fileKeys))
	for _, dirKey := range dirKeys {
		responses = append(responses, s.dirResponse(dirKey))
	}
	for _, fileKey := range fileKeys {
		responses = append(responses, s.fileResponse(fileKey, childFiles[fileKey]))
	}
	return responses
}

func (s *virtualWebDAVServer) isDir(key string) bool {
	if s.dirs[key] {
		return true
	}
	prefix := key
	if prefix != "" {
		prefix += "/"
	}
	for fileKey := range s.files {
		if strings.HasPrefix(fileKey, prefix) {
			return true
		}
	}
	for dirKey := range s.dirs {
		if strings.HasPrefix(dirKey, prefix) {
			return true
		}
	}
	return false
}

func (s *virtualWebDAVServer) ensureParentDirs(key string) {
	dir := path.Dir(key)
	if dir == "." {
		return
	}
	s.ensureDir(dir)
}

func (s *virtualWebDAVServer) ensureDir(dir string) {
	dir = strings.Trim(strings.TrimPrefix(path.Clean("/"+dir), "/"), "/")
	if dir == "." {
		dir = ""
	}
	parts := strings.Split(dir, "/")
	current := ""
	s.dirs[current] = true
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" {
			current = part
		} else {
			current += "/" + part
		}
		s.dirs[current] = true
	}
}

func (s *virtualWebDAVServer) nextETag() string {
	s.nextID++
	return fmt.Sprintf("\"etag-%d\"", s.nextID)
}

func (s *virtualWebDAVServer) dirETag(key string) string {
	return fmt.Sprintf("\"dir-%s\"", key)
}

func (s *virtualWebDAVServer) fileResponse(key string, file virtualWebDAVFile) propfindResponse {
	return propfindResponse{
		Href: "/" + key,
		PropStat: propfindPropStat{
			Prop: propfindProp{
				ETag:          file.etag,
				ContentLength: len(file.data),
			},
			Status: "HTTP/1.1 200 OK",
		},
	}
}

func (s *virtualWebDAVServer) dirResponse(key string) propfindResponse {
	href := "/" + key
	if !strings.HasSuffix(href, "/") {
		href += "/"
	}
	return propfindResponse{
		Href: href,
		PropStat: propfindPropStat{
			Prop: propfindProp{
				ETag:         s.dirETag(key),
				ResourceType: propfindResourceType{Collection: &struct{}{}},
			},
			Status: "HTTP/1.1 200 OK",
		},
	}
}

type propfindMultiStatus struct {
	XMLName   xml.Name           `xml:"d:multistatus"`
	XMLNS     string             `xml:"xmlns:d,attr"`
	Responses []propfindResponse `xml:"d:response"`
}

type propfindResponse struct {
	Href     string           `xml:"d:href"`
	PropStat propfindPropStat `xml:"d:propstat"`
}

type propfindPropStat struct {
	Prop   propfindProp `xml:"d:prop"`
	Status string       `xml:"d:status"`
}

type propfindProp struct {
	ETag          string               `xml:"d:getetag,omitempty"`
	ContentLength int                  `xml:"d:getcontentlength"`
	ResourceType  propfindResourceType `xml:"d:resourcetype"`
}

type propfindResourceType struct {
	Collection *struct{} `xml:"d:collection,omitempty"`
}

func TestVirtualWebDAVCompareAndSwapRejectsStaleHead(t *testing.T) {
	preserveEnv(t, "HOME", "USERPROFILE", "CC_BOX_WEBDAV_PASSWORD")
	webdavServer := newVirtualWebDAVServer(t)
	baseURL := webdavServer.server.URL + "/dav"
	root := "/cc-box-cas/"
	device := newVirtualDevice(t)
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"light"}`)

	activateDevice(t, device)
	if err := device.app.InitNewDevice(baseURL, "user", virtualWebDAVPassword, root, "secret", "device-a"); err != nil {
		t.Fatalf("InitNewDevice: %v", err)
	}
	client := newConfiguredWebDAVClient(mustLoadConfig(t), virtualWebDAVPassword)
	if _, err := client.PUT("HEAD", []byte("foreign-head"), ""); err != nil {
		t.Fatalf("mutate remote HEAD: %v", err)
	}
	writeTextFile(t, filepath.Join(device.claudeDir, "settings.json"), `{"theme":"dark"}`)
	err := waitAsyncError(t, device.app.QuickPush())
	if err == nil || !strings.Contains(err.Error(), "远程已更新") {
		t.Fatalf("expected stale HEAD rejection, got %v", err)
	}
}

func mustLoadConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}
