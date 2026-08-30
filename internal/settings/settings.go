// Package settings 定义全局设置（AppSettings）与默认值。
package settings

import (
	"os"
	"path/filepath"
	"runtime"

	"wps-enhancer-go/internal/template"
)

// AppSettings 全局应用设置（JSON 字段名与 Python 版 settings.json 逐字一致，保证老用户零迁移）。
type AppSettings struct {
	BuiltinColumns       []template.BuiltinColumn `json:"builtin_columns,omitempty"`
	PhoneValidate        bool                     `json:"phone_validate"`
	PhoneHighlight       bool                     `json:"phone_highlight"`
	PhoneMerge           bool                     `json:"phone_merge"`
	PhoneSeparators      []string                 `json:"phone_separators"`
	CSVEncoding          string                   `json:"csv_encoding"`
	CSVSeparator         string                   `json:"csv_separator"`
	TxtEncoding          string                   `json:"txt_encoding"`
	TxtSeparator         string                   `json:"txt_separator"`
	VCFFields            []string                 `json:"vcf_fields"`
	VCFNamePrefix        string                   `json:"vcf_name_prefix"`
	VCFNameSuffix        string                   `json:"vcf_name_suffix"`
	VCFTimestamp         bool                     `json:"vcf_timestamp"`
	VCFTimestampPosition string                   `json:"vcf_timestamp_position"`
	VCFShowImportGuide   bool                     `json:"vcf_show_import_guide"`
	DeclarationDetect    bool                     `json:"declaration_detect"`
	DeclarationKeywords  []string                 `json:"declaration_keywords"`
	SourceSeparator      string                   `json:"source_separator"`
	SourceEncoding       string                   `json:"source_encoding"`
	LogDebug             bool                     `json:"log_debug"`
	LogRetainDays        int                      `json:"log_retain_days"`
	LogAutoClean         bool                     `json:"log_auto_clean"`
	AutoUpdateEnabled    bool                     `json:"auto_update_enabled"`
	UseSystemProxy       bool                     `json:"use_system_proxy"`
	UpdateURL            string                   `json:"update_url"`
	DownloadDir          string                   `json:"download_dir"`
	InstallDir           string                   `json:"install_dir"`
}

// DefaultDeclarationKeywords 声明行检测默认关键词（覆盖常见表格导出平台）。
var DefaultDeclarationKeywords = []string{
	"企查查", "天眼查", "爱企查", "启信宝", "水滴信用",
	"导出数据", "导出声明", "数据来源", "声明",
}

// DefaultPhoneSeparators 手机号分隔符默认值（同一姓名多个手机号时的常用分隔形式）。
var DefaultPhoneSeparators = []string{",", "，", ";", "；", "、", " ", "\n", "|"}

// DefaultVCFFields vcf 导出字段默认值。
var DefaultVCFFields = []string{"name", "phone", "company", "website"}

// DefaultUpdateURL 自动更新源（统一发布仓 my-software-releases；见 docs/gitee-releases.md）。
// 每产品独立分支：raw/{产品名}/update.json。发客户端前须先有该分支并完成至少一次 publish-gitee.sh。
const DefaultUpdateURL = "https://gitee.com/pikachuprogrammer01/my-software-releases/raw/wps-enhancer/update.json"

// DefaultLogRetainDays 日志保留天数默认值。
const DefaultLogRetainDays = 30

// defaultDownloadDir 默认下载目录（macOS/Windows 均为用户 Downloads 目录）。
func defaultDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "."
	}
	return filepath.Join(home, "Downloads")
}

// defaultInstallDir 默认安装目录（macOS /Applications，Windows %LOCALAPPDATA%\WPSEnhancer）。
func defaultInstallDir() string {
	if runtime.GOOS == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "."
			}
			local = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(local, "WPSEnhancer")
	}
	return "/Applications"
}

// New 返回默认设置（与 Python 版 AppSettings() 默认值一致）。
func New() *AppSettings {
	return &AppSettings{
		BuiltinColumns:       defaultBuiltinColumns(),
		PhoneValidate:        true,
		PhoneHighlight:       true,
		PhoneMerge:           false,
		PhoneSeparators:      append([]string(nil), DefaultPhoneSeparators...),
		CSVEncoding:          "utf-8-bom",
		CSVSeparator:         ",",
		TxtEncoding:          "utf-8-bom",
		TxtSeparator:         " ",
		VCFFields:            append([]string(nil), DefaultVCFFields...),
		VCFNamePrefix:        "vcf_",
		VCFNameSuffix:        "",
		VCFTimestamp:         true,
		VCFTimestampPosition: "prefix",
		VCFShowImportGuide:   true,
		DeclarationDetect:    true,
		DeclarationKeywords:  append([]string(nil), DefaultDeclarationKeywords...),
		SourceSeparator:      "auto",
		SourceEncoding:       "auto",
		LogDebug:             false,
		LogRetainDays:        DefaultLogRetainDays,
		LogAutoClean:         true,
		AutoUpdateEnabled:    true,
		UseSystemProxy:       true,
		UpdateURL:            DefaultUpdateURL,
		DownloadDir:          defaultDownloadDir(),
		InstallDir:           defaultInstallDir(),
	}
}

// defaultBuiltinColumns 默认内置列（姓名/手机/公司名/网址，与 Python 版一致）。
func defaultBuiltinColumns() []template.BuiltinColumn {
	return []template.BuiltinColumn{
		{Key: "name", Label: "姓名", Aliases: []string{"姓名", "姓", "名称", "联系人", "名字"}, VCFProp: "FN"},
		{Key: "phone", Label: "手机", Aliases: []string{"手机", "手机号", "电话", "联系电话", "有效手机号", "家庭手机", "手机号码", "联系方式"}, VCFProp: "TEL;TYPE=CELL"},
		{Key: "company", Label: "公司名", Aliases: []string{"公司", "公司名", "公司名称", "单位", "企业名称"}, VCFProp: "ORG"},
		{Key: "website", Label: "网址", Aliases: []string{"网址", "官网", "网站", "官网网址", "主页", "网址链接"}, VCFProp: "URL"},
	}
}

// VCFPropMap 收集非核心四字段的自定义 vCard 属性（核心字段由 excel.keyToVCF 固定解析）。
func (st *AppSettings) VCFPropMap() map[string]string {
	if st == nil {
		return nil
	}
	core := map[string]bool{"name": true, "phone": true, "company": true, "website": true}
	out := make(map[string]string)
	for _, c := range st.BuiltinColumns {
		if core[c.Key] || c.VCFProp == "" {
			continue
		}
		out[c.Key] = c.VCFProp
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
