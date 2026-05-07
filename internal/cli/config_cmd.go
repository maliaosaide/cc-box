// config 子命令
// 配置管理 + 密钥轮转 (rekey)
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/webdav"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "查看配置项",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "修改配置项",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

var configRekeyCmd = &cobra.Command{
	Use:   "rekey",
	Short: "密钥轮转（更改加密密码）",
	RunE:  runConfigRekey,
}

var configWebdavCmd = &cobra.Command{
	Use:   "webdav",
	Short: "重新配置 WebDAV 连接",
	RunE:  runConfigWebdav,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configRekeyCmd)
	configCmd.AddCommand(configWebdavCmd)
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	val := getConfigValue(cfg, key)
	if val == "" {
		return fmt.Errorf("配置项 %s 不存在", key)
	}
	fmt.Printf("%s = %s\n", key, val)
	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := setConfigValue(cfg, key, value); err != nil {
		return err
	}

	return config.Save(cfg)
}

func runConfigRekey(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	cfg, client, oldKey, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 验证当前密码
	fmt.Print("输入当前加密密码: ")
	oldPassword, _ := reader.ReadString('\n')
	oldPassword = strings.TrimSpace(oldPassword)

	// 下载 salt 并派生旧密钥验证
	salt, _, err := client.GET("salt.bin")
	if err != nil {
		return fmt.Errorf("下载 salt 失败: %w", err)
	}

	derivedKey := crypto.DeriveKey(oldPassword, salt)
	// 简化验证：检查派生的密钥是否与本地一致
	if string(derivedKey) != string(oldKey) {
		return fmt.Errorf("密码不正确")
	}

	// 输入新密码
	fmt.Print("输入新加密密码: ")
	newPassword, _ := reader.ReadString('\n')
	newPassword = strings.TrimSpace(newPassword)
	fmt.Print("确认新密码: ")
	newPassword2, _ := reader.ReadString('\n')
	if newPassword != strings.TrimSpace(newPassword2) {
		return fmt.Errorf("两次密码不一致")
	}

	// 生成新 salt + 新密钥
	newSalt, err := crypto.GenerateSalt()
	if err != nil {
		return err
	}
	newKey := crypto.DeriveKey(newPassword, newSalt)

	// 轮转所有 objects 和 snapshots
	fmt.Println("开始密钥轮转...")

	// 列出所有 snapshots
	files, err := client.PROPFIND("snapshots/", 1)
	if err != nil {
		return fmt.Errorf("列出快照失败: %w", err)
	}

	rotated := 0
	for _, f := range files {
		if f.IsDir || !strings.HasSuffix(f.Path, ".enc") {
			continue
		}

		// 下载 → 旧密钥解密 → 新密钥加密 → 上传
		encrypted, _, err := client.GET("snapshots/" + f.Path)
		if err != nil {
			fmt.Printf("  跳过 %s: %v\n", f.Path, err)
			continue
		}

		plaintext, err := crypto.Decrypt(encrypted, oldKey)
		if err != nil {
			fmt.Printf("  解密失败 %s: %v\n", f.Path, err)
			continue
		}

		newEncrypted, err := crypto.Encrypt(plaintext, newKey)
		if err != nil {
			fmt.Printf("  加密失败 %s: %v\n", f.Path, err)
			continue
		}

		if _, err := client.PUT("snapshots/"+f.Path, newEncrypted, ""); err != nil {
			fmt.Printf("  上传失败 %s: %v\n", f.Path, err)
			continue
		}
		rotated++
	}

	// 轮转 objects
	objFiles, _ := client.PROPFIND("objects/", 1)
	for _, f := range objFiles {
		if f.IsDir || !strings.HasSuffix(f.Path, ".enc") {
			continue
		}

		encrypted, _, err := client.GET("objects/" + f.Path)
		if err != nil {
			continue
		}

		plaintext, err := crypto.Decrypt(encrypted, oldKey)
		if err != nil {
			continue
		}

		newEncrypted, _ := crypto.Encrypt(plaintext, newKey)
		client.PUT("objects/"+f.Path, newEncrypted, "")
		rotated++
	}

	// 上传新 salt
	client.PUT("salt.bin", newSalt, "")

	// 更新本地密钥
	crypto.SaveKey(newKey, config.KeyPath())

	fmt.Printf("密钥轮转完成，已处理 %d 个文件\n", rotated)
	_ = cfg
	return nil
}

func runConfigWebdav(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("当前 URL: %s\n", cfg.WebDAV.URL)
	fmt.Print("新 URL（留空保持）: ")
	url, _ := reader.ReadString('\n')
	if strings.TrimSpace(url) != "" {
		cfg.WebDAV.URL = strings.TrimSpace(url)
	}

	fmt.Printf("当前用户名: %s\n", cfg.WebDAV.Username)
	fmt.Print("新用户名（留空保持）: ")
	username, _ := reader.ReadString('\n')
	if strings.TrimSpace(username) != "" {
		cfg.WebDAV.Username = strings.TrimSpace(username)
	}

	fmt.Print("新密码（留空保持）: ")
	password, _ := reader.ReadString('\n')
	if strings.TrimSpace(password) != "" {
		config.SaveWebDAVPassword(strings.TrimSpace(password))
	}

	// 测试连接
	fmt.Print("测试连接... ")
	client := webdav.NewClient(cfg.WebDAV.URL, cfg.WebDAV.Username, "")
	if _, err := client.Exists("/"); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	fmt.Println("成功")

	return config.Save(cfg)
}

func getConfigValue(cfg *config.Config, key string) string {
	switch key {
	case "webdav.url":
		return cfg.WebDAV.URL
	case "webdav.username":
		return cfg.WebDAV.Username
	case "device.id":
		return cfg.Device.ID
	case "device.name":
		return cfg.Device.Name
	case "sync.conflict_strategy":
		return cfg.Sync.ConflictStrategy
	default:
		return ""
	}
}

func setConfigValue(cfg *config.Config, key, value string) error {
	switch key {
	case "device.name":
		cfg.Device.Name = value
	case "sync.conflict_strategy":
		cfg.Sync.ConflictStrategy = value
	default:
		return fmt.Errorf("不支持修改 %s", key)
	}
	return nil
}
