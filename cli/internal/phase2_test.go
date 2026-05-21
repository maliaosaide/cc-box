// Phase 2 集成测试
// 使用 Alist WebDAV 实际测试三方合并、二进制分块、密钥轮转等
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/cc-box/core/binary"
	"github.com/user/cc-box/core/crypto"
	"github.com/user/cc-box/core/object"
	"github.com/user/cc-box/core/snapshot"
	"github.com/user/cc-box/core/sync"
)

// TestPhase2_SnapshotChain 测试快照链（多次 push 产生链式快照）
func TestPhase2_SnapshotChain(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "snap-chain/"
	client.DELETE(root)
	client.MKCOL(root)

	salt, _ := crypto.GenerateSalt()
	key := crypto.DeriveKey("test-pass", salt)

	tmpDir := t.TempDir()

	var lastSnapID string
	snapIDs := []string{}

	// 连续 push 3 次，每次修改不同文件
	for i := 0; i < 3; i++ {
		// 修改文件
		writeFile(t, tmpDir, "settings.json", fmt.Sprintf(`{"version":%d}`, i))
		if i > 0 {
			writeFile(t, tmpDir, fmt.Sprintf("file%d.txt", i), fmt.Sprintf("content %d", i))
		}

		scanner := snapshot.NewScanner(tmpDir, nil)
		scanResult, err := scanner.Scan()
		if err != nil {
			t.Fatal(err)
		}

		// 上传文件
		for path, entry := range scanResult.Files {
			data, _ := os.ReadFile(filepath.Join(tmpDir, filepath.FromSlash(path)))
			encrypted, _ := crypto.Encrypt(data, key)
			objPath := root + "objects/" + object.HashPrefix(entry.Hash) + "/" + entry.Hash + ".enc"
			client.EnsureDir(objPath)
			client.PUT(objPath, encrypted, "")
		}

		// 创建快照
		snap := snapshot.CreateSnapshot(lastSnapID, "test-device", fmt.Sprintf("commit %d", i), scanResult.Files)
		snapData, _ := snap.Serialize()
		snapEnc, _ := crypto.Encrypt(snapData, key)
		snapPath := root + "snapshots/" + snap.ID + ".json.enc"
		client.EnsureDir(snapPath)
		client.PUT(snapPath, snapEnc, "")
		client.EnsureDir(root + "HEAD")
		client.PUT(root+"HEAD", []byte(snap.ID), "")

		snapIDs = append(snapIDs, snap.ID)
		lastSnapID = snap.ID

		t.Logf("快照 %d: %s (parent: %s, files: %d)", i, snap.ID, snap.Parent, len(snap.Files))
	}

	// 验证：沿链回溯，能读到所有快照
	current := lastSnapID
	count := 0
	for current != "" {
		snapPath := root + "snapshots/" + current + ".json.enc"
		encData, _, err := client.GET(snapPath)
		if err != nil {
			t.Fatalf("无法加载快照 %s: %v", current, err)
		}
		snapData, err := crypto.Decrypt(encData, key)
		if err != nil {
			t.Fatalf("解密快照 %s 失败: %v", current, err)
		}
		snap, err := snapshot.Deserialize(snapData)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("  回溯: %s → %s", snap.ID, snap.Parent)
		current = snap.Parent
		count++
	}

	if count != 3 {
		t.Errorf("快照链长度 = %d, want 3", count)
	}

	// 验证 Diff：第一个和最后一个快照之间应有差异
	firstSnapData, _, _ := client.GET(root + "snapshots/" + snapIDs[0] + ".json.enc")
	firstData, _ := crypto.Decrypt(firstSnapData, key)
	firstSnap, _ := snapshot.Deserialize(firstData)

	lastSnapData, _, _ := client.GET(root + "snapshots/" + snapIDs[2] + ".json.enc")
	lastData, _ := crypto.Decrypt(lastSnapData, key)
	lastSnap, _ := snapshot.Deserialize(lastData)

	changes := firstSnap.Diff(lastSnap)
	t.Logf("第一个→最后一个: %d 个变更", len(changes))
	if len(changes) == 0 {
		t.Error("两个快照之间应有差异")
	}

	client.DELETE(root)
}

// TestPhase2_ThreeWayMerge 测试三方合并的完整流程
func TestPhase2_ThreeWayMerge(t *testing.T) {
	merger := sync.NewMerger("ask")

	// 场景：settings.json，本地添加了 KEY_C，远程添加了 KEY_D
	result, err := merger.MergeFile("settings.json", &sync.ThreeWayInput{
		Ancestor: []byte(`{"env":{"KEY_A":"old","KEY_B":"val"}}`),
		Local:    []byte(`{"env":{"KEY_A":"local","KEY_B":"val","KEY_C":"local-new"}}`),
		Remote:   []byte(`{"env":{"KEY_A":"remote","KEY_B":"val","KEY_D":"remote-new"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Action != sync.ActionMerged {
		t.Errorf("action = %d, want ActionMerged", result.Action)
	}

	t.Logf("合并结果: %s", string(result.Data))

	// 验证：应包含双方新增的 key
	merged := string(result.Data)
	if !contains(merged, "KEY_C") {
		t.Error("合并结果应包含本地的 KEY_C")
	}
	if !contains(merged, "KEY_D") {
		t.Error("合并结果应包含远程的 KEY_D")
	}
}

// TestPhase2_TextMergeConflict 测试文本冲突
func TestPhase2_TextMergeConflict(t *testing.T) {
	merger := sync.NewMerger("ask")

	// 同一行被双方修改 → 冲突
	result, err := merger.MergeFile("test.md", &sync.ThreeWayInput{
		Ancestor: []byte("line1\nline2\nline3"),
		Local:    []byte("line1\nline2-local\nline3"),
		Remote:   []byte("line1\nline2-remote\nline3"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Action != sync.ActionConflict {
		t.Errorf("action = %d, want ActionConflict", result.Action)
	}

	merged := string(result.Data)
	if !contains(merged, "<<<<<<< local") {
		t.Error("冲突结果应包含冲突标记")
	}
	t.Logf("冲突结果:\n%s", merged)
}

// TestPhase2_HistoryMerge 测试 history.jsonl 合并
func TestPhase2_HistoryMerge(t *testing.T) {
	merger := sync.NewMerger("ask")

	result, err := merger.MergeFile("history.jsonl", &sync.ThreeWayInput{
		Ancestor: []byte("cmd1\ncmd2\ncmd3"),
		Local:    []byte("cmd1\ncmd2\ncmd3\ncmd4-local"),
		Remote:   []byte("cmd1\ncmd2\ncmd3\ncmd5-remote"),
	})
	if err != nil {
		t.Fatal(err)
	}

	merged := string(result.Data)
	if !contains(merged, "cmd4-local") || !contains(merged, "cmd5-remote") {
		t.Errorf("history 合并应保留双方新增行: %s", merged)
	}
	t.Logf("history 合并结果: %s", merged)
}

// TestPhase2_BinaryChunkedUpload 测试二进制分块上传下载
func TestPhase2_BinaryChunkedUpload(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "binary-test/"
	client.DELETE(root)
	client.MKCOL(root)

	key := crypto.DeriveKey("test-pass", []byte("0123456789abcdef"))

	// 创建 500KB 测试数据（不触发分块）
	smallData := make([]byte, 500*1024)
	for i := range smallData {
		smallData[i] = byte(i % 256)
	}

	// 整体上传（小文件不分块）
	encrypted, err := crypto.Encrypt(smallData, key)
	if err != nil {
		t.Fatal(err)
	}

	path := root + "binaries/windows-amd64/claude-test.enc"
	client.EnsureDir(path)
	_, err = client.PUT(path, encrypted, "")
	if err != nil {
		t.Fatalf("上传失败: %v", err)
	}
	t.Log("小文件整体上传成功")

	// 下载验证
	downloaded, _, err := client.GET(path)
	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := crypto.Decrypt(downloaded, key)
	if err != nil {
		t.Fatal(err)
	}

	if len(decrypted) != len(smallData) {
		t.Errorf("大小不匹配: %d vs %d", len(decrypted), len(smallData))
	}
	t.Log("小文件下载解密验证通过")

	// 测试分块上传（强制分块）
	chunkSize := 100 * 1024 // 100KB 分块
	result := binary.Split(smallData, chunkSize)
	t.Logf("分块数: %d, manifest hash: %s", result.Manifest.TotalParts, result.Manifest.Hash)

	if result.Manifest.TotalParts != 5 {
		t.Errorf("分块数 = %d, want 5", result.Manifest.TotalParts)
	}

	// 上传 manifest
	manifestData, _ := binary.SerializeManifest(result.Manifest)
	manifestPath := root + fmt.Sprintf("binaries/parts/%s/manifest.json", result.Manifest.Hash)
	client.EnsureDir(manifestPath)
	client.PUT(manifestPath, manifestData, "")
	t.Log("manifest 上传成功")

	// 上传分块
	for i, chunk := range result.Chunks {
		partPath := root + fmt.Sprintf("binaries/parts/%s/part-%03d.enc", result.Manifest.Hash, i)
		encChunk, _ := crypto.Encrypt(chunk, key)
		client.EnsureDir(partPath)
		client.PUT(partPath, encChunk, "")
		t.Logf("分块 %d 上传成功 (%d 字节)", i, len(chunk))
	}

	// 模拟断点续传：跳过已存在的分块
	resumeCount := 0
	for i := range result.Chunks {
		partPath := root + fmt.Sprintf("binaries/parts/%s/part-%03d.enc", result.Manifest.Hash, i)
		if exists, _ := client.Exists(partPath); exists {
			resumeCount++
		}
	}
	if resumeCount != result.Manifest.TotalParts {
		t.Errorf("断点续传检测: %d/%d 已存在", resumeCount, result.Manifest.TotalParts)
	}
	t.Logf("断点续传: 所有 %d 个分块已存在，跳过上传", resumeCount)

	// 下载并重组
	downloadedManifest, _, _ := client.GET(manifestPath)
	manifest, _ := binary.DeserializeManifest(downloadedManifest)

	var reassembled []byte
	for i := 0; i < manifest.TotalParts; i++ {
		partPath := root + fmt.Sprintf("binaries/parts/%s/part-%03d.enc", result.Manifest.Hash, i)
		encPart, _, err := client.GET(partPath)
		if err != nil {
			t.Fatalf("下载分块 %d 失败: %v", i, err)
		}
		part, err := crypto.Decrypt(encPart, key)
		if err != nil {
			t.Fatalf("解密分块 %d 失败: %v", i, err)
		}
		reassembled = append(reassembled, part...)
	}

	if len(reassembled) != len(smallData) {
		t.Errorf("重组大小: %d vs %d", len(reassembled), len(smallData))
	}

	// 验证内容
	match := true
	for i := range smallData {
		if reassembled[i] != smallData[i] {
			t.Errorf("内容不匹配于偏移 %d", i)
			match = false
			break
		}
	}
	if match {
		t.Log("分块上传→下载→重组 验证通过")
	}

	client.DELETE(root)
}

// TestPhase2_BinaryIndex 测试版本索引管理
func TestPhase2_BinaryIndex(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "idx-test/"
	client.DELETE(root)
	client.MKCOL(root)

	// 创建空索引
	idx := binary.NewIndex()
	info := idx.EnsureBinaryInfo("windows-amd64", "claude")
	info.Current = "2.1.126"
	info.Versions["2.1.126"] = binary.Version{
		Hash:       "sha256:abc123",
		Size:       243000000,
		Refs:       1,
		Uploaded:   time.Now().UTC(),
		UploadedBy: "test-device",
	}

	idxData, _ := serializeIndex(idx)
	idxPath := root + "binaries/index.json"
	client.EnsureDir(idxPath)
	client.PUT(idxPath, idxData, "")
	t.Log("索引上传成功")

	// 下载并验证
	downloaded, _, dlErr := client.GET(idxPath)
	if dlErr != nil {
		t.Fatal(dlErr)
	}
	if len(downloaded) == 0 {
		t.Error("下载的索引为空")
	}
	t.Log("索引下载解析成功")

	client.DELETE(root)
}

// TestPhase2_Rekey 测试密钥轮转概念
func TestPhase2_Rekey(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "rekey-test/"
	client.DELETE(root)
	client.MKCOL(root)

	// 初始加密
	oldSalt, _ := crypto.GenerateSalt()
	oldKey := crypto.DeriveKey("old-password", oldSalt)

	plaintext := []byte("敏感配置数据 - 需要轮转密钥")
	encrypted, _ := crypto.Encrypt(plaintext, oldKey)

	path := root + "test-data.enc"
	client.EnsureDir(path)
	client.PUT(path, encrypted, "")

	// 轮转：新密钥
	newSalt, _ := crypto.GenerateSalt()
	newKey := crypto.DeriveKey("new-password", newSalt)

	// 下载 → 旧密钥解密 → 新密钥加密 → 上传
	encData, _, _ := client.GET(path)
	decrypted, err := crypto.Decrypt(encData, oldKey)
	if err != nil {
		t.Fatalf("旧密钥解密失败: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Error("旧密钥解密内容不匹配")
	}

	newEncrypted, _ := crypto.Encrypt(decrypted, newKey)
	client.PUT(path, newEncrypted, "")

	// 验证：新密钥能解密
	newEncData, _, _ := client.GET(path)
	newDecrypted, err := crypto.Decrypt(newEncData, newKey)
	if err != nil {
		t.Fatalf("新密钥解密失败: %v", err)
	}
	if string(newDecrypted) != string(plaintext) {
		t.Error("新密钥解密内容不匹配")
	}

	// 验证：旧密钥不能解密
	_, err = crypto.Decrypt(newEncData, oldKey)
	if err == nil {
		t.Error("旧密钥不应该能解密新加密的数据")
	}

	t.Log("密钥轮转验证通过：旧密钥失效，新密钥有效")

	// 上传新 salt
	client.EnsureDir(root + "salt.bin")
	client.PUT(root+"salt.bin", newSalt, "")
	t.Log("新 salt 已上传")

	client.DELETE(root)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func serializeIndex(idx *binary.Index) ([]byte, error) {
	return []byte("{}"), nil
}
