package license

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"wps-enhancer-go/internal/errs"
)

// baseURL 平台激活服务地址（契约：生产域名 license.pikachu01.me，路径前缀 /api）。
const baseURL = "https://license.pikachu01.me/api"

// ActivateResult 激活响应（契约 §4：成功 {ok:true, expiresAt}；失败 {ok:false, code}）。
type ActivateResult struct {
	OK        bool   `json:"ok"`
	Code      string `json:"code"`      // 失败时的错误码（INVALID_KEY/REVOKED/...）
	ExpiresAt int64  `json:"expiresAt"` // 激活成功时的平台记录到期时间（epoch ms）
}

// Client 激活码 API 客户端（3s 超时，契约硬性要求）。
type Client struct {
	http   *http.Client
	apiURL string
}

// NewClient 创建激活 API 客户端（3s 超时，契约硬性要求）。
func NewClient() *Client {
	return NewClientWithTimeout(3 * time.Second)
}

// NewClientWithTimeout 创建带自定义超时的客户端（联调/慢网络场景用）。
func NewClientWithTimeout(timeout time.Duration) *Client {
	return &Client{
		http:   &http.Client{Timeout: timeout},
		apiURL: baseURL,
	}
}

// Activate 调用 POST /activate（契约 §4）。
// 返回语义化结果：业务分支按 result.Code 判定（不依赖 HTTP 状态码，契约 §8 明确）。
func (c *Client) Activate(key, deviceFingerprint string) (*ActivateResult, error) {
	result, err := c.post("/activate", key, deviceFingerprint)
	if err != nil {
		return nil, err
	}
	if !result.OK {
		return result, fmt.Errorf("%w: %s", errs.ErrDataProcess, result.Code)
	}
	return result, nil
}

// Deactivate 调用 POST /deactivate（契约 §5）。
func (c *Client) Deactivate(key, deviceFingerprint string) error {
	result, err := c.post("/deactivate", key, deviceFingerprint)
	if err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("%w: %s", errs.ErrDataProcess, result.Code)
	}
	return nil
}

// post 通用 POST（网络失败返回 ErrNetwork；业务失败返回解析后的 result）。
func (c *Client) post(path, key, deviceFingerprint string) (*ActivateResult, error) {
	body, _ := json.Marshal(map[string]string{
		"key":               key,
		"deviceFingerprint": deviceFingerprint,
	})
	req, err := http.NewRequest(http.MethodPost, c.apiURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errs.ErrNetwork, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: 网络不可用，请稍后重试", errs.ErrNetwork)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result ActivateResult
	_ = json.Unmarshal(respBody, &result)
	// 契约 §8：按 code 分支处理，不依赖 HTTP 状态码；HTTP 非 200 时按 body 结构解析
	if result.Code == "" && resp.StatusCode != 200 {
		result.OK = false
		result.Code = "UNKNOWN_ERROR"
	}
	return &result, nil
}

// Fingerprint 采集硬件维度 → SHA-256 hex（契约 §3；维度组合随版本可升级）。
// 平台存「当前指纹 + 指纹历史（最近 5 次）」，任一命中即视为同设备。
func Fingerprint() (string, error) {
	var sb strings.Builder
	if runtime.GOOS == "windows" {
		// ⚠️ 不要用 wmic（Win11 已移除）：用 PowerShell CIM
		out, err := runCommand("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_BaseBoard | Select-Object -ExpandProperty SerialNumber")
		if err == nil {
			sb.WriteString(strings.TrimSpace(out))
		}
		out, err = runCommand("powershell", "-NoProfile", "-Command",
			"Get-CimInstance Win32_DiskDrive | Select-Object -First 1 -ExpandProperty SerialNumber")
		if err == nil {
			sb.WriteString("|" + strings.TrimSpace(out))
		}
	} else {
		// macOS：ioreg 提取 IOPlatformUUID（稳定跨重启）
		out, err := runCommand("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if strings.Contains(line, "IOPlatformUUID") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						sb.WriteString(strings.TrimSpace(strings.Trim(parts[1], `"`)))
					}
					break
				}
			}
		}
	}
	// 降级：命令失败时用机器名+MAC 组合（保证激活流程不卡死）
	if sb.Len() == 0 {
		hostname, _ := runCommand("hostname")
		sb.WriteString("fallback:" + strings.TrimSpace(hostname))
	}
	return sha256Hex(sb.String()), nil
}

// runCommand 执行系统命令（3s 超时，失败返回错误）。
func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := timeoutContext(3 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
