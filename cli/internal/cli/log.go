// log / show 命令
// 查看快照历史和详情
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/core/config"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/webdav"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "查看快照历史",
	RunE:  runLog,
}

var showCmd = &cobra.Command{
	Use:   "show <snapshot-id>",
	Short: "查看指定快照详情",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(logCmd)
	rootCmd.AddCommand(showCmd)
	logCmd.Flags().Bool("oneline", false, "简洁模式")
	logCmd.Flags().IntP("number", "n", 10, "显示条数")
}

func runLog(cmd *cobra.Command, args []string) error {
	oneline, _ := cmd.Flags().GetBool("oneline")
	n, _ := cmd.Flags().GetInt("number")

	cfg, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	// 读取远程 HEAD
	headData, _, err := client.GET("HEAD")
	if err != nil {
		return fmt.Errorf("读取远程 HEAD 失败: %w", err)
	}
	currentID := string(headData)

	// 沿 parent 链回溯
	snapID := currentID
	for i := 0; i < n && snapID != ""; i++ {
		snap, err := loadRemoteSnapshot(client, key, snapID)
		if err != nil {
			fmt.Printf("%s  (无法加载)\n", snapID)
			break
		}

		shortID := snapshotShortID(snapID)
		if oneline {
			fmt.Printf("%s %s %s\n", shortID, snap.Timestamp.Format("2006-01-02 15:04"), snap.Message)
		} else {
			fmt.Printf("%s  %s  %-15s  %s\n",
				shortID,
				snap.Timestamp.Format("2006-01-02 15:04"),
				truncate(snap.Device, 15),
				snap.Message,
			)
		}

		snapID = snap.Parent
	}

	_ = cfg
	return nil
}

func runShow(cmd *cobra.Command, args []string) error {
	snapID := args[0]

	_, client, key, err := loadClientAndKey()
	if err != nil {
		return err
	}

	snap, err := loadRemoteSnapshot(client, key, snapID)
	if err != nil {
		return fmt.Errorf("加载快照失败: %w", err)
	}

	fmt.Printf("快照:  %s\n", snap.ID)
	fmt.Printf("时间:  %s\n", snap.Timestamp.Format(time.RFC3339))
	fmt.Printf("设备:  %s\n", snap.Device)
	fmt.Printf("信息:  %s\n", snap.Message)
	fmt.Printf("父级:  %s\n", snap.Parent)
	fmt.Printf("文件数: %d\n", len(snap.Files))

	if len(snap.Binary) > 0 {
		fmt.Println()
		fmt.Println("二进制版本:")
		for platform, bins := range snap.Binary {
			for name, version := range bins {
				fmt.Printf("  %s/%s: %s\n", platform, name, version)
			}
		}
	}

	if len(snap.Files) > 0 {
		fmt.Println()
		fmt.Println("文件列表:")
		for path, entry := range snap.Files {
			fmt.Printf("  %-40s  %s  %s\n", path, formatSize(entry.Size), entry.Modified.Format("2006-01-02 15:04"))
		}
	}

	return nil
}

func formatSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%dB", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(n)/(1024*1024))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}

// loadClientAndKey 加载配置、创建客户端、加载密钥
func loadClientAndKey() (*config.Config, *webdav.Client, []byte, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("请先运行 cc-box init")
	}

	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("加载密钥失败")
	}

	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return nil, nil, nil, err
	}

	client := webdav.NewClient(config.ConfiguredWebDAVURL(cfg), cfg.WebDAV.Username, pass)
	return cfg, client, key, nil
}

// loadLocalSnapID 加载本地 HEAD
func loadLocalSnapID() (string, error) {
	headData, err := readLocalHEAD()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(headData)), nil
}

func readLocalHEAD() ([]byte, error) {
	dir := config.CCBoxDir()
	return readFile(dir + "/HEAD")
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
