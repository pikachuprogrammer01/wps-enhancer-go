// license-probe 激活码联调 CLI：解析 + 验签 + 在线激活/解绑 + 状态展示。
//
// 用法：
//
//	go run ./cmd/license-probe "WPS-xxxx.yyyy"            # 解析+验签+激活
//	go run ./cmd/license-probe "WPS-xxxx.yyyy" --deactivate  # 解绑
//	go run ./cmd/license-probe --fingerprint              # 显示本机设备指纹
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"wps-enhancer-go/internal/license"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println("用法: license-probe <激活码> [--deactivate] | --fingerprint")
		os.Exit(1)
	}
	if args[0] == "--fingerprint" {
		fp, err := license.Fingerprint()
		if err != nil {
			fmt.Println("指纹获取失败:", err)
			os.Exit(1)
		}
		fmt.Println("本机设备指纹:", fp)
		return
	}

	key := args[0]
	deactivate := len(args) > 1 && args[1] == "--deactivate"
	fp, err := license.Fingerprint()
	if err != nil {
		fmt.Println("指纹获取失败:", err)
		os.Exit(1)
	}

	// ① 解析 + 本地验签
	payload, _, err := license.ParseAndVerify(key)
	if err != nil {
		fmt.Println("❌ 本地验签失败:", err)
		os.Exit(1)
	}
	fmt.Println("✅ 本地验签通过")
	body, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Println("payload:", string(body))
	fmt.Printf("过期时间: %s（%s）\n", payload.ExpiresAtTime(), map[bool]string{true: "已过期", false: "有效"}[payload.IsExpired()])

	client := license.NewClientWithTimeout(20 * time.Second)
	state := license.NewState(license.NewStore(tempDir()))

	if deactivate {
		fmt.Println("→ 在线解绑…")
		if err := client.Deactivate(key, fp); err != nil {
			fmt.Println("❌ 解绑失败:", err)
			os.Exit(1)
		}
		_ = state.Deactivate()
		fmt.Println("✅ 解绑成功（本机激活状态已清除）")
		return
	}

	// ②③ 完整激活流程
	fmt.Println("→ 在线激活…")
	result, err := license.ActivateFlow(key, fp, client, state)
	if err != nil {
		fmt.Println("❌ 激活流程异常:", err)
		os.Exit(1)
	}
	if !result.OK {
		fmt.Printf("❌ 激活失败: code=%s\n", result.Code)
		os.Exit(1)
	}
	if result.PendingOnline {
		fmt.Println("⚠️  网络不可用，已按 hybrid 策略本地放行（待在线确认）")
	} else {
		fmt.Println("✅ 激活成功（平台记录到期:", unixMs(result.ExpiresAt), "）")
	}
	fmt.Println("当前激活状态: Pro =", state.IsPro())
}

// unixMs epoch 毫秒 → 可读时间。
func unixMs(ms int64) string {
	if ms == 0 {
		return "-"
	}
	return time.UnixMilli(ms).Format("2006-01-02")
}

// tempDir 探针用临时激活存储目录（CLI 工具不污染应用配置）。
func tempDir() string {
	dir, err := os.MkdirTemp("", "license-probe-")
	if err != nil {
		return "."
	}
	return dir
}
