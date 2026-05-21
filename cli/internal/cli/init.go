// init 命令
// 交互式初始化向导：WebDAV 配置 + 加密设置 + 首次快照
package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/cc-box/cli/internal/config"
	"github.com/user/cc-box/cli/internal/crypto"
	"github.com/user/cc-box/cli/internal/object"
	"github.com/user/cc-box/cli/internal/snapshot"
	"github.com/user/cc-box/cli/internal/webdav"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "初始化 CC-Box（交互式向导）",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== CC-Box 初始化向导 ===")
	fmt.Println()

	// 检查是否已初始化
	if config.IsInitialized() {
		fmt.Print("已检测到现有配置。是否覆盖重新初始化？[y/N] ")
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			return fmt.Errorf("取消初始化")
		}
	}

	// 1. WebDAV 配置
	fmt.Println("--- WebDAV 配置 ---")
	fmt.Print("WebDAV URL（如 https://dav.jianguoyun.com/dav/）: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)
	if url == "" {
		return fmt.Errorf("URL 不能为空")
	}

	fmt.Print("用户名: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	fmt.Print("密码（输入后回车）: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)
	if password == "" {
		return fmt.Errorf("密码不能为空")
	}

	// 测试连接
	fmt.Print("测试连接... ")
	client := webdav.NewClient(url, username, password)
	client.SetTimeout(15e9) // 15 秒
	if _, err := client.Exists("/"); err != nil {
		return fmt.Errorf("连接失败: %w\n请检查 URL、用户名和密码", err)
	}
	fmt.Println("成功")

	// 2. 加密设置
	fmt.Println()
	fmt.Println("--- 加密设置 ---")
	fmt.Print("加密密码（用于端到端加密）: ")
	encPassword, _ := reader.ReadString('\n')
	encPassword = strings.TrimSpace(encPassword)
	if encPassword == "" {
		return fmt.Errorf("加密密码不能为空")
	}
	fmt.Print("确认加密密码: ")
	encPassword2, _ := reader.ReadString('\n')
	if encPassword != strings.TrimSpace(encPassword2) {
		return fmt.Errorf("两次密码不一致")
	}

	// 3. 设备配置
	fmt.Println()
	fmt.Println("--- 设备配置 ---")
	cfg := config.DefaultConfig()
	cfg.WebDAV.URL = url
	cfg.WebDAV.Username = username
	cfg.WebDAV.Root = "/cc-box/"
	fmt.Printf("设备 ID: %s\n", cfg.Device.ID)
	fmt.Printf("设备名称 [%s]: ", cfg.Device.Name)
	name, _ := reader.ReadString('\n')
	if strings.TrimSpace(name) != "" {
		cfg.Device.Name = strings.TrimSpace(name)
	}

	// 4. 创建目录结构
	fmt.Println()
	fmt.Print("创建本地目录... ")
	if err := config.InitCCBoxDir(); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	fmt.Println("完成")

	// 5. 加密初始化
	fmt.Print("初始化加密... ")
	salt, err := crypto.GenerateSalt()
	if err != nil {
		return fmt.Errorf("生成 salt 失败: %w", err)
	}
	key := crypto.DeriveKey(encPassword, salt)

	// 检查远程是否已有数据
	saltExists, _ := client.Exists("salt.bin")
	if saltExists {
		// 远程已有 salt，验证密码
		fmt.Println()
		fmt.Println("检测到远程已有数据，验证密码...")
		remoteSalt, _, err := client.GET("salt.bin")
		if err != nil {
			return fmt.Errorf("下载远程 salt 失败: %w", err)
		}
		remoteKey := crypto.DeriveKey(encPassword, remoteSalt)

		// 尝试解密 HEAD 指向的快照验证
		headData, _, err := client.GET("HEAD")
		if err == nil && string(headData) != "" {
			snapPath := "snapshots/" + string(headData) + ".json.enc"
			encData, _, err := client.GET(snapPath)
			if err == nil {
				_, err = crypto.Decrypt(encData, remoteKey)
				if err != nil {
					return fmt.Errorf("密码验证失败：与远程数据不匹配")
				}
				fmt.Println("密码验证通过")
			}
		}
		salt = remoteSalt
		key = remoteKey
	} else {
		// 新初始化，上传 salt
		if err := client.EnsureDir("salt.bin"); err != nil {
			return fmt.Errorf("创建远程目录失败: %w", err)
		}
		if _, err := client.PUT("salt.bin", salt, ""); err != nil {
			return fmt.Errorf("上传 salt 失败: %w", err)
		}
		fmt.Println("完成")
	}

	// 保存密钥
	if err := crypto.SaveKey(key, config.KeyPath()); err != nil {
		return fmt.Errorf("保存密钥失败: %w", err)
	}

	// 6. 扫描并创建首次快照
	fmt.Print("扫描配置文件... ")
	scanner := snapshot.NewScanner(config.ClaudeDir(), cfg.Exclude.Patterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		return fmt.Errorf("扫描失败: %w", err)
	}
	fmt.Printf("发现 %d 个文件\n", scanResult.Stats.TotalFiles)

	// 上传文件 objects
	fmt.Println("上传文件...")
	store := object.NewStore(client, key, "")
	uploaded := 0
	for path, entry := range scanResult.Files {
		// 读取文件内容
		fullPath := config.ClaudeDir() + "/" + path
		data, err := os.ReadFile(fullPath)
		if err != nil {
			fmt.Printf("  跳过 %s: %v\n", path, err)
			continue
		}

		// 规范化后计算哈希
		normData := normalizeContent(data)
		hash := object.ComputeHash(normData)
		entry.Hash = hash

		if _, err := store.Upload(normData); err != nil {
			fmt.Printf("  上传失败 %s: %v\n", path, err)
			continue
		}
		scanResult.Files[path] = entry
		uploaded++
		if uploaded%10 == 0 {
			fmt.Printf("  已上传 %d/%d 个文件\n", uploaded, scanResult.Stats.TotalFiles)
		}
	}

	// 创建首次快照
	snap := snapshot.CreateSnapshot("", cfg.Device.ID, "initial sync", scanResult.Files)
	if err := uploadSnapshot(client, store, snap); err != nil {
		return fmt.Errorf("上传快照失败: %w", err)
	}

	// 更新远程 HEAD
	if err := updateRemoteHEAD(client, snap.ID, ""); err != nil {
		return fmt.Errorf("更新远程 HEAD 失败: %w", err)
	}

	// 更新本地 HEAD
	if err := updateLocalHEAD(snap.ID); err != nil {
		return fmt.Errorf("更新本地 HEAD 失败: %w", err)
	}

	// 保存配置
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 保存 WebDAV 密码
	if err := config.SaveWebDAVPassword(password); err != nil {
		fmt.Printf("警告：保存 WebDAV 密码失败: %v\n", err)
		fmt.Println("可以设置环境变量 CC_BOX_WEBDAV_PASSWORD")
	}

	// 保存快照到本地缓存
	saveLocalSnapshot(snap)

	fmt.Println()
	fmt.Printf("=== 初始化完成 ===\n")
	fmt.Printf("快照 ID: %s\n", snap.ID)
	fmt.Printf("已同步 %d 个文件\n", uploaded)
	fmt.Printf("设备: %s (%s)\n", cfg.Device.Name, cfg.Device.ID)
	fmt.Println()
	fmt.Println("使用 'cc-box push' 推送变更")
	fmt.Println("使用 'cc-box pull' 拉取远程变更")
	fmt.Println("使用 'cc-box status' 查看状态")

	return nil
}

// normalizeContent 规范化文件内容
func normalizeContent(data []byte) []byte {
	// 简单的 CRLF -> LF 转换
	result := make([]byte, 0, len(data))
	for i := 0; i < len(data); i++ {
		if data[i] == '\r' {
			if i+1 < len(data) && data[i+1] == '\n' {
				result = append(result, '\n')
				i++
			} else {
				result = append(result, '\n')
			}
		} else {
			result = append(result, data[i])
		}
	}
	return result
}

// uploadSnapshot 加密并上传快照到 WebDAV
func uploadSnapshot(client *webdav.Client, store *object.Store, snap *snapshot.Snapshot) error {
	data, err := snap.Serialize()
	if err != nil {
		return fmt.Errorf("序列化快照失败: %w", err)
	}

	encrypted, err := crypto.Encrypt(data, store.Key())
	if err != nil {
		return fmt.Errorf("加密快照失败: %w", err)
	}

	path := "snapshots/" + snap.ID + ".json.enc"
	if err := client.EnsureDir(path); err != nil {
		return err
	}

	_, err = client.PUT(path, encrypted, "")
	return err
}

// updateRemoteHEAD 乐观锁更新远程 HEAD
func updateRemoteHEAD(client *webdav.Client, newID string, expectedETag string) error {
	if err := client.EnsureDir("HEAD"); err != nil {
		return err
	}
	_, err := client.PUT("HEAD", []byte(newID), expectedETag)
	return err
}

// updateLocalHEAD 更新本地 HEAD
func updateLocalHEAD(id string) error {
	dir := config.CCBoxDir()
	return os.WriteFile(dir+"/HEAD", []byte(id), 0600)
}

// saveLocalSnapshot 保存快照到本地缓存
func saveLocalSnapshot(snap *snapshot.Snapshot) {
	dir := config.CCBoxDir()
	data, err := snap.Serialize()
	if err != nil {
		return
	}
	os.WriteFile(dir+"/snapshots/"+snap.ID+".json", data, 0600)
}

// loadLocalHEAD 读取本地 HEAD
func loadLocalHEAD() (string, error) {
	dir := config.CCBoxDir()
	data, err := os.ReadFile(dir + "/HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// loadLocalSnapshot 从本地缓存加载快照
func loadLocalSnapshot(id string) (*snapshot.Snapshot, error) {
	dir := config.CCBoxDir()
	data, err := os.ReadFile(dir + "/snapshots/" + id + ".json")
	if err != nil {
		return nil, err
	}
	return snapshot.Deserialize(data)
}
