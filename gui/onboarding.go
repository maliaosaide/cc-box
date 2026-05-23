// Onboarding 后端绑定
// 引导流程：WebDAV 连接测试、新建设备、加入已有同步组
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/webdav"
)

// TestWebDAVConnection 测试 WebDAV 连接
func (a *App) TestWebDAVConnection(url, username, password, root string) error {
	fullURL := buildWebDAVURL(url, root)
	client := webdav.NewClient(fullURL, username, password)
	client.SetTimeout(8 * time.Second)
	_, err := client.PROPFIND("", 0)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	return nil
}

// DetectExistingSetup 检测云端是否已有初始化数据
func (a *App) DetectExistingSetup(url, username, password, root string) (bool, error) {
	fullURL := buildWebDAVURL(url, root)
	client := webdav.NewClient(fullURL, username, password)
	client.SetTimeout(8 * time.Second)
	exists, err := client.Exists("salt.bin")
	if err != nil {
		return false, err
	}
	return exists, nil
}

// InitNewDevice 新建设备初始化：生成密钥 + 创建初始快照 + 上传
func (a *App) InitNewDevice(url, username, password, root, encPassword, deviceName string) (err error) {
	cfg := config.DefaultConfig()
	cfg.WebDAV = config.WebDAVConfig{
		URL:      strings.TrimRight(url, "/") + "/",
		Username: username,
		Root:     root,
	}
	if deviceName != "" {
		cfg.Device.Name = deviceName
	}

	// 创建本地目录
	if err := config.InitCCBoxDir(); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 生成 salt 和密钥
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("生成 salt 失败: %w", err)
	}
	key := crypto.DeriveKey(encPassword, salt)

	// 构建 WebDAV 客户端
	fullURL := buildWebDAVURL(url, root)
	client := webdav.NewClient(fullURL, username, password)
	releaseInitLock, err := acquireRemoteInitLock(client, cfg.Device.ID)
	if err != nil {
		return err
	}
	defer releaseInitLock()
	createdRemote := []string{}
	cleanupRemote := true
	defer func() {
		if err != nil && cleanupRemote {
			cleanupRemoteFiles(client, createdRemote)
		}
	}()
	if exists, err := client.Exists("salt.bin"); err != nil {
		return fmt.Errorf("检查远程初始化状态失败: %w", err)
	} else if exists {
		return fmt.Errorf("远程已存在同步组，请选择加入已有同步组或更换根路径")
	}
	if exists, err := client.Exists("HEAD"); err != nil {
		return fmt.Errorf("检查远程 HEAD 失败: %w", err)
	} else if exists {
		return fmt.Errorf("远程已存在同步组，请选择加入已有同步组或更换根路径")
	}

	// 上传 salt
	if _, err := client.PUTIfAbsent("salt.bin", salt); err != nil {
		if err == webdav.ErrConflict {
			return fmt.Errorf("远程已存在同步组，请选择加入已有同步组或更换根路径")
		}
		return fmt.Errorf("上传 salt 失败: %w", err)
	}
	createdRemote = append(createdRemote, "salt.bin")

	// 扫描配置文件
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.ScanPartial()
	if err != nil {
		return fmt.Errorf("扫描配置文件失败: %w", err)
	}
	if err := requireCompleteScan(scanResult); err != nil {
		return err
	}

	// 上传文件 objects
	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	for path, entry := range scanResult.Files {
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		data, err := readObjectData(fullPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", path, err)
		}
		hash := object.ComputeHash(data)
		if hash != entry.Hash {
			return fmt.Errorf("文件 %s hash 不一致", path)
		}
		if _, err := store.Upload(data); err != nil {
			return fmt.Errorf("上传文件 %s 失败: %w", path, err)
		}
	}

	// 创建初始快照
	snap := snapshot.CreateSnapshot("", cfg.Device.ID, "initial sync", scanResult.Files)
	snap.Binary = currentBinaryVersions()
	snapData, err := snap.Serialize()
	if err != nil {
		return fmt.Errorf("serialize snapshot: %w", err)
	}
	encrypted, err := encryptRemoteData(snapData, key)
	if err != nil {
		return fmt.Errorf("encrypt snapshot: %w", err)
	}
	if err := client.EnsureDir("snapshots/"); err != nil {
		return fmt.Errorf("create snapshots dir: %w", err)
	}
	snapPath := "snapshots/" + snap.ID + ".json.enc"
	if _, err := client.PUT(snapPath, encrypted, ""); err != nil {
		return fmt.Errorf("upload snapshot: %w", err)
	}
	createdRemote = append(createdRemote, snapPath)
	if _, err := client.PUTIfAbsent("HEAD", []byte(snap.ID)); err != nil {
		if err == webdav.ErrConflict {
			return fmt.Errorf("远程已存在同步组，请选择加入已有同步组或更换根路径")
		}
		return fmt.Errorf("upload HEAD: %w", err)
	}
	createdRemote = append(createdRemote, "HEAD")
	cleanupRemote = false
	if err := os.WriteFile(config.CCBoxDir()+"/salt.bin", salt, 0600); err != nil {
		return fmt.Errorf("保存 salt 失败: %w", err)
	}
	if err := crypto.SaveKey(key, config.KeyPath()); err != nil {
		return fmt.Errorf("保存密钥失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(snap.ID), 0600); err != nil {
		return fmt.Errorf("保存本地 HEAD 失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+snap.ID+".json", snapData, 0600); err != nil {
		return fmt.Errorf("缓存快照失败: %w", err)
	}
	registerDeviceInfo(client, cfg)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	if err := config.SaveWebDAVPassword(password); err != nil {
		return fmt.Errorf("保存密码失败: %w", err)
	}

	return nil
}

// InitJoinExisting 加入已有同步组：验证密码 + 拉取最新快照
func (a *App) InitJoinExisting(url, username, password, root, encPassword, deviceName string) error {
	cfg := config.DefaultConfig()
	cfg.WebDAV = config.WebDAVConfig{
		URL:      strings.TrimRight(url, "/") + "/",
		Username: username,
		Root:     root,
	}
	if deviceName != "" {
		cfg.Device.Name = deviceName
	}

	// 创建本地目录
	if err := config.InitCCBoxDir(); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 连接远程，下载 salt
	fullURL := buildWebDAVURL(url, root)
	client := webdav.NewClient(fullURL, username, password)

	remoteSalt, _, err := client.GET("salt.bin")
	if err != nil {
		return fmt.Errorf("下载远程 salt 失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/salt.bin", remoteSalt, 0600); err != nil {
		return fmt.Errorf("保存 salt 失败: %w", err)
	}

	key := crypto.DeriveKey(encPassword, remoteSalt)

	// 验证密钥：尝试解密远程快照
	headData, _, err := client.GET("HEAD")
	if err != nil || string(headData) == "" {
		return fmt.Errorf("远程没有数据或无法读取")
	}
	remoteHead := strings.TrimSpace(string(headData))
	if err := validateSnapshotID(remoteHead); err != nil {
		return err
	}

	snapPath := "snapshots/" + remoteHead + ".json.enc"
	encData, _, err := client.GET(snapPath)
	if err != nil {
		return fmt.Errorf("下载快照失败: %w", err)
	}
	_, err = decryptRemoteData(encData, key)
	if err != nil {
		return fmt.Errorf("密码验证失败：与远程数据不匹配")
	}

	// 保存密钥
	if err := crypto.SaveKey(key, config.KeyPath()); err != nil {
		return fmt.Errorf("保存密钥失败: %w", err)
	}

	// 拉取最新快照的文件到本地
	decrypted, _ := decryptRemoteData(encData, key)
	remoteSnap, err := snapshot.Deserialize(decrypted)
	if err != nil {
		return fmt.Errorf("解析快照失败: %w", err)
	}

	store := object.NewStore(client, key, config.CCBoxDir()+"/cache/objects")
	for path, entry := range remoteSnap.Files {
		data, err := store.Download(entry.Hash)
		if err != nil {
			return fmt.Errorf("下载文件 %s 失败: %w", path, err)
		}
		fullPath, err := safeClaudePath(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		if err := os.WriteFile(fullPath, data, 0600); err != nil {
			return fmt.Errorf("写入文件 %s 失败: %w", path, err)
		}
	}

	// 更新本地 HEAD
	if err := os.WriteFile(config.CCBoxDir()+"/HEAD", []byte(remoteHead), 0600); err != nil {
		return fmt.Errorf("写入本地 HEAD 失败: %w", err)
	}
	if err := os.WriteFile(config.CCBoxDir()+"/snapshots/"+remoteHead+".json", decrypted, 0600); err != nil {
		return fmt.Errorf("缓存快照失败: %w", err)
	}

	// 注册设备
	registerDeviceInfo(client, cfg)

	// 保存配置和密码
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	if err := config.SaveWebDAVPassword(password); err != nil {
		return fmt.Errorf("保存密码失败: %w", err)
	}

	return nil
}

const remoteInitLockPath = ".init.lock"

func acquireRemoteInitLock(client *webdav.Client, deviceID string) (func(), error) {
	payload := []byte(fmt.Sprintf("%s\n%s\n", deviceID, time.Now().UTC().Format(time.RFC3339Nano)))
	if _, err := client.PUTIfAbsent(remoteInitLockPath, payload); err != nil {
		if err == webdav.ErrConflict {
			return nil, fmt.Errorf("远程同步组正在初始化，请稍后重试")
		}
		return nil, fmt.Errorf("创建远程初始化锁失败: %w", err)
	}
	return func() {
		if err := client.DELETE(remoteInitLockPath); err != nil && err != webdav.ErrNotFound {
			return
		}
	}, nil
}

func cleanupRemoteFiles(client *webdav.Client, paths []string) {
	for i := len(paths) - 1; i >= 0; i-- {
		_ = client.DELETE(paths[i])
	}
}

// registerDeviceInfo 注册/更新设备信息到 WebDAV
func registerDeviceInfo(client *webdav.Client, cfg *config.Config) {
	type deviceInfo struct {
		ID       string    `json:"id"`
		Name     string    `json:"name"`
		Platform string    `json:"platform"`
		LastSeen time.Time `json:"last_seen"`
	}
	info := deviceInfo{
		ID:       cfg.Device.ID,
		Name:     cfg.Device.Name,
		Platform: config.Platform(),
		LastSeen: time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	devicePath := "devices/" + cfg.Device.ID + ".json"
	client.EnsureDir("devices/")
	client.PUT(devicePath, data, "")
}

func newConfiguredWebDAVClient(cfg *config.Config, password string) *webdav.Client {
	return webdav.NewClient(config.ConfiguredWebDAVURL(cfg), cfg.WebDAV.Username, password)
}

// buildWebDAVURL 拼接完整的 WebDAV 地址（URL + root path）
func buildWebDAVURL(url, root string) string {
	return config.WebDAVBaseURL(url, root)
}
