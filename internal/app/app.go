// Package app Wails 命令层（等价 Python 版 panel.py 流程编排 + ui 的交互后端）。
// 唯一允许捕获并翻译错误的地方：errors.Is 判定后返回语义化错误给前端。
//
// 版本与授权 vs 业务边界：
//   - 订阅集中在 license.go + internal/license；导入/模板/导出等业务 API 不嵌入激活流程。
//   - 需要能力门禁时仅调用 licenseState.IsPro()，core/ 保持零侵入。
//   - 版本号来自 internal/version（Version / 更新检查），与具体业务功能无关。
package app

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"wps-enhancer-go/internal/errs"
	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/license"
	"wps-enhancer-go/internal/logger"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/template"
	"wps-enhancer-go/internal/version"
)

// App Wails 服务：依赖通过字段注入（不读全局变量）。
type App struct {
	settings     *settings.AppSettings // 运行期设置（启动时从磁盘加载）
	configDir    string
	licenseState *license.State // 订阅激活状态（nil=未初始化）
}

// NewApp 创建应用服务。
func NewApp(configDir string) *App {
	st, _ := settings.Load(configDir + "/settings.json")
	licenseState := license.NewState(license.NewStore(configDir))
	slog.Info("应用启动", "version", version.Version, "config_dir", configDir)
	return &App{settings: st, configDir: configDir, licenseState: licenseState}
}

// ListSheets 列出文件的所有 Sheet（下拉选择用）。
// xls 源文件先转换为 xlsx 再处理（统一处理管线，决策见 docs/migration/08-status-report.md）。
func (a *App) ListSheets(path string) ([]excel.SheetSummary, error) {
	path, err := a.normalizeSourcePath(path)
	if err != nil {
		return nil, translateErr(err)
	}
	reader, err := excel.GetReader(path)
	if err != nil {
		return nil, translateErr(err)
	}
	summaries, err := reader.GetSheetSummaries(path)
	if err != nil {
		return nil, translateErr(err)
	}
	return summaries, nil
}

// ReadSheet 读取指定 Sheet（前端预览用，返回前 200 行避免大表卡顿）。
// xls 源文件先转换为 xlsx 再读取。
func (a *App) ReadSheet(path, sheetName string, skipDeclaration bool) (*excel.SheetData, error) {
	return logger.LogCall1("ReadSheet", func() (*excel.SheetData, error) {
		path, err := a.normalizeSourcePath(path)
		if err != nil {
			return nil, translateErr(err)
		}
		reader, err := excel.GetReader(path)
		if err != nil {
			return nil, translateErr(err)
		}
		data, err := reader.ReadSheet(path, sheetName, excel.ReadOptions{
			SkipDeclaration:     skipDeclaration && a.settings.DeclarationDetect,
			DeclarationKeywords: a.settings.DeclarationKeywords,
			Separator:           a.settings.SourceSeparator,
			Encoding:            a.settings.SourceEncoding,
		})
		if err != nil {
			return nil, translateErr(err)
		}
		if len(data.Rows) > 200 {
			data.Rows = data.Rows[:200]
		}
		return data, nil
	})
}

// normalizeSourcePath 源文件路径规范化：后缀检测（仅 xlsx/xls/csv）+ xls 转 xlsx。
func (a *App) normalizeSourcePath(path string) (string, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xlsm", ".xltx", ".xltm", ".csv":
		return path, nil
	case ".xls":
		converted, err := excel.ConvertXlsToXlsx(path)
		if err != nil {
			return "", err
		}
		return converted, nil
	default:
		return "", fmt.Errorf("%w: 不支持的源文件格式：%s（仅支持 xlsx / xls / csv）", errs.ErrFileRead, filepath.Ext(path))
	}
}

// defaultTemplate 内置默认模板（姓名/手机/公司名/网址，与 Python 版内置一致）。
func defaultTemplate() *template.Template {
	return &template.Template{
		Name: "通讯录",
		Columns: []template.TemplateColumn{
			{Key: "name", Name: "姓名", Enabled: true},
			{Key: "phone", Name: "手机", Enabled: true},
			{Key: "company", Name: "公司名", Enabled: true},
			{Key: "website", Name: "网址", Enabled: true},
		},
	}
}

// translateErr 错误翻译：sentinel error → 前端语义化错误码（同时记录日志）。
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	coded := err
	switch {
	case errors.Is(err, errs.ErrDataProcess):
		coded = fmt.Errorf("DATA_PROCESS_ERROR: %v", err)
	case errors.Is(err, errs.ErrFileRead):
		coded = fmt.Errorf("FILE_READ_ERROR: %v", err)
	case errors.Is(err, errs.ErrFileWrite):
		coded = fmt.Errorf("FILE_WRITE_ERROR: %v", err)
	case errors.Is(err, errs.ErrTemplate):
		coded = fmt.Errorf("TEMPLATE_ERROR: %v", err)
	case errors.Is(err, errs.ErrSettings):
		coded = fmt.Errorf("SETTINGS_ERROR: %v", err)
	case errors.Is(err, errs.ErrNetwork):
		coded = fmt.Errorf("NETWORK_ERROR: %v", err)
	}
	slog.Error("命令执行失败", "error", coded.Error())
	return coded
}
