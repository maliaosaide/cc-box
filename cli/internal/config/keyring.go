// 系统密钥环操作
// 跨平台密钥环读写，降级到文件存储
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// keyringGet 从系统密钥环读取密码
// Phase 1 降级实现：使用本地文件存储，Phase 3 引入 gosoft/gkeyring
func keyringGet(service, username string) (string, error) {
	store := keyringPath()
	data, err := os.ReadFile(store)
	if err != nil {
		return "", fmt.Errorf("密钥环文件不存在: %w", err)
	}

	var entries map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		return "", fmt.Errorf("密钥环文件格式错误: %w", err)
	}

	key := service + ":" + username
	pass, ok := entries[key]
	if !ok {
		return "", fmt.Errorf("未找到 %s 的密码", key)
	}
	return pass, nil
}

// keyringSet 保存密码到系统密钥环
func keyringSet(service, username, password string) error {
	store := keyringPath()
	dir := filepath.Dir(store)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("创建密钥环目录失败: %w", err)
	}

	entries := make(map[string]string)
	data, err := os.ReadFile(store)
	if err == nil {
		json.Unmarshal(data, &entries)
	}

	key := service + ":" + username
	entries[key] = password

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化密钥环失败: %w", err)
	}

	if err := os.WriteFile(store, out, 0600); err != nil {
		return fmt.Errorf("写入密钥环失败: %w", err)
	}
	return nil
}

// keyringPath 返回本地密钥环文件路径
func keyringPath() string {
	return filepath.Join(CCBoxDir(), "secrets.json")
}
