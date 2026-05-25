package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

type RestoreIssue string

const (
	RestoreIssueNoSnapshotBinary     RestoreIssue = "no_binary_for_platform"
	RestoreIssueVersionMissing       RestoreIssue = "version_missing_in_index"
	RestoreIssueInstalledSameVersion RestoreIssue = "installed_same_version"
	RestoreIssueDownloadRequired     RestoreIssue = "download_required"
	RestoreIssueNotInstalled         RestoreIssue = "not_installed"
	RestoreIssuePathNotActive        RestoreIssue = "path_not_active"
	RestoreIssueSkipByUser           RestoreIssue = "skip_by_user"
)

type ClaudeRestorePolicy string

const (
	ClaudeRestoreExact  ClaudeRestorePolicy = "exact"
	ClaudeRestoreLatest ClaudeRestorePolicy = "latest"
	ClaudeRestoreSkip   ClaudeRestorePolicy = "skip"
)

type ClaudeRestoreAction string

const (
	ClaudeActionSkipAlreadyInstalled ClaudeRestoreAction = "skip_already_installed"
	ClaudeActionSkipNoSnapshot       ClaudeRestoreAction = "skip_no_snapshot"
	ClaudeActionDownload             ClaudeRestoreAction = "download"
	ClaudeActionNeedUserChoice       ClaudeRestoreAction = "need_user_choice"
	ClaudeActionUnavailable          ClaudeRestoreAction = "unavailable"
)

type ClaudeRestorePlan struct {
	Platform       string               `json:"platform"`
	PlatformLabel  string               `json:"platformLabel"`
	Policy         ClaudeRestorePolicy  `json:"policy"`
	TargetVersion  string               `json:"targetVersion"`
	CurrentVersion string               `json:"currentVersion"`
	TargetPath     string               `json:"targetPath"`
	Action         ClaudeRestoreAction  `json:"action"`
	PathActive     bool                 `json:"pathActive"`
	Issues         []RestoreIssue       `json:"issues"`
	PathConfig     *PathConfigureResult `json:"pathConfig,omitempty"`
}

func CurrentClaudeVersion() (string, error) {
	resolution := ResolveClaudeBinary()
	if !resolution.Valid {
		return "", fmt.Errorf("%s", resolution.Error)
	}
	if resolution.IsShim {
		return "", fmt.Errorf("当前 Claude 路径是脚本 shim，不支持作为可恢复二进制状态")
	}
	version := strings.TrimSpace(resolution.Version)
	if version == "" {
		return "", fmt.Errorf("无法识别当前 Claude 版本")
	}
	return version, nil
}

func CurrentClaudeVersionMap() map[string]map[string]string {
	version, err := CurrentClaudeVersion()
	if err != nil {
		return nil
	}
	return map[string]map[string]string{config.Platform(): {"claude": version}}
}

func SnapshotClaudeVersion(snap *snapshot.Snapshot) (string, bool) {
	if snap == nil || snap.Binary == nil {
		return "", false
	}
	tools, ok := snap.Binary[config.Platform()]
	if !ok {
		return "", false
	}
	version := strings.TrimSpace(tools["claude"])
	return version, version != ""
}

func SetSnapshotClaudeVersion(snap *snapshot.Snapshot, version string) {
	version = strings.TrimSpace(version)
	if snap == nil || version == "" {
		return
	}
	platform := config.Platform()
	if snap.Binary == nil {
		snap.Binary = make(map[string]map[string]string)
	}
	if snap.Binary[platform] == nil {
		snap.Binary[platform] = make(map[string]string)
	}
	snap.Binary[platform]["claude"] = version
}

func CloneSnapshotBinary(snap *snapshot.Snapshot) map[string]map[string]string {
	if snap == nil || len(snap.Binary) == 0 {
		return nil
	}
	cloned := make(map[string]map[string]string, len(snap.Binary))
	for platform, tools := range snap.Binary {
		if len(tools) == 0 {
			continue
		}
		cloned[platform] = make(map[string]string, len(tools))
		if version := strings.TrimSpace(tools["claude"]); version != "" {
			cloned[platform]["claude"] = version
		}
		if len(cloned[platform]) == 0 {
			delete(cloned, platform)
		}
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func RemoteClaudeVersionExists(client *webdav.Client, version string) (bool, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return false, nil
	}
	idx, err := LoadIndex(client)
	if err != nil {
		return false, err
	}
	info := idx.GetBinaryInfo(config.Platform(), "claude")
	if info == nil {
		return false, nil
	}
	_, exists := info.Versions[version]
	return exists, nil
}

func EnsureCurrentClaudeUploaded(client *webdav.Client, key []byte, progress UploadProgress) (string, bool, error) {
	version, err := CurrentClaudeVersion()
	if err != nil {
		return "", false, err
	}
	exists, err := RemoteClaudeVersionExists(client, version)
	if err != nil {
		return "", false, err
	}
	if exists {
		return version, false, nil
	}

	resolution := ResolveClaudeBinary()
	data, err := os.ReadFile(resolution.CurrentPath)
	if err != nil {
		return "", false, fmt.Errorf("读取当前 Claude 二进制失败: %w", err)
	}
	if err := Upload(client, key, "claude", data, version, progress); err != nil {
		if exists, checkErr := RemoteClaudeVersionExists(client, version); checkErr == nil && exists {
			return version, false, nil
		}
		return "", false, err
	}
	return version, true, nil
}

func PlanClaudeRestore(client *webdav.Client, _ []byte, snap *snapshot.Snapshot, policy ClaudeRestorePolicy) (*ClaudeRestorePlan, error) {
	if policy == "" {
		policy = ClaudeRestoreExact
	}
	platform := config.Platform()
	plan := &ClaudeRestorePlan{
		Platform:      platform,
		PlatformLabel: claudePlatformLabel(platform),
		Policy:        policy,
		TargetPath:    GetBinaryPath("claude"),
	}
	plan.PathActive = claudeCommandTargets(plan.TargetPath)
	if !plan.PathActive {
		plan.Issues = append(plan.Issues, RestoreIssuePathNotActive)
	}

	if policy == ClaudeRestoreSkip {
		plan.Action = ClaudeActionNeedUserChoice
		plan.Issues = append(plan.Issues, RestoreIssueSkipByUser)
		return plan, nil
	}
	if policy != ClaudeRestoreExact {
		plan.Action = ClaudeActionNeedUserChoice
		return plan, nil
	}

	targetVersion, ok := SnapshotClaudeVersion(snap)
	if !ok {
		plan.Action = ClaudeActionSkipNoSnapshot
		plan.Issues = append(plan.Issues, RestoreIssueNoSnapshotBinary)
		return plan, nil
	}
	plan.TargetVersion = targetVersion

	resolution := ResolveClaudeBinary()
	if resolution.Valid {
		plan.CurrentVersion = strings.TrimSpace(resolution.Version)
	} else {
		plan.Issues = append(plan.Issues, RestoreIssueNotInstalled)
	}

	if plan.CurrentVersion == targetVersion {
		plan.Action = ClaudeActionSkipAlreadyInstalled
		plan.Issues = append(plan.Issues, RestoreIssueInstalledSameVersion)
		return plan, nil
	}

	idx, err := LoadIndex(client)
	if err != nil {
		return nil, err
	}
	info := idx.GetBinaryInfo(platform, "claude")
	if info == nil {
		plan.Action = ClaudeActionUnavailable
		plan.Issues = append(plan.Issues, RestoreIssueNoSnapshotBinary)
		return plan, nil
	}
	if _, exists := info.Versions[targetVersion]; !exists {
		plan.Action = ClaudeActionUnavailable
		plan.Issues = append(plan.Issues, RestoreIssueVersionMissing)
		return plan, nil
	}
	plan.Action = ClaudeActionDownload
	plan.Issues = append(plan.Issues, RestoreIssueDownloadRequired)
	return plan, nil
}

func ApplyClaudeRestore(client *webdav.Client, key []byte, plan *ClaudeRestorePlan, progress DownloadProgress) error {
	if plan == nil {
		return nil
	}
	switch plan.Action {
	case ClaudeActionSkipAlreadyInstalled, ClaudeActionSkipNoSnapshot:
		return nil
	case ClaudeActionUnavailable:
		return fmt.Errorf("快照需要 Claude %s，但云端没有当前平台可用版本", plan.TargetVersion)
	case ClaudeActionNeedUserChoice:
		return nil
	case ClaudeActionDownload:
		data, err := DownloadData(client, key, "claude", plan.TargetVersion, progress)
		if err != nil {
			return err
		}
		version, err := installClaudeBinaryData(plan.TargetPath, data, plan.TargetVersion)
		if err != nil {
			return err
		}
		_ = ClearClaudeResolutionCache()
		_ = RememberClaudeBinarySource(plan.TargetPath, "webdav", version)
		plan.PathConfig = ConfigureClaudePathIfEnabledBestEffort()
		return nil
	default:
		return fmt.Errorf("未知 Claude binary 恢复动作: %s", plan.Action)
	}
}

func backupExistingClaude(targetPath string) error {
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s 是目录", targetPath)
	}

	version, err := DetectVersion(targetPath)
	if err != nil || version == "" {
		version = "unknown-" + time.Now().UTC().Format("20060102150405")
	}
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	shortHash := hex.EncodeToString(sum[:])[:8]
	backupPath := filepath.Join(config.VersionsDir(), version+"-"+shortHash, filepath.Base(targetPath))
	return BackupFileIfMissing(targetPath, backupPath)
}

func claudeCommandTargets(targetPath string) bool {
	if targetPath == "" {
		return false
	}
	for _, name := range claudeCandidateNames() {
		path, err := exec.LookPath(name)
		if err == nil && samePath(path, targetPath) {
			return true
		}
	}
	return false
}

func claudePlatformLabel(platform string) string {
	switch platform {
	case "windows-amd64":
		return "Windows x64"
	case "darwin-arm64":
		return "Mac M 系列"
	case "linux-amd64":
		return "Linux x64"
	default:
		return platform
	}
}
