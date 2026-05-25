// 配置管理
// config.toml 读写、设备 ID 生成、默认值
package config

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"
)

// Config 本地和运行时配置
type Config struct {
	WebDAV     WebDAVConfig     `mapstructure:"webdav"`
	Encryption EncryptionConfig `mapstructure:"encryption"`
	Sync       SyncConfig       `mapstructure:"sync"`
	Device     DeviceConfig     `mapstructure:"device"`
	Claude     ClaudeConfig     `mapstructure:"claude"`
	Binary     BinaryConfig     `mapstructure:"binary"`
	Exclude    ExcludeConfig    `mapstructure:"exclude"`
}

type WebDAVConfig struct {
	URL      string `mapstructure:"url"`
	Username string `mapstructure:"username"`
	Root     string `mapstructure:"root"`
}

type EncryptionConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type SyncConfig struct {
	SnapshotLimit    int    `mapstructure:"snapshot_limit"`
	ConflictStrategy string `mapstructure:"conflict_strategy"`
	MergeRetryMax    int    `mapstructure:"merge_retry_max"`
	AutoSyncInterval string `mapstructure:"auto_sync_interval"`
}

type DeviceConfig struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
}

type ClaudeConfig struct {
	Path     string `mapstructure:"path"`
	JSONPath string `mapstructure:"json_path"`
}

type BinaryConfig struct {
	Encrypt           bool   `mapstructure:"encrypt"`
	ChunkMode         string `mapstructure:"chunk_mode"`
	ChunkSizeMB       int    `mapstructure:"chunk_size_mb"`
	ChunkThresholdMB  int    `mapstructure:"chunk_threshold_mb"`
	AutoUpload        bool   `mapstructure:"auto_upload"`
	SyncEnabled       bool   `mapstructure:"sync_enabled"`
	AutoConfigurePath bool   `mapstructure:"auto_configure_path"`
	BinDir            string `mapstructure:"bin_dir"`
	VersionsDir       string `mapstructure:"versions_dir"`
	ClaudePath        string `mapstructure:"claude_path"`
}

type ExcludeConfig struct {
	Patterns []string `mapstructure:"patterns"`
}

// DefaultExcludePatterns 默认不排除任何目录
var DefaultExcludePatterns = []string{}

// DefaultConfig 返回带默认值的配置
func DefaultConfig() *Config {
	return &Config{
		WebDAV: WebDAVConfig{
			Root: "cc-box",
		},
		Encryption: EncryptionConfig{
			Enabled: true,
		},
		Sync: SyncConfig{
			SnapshotLimit:    50,
			ConflictStrategy: "ask",
			MergeRetryMax:    3,
		},
		Device: DeviceConfig{
			ID:   GenerateDeviceID(),
			Name: hostname(),
		},
		Binary: BinaryConfig{
			Encrypt:          false,
			ChunkMode:        "auto",
			ChunkSizeMB:      10,
			ChunkThresholdMB: 50,
		},
		Exclude: ExcludeConfig{
			Patterns: DefaultExcludePatterns,
		},
	}
}

// GenerateDeviceID 生成设备唯一标识（hostname + 随机 6 字符）
func GenerateDeviceID() string {
	b := make([]byte, 3)
	rand.Read(b)
	h := hostname()
	suffix := fmt.Sprintf("%x", b)
	return h + "-" + suffix
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		name = "unknown"
	}
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, " ", "-")
	return name
}

// CCBoxDir 返回 ~/.cc-box/ 路径
func CCBoxDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cc-box")
}

// DefaultClaudeDir 返回当前系统用户的默认 Claude 配置目录
func DefaultClaudeDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// ClaudeDir 返回 Claude 配置目录路径
func ClaudeDir() string {
	v := loadViper()
	if custom := v.GetString("claude.path"); custom != "" {
		return expandHome(custom)
	}
	return DefaultClaudeDir()
}

// DefaultClaudeJSONPath 返回当前系统用户的默认 Claude JSON 配置文件路径
func DefaultClaudeJSONPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude.json")
}

// ClaudeJSONPath 返回 Claude JSON 配置文件路径
func ClaudeJSONPath() string {
	v := loadViper()
	if custom := v.GetString("claude.json_path"); custom != "" {
		return expandHome(custom)
	}
	return DefaultClaudeJSONPath()
}

// LocalBinDir 返回二进制目录路径
// 直接用 viper 读取，绕过 Unmarshal 对 PascalCase key 的匹配问题
func LocalBinDir() string {
	v := loadViper()
	if val := v.GetString("binary.bin_dir"); val != "" {
		return expandHome(val)
	}
	if val := v.GetString("binary.bindir"); val != "" {
		return expandHome(val)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
}

// VersionsDir 返回版本存档目录路径
func VersionsDir() string {
	v := loadViper()
	if val := v.GetString("binary.versions_dir"); val != "" {
		return expandHome(val)
	}
	if val := v.GetString("binary.versionsdir"); val != "" {
		return expandHome(val)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "claude", "versions")
}

func NormalizeWebDAVRoot(root string) string {
	root = strings.TrimSpace(strings.ReplaceAll(root, "\\", "/"))
	root = strings.Trim(root, "/")
	if root == "" {
		return ""
	}

	parts := strings.Split(root, "/")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	return strings.Join(normalized, "/")
}

func WebDAVBaseURL(url, root string) string {
	base := strings.TrimRight(strings.TrimSpace(url), "/")
	root = NormalizeWebDAVRoot(root)
	if root != "" {
		base += "/" + root
	}
	return base + "/"
}

func ConfiguredWebDAVURL(cfg *Config) string {
	return WebDAVBaseURL(cfg.WebDAV.URL, cfg.WebDAV.Root)
}

// loadViper 加载配置文件的 viper 实例
func loadViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(CCBoxDir())
	v.ReadInConfig()
	return v
}

// LoadRaw 返回原始 viper 实例用于读取未映射字段
func LoadRaw() *viper.Viper {
	return loadViper()
}

// expandHome 展开路径中的 ~ 前缀
func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

// KeyPath 返回密钥文件路径
func KeyPath() string {
	return filepath.Join(CCBoxDir(), "key.bin")
}

// Platform 返回当前平台标识，如 "windows-amd64"
func Platform() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

// Load 从 ~/.cc-box/config.toml 加载配置
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(CCBoxDir())

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置失败: %w", err)
	}

	cfg := DefaultConfig()
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}
	cfg.WebDAV.Root = NormalizeWebDAVRoot(cfg.WebDAV.Root)
	if cfg.Binary.BinDir == "" {
		cfg.Binary.BinDir = v.GetString("binary.bindir")
	}
	if cfg.Binary.VersionsDir == "" {
		cfg.Binary.VersionsDir = v.GetString("binary.versionsdir")
	}
	if !v.IsSet("binary.sync_enabled") && v.IsSet("binary.auto_upload") {
		cfg.Binary.SyncEnabled = cfg.Binary.AutoUpload
	}
	cfg.Binary.AutoUpload = cfg.Binary.SyncEnabled
	return cfg, nil
}

// Save 写入配置到 ~/.cc-box/config.toml
func Save(cfg *Config) error {
	dir := CCBoxDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(dir)
	cfg.WebDAV.Root = NormalizeWebDAVRoot(cfg.WebDAV.Root)

	v.Set("webdav", map[string]interface{}{
		"url":      cfg.WebDAV.URL,
		"username": cfg.WebDAV.Username,
		"root":     cfg.WebDAV.Root,
	})
	v.Set("encryption", map[string]interface{}{
		"enabled": cfg.Encryption.Enabled,
	})
	v.Set("sync", map[string]interface{}{
		"snapshot_limit":     cfg.Sync.SnapshotLimit,
		"conflict_strategy":  cfg.Sync.ConflictStrategy,
		"merge_retry_max":    cfg.Sync.MergeRetryMax,
		"auto_sync_interval": cfg.Sync.AutoSyncInterval,
	})
	v.Set("device", map[string]interface{}{
		"id":   cfg.Device.ID,
		"name": cfg.Device.Name,
	})
	v.Set("claude", map[string]interface{}{
		"path":      cfg.Claude.Path,
		"json_path": cfg.Claude.JSONPath,
	})
	v.Set("binary", map[string]interface{}{
		"encrypt":             cfg.Binary.Encrypt,
		"chunk_mode":          cfg.Binary.ChunkMode,
		"chunk_size_mb":       cfg.Binary.ChunkSizeMB,
		"chunk_threshold_mb":  cfg.Binary.ChunkThresholdMB,
		"sync_enabled":        cfg.Binary.SyncEnabled,
		"auto_configure_path": cfg.Binary.AutoConfigurePath,
		"bin_dir":             cfg.Binary.BinDir,
		"versions_dir":        cfg.Binary.VersionsDir,
		"claude_path":         cfg.Binary.ClaudePath,
	})
	v.Set("exclude", map[string]interface{}{
		"patterns": cfg.Exclude.Patterns,
	})

	configPath := filepath.Join(dir, "config.toml")
	return v.WriteConfigAs(configPath)
}

// InitCCBoxDir 创建 ~/.cc-box/ 目录结构
func InitCCBoxDir() error {
	dir := CCBoxDir()
	dirs := []string{
		dir,
		filepath.Join(dir, "cache"),
		filepath.Join(dir, "cache", "objects"),
		filepath.Join(dir, "snapshots"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", d, err)
		}
	}
	return nil
}

// IsInitialized 检查是否已完成初始化
func IsInitialized() bool {
	configPath := filepath.Join(CCBoxDir(), "config.toml")
	_, err := os.Stat(configPath)
	return err == nil
}

// LoadWebDAVPassword 从环境变量或共享密码存储读取 WebDAV 密码
func LoadWebDAVPassword() (string, error) {
	if pass := os.Getenv("CC_BOX_WEBDAV_PASSWORD"); pass != "" {
		return pass, nil
	}

	pass, err := keyringGet("cc-box", "webdav-password")
	if err == nil {
		return pass, nil
	}

	return "", fmt.Errorf("未找到 WebDAV 密码，请设置环境变量 CC_BOX_WEBDAV_PASSWORD 或运行 cc-box config webdav")
}

// SaveWebDAVPassword 保存 WebDAV 密码到共享密码存储
func SaveWebDAVPassword(password string) error {
	return keyringSet("cc-box", "webdav-password", password)
}
