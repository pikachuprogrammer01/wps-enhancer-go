<script setup lang="ts">
// 任务流：批量导入通讯录（对齐 Python 版布局：数据源[文件+模板] → 列映射 → 预览与导出）
import { computed, h, reactive, ref, watch } from "vue";
import {
  NAlert,
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDivider,
  NEmpty,
  NInput,
  NModal,
  NSelect,
  NSteps,
  NStep,
  NSwitch,
  NTable,
  NTag,
  useDialog,
  useMessage,
} from "naive-ui";
import { Dialogs } from "@wailsio/runtime";
import { App } from "../../bindings/wps-enhancer-go/internal/app/index.js";
import type { BuiltinColumn } from "../../bindings/wps-enhancer-go/internal/template/models.js";
import PreviewPanel from "../components/PreviewPanel.vue";

const props = defineProps<{
  /** 由应用导航历史驱动的向导步骤（返回时回写） */
  navStep?: number;
}>();
const emit = defineEmits<{ back: []; step: [number]; home: [] }>();
const message = useMessage();
const dialog = useDialog();

// ---------- 状态 ----------
const step = ref(1);
const loading = ref(false);

/** 写入向导步骤并可选入导航历史（从历史回写时 record=false） */
function setStep(s: number, record = true) {
  if (s < 1 || s > 3 || s === step.value) return;
  step.value = s;
  if (record) emit("step", s);
}

watch(
  () => props.navStep,
  (s) => {
    if (typeof s === "number" && s >= 1 && s <= 3 && s !== step.value) {
      setStep(s, false);
    }
  },
);

const filePath = ref("");
const sheets = ref<{ name: string; rows: number }[]>([]);
const sheetName = ref<string | null>(null);
const currentData = ref<any>(null);

const templates = ref<{ name: string; columns: { name: string }[] }[]>([]);
const templateName = ref<string | null>(null); // null = 默认模板
const manualMap = reactive<Record<string, string>>({});
const matches = ref<any[]>([]);
/** 内置列缓存（vcf 扩展字段选择用） */
const builtins = ref<BuiltinColumn[]>([]);
const vcfAddBuiltinKey = ref<string | null>(null);
const format = ref<"vcf" | "xlsx" | "csv" | "txt">("vcf");
const outputPath = ref("");
const preview = ref<any>(null); // 预览数据（响应式：computed 依赖其触发更新）
const previewCols = ref<string[]>([]);
/** 已提示过数字截断的 Sheet 名（每 sheet 一次，对齐 Python） */
const truncationChecked = new Set<string>();

/** 读表后检测截断；用户选中止返回 false。 */
async function checkTruncatedNumbers(data: any): Promise<boolean> {
  if (!data) return true;
  const key = String(data.sheet_name ?? "");
  if (key && truncationChecked.has(key)) return true;
  if (key) truncationChecked.add(key);
  let hints: string[] = [];
  try {
    hints = (await App.DetectTruncatedNumbers(data)) ?? [];
  } catch {
    return true;
  }
  if (!hints.length) return true;
  return await new Promise<boolean>((resolve) => {
    dialog.warning({
      title: "数字截断提醒",
      content: () =>
        h("div", { style: "line-height:1.6;white-space:pre-wrap" }, [
          h("p", { style: "margin:0 0 8px" }, "检测到疑似号码/身份证被截断补零："),
          ...hints.map((line) => h("p", { style: "margin:0 0 4px" }, line)),
          h("p", { style: "margin:10px 0 0" },
            "建议在 Excel/WPS 中将相关列设置为「文本」格式后重新导出。是否仍要继续当前操作？"),
        ]),
      positiveText: "继续",
      negativeText: "中止",
      onPositiveClick: () => resolve(true),
      onNegativeClick: () => resolve(false),
      onClose: () => resolve(true),
    });
  });
}

// ---------- 步骤 3：vcf 导出选项（仅 format=vcf 时显示，写入全局设置） ----------
const vcfForm = reactive({
  prefix: "vcf_",
  suffix: "",
  timestamp: true,
  timestampPosition: "prefix" as "prefix" | "suffix",
  fields: ["name", "phone", "company", "website"] as string[],
});
const vcfRefreshKey = ref(0);
const vcfFieldOptions = [
  { label: "姓名", value: "name" },
  { label: "手机", value: "phone" },
  { label: "公司名", value: "company" },
  { label: "网址", value: "website" },
];
const vcfTsPosOptions = [
  { label: "姓名前", value: "prefix" },
  { label: "姓名后", value: "suffix" },
];
const previewRefreshKey = computed(() => `${format.value}:${vcfRefreshKey.value}`);

async function loadVcfSettings() {
  try {
    const st = await App.SettingsGet();
    if (!st) return;
    vcfForm.prefix = st.vcf_name_prefix ?? "";
    vcfForm.suffix = st.vcf_name_suffix ?? "";
    vcfForm.timestamp = !!st.vcf_timestamp;
    vcfForm.timestampPosition = (st.vcf_timestamp_position === "suffix" ? "suffix" : "prefix");
    vcfForm.fields = [...(st.vcf_fields ?? ["name", "phone", "company", "website"])];
  } catch {
    /* 读设置失败时保留当前表单默认值 */
  }
}

async function syncVcfSettings() {
  try {
    await App.SettingsUpdate({
      vcf_name_prefix: vcfForm.prefix,
      vcf_name_suffix: vcfForm.suffix,
      vcf_timestamp: vcfForm.timestamp,
      vcf_timestamp_position: vcfForm.timestampPosition,
      vcf_fields: vcfForm.fields,
    });
    vcfRefreshKey.value++;
    if (preview.value) {
      try {
        previewSummaryLine.value = await App.PreviewSummary(
          previewRows.value.length,
          format.value,
          invalidCount.value,
        );
      } catch { /* 汇总刷新失败不影响主流程 */ }
    }
  } catch (e: any) {
    message.error("保存 vcf 设置失败：" + (e?.message ?? e));
  }
}

// ---------- 模板弹窗 ----------
const modal = reactive<{ show: boolean; mode: "new" | "from-headers" | "rename"; name: string; cols: string }>({
  show: false,
  mode: "new",
  name: "",
  cols: "",
});
const modalTitle = computed(() => {
  if (modal.mode === "new") return "新建模板";
  if (modal.mode === "from-headers") return "从表头导入模板";
  return "重命名模板";
});

// ==================== 步骤 1：数据源 ====================
async function browseFile() {
  const selected = await Dialogs.OpenFile({
    Title: "选择数据文件",
    Filters: [{ DisplayName: "表格文件（xlsx/xls/csv）", Pattern: "*.xlsx;*.xls;*.csv" }],
    CanChooseFiles: true,
    CanChooseDirectories: false,
  });
  if (typeof selected === "string" && selected) {
    filePath.value = selected;
    await loadSheets();
  }
}

async function loadSheets() {
  if (!filePath.value.trim()) {
    message.warning("请先输入文件路径");
    return;
  }
  loading.value = true;
  try {
    sheets.value = await App.ListSheets(filePath.value.trim());
    sheetName.value = sheets.value[0]?.name ?? null;
    message.success(`找到 ${sheets.value.length} 个 Sheet`);
  } catch (e: any) {
    message.error("加载失败：" + (e?.message ?? e));
  } finally {
    loading.value = false;
  }
}

// Sheet 选择后自动读取（对齐 Python _on_sheet_changed 行为）
watch(sheetName, async (name) => {
  if (!name || !filePath.value.trim()) return;
  await readSheet();
});

async function readSheet() {
  if (!sheetName.value) return;
  loading.value = true;
  try {
    currentData.value = await App.ReadSheet(filePath.value.trim(), sheetName.value, true);
    if (!(await checkTruncatedNumbers(currentData.value))) {
      currentData.value = null;
      message.warning("已中止：请将相关列设为文本格式后重新选择文件");
      return;
    }
    await refreshTemplates();
    Object.keys(manualMap).forEach((k) => delete manualMap[k]);
    matches.value = []; // 换源表后清空会话列，避免沿用旧模板结构
    const suggestions = await App.SuggestTemplates(currentData.value.headers ?? []);
    if (suggestions?.length) {
      // 有可匹配模板 → 提示用户应用 / 自选 / 跳过
      await promptTemplateSuggestions(suggestions);
    } else if (templateName.value) {
      // 无自动建议，但用户已选模板 → 仍按该模板重建映射
      await applyTemplateByName(templateName.value);
    } else {
      // 无匹配模板 → 默认列进入列映射
      await refreshMapping();
    }
    setStep(2);
  } catch (e: any) {
    message.error("读取失败：" + (e?.message ?? e));
  } finally {
    loading.value = false;
  }
}

// ==================== 模板（步骤 1 分组） ====================
async function refreshTemplates() {
  templates.value = await App.TemplateList();
  if (!templates.value.some((t) => t.name === templateName.value)) {
    templateName.value = null; // 模板被删或未选择 → 默认模板
  }
}

function selectTemplate(name: string | null) {
  if (name) {
    void applyTemplateByName(name);
    return;
  }
  void applyDefaultTemplate();
}

// 切回内置默认模板（清空手动映射后按默认列重新匹配）
async function applyDefaultTemplate() {
  templateName.value = null;
  Object.keys(manualMap).forEach((k) => delete manualMap[k]);
  if (currentData.value) {
    await refreshMapping();
  }
  message.success("已使用默认模板");
}

// 应用模板（覆盖语义：清空 manualMap 后以返回 mapping 重建）
async function applyTemplateByName(name: string) {
  if (!name) return;
  if (!currentData.value) {
    templateName.value = name;
    message.success(`已选择模板「${name}」，读取数据后将自动匹配`);
    return;
  }
  try {
    const applied = await App.ApplyTemplate(currentData.value.headers, name);
    if (!applied) {
      message.error("应用模板失败：未返回匹配结果");
      return;
    }
    Object.keys(manualMap).forEach((k) => delete manualMap[k]);
    for (const [key, src] of Object.entries(applied.mapping ?? {})) {
      if (src) manualMap[key] = src;
    }
    templateName.value = applied.name;
    // 用 Apply 返回的匹配结果作为会话列，避免 refreshMapping 沿用旧会话列
    matches.value = applied.matches?.length ? applied.matches : [];
    if (!matches.value.length) {
      await refreshMapping();
    }
    const miss = applied.missing_cols?.length ?? 0;
    const mapped = Object.keys(applied.mapping ?? {}).length;
    if (miss) {
      message.warning(`已应用模板「${applied.name}」：${mapped} 列已匹配，${miss} 列未匹配将为空`);
    } else {
      message.success(`已应用模板「${applied.name}」：${mapped} 列全部匹配`);
    }
  } catch (e: any) {
    message.error("应用模板失败：" + (e?.message ?? e));
  }
}

// 读入数据源后提示可用模板（最佳匹配 / 自选 / 跳过）
function promptTemplateSuggestions(suggestions: { name: string; matched: number; total: number; missing_cols?: string[] }[]): Promise<void> {
  const summary = suggestions
    .map((s) => {
      const miss = s.missing_cols?.length ? `，缺 ${s.missing_cols.join("、")}` : "";
      return `${s.name}（${s.matched}/${s.total} 列匹配${miss}）`;
    })
    .join("\n");
  return new Promise((resolve) => {
    const d = dialog.create({
      title: "检测到可用模板",
      content: () =>
        h("div", { style: "white-space:pre-wrap;line-height:1.6" }, [
          h("p", { style: "margin:0 0 8px" }, `已对照模板目录，发现 ${suggestions.length} 个可匹配模板：`),
          h("p", { style: "margin:0;color:var(--text-secondary);font-size:13px" }, summary),
        ]),
      closable: false,
      maskClosable: false,
      action: () =>
        h("div", { style: "display:flex;gap:8px;justify-content:flex-end;flex-wrap:wrap" }, [
          h(NButton, { size: "small", onClick: () => { d.destroy(); void refreshMapping().then(resolve); } }, () => "跳过"),
          h(NButton, {
            size: "small",
            onClick: () => {
              d.destroy();
              void promptPickTemplate(suggestions).then(async (picked) => {
                if (picked) await applyTemplateByName(picked);
                else await refreshMapping();
                resolve();
              });
            },
          }, () => "自选…"),
          h(NButton, {
            size: "small",
            type: "primary",
            onClick: () => {
              d.destroy();
              void applyTemplateByName(suggestions[0].name).then(resolve);
            },
          }, () => "应用最佳匹配"),
        ]),
    });
  });
}

// 二次选择：从建议列表中指定模板
function promptPickTemplate(suggestions: { name: string; matched: number; total: number }[]): Promise<string | null> {
  let picked = suggestions[0].name;
  return new Promise((resolve) => {
    const d = dialog.create({
      title: "选择模板",
      content: () =>
        h(NSelect, {
          value: picked,
          options: suggestions.map((s) => ({
            label: `${s.name}（${s.matched}/${s.total} 列匹配）`,
            value: s.name,
          })),
          style: "width:100%",
          onUpdateValue: (v: string) => { picked = v; },
        }),
      positiveText: "应用",
      negativeText: "取消",
      onPositiveClick: () => { d.destroy(); resolve(picked); },
      onNegativeClick: () => { d.destroy(); resolve(null); },
    });
  });
}

// 点击模板列表行 / 「应用」：真正调用 ApplyTemplate 重建映射
async function chooseTemplate(name: string) {
  await applyTemplateByName(name);
}

async function deleteTemplate() {
  if (!templateName.value) {
    message.warning("默认模板不可删除");
    return;
  }
  const name = templateName.value;
  dialog.warning({
    title: "确认删除模板",
    content: `确定删除模板「${name}」吗？此操作不可恢复。`,
    positiveText: "确认删除",
    negativeText: "取消",
    onPositiveClick: async () => {
      try {
        await App.TemplateDelete(name);
        templateName.value = null;
        message.success("模板已删除");
        await refreshTemplates();
      } catch (e: any) {
        message.error("删除失败：" + (e?.message ?? e));
      }
    },
  });
}

function openModal(mode: "new" | "from-headers" | "rename") {
  modal.mode = mode;
  modal.name = "";
  modal.cols = "";
  modal.show = true;
}

async function submitModal() {
  const name = modal.name.trim();
  if (!name) {
    message.warning("请输入模板名称");
    return;
  }
  try {
    if (modal.mode === "new") {
      const columns = modal.cols
        .split(/[,，]/)
        .map((s) => s.trim())
        .filter(Boolean)
        .map((n) => ({ key: "", name: n, enabled: true }));
      if (columns.length === 0) {
        message.warning("请输入至少一列");
        return;
      }
      await App.TemplateCreate(name, columns);
      message.success("模板已创建");
    } else if (modal.mode === "from-headers") {
      if (!currentData.value) {
        message.warning("请先读取源表格");
        return;
      }
      await App.TemplateCreateFromHeaders(name, currentData.value.headers);
      message.success("已从表头创建模板");
    } else {
      if (!templateName.value) {
        message.warning("请先选择要重命名的模板");
        return;
      }
      await App.TemplateRename(templateName.value, name);
      message.success("模板已重命名");
    }
    modal.show = false;
    await refreshTemplates();
    if (currentData.value) {
      await applyTemplateByName(name);
    } else {
      templateName.value = name;
      message.success(`已选择模板「${name}」`);
    }
  } catch (e: any) {
    message.error("操作失败：" + (e?.message ?? e));
  }
}

// 模板表格行操作
function renameFromRow(t: any) {
  modal.mode = "rename";
  modal.name = t.name;
  modal.cols = "";
  modal.show = true;
}
async function deleteFromRow(t: any) {
  dialog.warning({
    title: "确认删除模板",
    content: `确定删除模板「${t.name}」吗？此操作不可恢复。`,
    positiveText: "确认删除",
    negativeText: "取消",
    onPositiveClick: async () => {
      try {
        await App.TemplateDelete(t.name);
        if (templateName.value === t.name) templateName.value = null;
        message.success("模板已删除");
        await refreshTemplates();
      } catch (e: any) {
        message.error("删除失败：" + (e?.message ?? e));
      }
    },
  });
}

// ==================== 步骤 2：列映射 ====================
async function refreshMapping() {
  if (!currentData.value) return;
  await refreshBuiltins();
  const cols = matches.value.map((m) => m.template_col);
  if (cols.length > 0) {
    // 已有会话列：按会话重匹配，不回读磁盘（避免覆盖用户未保存的增删改/排序）
    matches.value = await App.SessionMatch(currentData.value.headers, cols, manualMap);
    return;
  }
  matches.value = await App.TemplateMatches(currentData.value.headers, templateName.value ?? "", manualMap);
}

// 当前会话模板列（供预览/导出，不写磁盘）
function sessionColumns(): any[] {
  return matches.value.map((m) => ({ ...m.template_col, enabled: true }));
}

// ===== 列映射中模板列的增删改（仅改会话；导出后再问是否写回模板） =====

function nextCustomKey(extra: string[] = []): string {
  const keys = new Set<string>([
    ...matches.value.map((m) => m.template_col.key as string),
    ...builtins.value.map((b) => b.key),
    ...extra,
  ]);
  let n = 1;
  while (keys.has(`custom_${n}`)) n += 1;
  return `custom_${n}`;
}

async function refreshBuiltins() {
  try {
    const st = await App.SettingsGet();
    builtins.value = st.builtin_columns ?? [];
  } catch {
    /* 读内置列失败时保留缓存 */
  }
}

const vcfBuiltinOptions = computed(() => {
  const used = new Set(matches.value.map((m) => m.template_col.key as string));
  return builtins.value
    .filter((b) => b.key && !used.has(b.key))
    .map((b) => ({ label: `${b.label || b.key}（${b.key}）`, value: b.key }));
});

async function ensureVcfFieldInSettings(key: string) {
  const st = await App.SettingsGet();
  const fields = [...(st.vcf_fields ?? [])];
  if (fields.includes(key)) {
    vcfForm.fields = fields;
    return;
  }
  fields.push(key);
  await App.SettingsUpdate({ vcf_fields: fields });
  vcfForm.fields = fields;
  vcfRefreshKey.value++;
}

async function rematchSession() {
  if (!currentData.value) return;
  const cols = sessionColumns();
  if (cols.length === 0) return;
  matches.value = await App.SessionMatch(currentData.value.headers, cols, manualMap);
  preview.value = null; // 映射变更后需重新进入预览才有准确导出行数
}

async function renameTemplateCol(m: any, newName: string) {
  m.template_col.name = newName.trim();
  if (!newName.trim()) {
    await rematchSession();
    return;
  }
  try {
    await rematchSession();
  } catch (e: any) {
    message.error("列名更新失败：" + (e?.message ?? e));
  }
}

async function deleteTemplateCol(m: any) {
  if (matches.value.length <= 1) {
    message.warning("至少保留一列");
    return;
  }
  dialog.warning({
    title: "确认删除模板列",
    content: `删除列「${m.template_col.name}」？仅影响本次会话，原模板文件不会立刻改动；导出后可选择是否保存。`,
    positiveText: "删除",
    negativeText: "取消",
    onPositiveClick: async () => {
      matches.value = matches.value.filter((x) => x !== m);
      try {
        await rematchSession();
      } catch (e: any) {
        message.error("删除失败：" + (e?.message ?? e));
      }
    },
  });
}

async function addTemplateCol() {
  const key = nextCustomKey();
  matches.value = [
    ...matches.value,
    {
      template_col: { key, name: "新列", enabled: true },
      source_col: null,
      status: "none",
    },
  ];
  try {
    await rematchSession();
  } catch (e: any) {
    message.error("添加失败：" + (e?.message ?? e));
  }
}

/** vcf：从内置列扩展一条导出字段（写入会话映射 + vcf_fields） */
async function addVcfFromBuiltin() {
  const key = vcfAddBuiltinKey.value;
  if (!key) {
    message.warning("请先选择要添加的内置列");
    return;
  }
  const b = builtins.value.find((x) => x.key === key);
  if (!b) return;
  matches.value = [
    ...matches.value,
    {
      template_col: { key: b.key, name: b.label || b.key, enabled: true },
      source_col: null,
      status: "none",
    },
  ];
  vcfAddBuiltinKey.value = null;
  try {
    await ensureVcfFieldInSettings(b.key);
    await rematchSession();
    message.success(`已添加「${b.label || b.key}」，请选择对应源表列`);
  } catch (e: any) {
    message.error("添加失败：" + (e?.message ?? e));
  }
}

/** 常用 vCard 3.0 属性预设；亦可在选择框中直接输入任意属性名（含 X- 扩展） */
const VCF_PROP_PRESETS = [
  { label: "NOTE 备注", value: "NOTE" },
  { label: "EMAIL 邮箱", value: "EMAIL" },
  { label: "TITLE 职位", value: "TITLE" },
  { label: "ROLE 角色", value: "ROLE" },
  { label: "NICKNAME 昵称", value: "NICKNAME" },
  { label: "BDAY 生日", value: "BDAY" },
  { label: "ADR 地址", value: "ADR" },
  { label: "LABEL 地址标签", value: "LABEL" },
  { label: "CATEGORIES 分类", value: "CATEGORIES" },
  { label: "ORG 公司", value: "ORG" },
  { label: "URL 网址", value: "URL" },
  { label: "TEL 电话", value: "TEL" },
  { label: "TEL;TYPE=WORK 工作电话", value: "TEL;TYPE=WORK" },
  { label: "TEL;TYPE=HOME 住宅电话", value: "TEL;TYPE=HOME" },
  { label: "TEL;TYPE=FAX 传真", value: "TEL;TYPE=FAX" },
];

function normalizeVcfProp(raw: string): string {
  return raw.trim().toUpperCase().replace(/\s+/g, "");
}

function isValidVcfProp(prop: string): boolean {
  return /^[A-Z][A-Z0-9_-]*(;[A-Z][A-Z0-9_-]*(=[A-Z0-9_-]+)?)*$/i.test(prop);
}

function vcfPropSelectOptions(current?: string) {
  const opts = [...VCF_PROP_PRESETS];
  const cur = (current || "").trim();
  if (cur && !opts.some((o) => o.value === cur)) {
    opts.unshift({ label: cur, value: cur });
  }
  return opts;
}

/** vcf：自定义新字段（写入内置列 + vcf_fields + 会话映射；属性可自填） */
function addVcfCustomField() {
  let label = "";
  let vcfProp = "NOTE";
  dialog.create({
    title: "自定义 vcf 导出字段",
    content: () =>
      h("div", { style: "display:flex;flex-direction:column;gap:8px" }, [
        h("p", { style: "margin:0;font-size:12px;color:#666;line-height:1.5" },
          "示例：显示名填「职位」，vCard 属性选 TITLE（或自行输入 NICKNAME / X-DEPT 等）→ 添加后映射源列「职位」。"),
        h(NInput, {
          placeholder: "显示名，如：职位 / 备注 / 邮箱",
          autofocus: true,
          onUpdateValue: (v: string) => { label = v; },
        }),
        h("span", { style: "font-size:12px;color:#666" }, "vCard 属性（可选预设，也可直接输入）"),
        h(NSelect, {
          value: vcfProp,
          options: vcfPropSelectOptions(vcfProp),
          filterable: true,
          tag: true,
          size: "small",
          placeholder: "NOTE / TITLE / 或自填…",
          onUpdateValue: (v: string) => { vcfProp = v || "NOTE"; },
        }),
      ]),
    positiveText: "添加",
    negativeText: "取消",
    onPositiveClick: async () => {
      const name = label.trim();
      if (!name) {
        message.warning("请填写显示名");
        return false;
      }
      const prop = normalizeVcfProp(vcfProp || "NOTE");
      if (!isValidVcfProp(prop)) {
        message.warning("vCard 属性名无效，示例：NOTE、TITLE、TEL;TYPE=WORK、X-DEPT");
        return false;
      }
      const key = nextCustomKey();
      const col: BuiltinColumn = {
        key,
        label: name,
        aliases: [name],
        vcf_prop: prop,
      };
      try {
        const st = await App.SettingsGet();
        const cols = [...(st.builtin_columns ?? []), col];
        const fields = [...(st.vcf_fields ?? [])];
        if (!fields.includes(key)) fields.push(key);
        await App.SettingsUpdate({ builtin_columns: cols, vcf_fields: fields });
        builtins.value = cols;
        vcfForm.fields = fields;
        vcfRefreshKey.value++;
        matches.value = [
          ...matches.value,
          {
            template_col: { key, name, enabled: true },
            source_col: null,
            status: "none",
          },
        ];
        await rematchSession();
        message.success(`已添加「${name}」→ ${prop}，请选择源表列`);
      } catch (e: any) {
        message.error("添加失败：" + (e?.message ?? e));
        return false;
      }
    },
  });
}

function onSourceChange(m: any, val: string | null) {
  if (val) {
    manualMap[m.template_col.key] = val;
  } else {
    delete manualMap[m.template_col.key];
  }
  rematchSession();
}

// ===== 列映射拖拽排序（模板列顺序 = 导出列顺序；仅会话内） =====
const dragFrom = ref<number | null>(null);
const dragOver = ref<number | null>(null);

function onMapDragStart(idx: number, e: DragEvent) {
  dragFrom.value = idx;
  e.dataTransfer?.setData("text/plain", String(idx));
  if (e.dataTransfer) e.dataTransfer.effectAllowed = "move";
}

function onMapDragOver(idx: number, e: DragEvent) {
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  dragOver.value = idx;
}

function onMapDragLeave(idx: number) {
  if (dragOver.value === idx) dragOver.value = null;
}

async function onMapDrop(idx: number, e: DragEvent) {
  e.preventDefault();
  const from = dragFrom.value;
  dragFrom.value = null;
  dragOver.value = null;
  if (from === null || from === idx) return;
  const list = matches.value.slice();
  const [moved] = list.splice(from, 1);
  list.splice(idx, 0, moved);
  matches.value = list;
  // 仅改顺序，映射关系不变，无需重匹配
}

function onMapDragEnd() {
  dragFrom.value = null;
  dragOver.value = null;
}

const mapStatus = (m: any) => {
  if (m.status === "exact") return { type: "success", text: "精确" };
  if (m.status === "alias") return { type: "info", text: "别名" };
  if (m.status === "manual") return { type: "warning", text: "手动" };
  return { type: "error", text: "未匹配" };
};

// ==================== 步骤 3：预览与导出 ====================
const previewRows = computed(() => preview.value?.rows ?? []);
const invalidCount = computed(() => preview.value?.invalid_count ?? 0);
const invalidSummary = computed(() => preview.value?.invalid_summary ?? []);
const sourceRows = computed(() => currentData.value?.rows?.length ?? 0);
// 源表内容预览（前 5 行），供用户核对选中的文件/Sheet/映射是否正确
const previewHeaders = computed(() => currentData.value?.headers ?? []);
// 当前已匹配的源表列（精确/别名/手动均算），预览中高亮显示
const matchedSourceCols = computed(() => {
  const set = new Set<string>();
  for (const m of matches.value) {
    if (m.source_col) set.add(m.source_col);
  }
  return set;
});
const previewSampleRows = computed(() => {
  const rows = currentData.value?.rows ?? [];
  return rows.slice(0, 5).map((r: any) => previewHeaders.value.map((h: string) => r[h] ?? ""));
});

// 文本格式（vcf/csv/txt）预览由 PreviewPanel 组件按行数上限拉取后端 PreviewText，
// 行数上限与收起状态内聚在组件内，避免父子状态不一致。
const previewSummaryLine = ref("");
watch([preview, format, invalidCount], async () => {
  if (!preview.value) {
    previewSummaryLine.value = "";
    return;
  }
  try {
    previewSummaryLine.value = await App.PreviewSummary(
      previewRows.value.length,
      format.value,
      invalidCount.value,
    );
  } catch {
    previewSummaryLine.value = `共 ${previewRows.value.length} 行`;
  }
}, { immediate: true });

async function fetchPreviewText(limit: number): Promise<string> {
  if (!currentData.value) return "";
  try {
    return await App.PreviewTextWithColumns(
      currentData.value,
      sessionColumns(),
      manualMap,
      format.value,
      limit,
    );
  } catch (e: any) {
    message.error("预览生成失败：" + (e?.message ?? e));
    return "";
  }
}

// 预览表格行：values 按列序展开 + 手机号合法性标记（供组件标红）
const panelRows = computed(() =>
  previewRows.value.map((r: any) => ({ values: r.values ?? [], phoneValid: r.phone_valid })),
);

// 切换导出格式时同步输出路径扩展名（避免后缀停留在旧格式如 .vcf）
watch(format, () => {
  if (!outputPath.value) return;
  outputPath.value = outputPath.value.replace(/\.[^.\\/]+$/, `.${format.value}`);
});

function renderPreviewRow(row: any, colKey: string): string {
  const idx = previewCols.value.indexOf(colKey);
  return row.values[idx] ?? "";
}

const rowPhoneStyle = (row: any) =>
  row.phone_valid === false ? "color:#DC2626" : undefined;

function stamp(): string {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`;
}

async function buildPreview() {
  if (!currentData.value) return;
  try {
    await loadVcfSettings();
    preview.value = await App.PreviewWithColumns(currentData.value, sessionColumns(), manualMap);
    previewCols.value = matches.value
      .filter((m) => m.template_col.enabled)
      .map((m) => m.template_col.name);
    const base = filePath.value.trim().replace(/\.[^.]+$/, "");
    outputPath.value = `${base}_${stamp()}.${format.value}`;
    setStep(3);
  } catch (e: any) {
    message.error("预览失败：" + (e?.message ?? e));
  }
}

// 将当前会话列+映射写入指定模板名（覆盖或新建）
async function persistSessionToTemplate(name: string): Promise<boolean> {
  const cols = sessionColumns();
  try {
    const exists = templates.value.some((t) => t.name === name);
    if (exists) {
      await App.TemplateUpdate(name, cols);
    } else {
      await App.TemplateCreate(name, cols);
    }
    await App.TemplateSetMappings(name, { ...manualMap });
    templateName.value = name;
    await refreshTemplates();
    message.success(`模板「${name}」已保存`);
    return true;
  } catch (e: any) {
    message.error("保存模板失败：" + (e?.message ?? e));
    return false;
  }
}

// 导出成功后：把保存选择权交给用户（不自动覆盖原模板）
function promptPersistAfterExport() {
  const current = templateName.value;
  if (current) {
    const d = dialog.create({
      title: "是否保存模板改动？",
      content: `本次导出使用的列设置可能已调整。原模板「${current}」尚未自动覆盖，请选择：`,
      closable: false,
      maskClosable: false,
      action: () =>
        h("div", { style: "display:flex;gap:8px;justify-content:flex-end;flex-wrap:wrap" }, [
          h(NButton, { size: "small", onClick: () => { d.destroy(); } }, () => "不保存"),
          h(NButton, {
            size: "small",
            onClick: () => {
              d.destroy();
              promptSaveAsNewTemplate();
            },
          }, () => "另存为新模板"),
          h(NButton, {
            size: "small",
            type: "primary",
            onClick: () => {
              d.destroy();
              void persistSessionToTemplate(current);
            },
          }, () => `覆盖「${current}」`),
        ]),
    });
    return;
  }
  promptSaveAsNewTemplate();
}

// 另存为 / 首次保存：输入新名称
function promptSaveAsNewTemplate() {
  const base = filePath.value.trim().replace(/.*[\\/]/, "").replace(/\.[^.]+$/, "");
  let name = base ? `${base}模板` : "我的模板";
  dialog.info({
    title: "保存为模板",
    content: () =>
      h("div", null, [
        h("p", { style: "margin:0 0 8px;color:var(--text-secondary)" },
          "将当前列顺序、列名与映射保存为模板，下次处理同类表格可直接复用："),
        h(NInput, {
          defaultValue: name,
          placeholder: "模板名称",
          onUpdateValue: (v: string) => { name = v; },
        }),
      ]),
    positiveText: "保存",
    negativeText: "取消",
    onPositiveClick: async () => {
      const finalName = name.trim();
      if (!finalName) {
        message.warning("请输入模板名称");
        return false;
      }
      return await persistSessionToTemplate(finalName);
    },
  });
}

// 手动「保存为模板」按钮：始终另存/覆盖由用户输入名称决定
function promptSaveAsTemplate() {
  promptSaveAsNewTemplate();
}

async function browseOutput() {
  const selected = await Dialogs.SaveFile({
    Title: "选择导出位置",
    Filename: suggestedExportName(),
    Directory: sourceDir(),
    CanCreateDirectories: true,
    Filters: exportFilters(),
  });
  if (typeof selected === "string" && selected) {
    outputPath.value = ensureExportExt(selected);
  }
}

// 源文件所在目录（保存对话框默认打开位置）
function sourceDir(): string {
  const p = filePath.value.trim();
  const i = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return i > 0 ? p.slice(0, i) : "";
}

// 建议的导出文件名（源文件名_时间戳.格式）
function suggestedExportName(): string {
  if (outputPath.value.trim()) {
    return outputPath.value.trim().split(/[\\/]/).pop() || `export.${format.value}`;
  }
  const base = filePath.value.trim().replace(/.*[\\/]/, "").replace(/\.[^.]+$/, "") || "export";
  return `${base}_${stamp()}.${format.value}`;
}

// 当前格式对应的保存对话框过滤器
function exportFilters(): { DisplayName: string; Pattern: string }[] {
  const labels: Record<string, string> = {
    vcf: "vCard 文件 (*.vcf)",
    xlsx: "Excel 工作簿 (*.xlsx)",
    csv: "CSV 文件 (*.csv)",
    txt: "文本文件 (*.txt)",
  };
  const fmt = format.value;
  return [{ DisplayName: labels[fmt] || `*.${fmt}`, Pattern: `*.${fmt}` }];
}

// 确保路径后缀与当前导出格式一致
function ensureExportExt(path: string): string {
  const want = `.${format.value}`;
  if (path.toLowerCase().endsWith(want)) return path;
  return path.replace(/\.[^.\\/]+$/, "") + want;
}

function showVcfImportGuide(savedPath: string) {
  dialog.info({
    title: "vCard 导入指南",
    content: () =>
      h("div", { style: "line-height:1.7;white-space:pre-wrap" }, [
        h("p", { style: "margin:0 0 10px" }, `文件已保存至：${savedPath}`),
        h("p", { style: "margin:0 0 8px" }, "导入手机通讯录的方法："),
        h("p", { style: "margin:0 0 8px" },
          "【iPhone】用「文件」App 找到导出的 .vcf 文件 → 点按打开 → 选择「添加到通讯录」；或通过邮件发送给自己后，用「邮件」打开导入。"),
        h("p", { style: "margin:0 0 8px" },
          "【安卓】用文件管理器找到 .vcf 文件 → 使用系统「联系人/通讯录」应用打开 → 按提示确认导入。"),
        h("p", { style: "margin:0" },
          "【批量管理】导入后可在通讯录中按姓名前缀（如「客户-」）搜索联系人，批量编辑分组或删除。"),
      ]),
    positiveText: "知道了",
  });
}

async function doExport() {
  // 对齐 Python：确认导出时必选保存位置
  const selected = await Dialogs.SaveFile({
    Title: "保存文件",
    Filename: suggestedExportName(),
    Directory: sourceDir(),
    CanCreateDirectories: true,
    Filters: exportFilters(),
  });
  if (typeof selected !== "string" || !selected) return; // 用户取消
  const dest = ensureExportExt(selected);
  outputPath.value = dest;

  loading.value = true;
  try {
    await App.ExportWithColumns(currentData.value, sessionColumns(), manualMap, dest);
    message.success(`导出成功，共 ${previewRows.value.length} 行，文件已保存至：${dest}`);
    if (format.value === "vcf") {
      const st = await App.SettingsGet();
      if (st.vcf_show_import_guide !== false) {
        showVcfImportGuide(dest);
      }
    }
    // 不自动写回模板：由用户选择覆盖 / 另存 / 不保存
    promptPersistAfterExport();
  } catch (e: any) {
    message.error("导出失败：" + (e?.message ?? e));
  } finally {
    loading.value = false;
  }
}

// ==================== 底部常驻导航栏（对齐 Python） ====================
// 确认导出：仅步骤 3 且预览有数据时开放（Python _goto_step 逻辑）
const canExport = computed(() => step.value === 3 && previewRows.value.length > 0);
const canNext = computed(() => {
  if (step.value === 1) return !!currentData.value;
  if (step.value === 2) return matches.value.length > 0;
  return false;
});

async function goNext() {
  if (step.value === 1) {
    if (!currentData.value) return;
    await refreshMapping(); // 加载列匹配后再进入第 2 步
    setStep(2);
  } else if (step.value === 2) {
    await buildPreview();
  }
}

// 点击步骤条跳转：带前置条件守卫（未满足时提示而不是静默失败）
async function jumpStep(target: number) {
  if (target === step.value) return;
  if (target === 1) {
    setStep(1);
    return;
  }
  if (!currentData.value) {
    message.warning("请先在「数据源」中选择文件并读取数据");
    setStep(1);
    return;
  }
  if (target === 2) {
    await refreshMapping();
    setStep(2);
  } else if (target === 3) {
    await buildPreview();
  }
}

const formatOptions = ["vcf", "xlsx", "csv", "txt"].map((f) => ({ label: f.toUpperCase(), value: f }));
</script>

<template>
  <div class="canvas">
    <!-- 任务头部：功能名称 + 说明 -->
    <section class="header">
      <div>
        <h1>批量导入通讯录</h1>
        <p>选择源表格与模板，完成列映射，预览处理结果，并导出为需要的通讯录格式。</p>
      </div>
      <div class="header-actions">
        <button class="small-btn" @click="emit('back')">← 返回</button>
        <button class="small-btn" @click="emit('home')">首页</button>
      </div>
    </section>

    <!-- 步骤指示器 -->
    <n-steps :current="step" class="task-steps" size="small">
      <n-step title="数据源" description="文件与模板" @click="jumpStep(1)" />
      <n-step title="列映射" description="匹配源表列" @click="jumpStep(2)" />
      <n-step title="预览与导出" description="确认结果" @click="jumpStep(3)" />
    </n-steps>

    <!-- 步骤 1：数据源（文件选择 + 模板） -->
    <section v-if="step === 1" class="card task-panel">
      <h2 class="group-title">文件选择</h2>
      <div class="field-block">
        <span class="field-label">源文件</span>
        <div class="field-row">
          <n-input
            v-model:value="filePath"
            placeholder="可手动输入路径（回车确认），或点击浏览选择"
            clearable
            @keyup.enter="loadSheets"
          />
          <n-button @click="browseFile">浏览…</n-button>
        </div>
      </div>
      <div class="field-block">
        <span class="field-label">Sheet</span>
        <n-select
          v-model:value="sheetName"
          :options="sheets.map((s) => ({ label: `${s.name}（${s.rows} 行）`, value: s.name }))"
          :loading="loading"
          placeholder="选择文件后自动加载"
          style="max-width: 420px"
        />
        <p class="helper">选择文件后自动加载 Sheet；切换 Sheet 自动读取数据。</p>
        <template v-if="currentData">
          <p class="helper" style="margin-top: 10px">
            源表内容预览（前 {{ previewSampleRows.length }} 行，共 {{ sourceRows }} 行）——请核对是否为所需数据：
          </p>
          <div class="source-preview-wrap">
            <n-table :bordered="false" size="small" class="source-preview">
              <thead>
                <tr>
                  <th v-for="h in previewHeaders" :key="h" :class="{ 'col-hit': matchedSourceCols.has(h) }">{{ h }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(row, ri) in previewSampleRows" :key="ri">
                  <td v-for="(cell, ci) in row" :key="ci" :class="{ 'col-hit': matchedSourceCols.has(previewHeaders[ci]) }">{{ cell }}</td>
                </tr>
              </tbody>
            </n-table>
          </div>
        </template>
      </div>

      <h2 class="group-title">模板（可选）</h2>
      <div class="field-row">
        <n-button @click="openModal('new')">新建模板</n-button>
        <n-button @click="openModal('from-headers')">从表头导入</n-button>
        <n-button v-if="templateName" quaternary @click="selectTemplate(null)">使用默认模板</n-button>
        <span class="helper-inline" v-if="!templateName">未选择模板时使用默认模板（姓名/手机/公司名/网址）</span>
        <n-tag v-else type="success" size="small" :bordered="false">当前模板：{{ templateName }}</n-tag>
      </div>
      <n-table v-if="templates.length > 0" :bordered="false" size="small" class="template-table">
        <thead>
          <tr>
            <th style="width: 40%">模板名</th>
            <th style="width: 100px">模板列</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="t in templates"
            :key="t.name"
            :class="{ 'row-selected': templateName === t.name }"
            @click="chooseTemplate(t.name)"
          >
            <td class="cell-name">{{ t.name }}</td>
            <td>{{ t.columns?.length ?? 0 }} 列</td>
            <td class="cell-actions" @click.stop>
              <n-button size="tiny" type="primary" @click="applyTemplateByName(t.name)">应用</n-button>
              <n-button size="tiny" @click="renameFromRow(t)">重命名</n-button>
              <n-button size="tiny" type="error" secondary @click="deleteFromRow(t)">删除</n-button>
            </td>
          </tr>
        </tbody>
      </n-table>
      <n-empty v-else description="暂无模板，可新建或从表头导入" size="small" class="template-empty" />
    </section>

    <!-- 步骤 2：列映射 -->
    <section v-else-if="step === 2" class="card task-panel">
      <div class="field-row">
        <span class="field-label">当前模板</span>
        <n-tag type="success" size="small" :bordered="false">{{ templateName || "默认模板" }}</n-tag>
        <n-button size="tiny" @click="step = 1">更换模板</n-button>
      </div>
      <p class="helper">拖动左侧把手可调整模板列顺序（导出列序同步变化）；黄色 = 未匹配，可手动选择源列或「不映射」。</p>

      <n-table :bordered="false" size="small" class="mapping-table">
        <thead>
          <tr>
            <th style="width: 36px"></th>
            <th style="width: 150px">模板列（可编辑）</th>
            <th style="width: 90px">匹配状态</th>
            <th>源表列</th>
            <th style="width: 70px">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(m, idx) in matches"
            :key="m.template_col.key"
            :class="{
              'row-unmatched': m.status === 'none',
              'row-dragging': dragFrom === idx,
              'row-drag-over': dragOver === idx && dragFrom !== idx,
            }"
            @dragover="onMapDragOver(idx, $event)"
            @dragleave="onMapDragLeave(idx)"
            @drop="onMapDrop(idx, $event)"
          >
            <td
              class="drag-handle"
              draggable="true"
              title="拖动调整顺序"
              @dragstart="onMapDragStart(idx, $event)"
              @dragend="onMapDragEnd"
            >⋮⋮</td>
            <td class="cell-name">
              <n-input
                size="small"
                :default-value="m.template_col.name"
                @blur="(e: any) => renameTemplateCol(m, e.target.value)"
                @keyup.enter="($event.target as HTMLInputElement).blur()"
              />
            </td>
            <td>
              <n-tag :type="mapStatus(m).type" size="small" :bordered="false">
                {{ mapStatus(m).text }}
              </n-tag>
            </td>
            <td>
              <n-select
                :value="m.source_col ?? ''"
                :options="[{ label: '（不映射）', value: '' }, ...(currentData?.headers ?? []).map((h: string) => ({ label: h, value: h }))]"
                size="small"
                style="max-width: 320px"
                @update:value="(v: any) => onSourceChange(m, v)"
              />
            </td>
            <td>
              <n-button size="tiny" type="error" quaternary @click="deleteTemplateCol(m)">删除</n-button>
            </td>
          </tr>
        </tbody>
      </n-table>
      <n-button size="small" quaternary style="margin-top: 8px" @click="addTemplateCol">+ 添加模板列</n-button>
      <!-- vcf 额外：从内置列扩展 / 自定义导出字段（与「添加模板列」共存） -->
      <div v-if="format === 'vcf'" class="vcf-map-extend">
        <p class="helper">
          示例：源表有「职位」→ 选内置列或「自定义 vcf 字段」新建，再映射源列。
          vCard 不止 NOTE/EMAIL，可自填任意属性名（如 NICKNAME、BDAY、X-…）。行尾「删除」去掉本次映射列。
        </p>
        <div class="field-row" style="margin-bottom: 0">
          <span class="field-label">扩展 vcf 字段</span>
          <n-select
            v-model:value="vcfAddBuiltinKey"
            :options="vcfBuiltinOptions"
            size="small"
            clearable
            placeholder="从内置列选择…"
            style="width: 220px"
          />
          <n-button size="small" :disabled="!vcfAddBuiltinKey" @click="addVcfFromBuiltin">添加</n-button>
          <n-button size="small" tertiary @click="addVcfCustomField">自定义 vcf 字段…</n-button>
        </div>
      </div>

      <n-divider />
      <p class="helper">源表内容预览（前 {{ previewSampleRows.length }} 行）——请对照上方映射，确认每列选择正确：</p>
      <div class="source-preview-wrap" style="margin-top: 6px">
        <n-table :bordered="false" size="small" class="source-preview">
          <thead>
            <tr>
              <th v-for="h in previewHeaders" :key="h" :class="{ 'col-hit': matchedSourceCols.has(h) }">{{ h }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, ri) in previewSampleRows" :key="ri">
              <td v-for="(cell, ci) in row" :key="ci" :class="{ 'col-hit': matchedSourceCols.has(previewHeaders[ci]) }">{{ cell }}</td>
            </tr>
          </tbody>
        </n-table>
      </div>
    </section>

    <!-- 步骤 3：预览与导出 -->
    <section v-else class="card task-panel">
      <n-alert v-if="invalidCount > 0" type="warning" :show-icon="true" class="warn-alert">
        <div v-for="s in invalidSummary.slice(0, 5)" :key="s" class="warn-line">{{ s }}</div>
        <div v-if="invalidSummary.length > 5">… 等 {{ invalidSummary.length }} 条</div>
      </n-alert>
      <p v-if="previewSummaryLine" class="helper-inline">{{ previewSummaryLine }}</p>

      <!-- vcf 导出选项（对齐 Python 预览页；仅 vcf 时显示） -->
      <div v-if="format === 'vcf'" class="vcf-options">
        <div class="vcf-row">
          <div class="vcf-pair">
            <span class="vcf-pair-label">姓名前缀</span>
            <n-input
              v-model:value="vcfForm.prefix"
              size="small"
              placeholder="如：vcf_ / 客户-"
              class="vcf-pair-input"
              @blur="syncVcfSettings"
            />
          </div>
          <div class="vcf-pair">
            <span class="vcf-pair-label">后缀</span>
            <n-input
              v-model:value="vcfForm.suffix"
              size="small"
              class="vcf-pair-input"
              @blur="syncVcfSettings"
            />
          </div>
          <div class="vcf-pair">
            <n-switch v-model:value="vcfForm.timestamp" size="small" @update:value="syncVcfSettings" />
            <span class="vcf-pair-label">使用时间戳</span>
          </div>
          <div class="vcf-pair">
            <span class="vcf-pair-label">时间戳位置</span>
            <n-select
              v-model:value="vcfForm.timestampPosition"
              :options="vcfTsPosOptions"
              size="small"
              class="vcf-pair-select"
              :disabled="!vcfForm.timestamp"
              @update:value="syncVcfSettings"
            />
          </div>
        </div>
        <div class="vcf-row vcf-row-fields">
          <span class="vcf-pair-label">导出字段</span>
          <n-checkbox-group v-model:value="vcfForm.fields" @update:value="syncVcfSettings">
            <n-checkbox v-for="opt in vcfFieldOptions" :key="opt.value" :value="opt.value" size="small">
              {{ opt.label }}
            </n-checkbox>
          </n-checkbox-group>
        </div>
      </div>

      <!-- 数据预览：公共组件（固定高度、内部滚动、行数上限与收起内聚） -->
      <PreviewPanel
        :mode="format === 'xlsx' ? 'table' : 'text'"
        :columns="previewCols"
        :rows="panelRows"
        :total="previewRows.length"
        :fetch-text="fetchPreviewText"
        :refresh-key="previewRefreshKey"
        empty-text="暂无数据，请返回上一步调整映射"
      />

      <!-- 导出区：与预览区分离 -->
      <div class="field-row export-row">
        <span class="field-label">导出到</span>
        <n-input v-model:value="outputPath" placeholder="导出文件路径" clearable style="flex: 1" />
        <n-button size="small" @click="browseOutput">浏览…</n-button>
        <n-button size="small" tertiary @click="promptSaveAsTemplate">保存为模板</n-button>
      </div>
    </section>

    <!-- 底部常驻导航栏（对齐 Python） -->
    <section class="bottom-bar">
      <div class="bar-left">
        <span class="field-label">导出格式</span>
        <n-select v-model:value="format" :options="formatOptions" size="small" style="width: 110px" />
        <span class="row-hint">
          源 {{ sourceRows }} 行
          <template v-if="preview"> → 导出 {{ previewRows.length }} 行</template>
          <template v-else-if="sourceRows > 0"> → 导出待预览</template>
        </span>
      </div>
      <div class="bar-right">
        <n-button size="small" :disabled="step === 1" @click="emit('back')">上一步</n-button>
        <n-button v-if="step < 3" size="small" type="primary" :disabled="!canNext" @click="goNext">下一步</n-button>
        <n-button size="small" @click="emit('back')">取消</n-button>
        <n-button size="small" type="primary" :disabled="!canExport" :loading="loading" @click="doExport">确认导出</n-button>
      </div>
    </section>

    <!-- 模板弹窗 -->
    <n-modal v-model:show="modal.show" preset="dialog" :title="modalTitle" positive-text="确定" negative-text="取消"
      @positive-click="submitModal">
      <div class="modal-body">
        <n-input v-model:value="modal.name" placeholder="模板名称" class="modal-field" />
        <n-input
          v-if="modal.mode === 'new'"
          v-model:value="modal.cols"
          type="textarea"
          placeholder="模板列，逗号分隔（如：姓名,手机,公司名）"
          :rows="3"
          class="modal-field"
        />
      </div>
    </n-modal>
  </div>
</template>
