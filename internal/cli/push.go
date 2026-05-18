// push 命令
// 扫描变更 → 加密 → 上传 → 乐观锁更新 HEAD
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/normalize"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "推送配置变更到云端",
	RunE:  runPush,
}

func init() {
	rootCmd.AddCommand(pushCmd)
	pushCmd.Flags().StringP("message", "m", "auto sync", "提交信息")
	pushCmd.Flags().Bool("dry-run", false, "仅显示将要推送的变更，不实际上传")
}

func runPush(cmd *cobra.Command, args []string) error {
	msg, _ := cmd.Flags().GetString("message")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("请先运行 cc-box init")
	}

	key, err := crypto.LoadKey(config.KeyPath())
	if err != nil {
		return fmt.Errorf("加载密钥失败，请重新运行 cc-box init")
	}

	pass, err := config.LoadWebDAVPassword()
	if err != nil {
		return err
	}
	client := webdav.NewClient(config.ConfiguredWebDAVURL(cfg), cfg.WebDAV.Username, pass)

	localHead, err := loadLocalHEAD()
	if err != nil {
		return fmt.Errorf("读取本地 HEAD 失败: %w", err)
	}

	var localSnap *snapshot.Snapshot
	if localHead != "" {
		localSnap, err = loadRemoteSnapshot(client, key, localHead)
		if err != nil {
			return fmt.Errorf("加载快照失败: %w", err)
		}
	}

	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}

	var changes []snapshot.Change
	if localSnap != nil {
		currentSnap := snapshot.CreateSnapshot("", cfg.Device.ID, "", scanResult.Files)
		changes = localSnap.Diff(currentSnap)
	} else {
		for path, entry := range scanResult.Files {
			changes = append(changes, snapshot.Change{
				Path:    path,
				Type:    snapshot.Added,
				NewHash: entry.Hash,
				NewSize: entry.Size,
			})
		}
	}

	if len(changes) == 0 {
		fmt.Println("没有变更需要推送")
		return nil
	}

	fmt.Printf("发现 %d 个变更:\n", len(changes))
	for _, c := range changes {
		switch c.Type {
		case snapshot.Added:
			fmt.Printf("  A  %s\n", c.Path)
		case snapshot.Modified:
			fmt.Printf("  M  %s\n", c.Path)
		case snapshot.Deleted:
			fmt.Printf("  D  %s\n", c.Path)
		}
	}

	if dryRun {
		fmt.Println("\n(dry-run 模式，未实际上传)")
		return nil
	}

	store := object.NewStore(client, key, "")
	// 利用本地快照的哈希跳过已上传的 object（省去 Exists 请求）
	if localSnap != nil {
		knownHashes := make(map[string]bool)
		for _, entry := range localSnap.Files {
			knownHashes[entry.Hash] = true
		}
		store.SetKnownHashes(knownHashes)
	}
	uploaded := 0
	for _, c := range changes {
		if c.Type == snapshot.Deleted {
			continue
		}

		fullPath, err := safeClaudePath(c.Path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("读取文件 %s 失败: %w", c.Path, err)
		}

		normData := normalize.HashContent(data)
		hash, err := store.Upload(normData)
		if err != nil {
			return fmt.Errorf("上传文件 %s 失败: %w", c.Path, err)
		}
		if hash != c.NewHash {
			return fmt.Errorf("文件 %s hash 不一致", c.Path)
		}
		uploaded++
	}
	fmt.Printf("已上传 %d 个文件\n", uploaded)

	newSnap := snapshot.CreateSnapshot(localHead, cfg.Device.ID, msg, scanResult.Files)

	if err := uploadSnapshot(client, store, newSnap); err != nil {
		return fmt.Errorf("上传快照失败: %w", err)
	}

	if err := pushUpdateHEAD(client, cfg, newSnap.ID, localHead); err != nil {
		return err
	}

	if err := updateLocalHEAD(newSnap.ID); err != nil {
		return err
	}

	saveLocalSnapshot(newSnap)

	// 注册/更新设备信息
	if err := registerDevice(client, cfg); err != nil {
		fmt.Printf("警告：更新设备信息失败: %v\n", err)
	}

	fmt.Printf("已推送快照 %s（%d 个变更）\n", newSnap.ID, len(changes))
	return nil
}

// pushUpdateHEAD 带重试的乐观锁更新远程 HEAD
func pushUpdateHEAD(client *webdav.Client, cfg *config.Config, newID, expectedHead string) error {
	for attempt := 0; attempt < cfg.Sync.MergeRetryMax; attempt++ {
		currentData, currentETag, err := client.GET("HEAD")
		if err == webdav.ErrNotFound {
			if expectedHead != "" {
				return fmt.Errorf("远程 HEAD 不存在，请先 pull")
			}
		} else if err != nil {
			return fmt.Errorf("读取远程 HEAD 失败: %w", err)
		}

		currentHead := strings.TrimSpace(string(currentData))
		if currentHead != expectedHead {
			return fmt.Errorf("远程 HEAD 已更新为 %s，请先 pull", headDisplay(currentHead))
		}
		if currentHead != "" && currentETag == "" {
			return fmt.Errorf("远程服务未返回 HEAD ETag，无法安全更新")
		}

		result, err := client.CompareAndSwapHEAD("HEAD", newID, currentETag)
		if err != nil {
			return fmt.Errorf("更新 HEAD 失败: %w", err)
		}

		if result.Success {
			return nil
		}

		fmt.Printf("远程 HEAD 已被更新（冲突），重试 %d/%d...\n", attempt+1, cfg.Sync.MergeRetryMax)
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}

	return fmt.Errorf("更新 HEAD 失败：重试 %d 次后仍有冲突，请先 pull", cfg.Sync.MergeRetryMax)
}

// loadRemoteSnapshot 从 WebDAV 下载并解密快照
func loadRemoteSnapshot(client *webdav.Client, key []byte, id string) (*snapshot.Snapshot, error) {
	if snap, err := loadLocalSnapshot(id); err == nil {
		return snap, nil
	}

	snapPath := "snapshots/" + id + ".json.enc"
	encrypted, _, err := client.GET(snapPath)
	if err != nil {
		return nil, fmt.Errorf("下载快照 %s 失败: %w", id, err)
	}

	data, err := crypto.Decrypt(encrypted, key)
	if err != nil {
		return nil, fmt.Errorf("解密快照失败: %w", err)
	}

	return snapshot.Deserialize(data)
}
