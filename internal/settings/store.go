package settings

import (
	"encoding/json"
	"fmt"
	"os"

	"wps-enhancer-go/internal/errs"
	"wps-enhancer-go/internal/template"
)

// file 设置文件的磁盘结构（与 Python 版 settings.json 逐字一致）。
// 顶层含 builtin_columns（内置列定义）与 app_settings（应用设置）。
type file struct {
	BuiltinColumns []template.BuiltinColumn `json:"builtin_columns,omitempty"`
	AppSettings    map[string]any           `json:"app_settings"`
}

// Load 从文件读取设置（文件缺失或损坏时回退默认值）。
// 与 Python 版 _load_from_file 一致：app_settings 内逐字段带默认值兜底。
func Load(path string) (*AppSettings, error) {
	defaults := New()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaults, nil
		}
		return defaults, nil // 读取失败回退默认（与 Python 版一致，仅记录日志）
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		return defaults, nil // 损坏回退默认
	}
	if f.AppSettings == nil {
		f.AppSettings = map[string]any{}
	}
	if len(f.BuiltinColumns) > 0 {
		defaults.BuiltinColumns = f.BuiltinColumns
	}
	st := defaults
	st.PhoneValidate = boolField(f.AppSettings, "phone_validate", defaults.PhoneValidate)
	st.PhoneHighlight = boolField(f.AppSettings, "phone_highlight", defaults.PhoneHighlight)
	st.PhoneMerge = boolField(f.AppSettings, "phone_merge", defaults.PhoneMerge)
	st.PhoneSeparators = strSliceField(f.AppSettings, "phone_separators", defaults.PhoneSeparators)
	st.CSVEncoding = strField(f.AppSettings, "csv_encoding", defaults.CSVEncoding)
	st.CSVSeparator = strField(f.AppSettings, "csv_separator", defaults.CSVSeparator)
	st.TxtEncoding = strField(f.AppSettings, "txt_encoding", defaults.TxtEncoding)
	st.TxtSeparator = strField(f.AppSettings, "txt_separator", defaults.TxtSeparator)
	st.VCFFields = strSliceField(f.AppSettings, "vcf_fields", defaults.VCFFields)
	st.VCFNamePrefix = strField(f.AppSettings, "vcf_name_prefix", defaults.VCFNamePrefix)
	st.VCFNameSuffix = strField(f.AppSettings, "vcf_name_suffix", defaults.VCFNameSuffix)
	st.VCFTimestamp = boolField(f.AppSettings, "vcf_timestamp", defaults.VCFTimestamp)
	st.VCFTimestampPosition = strField(f.AppSettings, "vcf_timestamp_position", defaults.VCFTimestampPosition)
	st.VCFShowImportGuide = boolField(f.AppSettings, "vcf_show_import_guide", defaults.VCFShowImportGuide)
	st.DeclarationDetect = boolField(f.AppSettings, "declaration_detect", defaults.DeclarationDetect)
	st.DeclarationKeywords = strSliceField(f.AppSettings, "declaration_keywords", defaults.DeclarationKeywords)
	st.SourceSeparator = strField(f.AppSettings, "source_separator", defaults.SourceSeparator)
	st.SourceEncoding = strField(f.AppSettings, "source_encoding", defaults.SourceEncoding)
	st.LogDebug = boolField(f.AppSettings, "log_debug", defaults.LogDebug)
	st.LogRetainDays = intField(f.AppSettings, "log_retain_days", defaults.LogRetainDays)
	st.LogAutoClean = boolField(f.AppSettings, "log_auto_clean", defaults.LogAutoClean)
	st.AutoUpdateEnabled = boolField(f.AppSettings, "auto_update_enabled", defaults.AutoUpdateEnabled)
	st.UseSystemProxy = boolField(f.AppSettings, "use_system_proxy", defaults.UseSystemProxy)
	// 空值视为未设置（旧版设置残留可能存过空 update_url），回退默认源（与 Python 版一致）
	st.UpdateURL = strField(f.AppSettings, "update_url", "")
	if st.UpdateURL == "" {
		st.UpdateURL = DefaultUpdateURL // 勿用 defaults.UpdateURL：st 与 defaults 同指针，空串已写回
	}
	st.DownloadDir = strField(f.AppSettings, "download_dir", defaults.DownloadDir)
	st.InstallDir = strField(f.AppSettings, "install_dir", defaults.InstallDir)
	return st, nil
}

// Save 将设置写入文件（原子写入：先写临时文件再替换）。
func Save(path string, st *AppSettings) error {
	f := file{
		BuiltinColumns: st.BuiltinColumns,
		AppSettings: map[string]any{
			"phone_validate":         st.PhoneValidate,
			"phone_highlight":        st.PhoneHighlight,
			"phone_merge":            st.PhoneMerge,
			"phone_separators":       st.PhoneSeparators,
			"csv_encoding":           st.CSVEncoding,
			"csv_separator":          st.CSVSeparator,
			"txt_encoding":           st.TxtEncoding,
			"txt_separator":          st.TxtSeparator,
			"vcf_fields":             st.VCFFields,
			"vcf_name_prefix":        st.VCFNamePrefix,
			"vcf_name_suffix":        st.VCFNameSuffix,
			"vcf_timestamp":          st.VCFTimestamp,
			"vcf_timestamp_position": st.VCFTimestampPosition,
			"vcf_show_import_guide":  st.VCFShowImportGuide,
			"declaration_detect":     st.DeclarationDetect,
			"declaration_keywords":   st.DeclarationKeywords,
			"source_separator":       st.SourceSeparator,
			"source_encoding":        st.SourceEncoding,
			"log_debug":              st.LogDebug,
			"log_retain_days":        st.LogRetainDays,
			"log_auto_clean":         st.LogAutoClean,
			"auto_update_enabled":    st.AutoUpdateEnabled,
			"use_system_proxy":       st.UseSystemProxy,
			"update_url":             st.UpdateURL,
			"download_dir":           st.DownloadDir,
			"install_dir":            st.InstallDir,
		},
	}
	body, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: 无法写入设置文件 '%s': %v", errs.ErrSettings, path, err)
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, body, 0o644); err != nil {
		return fmt.Errorf("%w: 无法写入设置文件 '%s': %v", errs.ErrSettings, path, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("%w: 无法写入设置文件 '%s': %v", errs.ErrSettings, path, err)
	}
	return nil
}

// boolField 从 map 读取布尔字段，缺失/类型不符用默认值。
func boolField(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

// strField 从 map 读取字符串字段，缺失/类型不符用默认值。
func strField(m map[string]any, key string, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

// intField 从 map 读取整数字段，缺失/类型不符用默认值（兼容 JSON 浮点形态）。
func intField(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return def
}

// strSliceField 从 map 读取字符串切片字段，缺失/类型不符用默认值。
func strSliceField(m map[string]any, key string, def []string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return def
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
