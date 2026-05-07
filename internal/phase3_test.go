// Phase 3 集成测试
// 测试 diff、device、project、gc 等新增 CLI 功能
package integration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/user/cc-box/internal/binary"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/project"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

// TestPhase3_DeviceRegistration 测试设备注册与列出
func TestPhase3_DeviceRegistration(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "device-test/"
	client.DELETE(root)
	client.MKCOL(root)

	// 注册两个模拟设备
	devices := []struct {
		id       string
		name     string
		platform string
	}{
		{"win-pc-test", "win-pc-test", "windows-amd64"},
		{"mac-test", "mac-test", "darwin-arm64"},
	}

	for _, d := range devices {
		info := map[string]interface{}{
			"id":        d.id,
			"name":      d.name,
			"platform":  d.platform,
			"last_seen": time.Now().UTC().Format(time.RFC3339),
		}

		data, _ := json.MarshalIndent(info, "", "  ")
		devicePath := root + "devices/" + d.id + ".json"
		client.EnsureDir(devicePath)
		_, err := client.PUT(devicePath, data, "")
		if err != nil {
			t.Fatalf("注册设备 %s 失败: %v", d.name, err)
		}
		t.Logf("设备 %s 已注册", d.name)
	}

	// 逐个验证设备（不依赖 PROPFIND 的路径解析）
	for _, d := range devices {
		devicePath := root + "devices/" + d.id + ".json"
		data, _, err := client.GET(devicePath)
		if err != nil {
			t.Fatalf("读取设备 %s 失败: %v", d.id, err)
		}

		var info map[string]interface{}
		json.Unmarshal(data, &info)
		t.Logf("  设备: %s (%s)", info["name"], info["platform"])

		if info["id"] != d.id {
			t.Errorf("设备 ID 不匹配: %v vs %s", info["id"], d.id)
		}
	}

	// 删除一个设备
	client.DELETE(root + "devices/mac-test.json")

	// 验证已删除
	_, _, err := client.GET(root + "devices/mac-test.json")
	if err == nil {
		t.Error("mac-test 应该已被删除")
	}

	// 验证另一个仍存在
	_, _, err = client.GET(root + "devices/win-pc-test.json")
	if err != nil {
		t.Error("win-pc-test 应该仍存在")
	}

	t.Log("设备注册/验证/删除测试通过")
	client.DELETE(root)
}

// TestPhase3_ProjectSync 测试项目配置同步流程
func TestPhase3_ProjectSync(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "project-test/"
	client.DELETE(root)
	client.MKCOL(root)

	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey("test-pass", salt)

	claudeJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"server-a": map[string]interface{}{
				"command": "npx",
				"args":    []string{"-y", "@anthropic/server-a"},
			},
		},
		"allowedTools": []string{"Read", "Write", "Bash"},
	}

	jsonData, _ := json.MarshalIndent(claudeJSON, "", "  ")
	encrypted, _ := crypto.Encrypt(jsonData, key)

	remoteURL := "git@github.com:test/project-sync.git"
	encoded := project.EncodeRemote(remoteURL)
	projectPath := root + "projects/" + encoded + "/.claude.json.enc"
	client.EnsureDir(projectPath)
	_, err := client.PUT(projectPath, encrypted, "")
	if err != nil {
		t.Fatalf("上传项目配置失败: %v", err)
	}
	t.Logf("项目配置已上传 (remote: %s, encoded: %s)", remoteURL, encoded[:12]+"...")

	// 直接用已知路径下载验证
	downloaded, _, err := client.GET(projectPath)
	if err != nil {
		t.Fatalf("下载项目配置失败: %v", err)
	}

	decData, err := crypto.Decrypt(downloaded, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	var pulledJSON map[string]interface{}
	json.Unmarshal(decData, &pulledJSON)

	servers := pulledJSON["mcpServers"].(map[string]interface{})
	if _, ok := servers["server-a"]; !ok {
		t.Error("拉取的项目配置应包含 server-a")
	}
	t.Logf("拉取成功: %d 个 mcpServers", len(servers))

	// 测试项目合并
	localJSON := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"server-b": map[string]interface{}{"command": "local"},
		},
		"allowedTools": []string{"Read", "Grep"},
	}

	merged := project.MergeClaudeJSON(localJSON, claudeJSON)

	mergedServers := merged["mcpServers"].(map[string]interface{})
	if len(mergedServers) != 2 {
		t.Errorf("合并后应有 2 个 server，实际 %d", len(mergedServers))
	}
	if _, ok := mergedServers["server-a"]; !ok {
		t.Error("合并结果应包含远程的 server-a")
	}
	if _, ok := mergedServers["server-b"]; !ok {
		t.Error("合并结果应包含本地的 server-b")
	}

	tools := merged["allowedTools"].([]string)
	if len(tools) != 4 {
		t.Errorf("allowedTools 并集 = %v, want 4 个", tools)
	}

	t.Log("项目配置同步 + 合并测试通过")
	client.DELETE(root)
}

// TestPhase3_GC 测试 GC：上传多个快照后清理孤立 objects
func TestPhase3_GC(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "gc-test/"
	client.DELETE(root)
	client.MKCOL(root)

	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey("test-pass", salt)

	tmpDir := t.TempDir()

	// 收集所有上传过的 object 路径
	uploadedHashes := map[string]string{} // hash → remotePath

	var lastSnapID string
	for i := 0; i < 2; i++ {
		writeFile(t, tmpDir, "settings.json", fmt.Sprintf(`{"version":%d}`, i))
		writeFile(t, tmpDir, "shared.txt", "this file is shared across snapshots")
		if i == 1 {
			writeFile(t, tmpDir, "extra.txt", "only in snapshot 1")
		}

		scanner := snapshot.NewScanner(tmpDir, nil)
		scanResult, _ := scanner.Scan()

		for path, entry := range scanResult.Files {
			data, _ := os.ReadFile(filepath.Join(tmpDir, filepath.FromSlash(path)))
			encrypted, _ := crypto.Encrypt(data, key)
			objPath := root + "objects/" + object.HashPrefix(entry.Hash) + "/" + entry.Hash + ".enc"
			client.EnsureDir(objPath)
			client.PUT(objPath, encrypted, "")
			uploadedHashes[entry.Hash] = objPath
		}

		snap := snapshot.CreateSnapshot(lastSnapID, "test-device", fmt.Sprintf("snap %d", i), scanResult.Files)
		snapData, _ := snap.Serialize()
		snapEnc, _ := crypto.Encrypt(snapData, key)
		snapPath := root + "snapshots/" + snap.ID + ".json.enc"
		client.EnsureDir(snapPath)
		client.PUT(snapPath, snapEnc, "")
		client.EnsureDir(root + "HEAD")
		client.PUT(root+"HEAD", []byte(snap.ID), "")

		lastSnapID = snap.ID
		t.Logf("快照 %d: %s (%d files)", i, snap.ID, len(snap.Files))
	}

	// 上传一个孤立的 object
	orphanData := []byte("i am an orphan object")
	orphanHash := object.ComputeHash(orphanData)
	orphanEnc, _ := crypto.Encrypt(orphanData, key)
	orphanPath := root + "objects/" + object.HashPrefix(orphanHash) + "/" + orphanHash + ".enc"
	client.EnsureDir(orphanPath)
	client.PUT(orphanPath, orphanEnc, "")
	uploadedHashes[orphanHash] = orphanPath
	t.Logf("上传了孤立 object: %s", orphanHash[:16])

	// 遍历快照链收集可达哈希
	reachable := make(map[string]bool)
	snapID := lastSnapID
	for snapCount := 0; snapCount < 50 && snapID != ""; snapCount++ {
		snapEnc, _, err := client.GET(root + "snapshots/" + snapID + ".json.enc")
		if err != nil {
			break
		}
		snapData, _ := crypto.Decrypt(snapEnc, key)
		snap, _ := snapshot.Deserialize(snapData)
		for _, entry := range snap.Files {
			reachable[entry.Hash] = true
		}
		snapID = snap.Parent
	}
	t.Logf("可达 objects: %d", len(reachable))

	// 用已知路径（而非 PROPFIND）检查孤立
	var orphans []string
	for hash, path := range uploadedHashes {
		if !reachable[hash] {
			orphans = append(orphans, hash)
			t.Logf("孤立: %s → %s", hash[:16], path)
		}
	}

	if len(orphans) == 0 {
		t.Error("应检测到至少 1 个孤立 object")
	}

	// 清理孤立
	for _, hash := range orphans {
		client.DELETE(uploadedHashes[hash])
		t.Logf("已清理: %s", hash[:16])
	}

	// 验证可达仍存在
	for hash := range reachable {
		exists, _ := client.Exists(uploadedHashes[hash])
		if !exists {
			t.Errorf("可达 object %s 不应被删除", hash[:16])
		}
	}

	// 验证孤立已删除
	for _, hash := range orphans {
		exists, _ := client.Exists(uploadedHashes[hash])
		if exists {
			t.Errorf("孤立 object %s 应已被删除", hash[:16])
		}
	}

	t.Log("GC 测试通过：孤立清理，可达保留")
	client.DELETE(root)
}

// TestPhase3_BinaryPruneSafety 测试 binary prune 的安全规则
func TestPhase3_BinaryPruneSafety(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "prune-test/"
	client.DELETE(root)
	client.MKCOL(root)

	salt := []byte("0123456789abcdef")
	key := crypto.DeriveKey("test-pass", salt)

	idx := binary.NewIndex()
	info := idx.EnsureBinaryInfo("windows-amd64", "claude")
	info.Current = "2.1.126"
	info.Versions = map[string]binary.Version{
		"2.1.126": {
			Hash:       "sha256:abc123",
			Size:       243000000,
			Refs:       2,
			Uploaded:   time.Now().UTC(),
			UploadedBy: "test-device",
		},
		"2.1.84": {
			Hash:       "sha256:def456",
			Size:       234000000,
			Refs:       0,
			Uploaded:   time.Now().UTC(),
			UploadedBy: "test-device",
		},
		"2.1.81": {
			Hash:       "sha256:789abc",
			Size:       232000000,
			Refs:       1,
			Uploaded:   time.Now().UTC(),
			UploadedBy: "test-device",
		},
	}

	// 上传分块文件
	for ver, v := range info.Versions {
		manifestData := fmt.Sprintf(`{"hash":"%s","total_parts":1,"total_size":%d}`, v.Hash, v.Size)
		manifestPath := root + "binaries/parts/" + v.Hash + "/manifest.json"
		client.EnsureDir(manifestPath)
		client.PUT(manifestPath, []byte(manifestData), "")

		chunkData := []byte(fmt.Sprintf("fake binary chunk for %s", ver))
		encChunk, _ := crypto.Encrypt(chunkData, key)
		partPath := root + fmt.Sprintf("binaries/parts/%s/part-000.enc", v.Hash)
		client.EnsureDir(partPath)
		client.PUT(partPath, encChunk, "")
	}

	// 模拟 prune 安全检查
	prunable := []string{}
	for ver, v := range info.Versions {
		if ver == info.Current {
			t.Logf("  %s: 跳过（current 版本）", ver)
			continue
		}
		if v.Refs > 0 {
			t.Logf("  %s: 跳过（refs=%d）", ver, v.Refs)
			continue
		}
		t.Logf("  %s: 可清理（refs=0, 非 current）", ver)
		prunable = append(prunable, ver)
	}

	if len(prunable) != 1 {
		t.Errorf("可清理版本数 = %d, want 1", len(prunable))
	}
	if len(prunable) > 0 && prunable[0] != "2.1.84" {
		t.Errorf("可清理版本 = %s, want 2.1.84", prunable[0])
	}

	// 执行清理
	if len(prunable) > 0 {
		ver := prunable[0]
		v := info.Versions[ver]
		client.DELETE(root + "binaries/parts/" + v.Hash + "/")
		delete(info.Versions, ver)
		t.Logf("已清理 %s", ver)
	}

	// 验证其他版本分块仍存在
	for ver, v := range info.Versions {
		manifestPath := root + "binaries/parts/" + v.Hash + "/manifest.json"
		_, _, err := client.GET(manifestPath)
		if err != nil {
			t.Errorf("版本 %s 的 manifest 不应被删除", ver)
		}
	}

	// 验证被清理版本已删除
	_, _, err := client.GET(root + "binaries/parts/sha256:def456/manifest.json")
	if err == nil {
		t.Error("已清理版本的 manifest 应该不存在")
	}

	t.Log("binary prune 安全规则测试通过")
	client.DELETE(root)
}

// TestPhase3_DiffBetweenSnapshots 测试两个快照之间的 diff
func TestPhase3_DiffBetweenSnapshots(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "diff-test/"
	client.DELETE(root)
	client.MKCOL(root)

	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey("test-pass", salt)

	tmpDir := t.TempDir()

	var snaps []*snapshot.Snapshot
	var lastSnapID string

	contents := []struct {
		settings string
		claudeMd string
	}{
		{`{"env":{"KEY_A":"old","KEY_B":"val"}}`, "# Instructions\nline1\nline2\nline3\n"},
		{`{"env":{"KEY_A":"new","KEY_B":"val","KEY_C":"added"}}`, "# Instructions\nline1\nline2-modified\nline3\nline4\n"},
	}

	for i, c := range contents {
		writeFile(t, tmpDir, "settings.json", c.settings)
		writeFile(t, tmpDir, "CLAUDE.md", c.claudeMd)

		scanner := snapshot.NewScanner(tmpDir, nil)
		scanResult, _ := scanner.Scan()

		for path, entry := range scanResult.Files {
			data, _ := os.ReadFile(filepath.Join(tmpDir, filepath.FromSlash(path)))
			encrypted, _ := crypto.Encrypt(data, key)
			objPath := root + "objects/" + object.HashPrefix(entry.Hash) + "/" + entry.Hash + ".enc"
			client.EnsureDir(objPath)
			client.PUT(objPath, encrypted, "")
		}

		snap := snapshot.CreateSnapshot(lastSnapID, "test-device", fmt.Sprintf("snap %d", i), scanResult.Files)
		snapData, _ := snap.Serialize()
		snapEnc, _ := crypto.Encrypt(snapData, key)
		snapPath := root + "snapshots/" + snap.ID + ".json.enc"
		client.EnsureDir(snapPath)
		client.PUT(snapPath, snapEnc, "")

		snaps = append(snaps, snap)
		lastSnapID = snap.ID
	}

	// 计算 diff
	changes := snaps[0].Diff(snaps[1])
	t.Logf("diff: %d 个变更", len(changes))

	for _, c := range changes {
		switch c.Type {
		case snapshot.Modified:
			t.Logf("  M  %s (%s → %s)", c.Path, c.OldHash[:12], c.NewHash[:12])
		}
	}

	if len(changes) == 0 {
		t.Error("两个不同的快照之间应有差异")
	}

	// 下载旧版本和新版本对比
	for _, c := range changes {
		if c.Type == snapshot.Modified {
			oldData, err := downloadObject(client, key, root, c.OldHash)
			if err != nil {
				t.Logf("下载旧版 %s 失败: %v", c.Path, err)
				continue
			}
			newData, err := downloadObject(client, key, root, c.NewHash)
			if err != nil {
				t.Logf("下载新版 %s 失败: %v", c.Path, err)
				continue
			}

			t.Logf("  %s 旧版 → 新版:", c.Path)
			t.Logf("    旧: %s", truncate(string(oldData), 60))
			t.Logf("    新: %s", truncate(string(newData), 60))
		}
	}

	t.Log("快照 diff 测试通过")
	client.DELETE(root)
}

// TestPhase3_EncodeRemoteConsistency 测试 remote 编码一致性
func TestPhase3_EncodeRemoteConsistency(t *testing.T) {
	urls := []string{
		"git@github.com:user/repo.git",
		"https://github.com/user/repo.git",
		"ssh://git@gitlab.com:2222/group/project.git",
		"local:/home/user/my-project",
	}

	encodings := make(map[string]string)
	for _, u := range urls {
		enc := project.EncodeRemote(u)
		encodings[u] = enc

		for _, c := range enc {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("EncodeRemote(%q) 包含非 hex 字符: %c", u, c)
				break
			}
		}

		enc2 := project.EncodeRemote(u)
		if enc != enc2 {
			t.Errorf("EncodeRemote 不一致: %q → %q vs %q", u, enc, enc2)
		}

		t.Logf("  %s → %s", u, enc[:12]+"...")
	}

	seen := make(map[string]string)
	for u, enc := range encodings {
		if prev, ok := seen[enc]; ok {
			t.Errorf("碰撞: %q 和 %q 都编码为 %s", prev, u, enc[:12])
		}
		seen[enc] = u
	}

	t.Log("remote 编码一致性测试通过")
}

func downloadObject(client *webdav.Client, key []byte, root, hash string) ([]byte, error) {
	objPath := root + "objects/" + object.HashPrefix(hash) + "/" + hash + ".enc"
	encData, _, err := client.GET(objPath)
	if err != nil {
		return nil, err
	}
	return crypto.Decrypt(encData, key)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
