#!/usr/bin/env python3
"""生成 excel 层读取对照夹具：样例文件（xlsx/csv/xls）+ Python 读取基准 JSON。

输出: docs/migration/testdata/read_fixtures/ 下样例文件与 *_read.json 基准
用法: python3.14 gen_read_fixtures.py
"""
import importlib.util
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent.parent
sys.path.insert(0, str(ROOT))
OUT = Path(__file__).resolve().parent / "read_fixtures"
OUT.mkdir(parents=True, exist_ok=True)

# 绕过 features/__init__ 的 PyQt6 导入
def load(name, rel):
    spec = importlib.util.spec_from_file_location(name, str(ROOT / rel))
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod

xlsx_mod = load("xlsx_standalone", "core/file_io/xlsx_handler.py")
csv_mod = load("csv_standalone", "core/file_io/csv_handler.py")
xls_mod = load("xls_standalone", "core/file_io/xls_handler.py")
openpyxl = __import__("openpyxl")
xlwt = __import__("xlwt")

# ---- 1. 生成 sample.xlsx（数字/文本/空单元格/中文 + 双 Sheet） ----
wb = openpyxl.Workbook()
ws = wb.active
ws.title = "通讯录"
ws.append(["姓名", "手机号", "公司", "备注"])
ws.append(["张三", 13800000000, "A公司", ""])
ws.append(["李四", "bad", "B公司", "含,逗号"])
ws.append(["王五", None, "", None])
ws2 = wb.create_sheet("说明")
ws2.append(["列", "含义"])
ws2.append(["姓名", "联系人"])
wb.save(OUT / "sample.xlsx")

# ---- 2. 生成 sample.csv（中文 + 引号 + 空字段） ----
csv_text = '姓名,手机号,公司,备注\n张三,13800000000,A公司,"含,逗号"\n李四,bad,B公司,\n王五,,,\n'
(OUT / "sample.csv").write_text(csv_text, encoding="utf-8")

# ---- 3. 生成 sample.xls（xlwt：数字/文本/空） ----
w = xlwt.Workbook()
s = w.add_sheet("通讯录")
for c, h in enumerate(["姓名", "手机号", "公司"]):
    s.write(0, c, h)
s.write(1, 0, "张三")
s.write(1, 1, 13800000000)
s.write(1, 2, "A公司")
s.write(2, 0, "李四")
s.write(2, 1, "bad")
w.save(OUT / "sample.xls")

# ---- 4. Python 读取基准 ----
def sheet_to_json(sd):
    return {"sheet_name": sd.sheet_name, "headers": sd.headers,
            "rows": sd.rows, "declaration_skipped": sd.declaration_skipped}

results = {}

xr = xlsx_mod.XlsxReader()
results["xlsx_sheetnames"] = xr.get_sheet_names(str(OUT / "sample.xlsx"))
results["xlsx_summaries"] = xr.get_sheet_summaries(str(OUT / "sample.xlsx"))
results["xlsx_read"] = sheet_to_json(xr.read_sheet(str(OUT / "sample.xlsx"), "通讯录"))

cr = csv_mod.CsvReader()
results["csv_read"] = sheet_to_json(cr.read_sheet(str(OUT / "sample.csv"), "sample"))
results["csv_delim"] = csv_mod._detect_delimiter(csv_text)

lr = xls_mod.XlsReader()
results["xls_sheetnames"] = lr.get_sheet_names(str(OUT / "sample.xls"))
results["xls_read"] = sheet_to_json(lr.read_sheet(str(OUT / "sample.xls"), "通讯录"))

(OUT / "read_baseline.json").write_text(
    json.dumps(results, ensure_ascii=False, indent=2), encoding="utf-8")
print(f"夹具生成完成 → {OUT}")
print("xlsx 手机号单元格:", results["xlsx_read"]["rows"][0]["手机号"])
print("xls  手机号单元格:", results["xls_read"]["rows"][0]["手机号"])
