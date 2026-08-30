# 企查查真实源表夹具

用于 `internal/e2e/qcc_phase*_test.go` 的分阶段回归测试。**请把真实表格放在本目录**，不要放用户主目录或临时路径。

## 放置文件

将两个表格按下列文件名放入**本目录**（`internal/e2e/testdata/qcc/`）：

| 文件名 | 用途（请你补充说明后写入 manifest） |
|--------|-------------------------------------|
| `case_01.xlsx` | 标准企查查导出（含声明行，剔除后 10 列企业信息） |
| `case_02.xlsx` | 已处理表格（无声明行，8 列、列序不同） |

支持 `.xlsx` / `.xls`；若使用 `.xls`，请在 `manifest.json` 里把 `file` 字段改为实际文件名。

## 配置期望（manifest.json）

每个 case 在 `manifest.json` 中登记**可自动断言**的期望。你提供表格后：

1. 把文件放进本目录  
2. 运行探测命令（见下方）生成读表摘要  
3. 把摘要中的 `headers` / `row_count` / `preview_rows` 等填入 `manifest.json` 的 `expect`，本地设 `"ready": true`（**勿提交** ready 状态与 xlsx）
4. 再跑分阶段测试

```bash
cd wps-enhancer-go

# 探测：打印每个 case 的 Sheet 列表、声明行剔除后表头、行数（用于填写 manifest）
QCC_FIXTURE_DUMP=1 go test ./internal/e2e/ -run TestQCC_BootstrapDump -v

# 分阶段执行（manifest 未配全时，未就绪的 case 会自动 Skip）
go test ./internal/e2e/ -run 'TestQCC_Phase' -v

# 仅阶段 1（读入冒烟，最快）
go test ./internal/e2e/ -run TestQCC_Phase1 -v
```

## 分阶段说明

| 阶段 | 测试文件 | 验证内容 | 何时跑 |
|------|----------|----------|--------|
| **Phase 1** | `qcc_phase1_read_test.go` | 文件可读、声明行剔除、表头、行数 | 放入表格后立刻 |
| **Phase 2** | `qcc_phase2_mapping_test.go` | 模板建议、Apply、匹配状态、预览行数 | manifest 填好 expect 后 |
| **Phase 3** | `qcc_phase3_export_test.go` | 四格式导出 + 读回关键字段 | Phase 2 通过后 |

未放入文件或 `manifest.json` 中 `expect.ready` 为 `false` 的 case 会 **Skip**，不阻塞 CI。

## 调试建议

- 失败时看子测试名：`TestQCC_Phase1/case_01/headers` 直接定位 case 与断言项  
- 修改源表后先跑 `QCC_FIXTURE_DUMP=1` 更新 manifest，再跑 Phase 测试  
- 真实文件**不提交**（已在 `.gitignore`）；本地放 `case_01.xlsx` / `case_02.xlsx` 后设 `ready: true` 即可跑 Phase 测试

## 请你提供时附带的信息（可写在 PR / issue）

每个表格一句话说明：

- 来源场景（如：企查查批量导出、天眼查、手工改列名）  
- 目标 Sheet 名（若不止一个 Sheet）  
- 你期望的行为（如：必须剔除声明行、某列必须匹配到「手机」）
