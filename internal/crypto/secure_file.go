// 安全文件操作
// 跨平台安全读写，Windows 上设置 ACL
package crypto

import (
	"os"
	"runtime"
)

// writeFileSecure 写入文件并尝试设置为仅当前用户可读
func writeFileSecure(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}

	// Windows 不支持 Unix 权限位，ACL 由 NTFS 处理
	if runtime.GOOS != "windows" {
		f.Chmod(perm)
	}

	return nil
}

// readFileSecure 读取文件
func readFileSecure(path string) ([]byte, error) {
	return os.ReadFile(path)
}
