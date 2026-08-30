// Package updater 自动更新：版本比较 / 更新源检查 / 下载（对齐 core/updater.py）。
package updater

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"wps-enhancer-go/internal/errs"
)

// DefaultTimeout 网络请求默认超时（秒）。
const DefaultTimeout = 10

// userAgent 请求 UA（GitHub API 要求）。
const userAgent = "WPSEnhancer-Updater/1.0"

// ReleaseInfo GitHub Release 摘要信息（与 Python 版 ReleaseInfo 一致）。
type ReleaseInfo struct {
	TagName     string  `json:"tag_name"`     // 版本 tag，如 v1.1.0
	HTMLURL     string  `json:"html_url"`     // Release 页面
	ZipURL      *string `json:"zip_url"`      // 更新包下载地址（zip 资产）
	ZipSize     *int64  `json:"zip_size"`     // 更新包字节数
	PublishedAt string  `json:"published_at"` // 发布时间
	Notes       string  `json:"notes"`        // 更新说明
}

// CompareVersions 语义化版本比较（支持 v 前缀）：local < remote 返回 -1，相等 0，大于 1。
// 仅比较数字段（如 1.2.3）；非数字段（-beta 等）忽略。
func CompareVersions(local, remote string) int {
	a, b := numbers(local), numbers(remote)
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return 0
}

// numbers 提取版本号中的数字段（"1.2.3-beta" → [1,2,3]）。
func numbers(tag string) []int {
	parts := strings.Split(strings.TrimSpace(tag), ".")
	parts[0] = strings.TrimLeft(parts[0], "vV")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		digits := strings.Builder{}
		for _, ch := range p {
			if ch >= '0' && ch <= '9' {
				digits.WriteRune(ch)
			}
		}
		if digits.Len() == 0 {
			out = append(out, 0)
		} else {
			var n int
			fmt.Sscanf(digits.String(), "%d", &n)
			out = append(out, n)
		}
	}
	return out
}

// platformLabel 返回当前平台标签（macos/windows）。
func platformLabel() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	return "macos"
}

// archLabel 返回当前架构标签（arm64/x86_64/x86）。
func archLabel() string {
	switch runtime.GOARCH {
	case "arm64":
		return "arm64"
	case "amd64":
		return "x86_64"
	case "386":
		return "x86"
	}
	return runtime.GOARCH
}

// updateSource 自定义更新源 JSON（update.json，兼容多平台与旧单平台格式）。
type updateSource struct {
	Version string            `json:"version"`
	URLs    map[string]string `json:"urls"`
	URL     string            `json:"url"`
	Notes   string            `json:"notes"`
}

// CheckLatestRelease 查询最新版本（自定义源优先，失败回退 GitHub API；均失败返回错误）。
// 对齐 Python 版策略：update.json 格式错误必须暴露（不静默回退）。
func CheckLatestRelease(updateURL string, client *http.Client) (*ReleaseInfo, error) {
	if updateURL != "" {
		info, err := checkViaCustom(updateURL, client)
		if err == nil {
			return info, nil
		}
		if strings.Contains(err.Error(), "格式错误") {
			return nil, err // 源配置错误必须暴露，回退会掩盖问题
		}
		// 网络不可达 → 回退 GitHub API
	}
	return checkViaGitHubAPI(client)
}

// checkViaCustom 通过自定义更新源查询最新版本（update.json）。
func checkViaCustom(updateURL string, client *http.Client) (*ReleaseInfo, error) {
	req, err := http.NewRequest(http.MethodGet, updateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: 自定义更新源格式错误: %v", errs.ErrNetwork, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: 无法连接自定义更新源: %v", errs.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%w: 自定义更新源返回状态码 %d", errs.ErrNetwork, resp.StatusCode)
	}
	var src updateSource
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		return nil, fmt.Errorf("%w: 自定义更新源格式错误（非 JSON）: %v", errs.ErrNetwork, err)
	}
	version := strings.TrimSpace(src.Version)
	if version == "" {
		return nil, fmt.Errorf("%w: 自定义更新源格式错误：缺少 version 字段", errs.ErrNetwork)
	}
	tag := version
	if !strings.HasPrefix(version, "v") {
		tag = "v" + version
	}
	zipURL := ""
	if len(src.URLs) > 0 {
		key := platformLabel() + "-" + archLabel()
		zipURL = strings.TrimSpace(src.URLs[key])
		if zipURL == "" {
			return nil, fmt.Errorf("%w: 自定义更新源格式错误：缺少 %s 的下载地址（urls.%s）",
				errs.ErrNetwork, key, key)
		}
	} else {
		zipURL = strings.TrimSpace(src.URL)
		if zipURL == "" {
			return nil, fmt.Errorf("%w: 自定义更新源格式错误：缺少 url 字段", errs.ErrNetwork)
		}
	}
	return &ReleaseInfo{
		TagName: tag,
		HTMLURL: updateURL,
		ZipURL:  &zipURL,
		Notes:   src.Notes,
	}, nil
}

// githubAsset GitHub Release 资产（匹配用）。
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// githubRelease GitHub API 响应结构（仅取所需字段）。
type githubRelease struct {
	TagName   string        `json:"tag_name"`
	HTMLURL   string        `json:"html_url"`
	Published string        `json:"published_at"`
	Body      string        `json:"body"`
	Assets    []githubAsset `json:"assets"`
}

// githubAPIURL GitHub Releases API 端点（Go 版独立仓库；测试可注入替换）。
var githubAPIURL = "https://api.github.com/repos/pikachuprogrammer01/wps-enhancer-go/releases/latest"

// checkViaGitHubAPI 通过 GitHub Releases API 查询最新版本。
// 资产匹配：优先「平台+架构」精确匹配，回退「仅平台」匹配（兼容旧资产）。
func checkViaGitHubAPI(client *http.Client) (*ReleaseInfo, error) {
	url := githubAPIURL
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: 检查更新失败: %v", errs.ErrNetwork, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: 检查更新失败（API 不可达）: %v", errs.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("%w: 仓库无 Release", errs.ErrNetwork)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%w: GitHub API 返回状态码 %d", errs.ErrNetwork, resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("%w: GitHub API 响应解析失败: %v", errs.ErrNetwork, err)
	}
	platform, arch := platformLabel(), archLabel()
	asset := findAsset(rel.Assets, platform, arch)
	info := &ReleaseInfo{
		TagName:     rel.TagName,
		HTMLURL:     rel.HTMLURL,
		PublishedAt: rel.Published,
		Notes:       rel.Body,
	}
	if asset != nil {
		info.ZipURL = &asset.BrowserDownloadURL
		info.ZipSize = &asset.Size
	}
	return info, nil
}

// findAsset 按「平台+架构」精确匹配，回退「仅平台」匹配。
func findAsset(assets []githubAsset, platform, arch string) *githubAsset {
	lower := func(s string) string { return strings.ToLower(s) }
	for i := range assets {
		name := lower(assets[i].Name)
		if strings.Contains(name, platform) && strings.Contains(name, arch) {
			return &assets[i]
		}
	}
	for i := range assets {
		if lower(assets[i].Name) == platform+".zip" ||
			strings.Contains(lower(assets[i].Name), platform) && strings.HasSuffix(lower(assets[i].Name), ".zip") {
			return &assets[i]
		}
	}
	return nil
}

// DownloadFile 下载文件到 dest（带进度回调，网络错误返回 ErrNetwork）。
func DownloadFile(url, dest string, client *http.Client, progress func(done, total int64)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("%w: 下载失败：状态码 %d", errs.ErrNetwork, resp.StatusCode)
	}
	tmp := dest + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, err)
	}
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				_ = out.Close()
				return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, werr)
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = out.Close()
			_ = os.Remove(tmp)
			return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, readErr)
		}
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("%w: 下载失败: %v", errs.ErrNetwork, err)
	}
	return nil
}

// VerifyZipIntegrity 校验 zip 文件完整性（打开失败视为损坏）。
func VerifyZipIntegrity(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%w: 更新包损坏: %v", errs.ErrNetwork, err)
	}
	defer zr.Close()
	return nil
}

// NewClient 创建带超时的 HTTP 客户端。
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
