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
	SnapshotLimit   int    `mapstructure:"snapshot_limit"`
	ConflictStrategy string `mapstructure:"conflict_strategy"`
	MergeRetryMax   int    `mapstructure:"merge_retry_max"`
}

type DeviceConfig struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
}

type ClaudeConfig struct {
	Path string `mapstructure:"path"`
}

type BinaryConfig struct {
	ChunkSizeMB int  `mapstructure:"chunk_size_mb"`
	AutoUpload  bool `mapstructure:"auto_upload"`
}

type ExcludeConfig struct {
	Patterns []string `mapstructure:"patterns"`
}

// DefaultExcludePatterns 默认排除规则
var DefaultExcludePatterns = []string{
	"sessions/",
	"cache/",
	"debug/",
	"telemetry/",
	"downloads/",
	"paste-cache/",
	"shell-snapshots/",
	"file-history/",
	"session-env/",
	"ide/",
	"backups/",
	"plans/",
	"tasks/",
	"teams/",
	"plugins/data/",
	"*.lock",
}

// DefaultConfig 返回带默认值的配置
func DefaultConfig() *Config {
	return &Config{
		WebDAV: WebDAVConfig{
			Root: "/cc-box/",
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
			ChunkSizeMB: 10,
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

// ClaudeDir 返回 ~/.claude/ 路径
func ClaudeDir() string {
	if custom := viper.GetString("claude.path"); custom != "" {
		return custom
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// LocalBinDir 返回 ~/.local/bin/ 路径
func LocalBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "bin")
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

	v.Set("webdav", cfg.WebDAV)
	v.Set("encryption", cfg.Encryption)
	v.Set("sync", cfg.Sync)
	v.Set("device", cfg.Device)
	v.Set("claude", cfg.Claude)
	v.Set("binary", cfg.Binary)
	v.Set("exclude", cfg.Exclude)

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

// LoadWebDAVPassword 从密钥环或环境变量读取 WebDAV 密码
func LoadWebDAVPassword() (string, error) {
	// 优先环境变量
	if pass := os.Getenv("CC_BOX_WEBDAV_PASSWORD"); pass != "" {
		return pass, nil
	}

	// 尝试系统密钥环
	pass, err := keyringGet("cc-box", "webdav-password")
	if err == nil {
		return pass, nil
	}

	return "", fmt.Errorf("未找到 WebDAV 密码，请设置环境变量 CC_BOX_WEBDAV_PASSWORD 或运行 cc-box config webdav")
}

// SaveWebDAVPassword 保存密码到密钥环
func SaveWebDAVPassword(password string) error {
	return keyringSet("cc-box", "webdav-password", password)
}
