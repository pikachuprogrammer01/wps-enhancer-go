package updater

// 系统代理检测（对齐 core/updater.py 的 _system_proxy/_apply_system_proxy）。
// 打包版从 Finder/资源管理器启动时没有 shell 代理环境变量，
// Go 默认 Transport 不会走系统代理（国内访问 GitHub 的主要卡点之一）。

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// scutil 输出行形如 "HTTPSProxy : 127.0.0.1" / "HTTPSPort : 7890"
var (
	scutilHTTPSProxy = regexp.MustCompile(`HTTPSProxy\s*:\s*([^\s]+)`)
	scutilHTTPSPort  = regexp.MustCompile(`HTTPSPort\s*:\s*(\d+)`)
	scutilHTTPProxy  = regexp.MustCompile(`HTTPProxy\s*:\s*([^\s]+)`)
	scutilHTTPPort   = regexp.MustCompile(`HTTPPort\s*:\s*(\d+)`)
	regProxyServer   = regexp.MustCompile(`ProxyServer\s+REG_SZ\s+(\S+)`)
	regProxyEnable   = regexp.MustCompile(`ProxyEnable\s+REG_DWORD\s+0x([0-9a-fA-F]+)`)
)

// SystemProxyURL 读取系统代理地址，返回 http://host:port；未启用或读取失败返回空串。
func SystemProxyURL() string {
	switch runtime.GOOS {
	case "darwin":
		return darwinSystemProxy()
	case "windows":
		return windowsSystemProxy()
	}
	return ""
}

// darwinSystemProxy 通过 scutil --proxy 读取 macOS 系统代理（优先 HTTPS，回退 HTTP）。
func darwinSystemProxy() string {
	out, err := exec.Command("scutil", "--proxy").Output()
	if err != nil {
		return ""
	}
	return darwinSystemProxyFromOutput(string(out))
}

// darwinSystemProxyFromOutput 解析 scutil --proxy 输出（纯函数，可测）。
func darwinSystemProxyFromOutput(s string) string {
	host := findGroup(scutilHTTPSProxy, s)
	port := findGroup(scutilHTTPSPort, s)
	if host == "" {
		host = findGroup(scutilHTTPProxy, s)
		if p := findGroup(scutilHTTPPort, s); p != "" {
			port = p
		}
	}
	if host == "" {
		return ""
	}
	if port == "" {
		port = "80"
	}
	return "http://" + host + ":" + port
}

// windowsSystemProxy 通过注册表读取 Windows 系统代理（ProxyEnable=1 时才生效）。
func windowsSystemProxy() string {
	key := `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`
	enableOut, err := exec.Command("reg", "query", key, "/v", "ProxyEnable").Output()
	if err != nil {
		return ""
	}
	serverOut, err := exec.Command("reg", "query", key, "/v", "ProxyServer").Output()
	if err != nil {
		return ""
	}
	return windowsSystemProxyFromOutput(string(enableOut), string(serverOut))
}

// windowsSystemProxyFromOutput 解析 reg query 输出（纯函数，可测）。
func windowsSystemProxyFromOutput(enableOut, serverOut string) string {
	if findGroup(regProxyEnable, enableOut) == "0" || findGroup(regProxyEnable, enableOut) == "" {
		return ""
	}
	server := findGroup(regProxyServer, serverOut)
	if server == "" {
		return ""
	}
	// 形如 "http=127.0.0.1:7890;https=127.0.0.1:7891" 时取 https 段；裸 "host:port" 原样返回
	for _, part := range strings.Split(server, ";") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(part), "https="); ok && v != "" {
			return normalizeProxy(v)
		}
	}
	return normalizeProxy(server)
}

// ApplySystemProxy 把系统代理写入进程环境变量（仅在未显式设置 HTTPS_PROXY 时），供默认 Transport 使用。
func ApplySystemProxy() {
	if os.Getenv("HTTPS_PROXY") != "" || os.Getenv("https_proxy") != "" {
		return
	}
	proxy := SystemProxyURL()
	if proxy == "" {
		return
	}
	_ = os.Setenv("HTTPS_PROXY", proxy)
	_ = os.Setenv("HTTP_PROXY", proxy)
}

// findGroup 返回正则第一个分组匹配，无匹配返回空串。
func findGroup(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// normalizeProxy 补全代理地址的 http:// 前缀（缺 scheme 时）。
func normalizeProxy(hostPort string) string {
	if strings.Contains(hostPort, "://") {
		return hostPort
	}
	if _, err := strconv.Atoi(hostPort); err == nil {
		return "" // 纯端口号无法定位主机，视为无效
	}
	return "http://" + hostPort
}
