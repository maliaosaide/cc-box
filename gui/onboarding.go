// Onboarding 后端绑定
// 引导流程：WebDAV 连接测试、新建设备、加入已有同步组
package main

import (
	"fmt"
	"strings"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/webdav"
)

// TestWebDAVConnection 测试 WebDAV 连接
func (a *App) TestWebDAVConnection(url, username, password, root string) error {
	fullURL := buildWebDAVURL(url, root)
	client := webdav.NewClient(fullURL, username, password)
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
	exists, err := client.Exists("salt.bin")
	if err != nil {
		return false, err
	}
	return exists, nil
}

// InitNewDevice 新建设备初始化
func (a *App) InitNewDevice(url, username, password, root, encPassword, deviceName string) error {
	cfg := config.DefaultConfig()
	cfg.WebDAV = config.WebDAVConfig{
		URL:      strings.TrimRight(url, "/") + "/",
		Username: username,
		Root:     root,
	}
	if deviceName != "" {
		cfg.Device.Name = deviceName
	}

	if err := config.InitCCBoxDir(); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	if err := config.SaveWebDAVPassword(password); err != nil {
		return fmt.Errorf("保存密码失败: %w", err)
	}

	// TODO: 生成加密密钥、创建初始快照、上传到 WebDAV
	return nil
}

// InitJoinExisting 加入已有同步组
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

	if err := config.InitCCBoxDir(); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	if err := config.SaveWebDAVPassword(password); err != nil {
		return fmt.Errorf("保存密码失败: %w", err)
	}

	// TODO: 从 WebDAV 拉取 salt.bin、验证密码、拉取最新快照
	return nil
}

// buildWebDAVURL 拼接完整的 WebDAV 地址（URL + root path）
func buildWebDAVURL(url, root string) string {
	base := strings.TrimRight(url, "/")
	root = strings.TrimLeft(root, "/")
	if root != "" {
		base += "/" + root
	}
	return base + "/"
}
