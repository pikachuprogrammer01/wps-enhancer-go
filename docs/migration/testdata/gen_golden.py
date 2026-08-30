#!/usr/bin/env python3
"""生成 processor 纯函数的 golden 测试夹具（迁移对照基准，v2：input 自包含）。

输出: docs/migration/testdata/processor_golden.json
用法: python3.14 gen_golden.py
设计:
- 每个用例 input 自包含（template/builtins/settings/manual_map 全部序列化）
- 依赖 preview 的用例通过 preview_ref 引用，Go 测试先执行依赖用例
"""
import importlib.util
import json
import sys
from dataclasses import asdict
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent.parent
sys.path.insert(0, str(ROOT))

# 直接加载 processor.py（绕过 features/contacts_import/__init__.py 的 PyQt6 导入）
_spec = importlib.util.spec_from_file_location(
    "processor_standalone", str(ROOT / "features/contacts_import/processor.py"))
processor = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(processor)

from core.file_io.base import SheetData
from core.settings import AppSettings
from core.template.config import BuiltinColumn, Template, TemplateColumn
from core.template.matcher import match_columns

split_phones = processor.split_phones
validate_phone = processor.validate_phone
build_preview_data = processor.build_preview_data
build_preview_display = processor.build_preview_display
build_text_preview = processor.build_text_preview
build_write_request = processor.build_write_request

def S(**kw):
    """AppSettings 构造 helper：默认 vcf_timestamp=False（消除 golden 日期漂移）。"""
    kw.setdefault("vcf_timestamp", False)
    return AppSettings(**kw)

BUILTINS = [
    BuiltinColumn(key="name", label="姓名", aliases=["姓名", "姓", "名称"]),
    BuiltinColumn(key="phone", label="手机", aliases=["手机", "手机号", "电话", "有效手机号"]),
    BuiltinColumn(key="company", label="公司名", aliases=["公司", "公司名称"]),
    BuiltinColumn(key="website", label="网址", aliases=["网址", "官网"]),
]

def template(columns, name="通讯录"):
    return Template(name=name, columns=[TemplateColumn(key=k, name=n) for k, n in columns])

def sheet(headers, rows):
    return SheetData(sheet_name="s", headers=headers, rows=rows)

def clean(obj):
    """dataclass/元组键 → JSON 可序列化结构。"""
    if hasattr(obj, "__dataclass_fields__"):
        return clean(asdict(obj))
    if isinstance(obj, dict):
        return {str(k): clean(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [clean(v) for v in obj]
    return obj

def tpl_dict(tmpl):
    """模板 → 可序列化 dict（用于用例 input）。"""
    return {"name": tmpl.name, "columns": clean(tmpl.columns), "mappings": dict(tmpl.mappings)}

def builtins_dict(cols):
    """内置列列表 → 可序列化 dict。"""
    return clean(cols)

def settings_dict(s):
    """AppSettings → 可序列化 dict（processor 用到的最小字段集）。

    vcf_timestamp 固定 False：时间戳依赖生成日期，进入 golden 会产生日期漂移
    （跨天重跑测试即失败）；vcf 时间戳逻辑由专项用例覆盖（vcf_timestamp=False 的
    prefix/suffix 行为），不在此对照。
    """
    return {
        "phone_validate": s.phone_validate,
        "phone_highlight": s.phone_highlight,
        "phone_merge": s.phone_merge,
        "phone_separators": list(s.phone_separators),
        "csv_encoding": s.csv_encoding,
        "txt_encoding": s.txt_encoding,
        "txt_separator": s.txt_separator,
        "vcf_fields": list(s.vcf_fields),
        "vcf_name_prefix": s.vcf_name_prefix,
        "vcf_name_suffix": s.vcf_name_suffix,
        "vcf_timestamp": False,
        "vcf_timestamp_position": s.vcf_timestamp_position,
    }

CASES = []

def add(case):
    CASES.append(case)

# ---- split_phones ----
for raw, seps in [
    ("138;139", [";"]),
    (" 138 ; 139 ", [";"]),
    ("138;;139", [";"]),
    ("", [";"]),
    ("138,139；140、141 | 142", [",", "，", ";", "；", "、", " ", "\n", "|"]),
    ("138 139", [" "]),
]:
    add({"name": f"split_phones_{raw!r}", "func": "split_phones",
         "input": {"raw_phone": raw, "separators": seps},
         "output": split_phones(raw, seps)})

# ---- validate_phone ----
for phone in ["13800000000", "+8613800000000", "", "12345", "23800000000", "1380000000a"]:
    add({"name": f"validate_phone_{phone!r}", "func": "validate_phone",
         "input": {"phone": phone}, "output": validate_phone(phone)})

# ---- match_columns（默认/手动/未匹配） ----
t = template([("name", "姓名"), ("phone", "手机"), ("company", "公司名")])
add({"name": "match_default", "func": "match_columns",
     "input": {"headers": ["姓名", "手机号", "公司"], "template": tpl_dict(t),
               "builtins": builtins_dict(BUILTINS), "manual_map": {}},
     "output": [clean(m) for m in match_columns(["姓名", "手机号", "公司"], t, BUILTINS)]})
add({"name": "match_manual", "func": "match_columns",
     "input": {"headers": ["a", "b", "c"], "template": tpl_dict(t),
               "builtins": builtins_dict(BUILTINS), "manual_map": {"name": "a", "phone": ""}},
     "output": [clean(m) for m in match_columns(["a", "b", "c"], t, BUILTINS, {"name": "a", "phone": ""})]})
tmpl_website = template([("name", "姓名"), ("phone", "手机"), ("website", "网址")])
add({"name": "match_unmatched", "func": "match_columns",
     "input": {"headers": ["姓名", "手机号"], "template": tpl_dict(tmpl_website),
               "builtins": builtins_dict(BUILTINS), "manual_map": {}},
     "output": [clean(m) for m in match_columns(["姓名", "手机号"], tmpl_website, BUILTINS)]})

# ---- 数据场景 ----
def sheet_default():
    return sheet(["姓名", "手机号", "公司"], [
        {"姓名": "张三", "手机号": "13800000000;13900000000", "公司": "A公司"},
        {"姓名": "李四", "手机号": "bad", "公司": "B公司"},
        {"姓名": "王五", "手机号": "", "公司": ""},
    ])

def case_preview(name, data, tmpl, settings, manual_map=None):
    matches = match_columns(data.headers, tmpl, BUILTINS, manual_map or {})
    preview = build_preview_data(data, tmpl, matches, settings)
    add({"name": name, "func": "build_preview_data",
         "input": {"headers": data.headers, "rows": data.rows,
                   "template": tpl_dict(tmpl), "builtins": builtins_dict(BUILTINS),
                   "manual_map": manual_map or {}, "settings": settings_dict(settings)},
         "output": clean(preview)})
    return preview, matches

def case_write_request(name, preview_ref, preview, tmpl, matches, settings, output_path):
    req = build_write_request(preview, tmpl, matches, settings, output_path)
    add({"name": name, "func": "build_write_request",
         "input": {"preview_ref": preview_ref, "output_path": output_path},
         "output": clean(req)})

matches = match_columns(["姓名", "手机号", "公司"], t, BUILTINS)
settings = S()
p, _ = case_preview("preview_default", sheet_default(), t, settings)

# 非法号码 + 标红 + 合并
ms = S(phone_merge=True)
p2, _ = case_preview("preview_merge", sheet_default(), t, ms)
case_write_request("write_request_merge", "preview_merge", p2, t, matches, ms, "/tmp/out.xlsx")

# 校验关闭
ns = S(phone_validate=False)
case_preview("preview_validate_off", sheet_default(), t, ns)

# 无手机映射
tmpl_nophone = template([("name", "姓名"), ("company", "公司名")], name="无手机")
case_preview("preview_no_phone", sheet_default(), tmpl_nophone, settings)

# 未匹配列（website 空）
tmpl_multi = template([("name", "姓名"), ("phone", "手机"), ("website", "网址")], name="多列")
case_preview("preview_unmatched_col", sheet_default(), tmpl_multi, settings)

# 同名分组
data_dup = sheet(["姓名", "手机"], [
    {"姓名": "张三", "手机": "13800000000"},
    {"姓名": "李四", "手机": "13900000000"},
    {"姓名": "张三", "手机": "13700000000"},
])
tmpl_dup = template([("name", "姓名"), ("phone", "手机")], name="同名")
ds = S(phone_merge=True, phone_validate=False)
p_dup, m_dup = case_preview("preview_merge_group", data_dup, tmpl_dup, ds)
case_write_request("write_request_merge_group", "preview_merge_group", p_dup, tmpl_dup, m_dup, ds, "/tmp/out.xlsx")

# 不合并：每号一行
data_nom = sheet(["姓名", "手机"], [{"姓名": "张三", "手机": "13800000000,13900000000"}])
no_m = S(phone_merge=False, phone_validate=False)
p_nom, m_nom = case_preview("preview_no_merge", data_nom, t, no_m)
case_write_request("write_request_no_merge", "preview_no_merge", p_nom, t, m_nom, no_m, "/tmp/out.xlsx")

# 标红关闭
hs = S(phone_highlight=False)
p_h, _ = case_preview("preview_highlight_off", sheet_default(), t, hs)
case_write_request("write_request_no_highlight", "preview_highlight_off", p_h, t, matches, hs, "/tmp/out.xlsx")

# 编码/分隔符/vcf 参数（vcf_timestamp 固定 False 避免时间漂移）
es = S(csv_encoding="gbk", txt_encoding="utf-16", txt_separator="、",
                 vcf_fields=["name", "phone"], vcf_name_prefix="客户-",
                 vcf_timestamp=False)
p_e, _ = case_preview("preview_export_params", sheet_default(), t, es)
for path in ["/tmp/out.txt", "/tmp/out.csv", "/tmp/out.xlsx"]:
    case_write_request(f"write_request_enc_{path.rsplit('.', 1)[1]}", "preview_export_params",
                       p_e, t, matches, es, path)

# vcf 序号（同姓名多手机号）
vs = S(vcf_fields=["name", "phone"], vcf_name_prefix="客户-",
                 vcf_timestamp=False, phone_merge=True)
p_v, _ = case_preview("preview_vcf", sheet_default(), t, vs)
case_write_request("write_request_vcf_indexed", "preview_vcf", p_v, t, matches, vs, "/tmp/out.vcf")

# ---- build_preview_display ----
ts = S(vcf_fields=["name", "phone"], vcf_name_prefix="客户-",
                 vcf_name_suffix="-尾", vcf_timestamp=False)
p_d, m_d = case_preview("preview_display_src", sheet_default(), t, ts)
for fmt in ["csv", "txt", "vcf"]:
    headers, rows = build_preview_display(p_d, m_d, ts, fmt)
    add({"name": f"preview_display_{fmt}", "func": "build_preview_display",
         "input": {"preview_ref": "preview_display_src", "fmt": fmt}, "output": {"headers": headers, "rows": rows}})

# ---- build_text_preview ----
bt = S(txt_separator="、", vcf_fields=["name", "phone"],
                 vcf_name_prefix="客户-", vcf_timestamp=False)
p_t, m_t = case_preview("preview_text_src", sheet_default(), t, bt)
for fmt in ["csv", "txt", "vcf"]:
    text = build_text_preview(p_t, m_t, bt, fmt)
    add({"name": f"text_preview_{fmt}", "func": "build_text_preview",
         "input": {"preview_ref": "preview_text_src", "fmt": fmt}, "output": text})

# ---- 写出 ----
out = Path(__file__).resolve().parent / "processor_golden.json"
out.write_text(json.dumps({"cases": CASES}, ensure_ascii=False, indent=2), encoding="utf-8")
print(f"golden 生成完成：{len(CASES)} 个用例 → {out}")
