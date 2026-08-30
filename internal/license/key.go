// Package license 订阅体系：激活码解析/验签/在线验证/设备指纹/持久化/状态判定。
// 上游契约：docs/wps-activation-policy.md + LicenseHub 客户端激活对接文档（权威）。
package license

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wps-enhancer-go/internal/errs"
)

// ProductPrefixes 已知产品前缀集合（契约：按最长优先匹配，前缀自带 '-'）。
var ProductPrefixes = []string{"WPS-", "TF-"}

// Payload 激活码载荷（契约 payload_version=1；字段只追加不删改）。
type Payload struct {
	ID         string         `json:"id"`          // 激活码唯一 ID（签发时生成）
	Version    int            `json:"version"`     // payload 版本（当前 1）
	Type       string         `json:"type"`        // 授权类型："pro" | "trial" | "buyout"
	ExpiresAt  int64          `json:"expiresAt"`   // 到期时间（epoch ms；buyout 为远期固定值）
	MaxDevices int            `json:"maxDevices"`  // 设备上限（当前签发固定 1）
	BatchID    string         `json:"batchId"`     // 批次标识（可选）
	IssuedAt   int64          `json:"issuedAt"`    // 签发时间（epoch ms）
	Features   map[string]any `json:"features"`    // 功能开关（可选，透传）
}

// ErrTypeNotSupported 授权类型不受支持（当前仅 pro，buyout/trial 拒绝）。
var ErrTypeNotSupported = fmt.Errorf("%w: 暂不支持该授权类型", errs.ErrDataProcess)

// matchPrefix 按已知前缀集合最长优先匹配，返回匹配的前缀与剩余部分。
func matchPrefix(rawKey string) (prefix, rest string, ok bool) {
	for _, p := range ProductPrefixes {
		if len(p) > len(prefix) && strings.HasPrefix(rawKey, p) {
			prefix = p
			ok = true
		}
	}
	if !ok {
		return "", "", false
	}
	return prefix, rawKey[len(prefix):], true
}

// ParseKey 解析激活码：前缀匹配 → 首个 '.' 分割 payload/签名 → 去分组符 → base64 解码。
// 返回载荷、payloadB64（验签对象，契约 §3：签名对象 = payloadB64 字符串字节）与签名。
func ParseKey(rawKey string) (*Payload, string, []byte, error) {
	_, rest, ok := matchPrefix(rawKey)
	if !ok {
		return nil, "", nil, fmt.Errorf("%w: 激活码格式错误（未知前缀）", errs.ErrDataProcess)
	}
	dot := strings.Index(rest, ".")
	if dot <= 0 {
		return nil, "", nil, fmt.Errorf("%w: 激活码格式错误", errs.ErrDataProcess)
	}
	ungroup := func(s string) string { return strings.ReplaceAll(s, "-", "") }
	payloadB64 := ungroup(rest[:dot])
	sigB64 := ungroup(rest[dot+1:])
	if payloadB64 == "" || sigB64 == "" {
		return nil, "", nil, fmt.Errorf("%w: 激活码格式错误", errs.ErrDataProcess)
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: 激活码无效（载荷编码错误）", errs.ErrDataProcess)
	}
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, "", nil, fmt.Errorf("%w: 激活码无效（签名编码错误）", errs.ErrDataProcess)
	}
	var payload Payload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, "", nil, fmt.Errorf("%w: 激活码无效（载荷解析失败）", errs.ErrDataProcess)
	}
	return &payload, payloadB64, sig, nil
}

// IsExpired 判断载荷是否过期（expiresAt < now）。
func (p *Payload) IsExpired() bool {
	return p.ExpiresAt < time.Now().UnixMilli()
}

// ExpiresAtTime 过期时间的人类可读形式（UI 展示用）。
func (p *Payload) ExpiresAtTime() string {
	return time.UnixMilli(p.ExpiresAt).Format("2006-01-02")
}

// ValidateType 校验授权类型（产品策略：仅 pro；buyout/trial 明确拒绝）。
func ValidateType(p *Payload) error {
	if p.Type != "pro" {
		return fmt.Errorf("%w: %s", ErrTypeNotSupported, p.Type)
	}
	return nil
}
