package app

import (
	"fmt"

	"wps-enhancer-go/internal/license"
)

// LicenseStatusView 激活状态展示（前端激活页用）。
type LicenseStatusView struct {
	IsPro       bool   `json:"is_pro"`
	Type        string `json:"type"`         // 授权类型（"" = 未激活）
	ExpiresAt   int64  `json:"expires_at"`   // 到期时间戳（ms；0 = 未激活）
	ActivatedAt int64  `json:"activated_at"` // 激活时间戳（ms；0 = 未激活）
}

// LicenseActivateResult 激活操作结果（前端展示用）。
type LicenseActivateResult struct {
	OK            bool   `json:"ok"`
	Code          string `json:"code"`             // 失败错误码（契约 §8 速查表）
	PendingOnline bool   `json:"pending_online"`   // hybrid 放行（网络不可用，待在线确认）
	ExpiresAt     int64  `json:"expires_at"`       // 到期时间（平台或本地）
	Message       string `json:"message"`          // 用户可读提示
}

// LicenseStatus 返回当前激活状态。
func (a *App) LicenseStatus() *LicenseStatusView {
	view := &LicenseStatusView{}
	if a.licenseState == nil {
		return view
	}
	view.IsPro = a.licenseState.IsPro()
	if p := a.licenseState.Payload(); p != nil {
		view.Type = p.Type
		view.ExpiresAt = p.ExpiresAt
	}
	return view
}

// LicenseActivate 激活码激活（契约 §7 完整流程：验签 → 类型/过期检查 → 在线激活/hybrid 放行）。
func (a *App) LicenseActivate(key string) (*LicenseActivateResult, error) {
	if a.licenseState == nil {
		return nil, fmt.Errorf("激活模块未初始化")
	}
	fp, err := license.Fingerprint()
	if err != nil {
		return nil, fmt.Errorf("设备指纹获取失败: %v", err)
	}
	client := license.NewClient()
	result, err := license.ActivateFlow(key, fp, client, a.licenseState)
	if err != nil {
		return nil, translateErr(err)
	}
	view := &LicenseActivateResult{
		OK:            result.OK,
		Code:          result.Code,
		PendingOnline: result.PendingOnline,
		ExpiresAt:     result.ExpiresAt,
	}
	view.Message = activateMessage(result)
	return view, nil
}

// LicenseDeactivate 解绑当前设备（在线解绑成功才清除本地，契约 §5）。
func (a *App) LicenseDeactivate() error {
	if a.licenseState == nil {
		return fmt.Errorf("激活模块未初始化")
	}
	stored := a.licenseState.Stored()
	if stored == nil {
		return nil // 未激活，幂等成功
	}
	fp, err := license.Fingerprint()
	if err != nil {
		return fmt.Errorf("设备指纹获取失败: %v", err)
	}
	client := license.NewClient()
	if err := client.Deactivate(stored.Key, fp); err != nil {
		return translateErr(err)
	}
	if err := a.licenseState.Deactivate(); err != nil {
		return translateErr(err)
	}
	return nil
}

// activateMessage 激活结果 → 用户可读文案（契约 §8 错误码速查）。
func activateMessage(r *license.ActivateFlowResult) string {
	if r.OK {
		if r.PendingOnline {
			return "网络不可用，已按离线策略放行（将在联网后确认）"
		}
		return "激活成功"
	}
	switch r.Code {
	case "INVALID_KEY":
		return "激活码无效，请检查后重新输入"
	case "EXPIRED":
		return "激活码已过期，请联系开发者续费"
	case "REVOKED":
		return "该激活码已被撤销，请联系开发者"
	case "ALREADY_ACTIVATED":
		return "该激活码已绑定其他设备（可在原设备解绑后重试）"
	case "RATE_LIMITED":
		return "操作过于频繁，请稍后重试"
	case "TYPE_NOT_SUPPORTED":
		return "暂不支持该授权类型"
	case "NOT_FOUND":
		return "激活码不存在，请联系管理员"
	}
	return "激活失败：" + r.Code
}
