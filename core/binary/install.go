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

type officialInstallCommandSpec struct {
	name    string
	args    []string
	message string
}

type ClaudeInstallResult struct {
	Version       string               `json:"version"`
	Path          string               `json:"path"`
	Source        string               `json:"source"`
	Output        string               `json:"output,omitempty"`
	Warnings      []string             `json:"warnings,omitempty"`
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
	runClaudeInstallSubcommandForInstall = runClaudeInstallSubcommand
)

func InstallOfficialClaude(ctx context.Context, progress InstallProgress) (*ClaudeInstallResult, error) {
	commands, err := officialInstallCommands()
	if err != nil {
		return nil, err
	}
	if resolution := resolveClaudeBinaryForInstall(); resolution.Valid && !resolution.IsShim {
		if err := backupExistingClaudeForInstall(resolution.CurrentPath); err != nil {
			return nil, fmt.Errorf("备份当前 Claude binary 失败: %w", err)
		}
	}

	var output []byte
	var runErr error
	for i, command := range commands {
		if progress != nil {
			progress(int64(i), int64(len(commands)), command.message)
		}
		output, runErr = officialInstallRunner(ctx, command.name, command.args)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if runErr == nil {
			break
		}
		if runtime.GOOS == "windows" && i == 0 && len(commands) > 1 && progress != nil {
			progress(int64(i+1), int64(len(commands)), "CMD 官方安装失败，正在尝试 PowerShell 官方安装命令")
		}
	}
	if runErr != nil {
		return nil, fmt.Errorf("官方安装失败: %w%s", runErr, installOutputSuffix(output))
	}
	_ = ClearClaudeResolutionCache()
	resolution := redetectClaudeBinaryForInstall()
	if !resolution.Valid {
		return nil, fmt.Errorf("官方安装完成，但重新检测 Claude 失败: %s%s", resolution.Error, installOutputSuffix(output))
	}
	_ = rememberClaudeBinarySourceForInstall(resolution.CurrentPath, "official", resolution.Version)
	if progress != nil {
		progress(int64(len(commands)), int64(len(commands)), "官方安装完成")
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
		progress(0, 6, "正在获取 GitHub Release 信息")
	}
	release, err := FindGitHubClaudeRelease(ctx, version)
	if err != nil {
		return nil, err
	}
	if progress != nil {
		progress(1, 6, "正在下载 GitHub Release 压缩包")
	}
	archiveData, err := githubDownloadURL(ctx, release.AssetDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("下载 %s 失败: %w", release.AssetName, err)
	}
	if progress != nil {
		progress(2, 6, "正在下载校验文件")
	}
	shasums, err := githubDownloadURL(ctx, release.ShasumsDownloadURL)
	if err != nil {
		return nil, fmt.Errorf("下载 SHASUMS256.txt 失败: %w", err)
	}
	if err := verifySHA256Line(archiveData, string(shasums), release.AssetName); err != nil {
		return nil, err
	}
	warnings := []string{githubSignatureWarning(release)}
	if progress != nil {
		progress(3, 6, "SHA256 校验完成；SHASUMS256.txt.sig 签名校验未执行")
	}
	binaryData, err := extractClaudeBinary(release.AssetName, archiveData)
	if err != nil {
		return nil, err
	}
	targetPath := GetBinaryPath("claude")
	if progress != nil {
		progress(4, 6, "正在安装 Claude "+version)
	}
	detected, err := installClaudeBinaryDataContext(ctx, targetPath, binaryData, version)
	if err != nil {
		return nil, err
	}
	_ = ClearClaudeResolutionCache()
	_ = rememberClaudeBinarySourceForInstall(targetPath, "github", detected)
	pathResult := configureClaudePathBestEffort(configureClaudePathForInstall, commandStateForInstall)
	if progress != nil && pathResult != nil && pathResult.Error != "" {
		progress(5, 6, pathResult.Message)
	}
	if progress != nil {
		progress(6, 6, "GitHub Release 安装完成")
	}
	return &ClaudeInstallResult{
		Version:       detected,
		Path:          targetPath,
		Source:        "github",
		Warnings:      warnings,
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
	return installClaudeBinaryDataContext(context.Background(), targetPath, data, expectedVersion)
}

func installClaudeBinaryData(targetPath string, data []byte, expectedVersion string) (string, error) {
	return installClaudeBinaryDataContext(context.Background(), targetPath, data, expectedVersion)
}

func installClaudeBinaryDataContext(ctx context.Context, targetPath string, data []byte, expectedVersion string) (string, error) {
	targetPath = expandBinaryPath(targetPath)
	if targetPath == "" {
		return "", fmt.Errorf("Claude 安装目标路径不能为空")
	}
	detected, err := detectClaudeBinaryDataVersion(targetPath, data)
	if err != nil {
		return "", fmt.Errorf("检测 Claude 版本失败: %w", err)
	}
	if detected != expectedVersion {
		return "", fmt.Errorf("Claude 版本为 %s，期望 %s", detected, expectedVersion)
	}
	if err := EnsureClaudeInstallDirs(); err != nil {
		return "", fmt.Errorf("创建 Claude 安装目录失败: %w", err)
	}
	existingData, existingMode, hadExisting := readExistingFileForRollback(targetPath)
	if err := backupExistingClaudeForInstall(targetPath); err != nil {
		return "", fmt.Errorf("备份当前 Claude binary 失败: %w", err)
	}
	if err := WriteFileAtomic(targetPath, data, 0755); err != nil {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("写入 Claude binary 失败: %w", err)
	}
	output, err := runClaudeInstallSubcommandForInstall(ctx, targetPath)
	if err != nil {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("执行 Claude install 初始化失败: %w%s", err, installOutputSuffix(output))
	}
	if err := WriteFileAtomic(targetPath, data, 0755); err != nil {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("写回所选 Claude binary 失败: %w", err)
	}
	_ = ClearClaudeResolutionCache()
	finalDetected, err := detectVersionForInstall(targetPath)
	if err != nil {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("校验最终 Claude 版本失败: %w", err)
	}
	if finalDetected != expectedVersion {
		restoreExistingFileAfterInstallFailure(targetPath, existingData, existingMode, hadExisting)
		return "", fmt.Errorf("最终 Claude 版本为 %s，期望 %s", finalDetected, expectedVersion)
	}
	return finalDetected, nil
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

func EnsureClaudeInstallDirs() error {
	for _, dir := range []string{
		config.LocalBinDir(),
		filepath.Dir(config.VersionsDir()),
		config.VersionsDir(),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

func runClaudeInstallSubcommand(ctx context.Context, targetPath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, targetPath, "install")
	hideCommandWindow(cmd)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return output, ctx.Err()
	}
	return output, err
}

func officialInstallCommands() ([]officialInstallCommandSpec, error) {
	switch runtime.GOOS {
	case "windows":
		return []officialInstallCommandSpec{
			{
				name:    "cmd.exe",
				args:    []string{"/C", "curl -fsSL https://claude.ai/install.cmd -o install.cmd && install.cmd && del install.cmd"},
				message: "正在执行官方 CMD 安装命令",
			},
			{
				name:    "powershell.exe",
				args:    []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "irm https://claude.ai/install.ps1 | iex"},
				message: "正在执行官方 PowerShell 安装命令",
			},
		}, nil
	case "darwin", "linux":
		return []officialInstallCommandSpec{{
			name:    "/bin/sh",
			args:    []string{"-c", "curl -fsSL https://claude.ai/install.sh | bash"},
			message: "正在执行官方 shell 安装命令",
		}}, nil
	default:
		return nil, fmt.Errorf("当前平台不支持官方安装: %s", config.Platform())
	}
}

func officialInstallCommand() (string, []string, error) {
	commands, err := officialInstallCommands()
	if err != nil {
		return "", nil, err
	}
	return commands[0].name, commands[0].args, nil
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

func installOutputSuffix(output []byte) string {
	text := strings.TrimSpace(outputTail(output, 4096))
	if text == "" {
		return ""
	}
	return "\n" + text
}

func outputTail(data []byte, max int) string {
	data = bytes.TrimSpace(data)
	if len(data) <= max {
		return string(data)
	}
	return string(data[len(data)-max:])
}
