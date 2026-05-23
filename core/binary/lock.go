package binary

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/user/cc-box/core/webdav"
)

func withBinaryVersionLock(client *webdav.Client, platform, name, version string, fn func() error) error {
	lockPath := fmt.Sprintf("binaries/locks/version/%s/%s/%s.lock", encodeLockPart(platform), encodeLockPart(name), encodeLockPart(version))
	return withBinaryRemoteLock(client, lockPath, "二进制版本正在被其他设备修改，请稍后重试", fn)
}

func withBinaryHashLock(client *webdav.Client, hash string, fn func() error) error {
	lockPath := fmt.Sprintf("binaries/locks/hash/%s.lock", encodeLockPart(hash))
	return withBinaryRemoteLock(client, lockPath, "二进制内容正在被其他设备修改，请稍后重试", fn)
}

func withBinaryRemoteLock(client *webdav.Client, lockPath, conflictMessage string, fn func() error) error {
	if err := client.EnsureDir(lockPath); err != nil {
		return err
	}
	payload := []byte(time.Now().UTC().Format(time.RFC3339Nano))
	if _, err := client.PUTIfAbsent(lockPath, payload); err != nil {
		if err == webdav.ErrConflict {
			return fmt.Errorf("%s", conflictMessage)
		}
		return fmt.Errorf("创建远程二进制锁失败: %w", err)
	}
	defer func() { _ = client.DELETE(lockPath) }()
	return fn()
}

func encodeLockPart(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
