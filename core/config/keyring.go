package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type SecretStore interface {
	Get(service, username string) (string, error)
	Set(service, username, password string) error
}

type fileSecretStore struct {
	path string
}

func keyringGet(service, username string) (string, error) {
	return defaultSecretStore().Get(service, username)
}

func keyringSet(service, username, password string) error {
	return defaultSecretStore().Set(service, username, password)
}

func defaultSecretStore() SecretStore {
	return fileSecretStore{path: secretStorePath()}
}

func (s fileSecretStore) Get(service, username string) (string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", fmt.Errorf("密码存储文件不存在: %w", err)
	}

	entries, err := readSecretEntries(data)
	if err != nil {
		return "", err
	}

	key := secretEntryKey(service, username)
	pass, ok := entries[key]
	if !ok {
		return "", fmt.Errorf("未找到 %s 的密码", key)
	}
	return pass, nil
}

func (s fileSecretStore) Set(service, username, password string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return fmt.Errorf("创建密码存储目录失败: %w", err)
	}

	entries := make(map[string]string)
	data, err := os.ReadFile(s.path)
	if err == nil {
		entries, err = readSecretEntries(data)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取密码存储失败: %w", err)
	}

	entries[secretEntryKey(service, username)] = password

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化密码存储失败: %w", err)
	}
	if err := os.WriteFile(s.path, out, 0600); err != nil {
		return fmt.Errorf("写入密码存储失败: %w", err)
	}
	return nil
}

func readSecretEntries(data []byte) (map[string]string, error) {
	entries := make(map[string]string)
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("密码存储文件格式错误: %w", err)
	}
	return entries, nil
}

func secretEntryKey(service, username string) string {
	return service + ":" + username
}

func secretStorePath() string {
	return filepath.Join(CCBoxDir(), "secrets.json")
}
