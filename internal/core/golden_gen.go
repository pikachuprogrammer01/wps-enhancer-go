package core

import (
	"encoding/json"

	"wps-enhancer-go/internal/excel"
	"wps-enhancer-go/internal/settings"
	"wps-enhancer-go/internal/template"
)

// GoldenCase processor golden 文件中的单个用例（与 processor_golden.json 结构一致）。
type GoldenCase struct {
	Name   string         `json:"name"`
	Func   string         `json:"func"`
	Input  map[string]any `json:"input"`
	Output any            `json:"output"`
}

// GoldenFile processor golden 文件根结构。
type GoldenFile struct {
	Cases []GoldenCase `json:"cases"`
}

var goldenBuiltins = []template.BuiltinColumn{
	{Key: "name", Label: "姓名", Aliases: []string{"姓名", "姓", "名称"}},
	{Key: "phone", Label: "手机", Aliases: []string{"手机", "手机号", "电话", "有效手机号"}},
	{Key: "company", Label: "公司名", Aliases: []string{"公司", "公司名称"}},
	{Key: "website", Label: "网址", Aliases: []string{"网址", "官网"}},
}

// GenerateGoldenCases 生成 processor 全量 golden 用例（等价 gen_golden.py）。
func GenerateGoldenCases() []GoldenCase {
	var cases []GoldenCase
	add := func(c GoldenCase) { cases = append(cases, c) }

	for _, raw := range []struct {
		raw  string
		seps []string
	}{
		{"138;139", []string{";"}},
		{" 138 ; 139 ", []string{";"}},
		{"138;;139", []string{";"}},
		{"", []string{";"}},
		{"138,139；140、141 | 142", []string{",", "，", ";", "；", "、", " ", "\n", "|"}},
		{"138 139", []string{" "}},
	} {
		add(GoldenCase{
			Name:   "split_phones_" + raw.raw,
			Func:   "split_phones",
			Input:  map[string]any{"raw_phone": raw.raw, "separators": raw.seps},
			Output: normalizeGolden(SplitPhones(raw.raw, raw.seps)),
		})
	}

	for _, phone := range []string{"13800000000", "+8613800000000", "", "12345", "23800000000", "1380000000a"} {
		add(GoldenCase{
			Name:   "validate_phone_" + phone,
			Func:   "validate_phone",
			Input:  map[string]any{"phone": phone},
			Output: ValidatePhone(phone),
		})
	}

	t := goldenTemplate("通讯录", [][2]string{{"name", "姓名"}, {"phone", "手机"}, {"company", "公司名"}})
	add(GoldenCase{
		Name: "match_default", Func: "match_columns",
		Input: goldenMatchInput([]string{"姓名", "手机号", "公司"}, t, nil),
		Output: normalizeGolden(template.MatchColumns(
			[]string{"姓名", "手机号", "公司"}, t, goldenBuiltins, nil)),
	})
	add(GoldenCase{
		Name: "match_manual", Func: "match_columns",
		Input: goldenMatchInput([]string{"a", "b", "c"}, t, map[string]string{"name": "a", "phone": ""}),
		Output: normalizeGolden(template.MatchColumns(
			[]string{"a", "b", "c"}, t, goldenBuiltins, map[string]string{"name": "a", "phone": ""})),
	})
	tmplWeb := goldenTemplate("多列", [][2]string{{"name", "姓名"}, {"phone", "手机"}, {"website", "网址"}})
	add(GoldenCase{
		Name: "match_unmatched", Func: "match_columns",
		Input: goldenMatchInput([]string{"姓名", "手机号"}, tmplWeb, nil),
		Output: normalizeGolden(template.MatchColumns(
			[]string{"姓名", "手机号"}, tmplWeb, goldenBuiltins, nil)),
	})

	sheetDef := goldenSheet(
		[]string{"姓名", "手机号", "公司"},
		[]map[string]string{
			{"姓名": "张三", "手机号": "13800000000;13900000000", "公司": "A公司"},
			{"姓名": "李四", "手机号": "bad", "公司": "B公司"},
			{"姓名": "王五", "手机号": "", "公司": ""},
		},
	)
	stDefault := goldenSettings(nil)
	matches := template.MatchColumns(sheetDef.Headers, t, goldenBuiltins, nil)

	addPreview := func(name string, data *excel.SheetData, tmpl *template.Template, st *settings.AppSettings, manual map[string]string) (*PreviewData, []template.ColumnMatch) {
		ms := template.MatchColumns(data.Headers, tmpl, goldenBuiltins, manual)
		preview, err := BuildPreviewData(data, tmpl, ms, st)
		if err != nil {
			panic(err)
		}
		add(GoldenCase{
			Name: name, Func: "build_preview_data",
			Input:  goldenPreviewInput(data, tmpl, manual, st),
			Output: normalizeGolden(preview),
		})
		return preview, ms
	}
	addWrite := func(name, previewRef string, preview *PreviewData, tmpl *template.Template, ms []template.ColumnMatch, st *settings.AppSettings, path string) {
		add(GoldenCase{
			Name: name, Func: "build_write_request",
			Input:  map[string]any{"preview_ref": previewRef, "output_path": path},
			Output: normalizeGolden(BuildWriteRequest(preview, tmpl, ms, st, path)),
		})
	}

	p, _ := addPreview("preview_default", sheetDef, t, stDefault, nil)

	stMerge := goldenSettings(map[string]any{"phone_merge": true})
	p2, _ := addPreview("preview_merge", sheetDef, t, stMerge, nil)
	addWrite("write_request_merge", "preview_merge", p2, t, matches, stMerge, "/tmp/out.xlsx")

	stNoVal := goldenSettings(map[string]any{"phone_validate": false})
	addPreview("preview_validate_off", sheetDef, t, stNoVal, nil)

	tmplNoPhone := goldenTemplate("无手机", [][2]string{{"name", "姓名"}, {"company", "公司名"}})
	addPreview("preview_no_phone", sheetDef, tmplNoPhone, stDefault, nil)

	addPreview("preview_unmatched_col", sheetDef, tmplWeb, stDefault, nil)

	dataDup := goldenSheet(
		[]string{"姓名", "手机"},
		[]map[string]string{
			{"姓名": "张三", "手机": "13800000000"},
			{"姓名": "李四", "手机": "13900000000"},
			{"姓名": "张三", "手机": "13700000000"},
		},
	)
	tmplDup := goldenTemplate("同名", [][2]string{{"name", "姓名"}, {"phone", "手机"}})
	stDup := goldenSettings(map[string]any{"phone_merge": true, "phone_validate": false})
	pDup, mDup := addPreview("preview_merge_group", dataDup, tmplDup, stDup, nil)
	addWrite("write_request_merge_group", "preview_merge_group", pDup, tmplDup, mDup, stDup, "/tmp/out.xlsx")

	dataNoM := goldenSheet([]string{"姓名", "手机"}, []map[string]string{
		{"姓名": "张三", "手机": "13800000000,13900000000"},
	})
	stNoM := goldenSettings(map[string]any{"phone_merge": false, "phone_validate": false})
	pNoM, mNoM := addPreview("preview_no_merge", dataNoM, t, stNoM, nil)
	addWrite("write_request_no_merge", "preview_no_merge", pNoM, t, mNoM, stNoM, "/tmp/out.xlsx")

	stHL := goldenSettings(map[string]any{"phone_highlight": false})
	pHL, _ := addPreview("preview_highlight_off", sheetDef, t, stHL, nil)
	addWrite("write_request_no_highlight", "preview_highlight_off", pHL, t, matches, stHL, "/tmp/out.xlsx")

	stEnc := goldenSettings(map[string]any{
		"csv_encoding": "gbk", "txt_encoding": "utf-16", "txt_separator": "、",
		"vcf_fields": []any{"name", "phone"}, "vcf_name_prefix": "客户-",
	})
	pEnc, _ := addPreview("preview_export_params", sheetDef, t, stEnc, nil)
	for _, ext := range []string{"txt", "csv", "xlsx"} {
		addWrite("write_request_enc_"+ext, "preview_export_params", pEnc, t, matches, stEnc, "/tmp/out."+ext)
	}

	stVcf := goldenSettings(map[string]any{
		"vcf_fields": []any{"name", "phone"}, "vcf_name_prefix": "客户-",
		"phone_merge": true,
	})
	pVcf, _ := addPreview("preview_vcf", sheetDef, t, stVcf, nil)
	addWrite("write_request_vcf_indexed", "preview_vcf", pVcf, t, matches, stVcf, "/tmp/out.vcf")

	stDisp := goldenSettings(map[string]any{
		"vcf_fields": []any{"name", "phone"}, "vcf_name_prefix": "客户-",
		"vcf_name_suffix": "-尾",
	})
	pDisp, mDisp := addPreview("preview_display_src", sheetDef, t, stDisp, nil)
	for _, fmt := range []string{"csv", "txt", "vcf"} {
		h, rows := BuildPreviewDisplay(pDisp, mDisp, stDisp, fmt)
		add(GoldenCase{
			Name:   "preview_display_" + fmt,
			Func:   "build_preview_display",
			Input:  map[string]any{"preview_ref": "preview_display_src", "fmt": fmt},
			Output: normalizeGolden(map[string]any{"headers": h, "rows": rows}),
		})
	}

	stTxt := goldenSettings(map[string]any{
		"txt_separator": "、", "vcf_fields": []any{"name", "phone"}, "vcf_name_prefix": "客户-",
	})
	pTxt, mTxt := addPreview("preview_text_src", sheetDef, t, stTxt, nil)
	for _, fmt := range []string{"csv", "txt", "vcf"} {
		text, err := BuildTextPreview(pTxt, mTxt, stTxt, fmt, 30)
		if err != nil {
			panic(err)
		}
		add(GoldenCase{
			Name:   "text_preview_" + fmt,
			Func:   "build_text_preview",
			Input:  map[string]any{"preview_ref": "preview_text_src", "fmt": fmt},
			Output: text,
		})
	}

	_ = p // preview_default referenced by dependency chain only
	return cases
}

// goldenTemplate 构造 golden 用模板。
func goldenTemplate(name string, cols [][2]string) *template.Template {
	tmpl := &template.Template{Name: name, Mappings: map[string]string{}}
	for _, c := range cols {
		tmpl.Columns = append(tmpl.Columns, template.TemplateColumn{Key: c[0], Name: c[1], Enabled: true})
	}
	return tmpl
}

// goldenSheet 构造 golden 用 SheetData。
func goldenSheet(headers []string, rows []map[string]string) *excel.SheetData {
	return &excel.SheetData{SheetName: "s", Headers: headers, Rows: rows}
}

// goldenSettings 构造 golden 用设置（vcf_timestamp 固定 false，避免日期漂移）。
func goldenSettings(overrides map[string]any) *settings.AppSettings {
	st := settings.New()
	st.VCFTimestamp = false
	if overrides == nil {
		return st
	}
	raw, _ := json.Marshal(overrides)
	_ = json.Unmarshal(raw, st)
	st.VCFTimestamp = false
	return st
}

func goldenTplDict(tmpl *template.Template) map[string]any {
	return normalizeGolden(map[string]any{
		"name": tmpl.Name, "columns": tmpl.Columns, "mappings": tmpl.Mappings,
	}).(map[string]any)
}

func goldenSettingsDict(st *settings.AppSettings) map[string]any {
	return map[string]any{
		"phone_validate": st.PhoneValidate, "phone_highlight": st.PhoneHighlight,
		"phone_merge": st.PhoneMerge, "phone_separators": st.PhoneSeparators,
		"csv_encoding": st.CSVEncoding, "txt_encoding": st.TxtEncoding,
		"txt_separator": st.TxtSeparator, "vcf_fields": st.VCFFields,
		"vcf_name_prefix": st.VCFNamePrefix, "vcf_name_suffix": st.VCFNameSuffix,
		"vcf_timestamp": false, "vcf_timestamp_position": st.VCFTimestampPosition,
	}
}

func goldenMatchInput(headers []string, tmpl *template.Template, manual map[string]string) map[string]any {
	if manual == nil {
		manual = map[string]string{}
	}
	return map[string]any{
		"headers": headers, "template": goldenTplDict(tmpl),
		"builtins": normalizeGolden(goldenBuiltins), "manual_map": manual,
	}
}

func goldenPreviewInput(data *excel.SheetData, tmpl *template.Template, manual map[string]string, st *settings.AppSettings) map[string]any {
	if manual == nil {
		manual = map[string]string{}
	}
	rows := make([]any, len(data.Rows))
	for i, r := range data.Rows {
		row := make(map[string]any, len(r))
		for k, v := range r {
			row[k] = v
		}
		rows[i] = row
	}
	return map[string]any{
		"headers": data.Headers, "rows": rows,
		"template": goldenTplDict(tmpl), "builtins": normalizeGolden(goldenBuiltins),
		"manual_map": manual, "settings": goldenSettingsDict(st),
	}
}

// normalizeGolden JSON 往返消除类型差异（与 processor_test.normalize 一致）。
func normalizeGolden(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
