// device 子命令
// 设备管理：list / rename / forget
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/webdav"
)

var deviceCmd = &cobra.Command{
	Use:   "device",
	Short: "设备管理",
}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出已注册设备",
	RunE:  runDeviceList,
}

var deviceRenameCmd = &cobra.Command{
	Use:   "rename <name>",
	Short: "重命名当前设备",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeviceRename,
}

var deviceForgetCmd = &cobra.Command{
	Use:   "forget <device-id>",
	Short: "移除设备注册信息",
	Args:  cobra.ExactArgs(1),
	RunE:  runDeviceForget,
}

func init() {
	rootCmd.AddCommand(deviceCmd)
	deviceCmd.AddCommand(deviceListCmd)
	deviceCmd.AddCommand(deviceRenameCmd)
	deviceCmd.AddCommand(deviceForgetCmd)
}

// DeviceInfo 设备注册信息
type DeviceInfo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
	LastSeen time.Time `json:"last_seen"`
	HEAD     string    `json:"head"`
}

func runDeviceList(cmd *cobra.Command, args []string) error {
	_, client, _, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 列出 devices/ 目录
	files, err := client.PROPFIND("devices/", 1)
	if err != nil {
		if err.Error() != "" && strings.Contains(err.Error(), "404") {
			fmt.Println("没有已注册的设备")
			return nil
		}
		return fmt.Errorf("列出设备失败: %w", err)
	}

	var devices []DeviceInfo
	for _, f := range files {
		if f.IsDir || !strings.HasSuffix(f.Path, ".json") {
			continue
		}

		// f.Path 可能包含完整路径前缀，提取文件名
		fileName := f.Path
		if idx := strings.LastIndex(fileName, "/"); idx >= 0 {
			fileName = fileName[idx+1:]
		}

		data, _, err := client.GET("devices/" + fileName)
		if err != nil {
			continue
		}

		var info DeviceInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		devices = append(devices, info)
	}

	if len(devices) == 0 {
		fmt.Println("没有已注册的设备")
		return nil
	}

	fmt.Printf("%-20s %-15s %-15s %s\n", "设备 ID", "名称", "平台", "最后活跃")
	fmt.Println(strings.Repeat("-", 70))
	for _, d := range devices {
		ago := formatTimeAgo(d.LastSeen)
		fmt.Printf("%-20s %-15s %-15s %s\n", d.ID, d.Name, d.Platform, ago)
	}

	return nil
}

func runDeviceRename(cmd *cobra.Command, args []string) error {
	newName := args[0]

	cfg, client, _, err := loadClientAndKey()
	if err != nil {
		return err
	}

	oldName := cfg.Device.Name
	cfg.Device.Name = newName

	// 更新远程设备注册信息
	info := DeviceInfo{
		ID:       cfg.Device.ID,
		Name:     newName,
		Platform: config.Platform(),
		LastSeen: time.Now().UTC(),
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	devicePath := "devices/" + cfg.Device.ID + ".json"
	if err := client.EnsureDir(devicePath); err != nil {
		return err
	}
	if _, err := client.PUT(devicePath, data, ""); err != nil {
		return fmt.Errorf("更新远程设备信息失败: %w", err)
	}

	// 保存本地配置
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	fmt.Printf("设备已重命名: %s → %s\n", oldName, newName)
	return nil
}

func runDeviceForget(cmd *cobra.Command, args []string) error {
	deviceID := args[0]

	cfg, client, _, err := loadClientAndKey()
	if err != nil {
		return err
	}

	if deviceID == cfg.Device.ID {
		return fmt.Errorf("不能移除当前设备，请使用其他设备执行此操作")
	}

	devicePath := "devices/" + deviceID + ".json"
	if err := client.DELETE(devicePath); err != nil {
		return fmt.Errorf("移除设备失败: %w", err)
	}

	fmt.Printf("已移除设备 %s\n", deviceID)
	return nil
}

// registerDevice 注册/更新当前设备信息到 WebDAV
func registerDevice(client *webdav.Client, cfg *config.Config) error {
	info := DeviceInfo{
		ID:       cfg.Device.ID,
		Name:     cfg.Device.Name,
		Platform: config.Platform(),
		LastSeen: time.Now().UTC(),
	}

	// 读取本地 HEAD 作为设备 HEAD
	head, err := loadLocalHEAD()
	if err == nil {
		info.HEAD = head
	}

	data, _ := json.MarshalIndent(info, "", "  ")
	devicePath := "devices/" + cfg.Device.ID + ".json"
	if err := client.EnsureDir(devicePath); err != nil {
		return err
	}
	_, err = client.PUT(devicePath, data, "")
	return err
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "未知"
	}
	d := time.Since(t)
	if d < time.Minute {
		return "刚刚"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
	return t.Format("2006-01-02")
}
