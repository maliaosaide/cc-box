// WebDAV HTTP 客户端
// 支持 ETag、条件请求、Basic 认证
package webdav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// Client WebDAV 客户端
type Client struct {
	baseURL  string
	username string
	password string
	http     *http.Client
}

// NewClient 创建 WebDAV 客户端
func NewClient(url, username, password string) *Client {
	// 确保 URL 以 / 结尾
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return &Client{
		baseURL:  url,
		username: username,
		password: password,
		http: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetTimeout 设置 HTTP 超时
func (c *Client) SetTimeout(d time.Duration) {
	c.http.Timeout = d
}

// url 拼接完整 URL
func (c *Client) url(p string) string {
	p = strings.TrimPrefix(p, "/")
	return c.baseURL + p
}

// newRequest 创建带认证的 HTTP 请求
func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	return req, nil
}

// GET 下载文件，返回内容和 ETag
func (c *Client) GET(remotePath string) ([]byte, string, error) {
	req, err := c.newRequest("GET", c.url(remotePath), nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("GET %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("GET %s 返回 %d: %s", remotePath, resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应体失败: %w", err)
	}

	etag := resp.Header.Get("ETag")
	return data, etag, nil
}

// PUT 上传文件，支持 If-Match 条件请求
// 返回新的 ETag
func (c *Client) PUT(remotePath string, data []byte, ifMatch string) (string, error) {
	req, err := c.newRequest("PUT", c.url(remotePath), bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("PUT %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return "", ErrConflict
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PUT %s 返回 %d: %s", remotePath, resp.StatusCode, string(body))
	}

	return resp.Header.Get("ETag"), nil
}

// HEAD 获取文件元信息（大小、ETag、是否存在）
func (c *Client) HEAD(remotePath string) (*FileInfo, error) {
	req, err := c.newRequest("HEAD", c.url(remotePath), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HEAD %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HEAD %s 返回 %d", remotePath, resp.StatusCode)
	}

	info := &FileInfo{
		Path: remotePath,
		Size: resp.ContentLength,
		ETag: resp.Header.Get("ETag"),
	}
	return info, nil
}

// MKCOL 创建目录（幂等，已存在不报错）
func (c *Client) MKCOL(remotePath string) error {
	req, err := c.newRequest("MKCOL", c.url(remotePath), nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("MKCOL %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	// 405/301 表示已存在
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusMovedPermanently {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("MKCOL %s 返回 %d", remotePath, resp.StatusCode)
	}
	return nil
}

// DELETE 删除文件或目录
func (c *Client) DELETE(remotePath string) error {
	req, err := c.newRequest("DELETE", c.url(remotePath), nil)
	if err != nil {
		return err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("DELETE %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("DELETE %s 返回 %d", remotePath, resp.StatusCode)
	}
	return nil
}

// PROPFIND 列出目录内容（depth=1）或获取文件属性（depth=0）
func (c *Client) PROPFIND(remotePath string, depth int) ([]FileInfo, error) {
	propfindBody := `<?xml version="1.0" encoding="UTF-8"?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:getetag/>
    <d:getcontentlength/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`

	req, err := c.newRequest("PROPFIND", c.url(remotePath), strings.NewReader(propfindBody))
	if err != nil {
		return nil, err
	}

	depthStr := "0"
	if depth == 1 {
		depthStr = "1"
	}
	req.Header.Set("Depth", depthStr)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("PROPFIND %s 失败: %w", remotePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("PROPFIND %s 返回 %d", remotePath, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取 PROPFIND 响应失败: %w", err)
	}

	return parseMultiStatus(body, c.url(remotePath))
}

// Exists 检查文件或目录是否存在
func (c *Client) Exists(remotePath string) (bool, error) {
	_, err := c.HEAD(remotePath)
	if err == ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// EnsureDir 确保远程路径的父目录存在
func (c *Client) EnsureDir(remotePath string) error {
	dir := path.Dir(remotePath)
	parts := strings.Split(strings.Trim(dir, "/"), "/")
	current := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		current += "/" + p
		if err := c.MKCOL(current); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", current, err)
		}
	}
	return nil
}

// parseMultiStatus 解析 WebDAV PROPFIND 多状态响应
func parseMultiStatus(data []byte, baseURL string) ([]FileInfo, error) {
	var ms multistatus
	if err := xml.Unmarshal(data, &ms); err != nil {
		return nil, fmt.Errorf("解析 PROPFIND XML 失败: %w", err)
	}

	var files []FileInfo
	for _, resp := range ms.Responses {
		href, err := decodeHref(resp.Href)
		if err != nil {
			continue
		}

		// 跳过根路径自身
		if strings.TrimSuffix(href, "/") == strings.TrimSuffix(baseURL, "/") {
			continue
		}

		prop := resp.PropStat[0].Prop
		isDir := prop.ResourceType.Collection != ""

		// 从完整 URL 中提取相对路径
		relPath := strings.TrimPrefix(href, baseURL)
		relPath = strings.TrimPrefix(relPath, "/")

		files = append(files, FileInfo{
			Path:  relPath,
			IsDir: isDir,
			Size:  prop.ContentLength,
			ETag:  strings.Trim(prop.ETag, `"`),
		})
	}
	return files, nil
}

func decodeHref(s string) (string, error) {
	// XML 已自动解码实体
	return s, nil
}
