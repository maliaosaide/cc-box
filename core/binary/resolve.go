package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/core/config"
)

const ClaudePathEnv = "CC_BOX_CLAUDE_PATH"

const claudeResolutionCacheTTL = 24 * time.Hour

type ClaudeResolution struct {
	CurrentPath string `json:"currentPath"`
	ManagedPath string `json:"managedPath"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Valid       bool   `json:"valid"`
	ReadOnly    bool   `json:"readOnly"`
	IsShim      bool   `json:"isShim"`
	Stale       bool   `json:"stale"`
	Error       string `json:"error,omitempty"`
}

type claudeCandidate struct {
	path      string
	source    string
	cacheable bool
	readOnly  bool
}

type claudeBinaryCache struct {
	Path        string    `json:"path"`
	ManagedPath string    `json:"managed_path"`
	Source      string    `json:"source"`
	Version     string    `json:"version"`
	ReadOnly    bool      `json:"read_only"`
	IsShim      bool      `json:"is_shim"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func ResolveClaudeBinary() ClaudeResolution {
	return resolveClaudeBinary(false)
}

func ResolveClaudeBinaryCached() ClaudeResolution {
	return resolveClaudeBinaryFast(claudeResolutionCacheTTL)
}

func RedetectClaudeBinary() ClaudeResolution {
	_ = ClearClaudeResolutionCache()
	return resolveClaudeBinary(true)
}

func ResolveClaudeManagedPath() string {
	cfg, err := config.Load()
	if err == nil && strings.TrimSpace(cfg.Binary.ClaudePath) != "" {
		if path, ok := configuredManagedPath(cfg.Binary.ClaudePath); ok {
			return path
		}
	}
	return defaultClaudeManagedPath()
}

func configuredManagedPath(path string) (string, bool) {
	path = expandBinaryPath(path)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || !info.Mode().IsRegular() || isScriptShim(path) {
		return "", false
	}
	return path, true
}

func defaultClaudeManagedPath() string {
	path := filepath.Join(config.LocalBinDir(), managedBinaryName("claude"))
	if isScriptShim(path) {
		return filepath.Join(config.CCBoxDir(), "bin", managedBinaryName("claude"))
	}
	return path
}

func ClearClaudeResolutionCache() error {
	err := os.Remove(claudeCachePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DetectVersion(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	hideCommandWindow(cmd)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", fmt.Errorf("执行 %s --version 超时", path)
	}
	if err != nil {
		return "", fmt.Errorf("执行 %s --version 失败: %w", path, err)
	}
	version := ParseVersionOutput(string(output))
	if version == "" {
		return "", fmt.Errorf("无法从 %s --version 输出识别版本", path)
	}
	return version, nil
}

func ParseVersionOutput(output string) string {
	for _, field := range strings.Fields(output) {
		if version := cleanVersionToken(field); version != "" {
			return version
		}
	}
	return ""
}

func resolveClaudeBinary(skipCache bool) ClaudeResolution {
	managedPath := ResolveClaudeManagedPath()
	var failures []string

	cfg, cfgErr := config.Load()
	if cfgErr == nil && strings.TrimSpace(cfg.Binary.ClaudePath) != "" {
		res, err := resolveCandidate(claudeCandidate{path: cfg.Binary.ClaudePath, source: "configured"}, managedPath)
		if err != nil {
			return invalidClaudeResolution(managedPath, fmt.Sprintf("手动配置的 Claude 路径不可用: %v", err))
		}
		_ = saveClaudeCache(res, "configured")
		return res
	}

	if envPath := strings.TrimSpace(os.Getenv(ClaudePathEnv)); envPath != "" {
		res, err := resolveCandidate(claudeCandidate{path: envPath, source: "environment", readOnly: true}, managedPath)
		if err != nil {
			return invalidClaudeResolution(managedPath, fmt.Sprintf("环境变量 %s 指向的 Claude 路径不可用: %v", ClaudePathEnv, err))
		}
		_ = saveClaudeCache(res, "environment")
		return res
	}

	if !skipCache {
		if res, ok := resolveCachedClaude(managedPath); ok {
			return res
		}
	}

	for _, candidate := range append(binDirCandidates(), append(pathCandidates(), commonCandidates()...)...) {
		res, err := resolveCandidate(candidate, managedPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", candidate.source, err))
			continue
		}
		if candidate.cacheable {
			_ = saveClaudeCache(res, candidate.source)
		}
		return res
	}

	message := "未找到可用的 Claude 二进制"
	if len(failures) > 0 {
		message += "；" + strings.Join(limitStrings(failures, 4), "；")
	}
	return invalidClaudeResolution(managedPath, message)
}

func resolveClaudeBinaryFast(maxAge time.Duration) ClaudeResolution {
	managedPath := ResolveClaudeManagedPath()
	cached, hasCache := readClaudeCache(managedPath)

	cfg, cfgErr := config.Load()
	if cfgErr == nil && strings.TrimSpace(cfg.Binary.ClaudePath) != "" {
		res, err := resolveCandidateFast(claudeCandidate{path: cfg.Binary.ClaudePath, source: "configured"}, managedPath, cached, hasCache, maxAge)
		if err != nil {
			return invalidClaudeResolution(managedPath, fmt.Sprintf("手动配置的 Claude 路径不可用: %v", err))
		}
		return res
	}

	if envPath := strings.TrimSpace(os.Getenv(ClaudePathEnv)); envPath != "" {
		res, err := resolveCandidateFast(claudeCandidate{path: envPath, source: "environment", readOnly: true}, managedPath, cached, hasCache, maxAge)
		if err != nil {
			return invalidClaudeResolution(managedPath, fmt.Sprintf("环境变量 %s 指向的 Claude 路径不可用: %v", ClaudePathEnv, err))
		}
		return res
	}

	if hasCache {
		res, err := resolveCandidateFast(claudeCandidate{path: cached.Path, source: "cache", readOnly: cached.ReadOnly}, managedPath, cached, hasCache, maxAge)
		if err == nil {
			res.Source = "cache"
			return res
		}
	}

	for _, candidate := range append(binDirCandidates(), append(pathCandidates(), commonCandidates()...)...) {
		res, err := resolveCandidateFast(candidate, managedPath, cached, hasCache, maxAge)
		if err == nil {
			return res
		}
	}

	return invalidClaudeResolution(managedPath, "未找到可用的 Claude 二进制")
}

func resolveCandidate(candidate claudeCandidate, managedPath string) (ClaudeResolution, error) {
	path := expandBinaryPath(candidate.path)
	if path == "" {
		return ClaudeResolution{}, errors.New("路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return ClaudeResolution{}, err
	}
	if info.IsDir() {
		return ClaudeResolution{}, fmt.Errorf("%s 是目录", path)
	}
	isShim := isScriptShim(path)
	version, err := DetectVersion(path)
	if err != nil {
		return ClaudeResolution{}, err
	}
	readOnly := candidate.readOnly || isShim || !samePath(path, managedPath)
	if candidate.source == "configured" && !isShim {
		readOnly = false
	}
	if candidate.source == "bin_dir" && samePath(path, managedPath) && !isShim {
		readOnly = false
	}
	return ClaudeResolution{
		CurrentPath: path,
		ManagedPath: managedPath,
		Source:      candidate.source,
		Version:     version,
		Valid:       true,
		ReadOnly:    readOnly,
		IsShim:      isShim,
	}, nil
}

func resolveCandidateFast(candidate claudeCandidate, managedPath string, cached claudeBinaryCache, hasCache bool, maxAge time.Duration) (ClaudeResolution, error) {
	path := expandBinaryPath(candidate.path)
	if path == "" {
		return ClaudeResolution{}, errors.New("路径为空")
	}
	info, err := os.Stat(path)
	if err != nil {
		return ClaudeResolution{}, err
	}
	if info.IsDir() {
		return ClaudeResolution{}, fmt.Errorf("%s 是目录", path)
	}
	isShim := isScriptShim(path)
	readOnly := candidate.readOnly || isShim || !samePath(path, managedPath)
	if candidate.source == "configured" && !isShim {
		readOnly = false
	}
	if candidate.source == "bin_dir" && samePath(path, managedPath) && !isShim {
		readOnly = false
	}

	version := ""
	stale := true
	if hasCache && samePath(cached.Path, path) {
		version = cached.Version
		readOnly = cached.ReadOnly || readOnly
		isShim = cached.IsShim || isShim
		stale = isClaudeCacheStale(cached, maxAge)
	}
	if version == "" {
		stale = true
	}

	return ClaudeResolution{
		CurrentPath: path,
		ManagedPath: managedPath,
		Source:      candidate.source,
		Version:     version,
		Valid:       true,
		ReadOnly:    readOnly,
		IsShim:      isShim,
		Stale:       stale,
	}, nil
}

func readClaudeCache(managedPath string) (claudeBinaryCache, bool) {
	data, err := os.ReadFile(claudeCachePath())
	if err != nil {
		return claudeBinaryCache{}, false
	}
	var cached claudeBinaryCache
	if err := json.Unmarshal(data, &cached); err != nil {
		_ = ClearClaudeResolutionCache()
		return claudeBinaryCache{}, false
	}
	if cached.Path == "" || !samePath(cached.ManagedPath, managedPath) {
		_ = ClearClaudeResolutionCache()
		return claudeBinaryCache{}, false
	}
	return cached, true
}

func isClaudeCacheStale(cached claudeBinaryCache, maxAge time.Duration) bool {
	if maxAge <= 0 || cached.UpdatedAt.IsZero() {
		return true
	}
	return time.Since(cached.UpdatedAt) > maxAge
}

func resolveCachedClaude(managedPath string) (ClaudeResolution, bool) {
	cached, ok := readClaudeCache(managedPath)
	if !ok {
		return ClaudeResolution{}, false
	}
	res, err := resolveCandidate(claudeCandidate{path: cached.Path, source: "cache", readOnly: cached.ReadOnly}, managedPath)
	if err != nil {
		_ = ClearClaudeResolutionCache()
		return ClaudeResolution{}, false
	}
	res.Source = "cache"
	res.ReadOnly = cached.ReadOnly || res.ReadOnly
	res.IsShim = cached.IsShim || res.IsShim
	return res, true
}

func saveClaudeCache(res ClaudeResolution, source string) error {
	cache := claudeBinaryCache{
		Path:        res.CurrentPath,
		ManagedPath: res.ManagedPath,
		Source:      source,
		Version:     res.Version,
		ReadOnly:    res.ReadOnly,
		IsShim:      res.IsShim,
		UpdatedAt:   time.Now().UTC(),
	}
	path := claudeCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func binDirCandidates() []claudeCandidate {
	var candidates []claudeCandidate
	for _, name := range claudeCandidateNames() {
		candidates = append(candidates, claudeCandidate{
			path:      filepath.Join(config.LocalBinDir(), name),
			source:    "bin_dir",
			cacheable: true,
		})
	}
	return candidates
}

func pathCandidates() []claudeCandidate {
	seen := make(map[string]bool)
	var candidates []claudeCandidate
	for _, name := range claudeCandidateNames() {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		path = expandBinaryPath(path)
		key := comparablePath(path)
		if seen[key] {
			continue
		}
		seen[key] = true
		candidates = append(candidates, claudeCandidate{
			path:      path,
			source:    "path",
			cacheable: true,
			readOnly:  true,
		})
	}
	return candidates
}

func commonCandidates() []claudeCandidate {
	dirs := commonClaudeDirs()
	var candidates []claudeCandidate
	for _, dir := range dirs {
		for _, name := range claudeCandidateNames() {
			candidates = append(candidates, claudeCandidate{
				path:      filepath.Join(dir, name),
				source:    "common",
				cacheable: true,
				readOnly:  true,
			})
		}
	}
	return candidates
}

func invalidClaudeResolution(managedPath, message string) ClaudeResolution {
	return ClaudeResolution{
		ManagedPath: managedPath,
		Source:      "not_found",
		Error:       message,
	}
}

func claudeCachePath() string {
	return filepath.Join(config.CCBoxDir(), "cache", "claude-binary.json")
}

func expandBinaryPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = expandPercentEnv(path)
	path = os.ExpandEnv(path)
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return filepath.Clean(path)
}

func expandPercentEnv(path string) string {
	if !strings.Contains(path, "%") {
		return path
	}
	var b strings.Builder
	for i := 0; i < len(path); {
		if path[i] != '%' {
			b.WriteByte(path[i])
			i++
			continue
		}
		j := strings.IndexByte(path[i+1:], '%')
		if j < 0 {
			b.WriteString(path[i:])
			break
		}
		key := path[i+1 : i+1+j]
		if value, ok := os.LookupEnv(key); ok {
			b.WriteString(value)
		} else {
			b.WriteString(path[i : i+j+2])
		}
		i += j + 2
	}
	return b.String()
}

func cleanVersionToken(token string) string {
	token = strings.Trim(token, " \t\r\n,;()[]{}")
	token = strings.TrimPrefix(strings.TrimPrefix(token, "v"), "V")
	if token == "" || token[0] < '0' || token[0] > '9' {
		return ""
	}
	return token
}

func isScriptShim(path string) bool {
	if isScriptShimExt(path) {
		return true
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil && resolved != path {
		if isScriptShimExt(resolved) {
			return true
		}
		path = resolved
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	data := make([]byte, 512)
	n, err := file.Read(data)
	if err != nil && n == 0 {
		return false
	}
	data = data[:n]
	if isBinaryMagic(data) {
		return false
	}
	if bytes.HasPrefix(data, []byte("#!")) {
		return true
	}
	return bytes.IndexByte(data, 0) < 0 && isMostlyText(data)
}

func isScriptShimExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmd", ".bat", ".ps1", ".sh", ".zsh", ".fish", ".js", ".mjs", ".cjs":
		return true
	default:
		return false
	}
}

func isBinaryMagic(data []byte) bool {
	if bytes.HasPrefix(data, []byte{0x7f, 'E', 'L', 'F'}) || bytes.HasPrefix(data, []byte("MZ")) {
		return true
	}
	for _, magic := range [][]byte{
		{0xfe, 0xed, 0xfa, 0xce},
		{0xfe, 0xed, 0xfa, 0xcf},
		{0xce, 0xfa, 0xed, 0xfe},
		{0xcf, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe},
		{0xca, 0xfe, 0xba, 0xbf},
	} {
		if bytes.HasPrefix(data, magic) {
			return true
		}
	}
	return false
}

func isMostlyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	printable := 0
	for _, b := range data {
		if b == '\n' || b == '\r' || b == '\t' || (b >= 32 && b < 127) {
			printable++
		}
	}
	return printable*100/len(data) >= 90
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, value := range values {
		if value == "" {
			continue
		}
		key := comparablePath(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func limitStrings(values []string, max int) []string {
	if len(values) <= max {
		return values
	}
	out := append([]string{}, values[:max]...)
	out = append(out, fmt.Sprintf("另有 %d 个候选失败", len(values)-max))
	return out
}
