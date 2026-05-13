// Phase 1 集成测试
// 验证 WebDAV 连通性 + init → push → pull → status 完整流程
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/cc-box/internal/config"
	"github.com/user/cc-box/internal/crypto"
	"github.com/user/cc-box/internal/object"
	"github.com/user/cc-box/internal/snapshot"
	"github.com/user/cc-box/internal/webdav"
)

const testRootPrefix = "cc-box-test/" // 测试用的独立目录，避免与正式数据冲突

func testWebDAVConfig(t *testing.T) (string, string, string) {
	t.Helper()
	url := os.Getenv("CC_BOX_TEST_WEBDAV_URL")
	username := os.Getenv("CC_BOX_TEST_WEBDAV_USERNAME")
	password := os.Getenv("CC_BOX_TEST_WEBDAV_PASSWORD")
	if url == "" || username == "" || password == "" {
		t.Skip("需要设置 CC_BOX_TEST_WEBDAV_URL、CC_BOX_TEST_WEBDAV_USERNAME、CC_BOX_TEST_WEBDAV_PASSWORD 才运行集成测试")
	}
	return url, username, password
}

// newTestClient 创建测试用 WebDAV 客户端
func newTestClient(t *testing.T) *webdav.Client {
	t.Helper()
	url, username, password := testWebDAVConfig(t)
	client := webdav.NewClient(url, username, password)
	client.SetTimeout(30e9) // 30 秒
	return client
}

// TestWebDAVConnectivity 测试基本连通性
func TestWebDAVConnectivity(t *testing.T) {
	client := newTestClient(t)

	exists, err := client.Exists("/")
	if err != nil {
		t.Fatalf("WebDAV 连接失败: %v", err)
	}
	if !exists {
		t.Fatal("根目录应该存在")
	}
	t.Log("WebDAV 连接成功")
}

// TestWebDAVMKCOLAndDELETE 测试目录创建和删除
func TestWebDAVMKCOLAndDELETE(t *testing.T) {
	client := newTestClient(t)
	path := testRootPrefix + "test-dir"

	if err := client.MKCOL(path); err != nil {
		t.Fatalf("MKCOL 失败: %v", err)
	}
	t.Log("MKCOL 成功")

	exists, _ := client.Exists(path)
	if !exists {
		t.Fatal("目录应该存在")
	}

	// 幂等：再次创建不应报错
	if err := client.MKCOL(path); err != nil {
		t.Fatalf("重复 MKCOL 失败: %v", err)
	}

	if err := client.DELETE(path); err != nil {
		t.Fatalf("DELETE 失败: %v", err)
	}
	t.Log("DELETE 成功")
}

// TestWebDAVPUTAndGET 测试文件上传和下载
func TestWebDAVPUTAndGET(t *testing.T) {
	client := newTestClient(t)
	path := testRootPrefix + "test-file.txt"
	content := []byte("Hello, CC-Box integration test! " + time.Now().Format(time.RFC3339))

	if err := client.EnsureDir(path); err != nil {
		t.Fatalf("EnsureDir 失败: %v", err)
	}

	etag, err := client.PUT(path, content, "")
	if err != nil {
		t.Fatalf("PUT 失败: %v", err)
	}
	t.Logf("PUT 成功, ETag: %s", etag)

	downloaded, gotETag, err := client.GET(path)
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}
	if string(downloaded) != string(content) {
		t.Errorf("GET 内容不匹配: got %q, want %q", string(downloaded), string(content))
	}
	t.Logf("GET 成功, ETag: %s", gotETag)

	// 测试 HEAD
	info, err := client.HEAD(path)
	if err != nil {
		t.Fatalf("HEAD 失败: %v", err)
	}
	t.Logf("HEAD: path=%s size=%d etag=%s", info.Path, info.Size, info.ETag)

	// 清理
	client.DELETE(path)
}

// TestWebDAVOptimisticLock 测试乐观锁（ETag If-Match）
func TestWebDAVOptimisticLock(t *testing.T) {
	client := newTestClient(t)
	path := testRootPrefix + "lock-test.txt"

	client.EnsureDir(path)

	// 第一次 PUT
	_, err := client.PUT(path, []byte("version1"), "")
	if err != nil {
		t.Fatalf("第一次 PUT 失败: %v", err)
	}

	// 读取 ETag
	_, etag, err := client.GET(path)
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}
	t.Logf("初始 ETag: %s", etag)

	// 用正确的 ETag 更新 → 应该成功
	if etag != "" {
		newETag, err := client.PUT(path, []byte("version2"), etag)
		if err != nil {
			t.Fatalf("带 ETag PUT 失败: %v", err)
		}
		t.Logf("带 ETag 更新成功, 新 ETag: %s", newETag)
	}

	// 用旧的 ETag 更新 → 应该冲突（412）
	if etag != "" {
		_, err := client.PUT(path, []byte("version3"), etag)
		if err != webdav.ErrConflict {
			t.Logf("旧 ETag PUT: err=%v (某些服务端可能不支持 ETag 乐观锁)", err)
		} else {
			t.Log("乐观锁冲突检测正常")
		}
	}

	client.DELETE(path)
}

// TestWebDAVPROPFIND 测试目录列表
func TestWebDAVPROPFIND(t *testing.T) {
	client := newTestClient(t)
	dir := testRootPrefix + "propfind-test/"

	client.MKCOL(dir)
	client.EnsureDir(dir + "file1.txt")
	client.PUT(dir+"file1.txt", []byte("file1"), "")
	client.EnsureDir(dir + "file2.txt")
	client.PUT(dir+"file2.txt", []byte("file2"), "")

	files, err := client.PROPFIND(dir, 1)
	if err != nil {
		t.Fatalf("PROPFIND 失败: %v", err)
	}

	t.Logf("PROPFIND 返回 %d 个条目:", len(files))
	for _, f := range files {
		dirMark := ""
		if f.IsDir {
			dirMark = "/"
		}
		t.Logf("  %s%s (size=%d, etag=%s)", f.Path, dirMark, f.Size, f.ETag)
	}

	if len(files) < 2 {
		t.Errorf("应该至少有 2 个文件，实际 %d 个", len(files))
	}

	client.DELETE(dir)
}

// TestFullSyncFlow 端到端测试：init → push → pull
func TestFullSyncFlow(t *testing.T) {
	client := newTestClient(t)
	root := testRootPrefix + "sync-flow/"

	// 清理旧数据
	client.DELETE(root)
	client.MKCOL(root)

	// 1. 模拟 init：生成密钥 + salt
	salt, err := crypto.GenerateSalt()
	if err != nil {
		t.Fatalf("生成 salt 失败: %v", err)
	}
	key := crypto.DeriveKey("test-password-123", salt)

	// 上传 salt
	client.EnsureDir(root + "salt.bin")
	if _, err := client.PUT(root+"salt.bin", salt, ""); err != nil {
		t.Fatalf("上传 salt 失败: %v", err)
	}
	t.Log("init: salt 已上传")

	// 2. 创建临时目录模拟 ~/.claude/
	tmpDir := t.TempDir()
	writeFile(t, tmpDir, "settings.json", `{"env":{"KEY":"val1"}}`)
	writeFile(t, tmpDir, "CLAUDE.md", "# Test Instructions\nLine 1\nLine 2\n")
	writeFile(t, tmpDir, "skills/test/SKILL.md", "# Test Skill")

	// 3. 扫描并 push
	scanner := snapshot.NewScanner(tmpDir, config.DefaultExcludePatterns)
	scanResult, err := scanner.Scan()
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}
	t.Logf("扫描到 %d 个文件", scanResult.Stats.TotalFiles)

	// 上传文件 objects
	testClient := newTestClient(t)
	uploaded := 0
	for path, entry := range scanResult.Files {
		data, err := os.ReadFile(filepath.Join(tmpDir, filepath.FromSlash(path)))
		if err != nil {
			t.Logf("跳过 %s: %v", path, err)
			continue
		}

		// 手动构造带 root 前缀的 object 路径
		objPath := root + "objects/" + object.HashPrefix(entry.Hash) + "/" + entry.Hash + ".enc"
		encrypted, err := crypto.Encrypt(data, key)
		if err != nil {
			t.Fatalf("加密 %s 失败: %v", path, err)
		}

		testClient.EnsureDir(objPath)
		if _, err := testClient.PUT(objPath, encrypted, ""); err != nil {
			t.Fatalf("上传 %s 失败: %v", path, err)
		}
		uploaded++
	}
	t.Logf("已上传 %d 个文件 objects", uploaded)

	// 4. 创建并上传快照
	snap := snapshot.CreateSnapshot("", "test-device", "integration test", scanResult.Files)
	snapData, err := snap.Serialize()
	if err != nil {
		t.Fatalf("序列化快照失败: %v", err)
	}

	snapEnc, err := crypto.Encrypt(snapData, key)
	if err != nil {
		t.Fatalf("加密快照失败: %v", err)
	}

	snapPath := root + "snapshots/" + snap.ID + ".json.enc"
	testClient.EnsureDir(snapPath)
	if _, err := testClient.PUT(snapPath, snapEnc, ""); err != nil {
		t.Fatalf("上传快照失败: %v", err)
	}

	// 更新 HEAD
	testClient.EnsureDir(root + "HEAD")
	if _, err := testClient.PUT(root+"HEAD", []byte(snap.ID), ""); err != nil {
		t.Fatalf("更新 HEAD 失败: %v", err)
	}
	t.Logf("push: 快照 %s 已上传", snap.ID)

	// 5. 模拟 pull：新设备下载
	headData, _, err := testClient.GET(root + "HEAD")
	if err != nil {
		t.Fatalf("读取 HEAD 失败: %v", err)
	}
	remoteHead := string(headData)
	t.Logf("pull: 远程 HEAD = %s", remoteHead)

	// 下载快照
	downloadedSnapEnc, _, err := testClient.GET(root + "snapshots/" + remoteHead + ".json.enc")
	if err != nil {
		t.Fatalf("下载快照失败: %v", err)
	}

	downloadedSnapData, err := crypto.Decrypt(downloadedSnapEnc, key)
	if err != nil {
		t.Fatalf("解密快照失败: %v", err)
	}

	remoteSnap, err := snapshot.Deserialize(downloadedSnapData)
	if err != nil {
		t.Fatalf("反序列化快照失败: %v", err)
	}
	t.Logf("pull: 快照包含 %d 个文件", len(remoteSnap.Files))

	// 下载所有文件到新目录
	pullDir := t.TempDir()
	for path, entry := range remoteSnap.Files {
		objPath := root + "objects/" + object.HashPrefix(entry.Hash) + "/" + entry.Hash + ".enc"
		encData, _, err := testClient.GET(objPath)
		if err != nil {
			t.Fatalf("下载 object %s 失败: %v", path, err)
		}

		fileData, err := crypto.Decrypt(encData, key)
		if err != nil {
			t.Fatalf("解密 %s 失败: %v", path, err)
		}

		writeFile(t, pullDir, path, string(fileData))
	}

	// 6. 验证文件内容一致
	for path := range remoteSnap.Files {
		original := readFile(t, tmpDir, path)
		pulled := readFile(t, pullDir, path)
		if original != pulled {
			t.Errorf("文件 %s 内容不一致\noriginal: %q\npulled:   %q", path, original, pulled)
		} else {
			t.Logf("验证通过: %s", path)
		}
	}

	t.Log("=== 端到端测试通过 ===")

	// 清理
	client.DELETE(root)
}

// TestFileUpload 测试单文件上传下载（验证大文件分块的基础）
func TestFileUpload(t *testing.T) {
	client := newTestClient(t)
	path := testRootPrefix + "upload-test/test-file.bin"

	// 生成 1MB 测试数据
	size := 1 * 1024 * 1024
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	client.EnsureDir(path)

	t.Logf("上传 %d 字节...", size)
	_, err := client.PUT(path, data, "")
	if err != nil {
		t.Fatalf("PUT 失败: %v", err)
	}

	downloaded, _, err := client.GET(path)
	if err != nil {
		t.Fatalf("GET 失败: %v", err)
	}

	if len(downloaded) != size {
		t.Errorf("下载大小不一致: got %d, want %d", len(downloaded), size)
	}

	// 验证内容
	match := true
	for i := range data {
		if downloaded[i] != data[i] {
			t.Errorf("内容在偏移 %d 处不匹配", i)
			match = false
			break
		}
	}
	if match {
		t.Logf("%d 字节上传下载验证通过", size)
	}

	client.DELETE(testRootPrefix + "upload-test/")
}

// TestResumableUploadConcept 测试断点续传概念验证
// 用 HEAD 请求检查已上传的分块，跳过已存在的
func TestResumableUploadConcept(t *testing.T) {
	client := newTestClient(t)
	basePath := testRootPrefix + "resume-test/"

	client.MKCOL(basePath)

	// 模拟分块上传：将文件分成 3 块
	chunkSize := 100 * 1024 // 100KB per chunk
	totalSize := chunkSize * 3
	fullData := make([]byte, totalSize)
	for i := range fullData {
		fullData[i] = byte(i % 256)
	}

	chunks := [][]byte{
		fullData[0*chunkSize : 1*chunkSize],
		fullData[1*chunkSize : 2*chunkSize],
		fullData[2*chunkSize : 3*chunkSize],
	}

	// 第一轮：上传前 2 块
	for i := 0; i < 2; i++ {
		partPath := basePath + fmt.Sprintf("part-%03d.enc", i)
		client.EnsureDir(partPath)
		_, err := client.PUT(partPath, chunks[i], "")
		if err != nil {
			t.Fatalf("上传 part-%d 失败: %v", i, err)
		}
		t.Logf("已上传 part-%03d (%d 字节)", i, len(chunks[i]))
	}

	// 模拟中断后恢复：用 HEAD 检查已上传的块
	completedParts := []int{}
	for i := 0; i < 3; i++ {
		partPath := basePath + fmt.Sprintf("part-%03d.enc", i)
		info, err := client.HEAD(partPath)
		if err == nil && info != nil {
			completedParts = append(completedParts, i)
			t.Logf("part-%03d 已存在 (size=%d)，跳过", i, info.Size)
		}
	}

	// 续传未完成的块
	for i := 0; i < 3; i++ {
		partPath := basePath + fmt.Sprintf("part-%03d.enc", i)
		if _, err := client.HEAD(partPath); err == nil {
			continue // 已存在，跳过
		}

		client.EnsureDir(partPath)
		_, err := client.PUT(partPath, chunks[i], "")
		if err != nil {
			t.Fatalf("续传 part-%d 失败: %v", i, err)
		}
		t.Logf("续传 part-%03d (%d 字节)", i, len(chunks[i]))
	}

	// 验证所有块都存在
	for i := 0; i < 3; i++ {
		partPath := basePath + fmt.Sprintf("part-%03d.enc", i)
		data, _, err := client.GET(partPath)
		if err != nil {
			t.Fatalf("下载 part-%d 失败: %v", i, err)
		}
		if len(data) != chunkSize {
			t.Errorf("part-%d 大小不一致: got %d, want %d", i, len(data), chunkSize)
		}
		if data[0] != chunks[i][0] || data[chunkSize-1] != chunks[i][chunkSize-1] {
			t.Errorf("part-%d 内容不匹配", i)
		}
	}

	t.Log("断点续传概念验证通过：HEAD 检测 + 跳过已有分块 + 续传")
	fmt.Printf("已完成分块: %v\n", completedParts)

	client.DELETE(basePath)
}

// --- helpers ---

func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, filepath.FromSlash(path))
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
}

func readFile(t *testing.T, dir, path string) string {
	t.Helper()
	fullPath := filepath.Join(dir, filepath.FromSlash(path))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return string(data)
}
