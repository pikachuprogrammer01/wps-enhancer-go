package app

import (
	"encoding/json"
	"fmt"

	"wps-enhancer-go/internal/settings"
)

// SettingsGet 返回当前全局设置（设置页展示用）。
func (a *App) SettingsGet() *settings.AppSettings {
	return a.settings
}

// SettingsUpdate 合并更新全局设置（前端传变更字段，缺失字段保持原值）并持久化。
// 支持字段（JSON key 与 settings.json 逐字一致）：
//
//	phone_validate / phone_highlight / phone_merge / phone_separators /
//	csv_encoding / txt_encoding / txt_separator / vcf_fields /
//	vcf_name_prefix / vcf_name_suffix / vcf_timestamp / vcf_timestamp_position /
//	vcf_show_import_guide / declaration_detect / declaration_keywords
func (a *App) SettingsUpdate(partial map[string]any) error {
	raw, err := json.Marshal(partial)
	if err != nil {
		return fmt.Errorf("设置格式错误: %v", err)
	}
	if err := json.Unmarshal(raw, a.settings); err != nil {
		return fmt.Errorf("设置格式错误: %v", err)
	}
	if err := settings.Save(a.configDir+"/settings.json", a.settings); err != nil {
		return translateErr(err)
	}
	return nil
}
