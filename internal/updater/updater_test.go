package updater

import (
	"archive/zip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCompareVersions 版本比较（对齐 Python 版语义：数字段、v 前缀、非数字忽略）。
func TestCompareVersions(t *testing.T) {
	cases := []struct {
		local, remote string
		want          int
	}{
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.1.0", "1.1.0", 0},
		{"v1.1.0", "1.1.0", 0},
		{"1.1", "1.1.0", -1},           // 段数不同
		{"1.1.0-beta", "1.1.0", 0},     // beta 无数字段，相等（对齐 Python）
		{"1.0.0", "1.0.0-rc1", -1},     // rc1 的数字 1 被计入（对齐 Python 提取全部数字）
		{"2.0", "1.9.9", 1},            // 段数不同
		{"1.2.3.4", "1.2.3", 1},        // 更多段
	}
	for _, c := range cases {
		if got := CompareVersions(c.local, c.remote); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.local, c.remote, got, c.want)
		}
	}
}

// TestCheckViaCustom 自定义更新源（httptest 模拟 update.json，多平台与旧格式）。
func TestCheckViaCustom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": "1.2.0",
			"urls": {"macos-arm64": "https://example.com/pkg.zip", "windows-x86_64": "https://example.com/win.zip"},
			"notes": "更新说明"
		}`))
	}))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	info, err := CheckLatestRelease(srv.URL, client)
	if err != nil {
		t.Fatalf("CheckLatestRelease: %v", err)
	}
	if info.TagName != "v1.2.0" {
		t.Errorf("TagName = %q, want v1.2.0", info.TagName)
	}
	if info.ZipURL == nil || *info.ZipURL == "" {
		t.Error("ZipURL 应为当前平台的下载地址")
	}
	if info.Notes != "更新说明" {
		t.Errorf("Notes = %q", info.Notes)
	}

	// 格式错误必须暴露（不静默回退）
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer badSrv.Close()
	if _, err := CheckLatestRelease(badSrv.URL, client); err == nil {
		t.Error("格式错误的源应返回错误")
	} else if !strings.Contains(err.Error(), "格式错误") {
		t.Errorf("错误信息应包含'格式错误': %v", err)
	}
}

// TestCheckViaGitHubAPI GitHub API 回退路径（httptest 模拟，注入 API 端点）。
func TestCheckViaGitHubAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.3.0",
			"html_url": "https://github.com/x/releases/v1.3.0",
			"published_at": "2026-01-01T00:00:00Z",
			"body": "说明",
			"assets": [
				{"name": "WPSEnhancer-macOS-arm64.zip", "browser_download_url": "https://example.com/mac.zip", "size": 123},
				{"name": "WPSEnhancer-Windows-x86_64.zip", "browser_download_url": "https://example.com/win.zip", "size": 456}
			]
		}`))
	}))
	defer srv.Close()

	old := githubAPIURL
	githubAPIURL = srv.URL
	defer func() { githubAPIURL = old }()

	client := NewClient(5 * time.Second)
	info, err := checkViaGitHubAPI(client)
	if err != nil {
		t.Fatalf("checkViaGitHubAPI: %v", err)
	}
	if info.TagName != "v1.3.0" {
		t.Errorf("TagName = %q, want v1.3.0", info.TagName)
	}
	if info.ZipURL == nil || *info.ZipURL == "" {
		t.Error("ZipURL 应为匹配平台架构的资产")
	}
	// 资产匹配：当前平台架构应命中对应资产
	expected := "mac"
	if platformLabel() == "windows" {
		expected = "win"
	}
	if !strings.Contains(*info.ZipURL, expected) {
		t.Errorf("ZipURL = %q, 应包含 %q", *info.ZipURL, expected)
	}
}

// TestDownloadAndVerify 下载 + zip 完整性校验。
func TestDownloadAndVerify(t *testing.T) {
	// 构造 zip 内容
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "pkg.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("hello.txt")
	_, _ = w.Write([]byte("hello"))
	_ = zw.Close()
	_ = f.Close()

	srv := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer srv.Close()

	client := NewClient(5 * time.Second)
	dest := filepath.Join(dir, "downloaded.zip")
	if err := DownloadFile(srv.URL+"/pkg.zip", dest, client, nil); err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if err := VerifyZipIntegrity(dest); err != nil {
		t.Errorf("VerifyZipIntegrity: %v", err)
	}
	// 损坏文件应报错
	badPath := filepath.Join(dir, "bad.zip")
	_ = os.WriteFile(badPath, []byte("not a zip"), 0o644)
	if err := VerifyZipIntegrity(badPath); err == nil {
		t.Error("损坏 zip 应报错")
	}
}
