package license

import (
	"errors"
	"fmt"

	"wps-enhancer-go/internal/errs"
)

// ActivateFlowResult 激活流程结果（契约 §7 五分支 + hybrid 容错）。
type ActivateFlowResult struct {
	OK              bool   `json:"ok"`
	Code            string `json:"code"`             // 失败错误码（契约 §8 速查表）
	PendingOnline   bool   `json:"pending_online"`   // hybrid：本地验签通过但网络失败，暂放行待在线确认
	ExpiresAt       int64  `json:"expiresAt"`        // 平台记录或本地载荷的到期时间
}

// ActivateFlow 完整激活流程（契约 §7 ①②③）：
//
//	① 本地解析 + 验签（失败 → INVALID_KEY 终止）
//	② 类型校验（仅 pro）+ 本地 expiresAt 检查（过期 → EXPIRED）
//	③ POST /activate：
//	   - 200 → 持久化，进入激活态
//	   - 409/403/410/429 → 按 code 返回
//	   - 网络错误 → hybrid 容错：本地验签已通过可暂放行（pending_online），后台重试在线确认
func ActivateFlow(key, deviceFingerprint string, client *Client, state *State) (*ActivateFlowResult, error) {
	// ① 本地解析 + 验签
	payload, _, err := ParseAndVerify(key)
	if err != nil {
		return &ActivateFlowResult{OK: false, Code: "INVALID_KEY"}, nil
	}
	// ② 类型校验（产品策略：仅 pro）
	if err := ValidateType(payload); err != nil {
		return &ActivateFlowResult{OK: false, Code: "TYPE_NOT_SUPPORTED"}, nil
	}
	// ② 本地过期检查
	if payload.IsExpired() {
		return &ActivateFlowResult{OK: false, Code: "EXPIRED", ExpiresAt: payload.ExpiresAt}, nil
	}
	// ③ 在线激活
	result, err := client.Activate(key, deviceFingerprint)
	if err != nil {
		if errors.Is(err, errs.ErrNetwork) {
			// hybrid 容错：本地验签已通过 → 持久化放行（离线可用），后台重试在线确认
			if err := state.Activate(key, payload); err != nil {
				return nil, fmt.Errorf("%w: 激活状态保存失败: %v", errs.ErrSettings, err)
			}
			return &ActivateFlowResult{
				OK: true, PendingOnline: true, ExpiresAt: payload.ExpiresAt,
			}, nil
		}
		// 业务错误（REVOKED/ALREADY_ACTIVATED/...）：透传 code
		return &ActivateFlowResult{OK: false, Code: result.Code, ExpiresAt: payload.ExpiresAt}, nil
	}
	// 200：持久化激活状态
	if err := state.Activate(key, payload); err != nil {
		return nil, fmt.Errorf("%w: 激活状态保存失败: %v", errs.ErrSettings, err)
	}
	return &ActivateFlowResult{OK: true, ExpiresAt: result.ExpiresAt}, nil
}
