package binary

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/user/cc-box/core/config"
)

type InstallProgress func(current, total int64, message string)

type ClaudeCommandStatus struct {
	Status       string `json:"status"`
	CommandPath  string `json:"commandPath"`
	TargetPath   string `json:"targetPath"`
	TargetExists bool   `json:"targetExists"`
}

type PathConfigureResult struct {
	Enabled       bool                `json:"enabled"`
	Changed       bool                `json:"changed"`
	AlreadyActive bool                `json:"alreadyActive"`
	TargetDir     string              `json:"targetDir"`
	ConfigPath    string              `json:"configPath"`
	Message       string              `json:"message"`
	Error         string              `json:"error,omitempty"`
	CommandStatus ClaudeCommandStatus `json:"commandStatus"`
}

type ClaudeInstallResult struct {
	Version       string               `json:"version"`
	Path          string               `json:"path"`
	Source        string               `json:"source"`
	Output        string               `json:"output,omitempty"`
	CommandStatus ClaudeCommandStatus  `json:"commandStatus"`
	PathConfig    *PathConfigureResult `json:"pathConfig,omitempty"`
}

var (
	officialInstallRunner                = runOfficialInstallCommand
	resolveClaudeBinaryForInstall        = ResolveClaudeBinary
	redetectClaudeBinaryForInstall       = RedetectClaudeBinary
	backupExistingClaudeForInstall       = backupExistingClaude
	detectVersionForInstall              = DetectVersion
	rememberClaudeBinarySourceForInstall = RememberClaudeBinarySource
	configureClaudePathForInstall        = ConfigureClaudePathIfEnabled
	commandStateForInstall               = ClaudeCommandState
	configureUserPathDirForInstall       = configureUserPathDir
)

func InstallOfficialClaude(ctx context.Context, progress InstallProgress) (*ClaudeInstallResult, error) {
	name, args, err := officialInstallCommand()
	if err != nil {
		return nil, err
	}
	if resolution := resolveClaudeBinaryForInstall(); resolution.Valid && !resolution.IsShim {
		if err := backupExistingClaudeForInstall(resolution.CurrentPath); err != nil {
			return nil, fmt.Errorf("备份当前 Claude binary 失败: %w", err)
		}
	}
	if progress != nil {
		progress(0, 1, "正在执行官方安装命令")
	}
	output, err := officialInstallRunner(ctx, name, args)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, fmt.Errorf("官方安装失败: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	_ = ClearClaudeResolutionCache()
	resolution := redetectClaudeBinaryForInstall()
	if !resolution.Valid {
		return nil, fmt.Errorf("官方安装完成，但重新检测 Claude 失败: %s\n%s", resolution.Error, strings.TrimSpace(string(output)))
	}
	_ = rememberClaudeBinarySourceForInstall(resolution.CurrentPath, "official", resolution.Version)
	if progress != nil {
		progress(1, 1, "官方安装完成")
	}
	return &ClaudeInstallResult{
		Version:       resolution.Version,
		Path:          resolution.CurrentPath,
		Source:        "official",
		Output:        strings.TrimSpace(string(output)),
		CommandStatus: commandStateForInstall(resolution.CurrentPath),
	}, nil
}

func InstallGitHubClaude(ctx context.Context, version string, progress InstallProgress) (*ClaudeInstallResult, error) {
	version = cleanVersionToken(version)
	if version == "" {
		return nil, fmt.Errorf("版本号不能为空")
	}
	if progress != nil {
		progress(0, 5, "正在获取 GitHub Release 信息")
	}
	release, err := FindGitHubClaudeRelease(ctx, version)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress(1, 5, "正在下载 GitHub Release 压缩包")
	}
	archiveData, err := githubDownloadURL(ctx, release.AssetDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("下载 %s 失败: %w", release.AssetName, err)
	}
	if progress != nil {
		progress(2, 5, "正在下载校验文件")
	}
	shasums, err := githubDownloadURL(ctx, release.ShasumsDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("下载 SHASUMS256.txt 失败: %w", err)
	}
	if err := verifySHA256Line(archiveData, string(shasums), release.AssetName); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(3, 5, "正在解压 Claude binary")
	}
	binaryData, err := extractClaudeBinary(release.AssetName, archiveData)
	if err != nil {
		return nil, err
	}
	targetPath := GetBinaryPath("claude")
	if progress != nil {
		progress(4, 5, "正在安装 Claude "+version)
	}
	detected, err := installClaudeBinaryData(targetPath, binaryData, version)
	if err != nil {
		return nil, err
	}
	_ = ClearClaudeResolutionCache()
	_ = rememberClaudeBinarySourceForInstall(targetPath, "github", detected)
	pathResult := configureClaudePathBestEffort(configureClaudePathForInstall, commandStateForInstall)
	if progress != nil && pathResult != nil && pathResult.Error != "" {
		progress(4, 5, pathResult.Message)
	}
	if progress != nil {
		progress(5, 5, "GitHub Release 安装完成")
	}
	return &ClaudeInstallResult{
		Version:       detected,
		Path:          targetPath,
		Source:        "github",
		CommandStatus: commandStateForInstall(targetPath),
		PathConfig:    pathResult,
	}, nil
}

func runOfficialInstallCommand(ctx context.Context, name string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	hideCommandWindow(cmd)
	return cmd.CombinedOutput()
}

func InstallClaudeBinaryData(targetPath string, data []byte, expectedVersion string) (string, error) {
	return installClaudeBinaryData(targetPath, data, expectedVersion)
}

func installClaudeBinaryData(targetPath string, data []byte, expectedVersion string) (string, error) {
	detected, err := detectClaudeBinaryDataVersion(targetPath, data)
	if err != nil {
		return "", fmt.Errorf("检测 Claude 版本失败: %w", err)
	}
	if detected != expectedVersion {
		return "", fmt.Errorf("Claude 版本为 %s，期望 %s", detected, expectedVersion)
	}
	existingData, existingMode, hadExisting := readExistingFileForRollback(targetPath)
	if err := backupExistingClaudeForInstall(targetPath); err != nil {
		return "", fmt.Errorf("备份当前 Claude binary 失败: %w", err)
	}
	if err := WriteFileAtomic(targetPath, data, 0755); err != nil {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("写入 Claude binary 失败: %w", err)
	}
	return detected, nil
}

func detectClaudeBinaryDataVersion(targetPath string, data []byte) (string, error) {
	dir := filepath.Dir(targetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	pattern := "." + filepath.Base(targetPath) + ".verify-*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(0755); err != nil {
		_ = tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return detectVersionForInstall(tmpPath)
}

func officialInstallCommand() (string, []string, error) {
	switch runtime.GOOS {
	case "windows":
		return "powershell.exe", []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://claude.ai/install.ps1 | iex"}, nil
	case "darwin", "linux":
		return "/bin/sh", []string{"-c", "curl -fsSL https://claude.ai/install.sh | bash"}, nil
	default:
		return "", nil, fmt.Errorf("当前平台不支持官方安装: %s", config.Platform())
	}
}

func ClaudeCommandState(targetPath string) ClaudeCommandStatus {
	targetPath = strings.TrimSpace(targetPath)
	status := ClaudeCommandStatus{TargetPath: targetPath}
	if targetPath == "" {
		status.Status = "not_installed"
		return status
	}
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		status.TargetExists = true
	}
	commandPath, err := exec.LookPath("claude")
	if err != nil && runtime.GOOS == "windows" {
		commandPath, err = exec.LookPath("claude.exe")
	}
	if err == nil {
		status.CommandPath = commandPath
		if samePath(commandPath, targetPath) {
			status.Status = "activated"
			return status
		}
		if status.TargetExists {
			status.Status = "shadowed_by_other_binary"
			return status
		}
	}
	if status.TargetExists {
		status.Status = "installed_not_activated"
	} else {
		status.Status = "not_installed"
	}
	return status
}

func ConfigureClaudePathIfEnabled() (*PathConfigureResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return &PathConfigureResult{Enabled: false, Message: "未加载配置，跳过 PATH 自动配置"}, nil
	}
	if !cfg.Binary.AutoConfigurePath {
		return &PathConfigureResult{Enabled: false, Message: "PATH 自动配置未开启"}, nil
	}
	return ConfigureClaudePath()
}

func ConfigureClaudePathIfEnabledBestEffort() *PathConfigureResult {
	return configureClaudePathBestEffort(ConfigureClaudePathIfEnabled, ClaudeCommandState)
}

func configureClaudePathBestEffort(run func() (*PathConfigureResult, error), commandState func(string) ClaudeCommandStatus) *PathConfigureResult {
	result, err := run()
	if err == nil {
		return result
	}
	targetPath := GetBinaryPath("claude")
	return &PathConfigureResult{
		Enabled:       true,
		TargetDir:     filepath.Dir(targetPath),
		Message:       "PATH 自动配置失败: " + err.Error(),
		Error:         err.Error(),
		CommandStatus: commandState(targetPath),
	}
}

func ConfigureClaudePath() (*PathConfigureResult, error) {
	targetPath := GetBinaryPath("claude")
	status := ClaudeCommandState(targetPath)
	result := &PathConfigureResult{
		Enabled:       true,
		AlreadyActive: status.Status == "activated",
		TargetDir:     filepath.Dir(targetPath),
		CommandStatus: status,
	}
	if result.AlreadyActive {
		result.Message = "claude 命令已命中当前目标路径"
		return result, nil
	}
	if !status.TargetExists {
		result.Message = "Claude binary 尚未安装，未配置 PATH"
		return result, nil
	}
	configPath, changed, err := configureUserPathDirForInstall(result.TargetDir)
	if err != nil {
		return nil, err
	}
	result.ConfigPath = configPath
	result.Changed = changed
	if changed {
		result.Message = "已配置用户级 PATH，重新打开终端后生效"
	} else {
		result.Message = "用户级 PATH 已包含目标目录"
	}
	return result, nil
}

func readExistingFileForRollback(path string) ([]byte, os.FileMode, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil, 0, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false
	}
	return data, info.Mode(), true
}

func restoreExistingFileAfterInstallFailure(path string, data []byte, mode os.FileMode, hadExisting bool) {
	if hadExisting {
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		_ = os.WriteFile(path, data, mode)
		return
	}
	_ = os.Remove(path)
}

func RememberClaudeBinarySource(path, source, version string) error {
	path = expandBinaryPath(path)
	if path == "" || source == "" {
		return nil
	}
	managedPath := ResolveClaudeManagedPath()
	if version == "" {
		version, _ = DetectVersion(path)
	}
	return saveClaudeCache(ClaudeResolution{
		CurrentPath: path,
		ManagedPath: managedPath,
		Source:      source,
		Version:     version,
		Valid:       true,
		ReadOnly:    !samePath(path, managedPath),
		IsShim:      isScriptShim(path),
	}, source)
}

func extractClaudeBinary(assetName string, data []byte) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractClaudeFromZip(data)
	}
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		return extractClaudeFromTarGz(data)
	}
	return nil, fmt.Errorf("不支持的 GitHub Release 产物格式: %s", assetName)
}

func outputTail(data []byte, max int) string {
	data = bytes.TrimSpace(data)
	if len(data) <= max {
		return string(data)
	}
	return string(data[len(data)-max:])
}
