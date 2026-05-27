package binary

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/cc-box/core/config"
)

const githubClaudeReleasesAPI = "https://api.github.com/repos/anthropics/claude-code/releases"
const githubDefaultReleaseLimit = 30

type GitHubDownloadProgress func(downloaded, total int64)

type GitHubDownloader func(ctx context.Context, url string, progress GitHubDownloadProgress) ([]byte, error)

var (
	githubClaudeReleasesAPIURL                  = githubClaudeReleasesAPI
	githubDownloadURL          GitHubDownloader = downloadURL
	githubNowUTC                                = func() time.Time { return time.Now().UTC() }
)

type GitHubClaudeRelease struct {
	Version                   string `json:"version"`
	Tag                       string `json:"tag"`
	Name                      string `json:"name"`
	PublishedAt               string `json:"publishedAt"`
	AssetName                 string `json:"assetName"`
	AssetSize                 int64  `json:"assetSize"`
	AssetDownloadURL          string `json:"assetDownloadUrl"`
	ShasumsDownloadURL        string `json:"shasumsDownloadUrl"`
	ShasumsSignatureURL       string `json:"shasumsSignatureUrl,omitempty"`
	SignatureVerification     string `json:"signatureVerification"`
	SignatureVerificationText string `json:"signatureVerificationText"`
}

type GitHubClaudeReleaseList struct {
	Platform      string                `json:"platform"`
	PlatformLabel string                `json:"platformLabel"`
	AssetName     string                `json:"assetName"`
	Supported     bool                  `json:"supported"`
	FetchedAt     string                `json:"fetchedAt"`
	FromCache     bool                  `json:"fromCache"`
	Limit         int                   `json:"limit"`
	Releases      []GitHubClaudeRelease `json:"releases"`
	Error         string                `json:"error,omitempty"`
}

type githubReleaseCache struct {
	Platform  string                `json:"platform"`
	FetchedAt string                `json:"fetchedAt"`
	Releases  []GitHubClaudeRelease `json:"releases"`
}

type githubAPIRelease struct {
	TagName     string           `json:"tag_name"`
	Name        string           `json:"name"`
	PublishedAt time.Time        `json:"published_at"`
	Assets      []githubAPIAsset `json:"assets"`
}

type githubAPIAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func CachedGitHubClaudeReleases(limit int) (*GitHubClaudeReleaseList, error) {
	list := newGitHubReleaseList(limit)
	cache, err := readGitHubReleaseCache()
	if err != nil {
		return list, nil
	}
	if cache.Platform != list.Platform {
		return list, nil
	}
	list.FromCache = true
	list.FetchedAt = cache.FetchedAt
	list.Releases = limitReleases(cache.Releases, normalizeReleaseLimit(limit))
	return list, nil
}

func RefreshGitHubClaudeReleases(ctx context.Context, limit int) (*GitHubClaudeReleaseList, error) {
	list := newGitHubReleaseList(limit)
	if !list.Supported {
		return list, nil
	}
	releases, err := fetchGitHubClaudeReleases(ctx, normalizeReleaseLimit(limit))
	if err != nil {
		cached, _ := CachedGitHubClaudeReleases(limit)
		if cached != nil {
			cached.Error = err.Error()
			return cached, err
		}
		list.Error = err.Error()
		return list, err
	}
	list.Releases = releases
	list.FetchedAt = githubNowUTC().Format(time.RFC3339)
	if err := writeGitHubReleaseCache(githubReleaseCache{Platform: list.Platform, FetchedAt: list.FetchedAt, Releases: releases}); err != nil {
		return nil, err
	}
	return list, nil
}

func FindGitHubClaudeRelease(ctx context.Context, version string) (*GitHubClaudeRelease, error) {
	version = cleanVersionToken(version)
	if version == "" {
		return nil, fmt.Errorf("版本号不能为空")
	}
	cached, _ := CachedGitHubClaudeReleases(120)
	if cached != nil {
		for _, rel := range cached.Releases {
			if rel.Version == version {
				return &rel, nil
			}
		}
	}
	releases, err := fetchGitHubClaudeReleases(ctx, 120)
	if err != nil {
		return nil, err
	}
	for _, rel := range releases {
		if rel.Version == version {
			return &rel, nil
		}
	}
	return nil, fmt.Errorf("GitHub Release 中未找到当前平台可安装版本 %s", version)
}

func newGitHubReleaseList(limit int) *GitHubClaudeReleaseList {
	platform := config.Platform()
	asset, supported := githubAssetNameForPlatform(platform)
	return &GitHubClaudeReleaseList{
		Platform:      platform,
		PlatformLabel: claudePlatformLabel(platform),
		AssetName:     asset,
		Supported:     supported,
		Limit:         normalizeReleaseLimit(limit),
	}
}

func fetchGitHubClaudeReleases(ctx context.Context, limit int) ([]GitHubClaudeRelease, error) {
	assetName, supported := githubAssetNameForPlatform(config.Platform())
	if !supported {
		return nil, fmt.Errorf("当前平台暂不支持 GitHub Release 安装: %s", config.Platform())
	}
	limit = normalizeReleaseLimit(limit)
	var out []GitHubClaudeRelease
	for page := 1; len(out) < limit && page <= 5; page++ {
		url := fmt.Sprintf("%s?per_page=100&page=%d", githubClaudeReleasesAPIURL, page)
		payload, err := downloadJSON(ctx, url)
		if err != nil {
			return nil, err
		}
		var apiReleases []githubAPIRelease
		if err := json.Unmarshal(payload, &apiReleases); err != nil {
			return nil, fmt.Errorf("解析 GitHub Releases 失败: %w", err)
		}
		if len(apiReleases) == 0 {
			break
		}
		for _, rel := range apiReleases {
			asset := findGitHubAsset(rel.Assets, assetName)
			shasums := findGitHubAsset(rel.Assets, "SHASUMS256.txt")
			if asset == nil || shasums == nil {
				continue
			}
			signature := findGitHubAsset(rel.Assets, "SHASUMS256.txt.sig")
			signatureURL := ""
			signatureVerification := "unavailable"
			if signature != nil {
				signatureURL = signature.BrowserDownloadURL
				signatureVerification = "not_verified"
			}
			version := cleanVersionToken(rel.TagName)
			if version == "" {
				continue
			}
			out = append(out, GitHubClaudeRelease{
				Version:                   version,
				Tag:                       rel.TagName,
				Name:                      rel.Name,
				PublishedAt:               rel.PublishedAt.UTC().Format(time.RFC3339),
				AssetName:                 asset.Name,
				AssetSize:                 asset.Size,
				AssetDownloadURL:          asset.BrowserDownloadURL,
				ShasumsDownloadURL:        shasums.BrowserDownloadURL,
				ShasumsSignatureURL:       signatureURL,
				SignatureVerification:     signatureVerification,
				SignatureVerificationText: githubSignatureVerificationText(signatureVerification),
			})
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func githubAssetNameForPlatform(platform string) (string, bool) {
	switch platform {
	case "windows-amd64":
		return "claude-win32-x64.zip", true
	case "darwin-arm64":
		return "claude-darwin-arm64.tar.gz", true
	case "linux-amd64":
		return "claude-linux-x64.tar.gz", true
	default:
		return "", false
	}
}

func findGitHubAsset(assets []githubAPIAsset, name string) *githubAPIAsset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

func githubSignatureVerificationText(status string) string {
	switch status {
	case "not_verified":
		return "签名校验未启用，安装时校验 SHA256"
	case "unavailable":
		return "未找到签名文件，安装时校验 SHA256"
	default:
		return status
	}
}

func githubSignatureWarning(release *GitHubClaudeRelease) string {
	if release != nil && release.ShasumsSignatureURL == "" {
		return "GitHub Release 已完成 SHA256 校验；未找到 SHASUMS256.txt.sig，跳过签名校验"
	}
	return "GitHub Release 已完成 SHA256 校验；签名校验未启用"
}

func downloadJSON(ctx context.Context, url string) ([]byte, error) {
	data, err := githubDownloadURL(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var githubHTTPClient = &http.Client{
	Timeout: 10 * time.Minute,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

func downloadURL(ctx context.Context, url string, progress GitHubDownloadProgress) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cc-box")
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("请求 %s 失败: HTTP %d %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if progress == nil {
		return io.ReadAll(resp.Body)
	}
	return readAllWithProgress(resp.Body, resp.ContentLength, progress)
}

func readAllWithProgress(reader io.Reader, total int64, progress GitHubDownloadProgress) ([]byte, error) {
	var out bytes.Buffer
	buf := make([]byte, 256*1024)
	downloaded := int64(0)
	lastEmit := time.Now()
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
			downloaded += int64(n)
			if time.Since(lastEmit) >= 150*time.Millisecond || (total > 0 && downloaded >= total) {
				progress(downloaded, total)
				lastEmit = time.Now()
			}
		}
		if err == io.EOF {
			progress(downloaded, total)
			return out.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func formatBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func verifySHA256Line(data []byte, shasums, assetName string) error {
	actual := sha256.Sum256(data)
	actualHex := hex.EncodeToString(actual[:])
	for _, line := range strings.Split(shasums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) != assetName {
			continue
		}
		if !strings.EqualFold(fields[0], actualHex) {
			return fmt.Errorf("GitHub Release hash 校验失败: %s", assetName)
		}
		return nil
	}
	return fmt.Errorf("SHASUMS256.txt 中未找到 %s", assetName)
}

func extractClaudeFromZip(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("解析 zip 失败: %w", err)
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || filepath.Base(file.Name) != "claude.exe" {
			continue
		}
		r, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return nil, fmt.Errorf("压缩包中未找到 claude.exe")
}

func extractClaudeFromTarGz(data []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("解析 tar.gz 失败: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.FileInfo().IsDir() || filepath.Base(header.Name) != "claude" {
			continue
		}
		return io.ReadAll(tr)
	}
	return nil, fmt.Errorf("压缩包中未找到 claude")
}

func readGitHubReleaseCache() (githubReleaseCache, error) {
	data, err := os.ReadFile(githubReleaseCachePath())
	if err != nil {
		return githubReleaseCache{}, err
	}
	var cache githubReleaseCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return githubReleaseCache{}, err
	}
	return cache, nil
}

func writeGitHubReleaseCache(cache githubReleaseCache) error {
	path := githubReleaseCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func githubReleaseCachePath() string {
	return filepath.Join(config.CCBoxDir(), "cache", "github-claude-releases.json")
}

func normalizeReleaseLimit(limit int) int {
	if limit <= 0 {
		return githubDefaultReleaseLimit
	}
	if limit > 150 {
		return 150
	}
	return limit
}

func limitReleases(releases []GitHubClaudeRelease, limit int) []GitHubClaudeRelease {
	if len(releases) <= limit {
		return releases
	}
	return releases[:limit]
}
