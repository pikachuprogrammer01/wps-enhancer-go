<script setup lang="ts">
// 设置页：六 Tab（导入处理/导出格式/内置列/日志/更新/关于），对齐 Python 版设置对话框
import { onMounted, reactive, ref } from "vue";
import {
  NButton,
  NCheckbox,
  NCode,
  NDivider,
  NInput,
  NInputNumber,
  NModal,
  NProgress,
  NSelect,
  NSwitch,
  NTabPane,
  NTabs,
  NTag,
  useDialog,
  useMessage,
} from "naive-ui";
import { Dialogs } from "@wailsio/runtime";
import { App } from "../../bindings/wps-enhancer-go/internal/app/index.js";
import { useAppMeta } from "../composables/useAppMeta";

const message = useMessage();
const dialog = useDialog();
const saving = ref(false);
const activeTab = ref("import");
const { version: aboutVersion, ensureLoaded: ensureMetaLoaded } = useAppMeta();

const emit = defineEmits<{ back: []; home: [] }>();

const form = reactive({
  source_separator: "auto",
  source_encoding: "auto",
  phone_validate: true,
  phone_highlight: true,
  phone_merge: true,
  phone_separators: ";，、",
  declaration_detect: true,
  declaration_keywords: "声明,导出",
  csv_encoding: "utf-8-bom",
  csv_separator: ",",
  csv_separator_custom: "",
  txt_separator: " ",
  txt_separator_custom: "",
  txt_encoding: "utf-16",
  vcf_fields: ["name", "phone"] as string[],
  vcf_name_prefix: "",
  vcf_name_suffix: "",
  vcf_timestamp: false,
  vcf_timestamp_position: "suffix",
  vcf_show_import_guide: true,
  log_debug: false,
  log_retain_days: 30,
  log_auto_clean: true,
  auto_update_enabled: true,
  use_system_proxy: true,
  download_dir: "",
  install_dir: "",
  builtins: [
    { key: "name", label: "姓名", aliases: "姓名,姓,名称", vcf_prop: "FN" },
    { key: "phone", label: "手机", aliases: "手机,手机号,电话,有效手机号", vcf_prop: "TEL;TYPE=CELL" },
    { key: "company", label: "公司名", aliases: "公司,公司名称", vcf_prop: "ORG" },
    { key: "website", label: "网址", aliases: "网址,官网", vcf_prop: "URL" },
  ],
});

const CORE_VCF_KEYS = new Set(["name", "phone", "company", "website"]);
/** 常用 vCard 3.0 属性；选择框支持直接输入未列出的属性名 */
const vcfPropOptions = [
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
const coreVcfPropLabel: Record<string, string> = {
  name: "FN",
  phone: "TEL;TYPE=CELL",
  company: "ORG",
  website: "URL",
};

function vcfPropOptionsFor(current: string) {
  const opts = [...vcfPropOptions];
  const cur = (current || "").trim();
  if (cur && !opts.some((o) => o.value === cur)) {
    opts.unshift({ label: cur, value: cur });
  }
  return opts;
}

function normalizeVcfProp(raw: string): string {
  return raw.trim().toUpperCase().replace(/\s+/g, "");
}
const sepOptions = [
  { label: "自动检测", value: "auto" },
  { label: "逗号 ,", value: "," },
  { label: "分号 ;", value: ";" },
  { label: "制表符 Tab", value: "tab" },
  { label: "竖线 |", value: "|" },
  { label: "自定义…", value: "__custom__" },
];
const CUSTOM_SEP = "__custom__";
const customSep = ref(""); // 自定义分隔符（单个字符）
const encOptions = [
  { label: "自动检测", value: "auto" },
  { label: "UTF-8", value: "utf-8" },
  { label: "GBK", value: "gbk" },
  { label: "UTF-16", value: "utf-16" },
];
const exportEncOptions = [
  { label: "UTF-8 带 BOM", value: "utf-8-bom" },
  { label: "UTF-8", value: "utf-8" },
  { label: "GBK", value: "gbk" },
  { label: "UTF-16", value: "utf-16" },
];
const txtSepOptions = [
  { label: "空格", value: " " },
  { label: "Tab", value: "\t" },
  { label: "逗号", value: "," },
  { label: "顿号", value: "、" },
  { label: "竖线", value: "|" },
  { label: "自定义…", value: "__custom__" },
];
const tsPosOptions = [
  { label: "姓名前", value: "prefix" },
  { label: "姓名后", value: "suffix" },
];

// 日志 tab
const logs = ref<string[]>([]);
const logsLoading = ref(false);

// 更新 tab
const updateInfo = ref<{ current: string; latest: string; has_update: boolean; notes?: string; error?: string } | null>(null);
const updateLoading = ref(false);

// 下载进度（轮询 DownloadProgress）
const dlProgress = ref<{ status: string; percent: number; file_path?: string; guide?: string; error?: string } | null>(null);
let dlTimer: number | null = null;

// 卸载
const uninstallShow = ref(false);
const uninstallItems = ref<{ key: string; label: string; risky: boolean; default_checked: boolean; exists: boolean }[]>([]);
const uninstallChecked = reactive<Record<string, boolean>>({});
const uninstalling = ref(false);

async function openUninstall() {
  try {
    uninstallItems.value = await App.UninstallItems();
    for (const it of uninstallItems.value) {
      uninstallChecked[it.key] = it.default_checked && it.exists;
    }
    uninstallShow.value = true;
  } catch (e: any) {
    message.error("获取清理项失败：" + (e?.message ?? e));
  }
}

function confirmUninstall() {
  const keys = uninstallItems.value.filter((it) => it.exists && uninstallChecked[it.key]).map((it) => it.key);
  if (keys.length === 0) {
    message.warning("请至少勾选一项");
    return;
  }
  const risky = keys.filter((k) => uninstallItems.value.find((it) => it.key === k)?.risky);
  dialog.warning({
    title: "确认卸载",
    content: risky.length > 0
      ? `即将删除 ${keys.length} 项（含 ⚠️ 高风险项：本地数据），此操作不可恢复。确定继续？`
      : `即将删除选中的 ${keys.length} 项，确定继续？`,
    positiveText: "确认删除",
    negativeText: "取消",
    onPositiveClick: async () => {
      uninstalling.value = true;
      try {
        const results = await App.UninstallRemove(keys);
        const failed = results.filter((res) => res.error);
        if (failed.length > 0) {
          message.error(`已删除 ${results.length - failed.length} 项，失败 ${failed.length} 项：${failed[0].error}`);
        } else {
          message.success(`已清理 ${results.length} 项。应用本体删除需完全退出后生效。`);
        }
        uninstallShow.value = false;
      } catch (e: any) {
        message.error("卸载执行失败：" + (e?.message ?? e));
      } finally {
        uninstalling.value = false;
      }
    },
  });
}

function splitList(s: string): string[] {
  return s.split(/[,，]/).map((x) => x.trim()).filter(Boolean);
}

function toggleVcfField(key: string, checked: boolean) {
  const set = new Set(form.vcf_fields);
  if (checked) set.add(key);
  else set.delete(key);
  form.vcf_fields = [...set];
}

function effectiveTxtSeparator(): string {
  return form.txt_separator === "__custom__" ? form.txt_separator_custom : form.txt_separator;
}

async function load() {
  try {
    const st = await App.SettingsGet();
    const knownSep = sepOptions.some((o) => o.value === (st.source_separator || "auto"));
    if (knownSep) {
      form.source_separator = st.source_separator || "auto";
    } else {
      form.source_separator = CUSTOM_SEP;
      customSep.value = st.source_separator || "";
    }
    form.source_encoding = st.source_encoding || "auto";
    form.phone_validate = st.phone_validate;
    form.phone_highlight = st.phone_highlight;
    form.phone_merge = st.phone_merge;
    form.phone_separators = (st.phone_separators ?? []).join(",");
    form.declaration_detect = st.declaration_detect;
    form.declaration_keywords = (st.declaration_keywords ?? []).join(",");
    form.csv_encoding = st.csv_encoding;
    if ([";", "\t", "|", ","].includes(st.csv_separator ?? ",")) {
      form.csv_separator = st.csv_separator || ",";
    } else {
      form.csv_separator = "__csv_custom__";
      form.csv_separator_custom = st.csv_separator || ",";
    }
    form.txt_encoding = st.txt_encoding;
    const sep = st.txt_separator;
    if (txtSepOptions.some((o) => o.value === sep)) {
      form.txt_separator = sep;
    } else {
      form.txt_separator = "__custom__";
      form.txt_separator_custom = sep;
    }
    form.vcf_fields = st.vcf_fields ?? ["name", "phone"];
    form.vcf_name_prefix = st.vcf_name_prefix ?? "";
    form.vcf_name_suffix = st.vcf_name_suffix ?? "";
    form.vcf_timestamp = st.vcf_timestamp;
    form.vcf_timestamp_position = st.vcf_timestamp_position;
    form.vcf_show_import_guide = st.vcf_show_import_guide !== false;
    form.log_debug = st.log_debug;
    form.log_retain_days = st.log_retain_days ?? 30;
    form.log_auto_clean = st.log_auto_clean;
    form.auto_update_enabled = st.auto_update_enabled;
    form.use_system_proxy = st.use_system_proxy;
    form.download_dir = st.download_dir ?? "";
    form.install_dir = st.install_dir ?? "";
    form.builtins = (st.builtin_columns ?? []).map((b) => ({
      key: b.key,
      label: b.label,
      aliases: (b.aliases ?? []).join(","),
      vcf_prop: b.vcf_prop || coreVcfPropLabel[b.key] || "NOTE",
    }));
  } catch (e: any) {
    message.error("设置加载失败：" + (e?.message ?? e));
  }
}

async function save() {
  saving.value = true;
  try {
    let sepOut = form.source_separator;
    if (sepOut === CUSTOM_SEP) {
      const v = customSep.value.trim();
      if ([...v].length !== 1) {
        message.warning("自定义分隔符必须为单个字符");
        saving.value = false;
        return;
      }
      sepOut = v;
    }
    await App.SettingsUpdate({
      source_separator: sepOut,
      source_encoding: form.source_encoding,
      phone_validate: form.phone_validate,
      phone_highlight: form.phone_highlight,
      phone_merge: form.phone_merge,
      phone_separators: splitList(form.phone_separators),
      declaration_detect: form.declaration_detect,
      declaration_keywords: splitList(form.declaration_keywords),
      csv_encoding: form.csv_encoding,
      csv_separator: (() => {
        if (form.csv_separator === "__csv_custom__") {
          const v = form.csv_separator_custom.trim();
          if ([...v].length !== 1) {
            message.warning("csv 自定义分隔符必须为单个字符，已回退为逗号");
            return ",";
          }
          return v;
        }
        return form.csv_separator;
      })(),
      txt_encoding: form.txt_encoding,
      txt_separator: effectiveTxtSeparator(),
      vcf_fields: form.vcf_fields,
      vcf_name_prefix: form.vcf_name_prefix,
      vcf_name_suffix: form.vcf_name_suffix,
      vcf_timestamp: form.vcf_timestamp,
      vcf_timestamp_position: form.vcf_timestamp_position,
      vcf_show_import_guide: form.vcf_show_import_guide,
      log_debug: form.log_debug,
      log_retain_days: Number(form.log_retain_days) || 30,
      log_auto_clean: form.log_auto_clean,
      auto_update_enabled: form.auto_update_enabled,
      use_system_proxy: form.use_system_proxy,
      download_dir: form.download_dir.trim(),
      install_dir: form.install_dir.trim(),
      builtin_columns: (() => {
        const seen = new Set<string>();
        const rows = form.builtins
          .map((b) => ({
            key: b.key.trim(),
            label: b.label.trim(),
            aliases: splitList(b.aliases),
            vcf_prop: CORE_VCF_KEYS.has(b.key.trim())
              ? (coreVcfPropLabel[b.key.trim()] || b.vcf_prop)
              : normalizeVcfProp(b.vcf_prop || "NOTE"),
          }))
          .filter((b) => b.key || b.label || b.aliases.length > 0);
        return rows.map((b) => {
          let key = b.key;
          if (!key) key = `custom_${seen.size + 1}`;
          while (seen.has(key)) key = `${key}_2`;
          seen.add(key);
          return { key, label: b.label, aliases: b.aliases, vcf_prop: b.vcf_prop };
        });
      })(),
    });
    message.success("设置已保存");
    emit("back"); // 保存后返回上一步（导航历史）
  } catch (e: any) {
    message.error("保存失败：" + (e?.message ?? e));
  } finally {
    saving.value = false;
  }
}

function confirmBuiltinDelete(index: number) {
  const row = form.builtins[index];
  if (!row) return;
  dialog.warning({
    title: "确认删除内置列",
    content: `确定删除内置列「${row.label || row.key || "未命名"}」吗？保存后生效，且不可恢复。`,
    positiveText: "确认删除",
    negativeText: "取消",
    onPositiveClick: () => {
      const key = form.builtins[index]?.key;
      form.builtins.splice(index, 1);
      if (key) form.vcf_fields = form.vcf_fields.filter((k) => k !== key);
    },
  });
}

function resetDefaults() {
  dialog.warning({
    title: "恢复默认设置",
    content: "确定将所有设置恢复为默认值吗？当前设置将被覆盖。",
    positiveText: "确定",
    negativeText: "取消",
    onPositiveClick: async () => {
      try {
        await App.SettingsReset();
        await load();
        message.success("已恢复默认设置");
      } catch (e: any) {
        message.error("恢复失败：" + (e?.message ?? e));
      }
    },
  });
}

async function loadLogs() {
  logsLoading.value = true;
  try {
    logs.value = await App.ReadLogs(200);
  } catch (e: any) {
    message.error("日志读取失败：" + (e?.message ?? e));
  } finally {
    logsLoading.value = false;
  }
}

async function exportLogs() {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, "0");
  const fname = `wps-enhancer-${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}.log`;
  try {
    const dest = await Dialogs.SaveFile({
      Title: "导出日志文件",
      Filename: fname,
      CanCreateDirectories: true,
      Filters: [{ DisplayName: "日志文件", Pattern: "*.log" }],
    });
    if (typeof dest !== "string" || !dest) return;
    const saved = await App.ExportLogs(dest);
    message.success("日志已导出：" + saved);
  } catch (e: any) {
    message.error("导出失败：" + (e?.message ?? e));
  }
}

async function checkUpdate() {
  updateLoading.value = true;
  try {
    updateInfo.value = await App.CheckUpdate();
  } finally {
    updateLoading.value = false;
  }
}

async function startDownload() {
  try {
    await App.StartDownloadUpdate();
    pollProgress();
  } catch (e: any) {
    message.error("下载启动失败：" + (e?.message ?? e));
  }
}

function pollProgress() {
  if (dlTimer !== null) return;
  dlTimer = window.setInterval(async () => {
    try {
      dlProgress.value = await App.DownloadProgress();
      const st = dlProgress.value.status;
      if (st === "done") {
        message.success("更新包下载完成");
        stopPolling();
      } else if (st === "error") {
        message.error(dlProgress.value.error || "下载失败");
        stopPolling();
      } else if (st === "idle") {
        stopPolling();
      }
    } catch {
      stopPolling();
    }
  }, 500);
}

function stopPolling() {
  if (dlTimer !== null) {
    clearInterval(dlTimer);
    dlTimer = null;
  }
}

function openDownloadFolder() {
  if (!dlProgress.value?.file_path) return;
  App.OpenPath(dlProgress.value.file_path).catch((e: any) =>
    message.error("打开失败：" + (e?.message ?? e)),
  );
}

function openInstallFolder() {
  const dir = (form.install_dir || "").trim();
  if (!dir) {
    message.warning("请先填写安装目录");
    return;
  }
  App.OpenPath(dir).catch((e: any) =>
    message.error("打开失败：" + (e?.message ?? e)),
  );
}

onMounted(async () => {
  await load();
  await ensureMetaLoaded();
});
</script>

<template>
  <div class="canvas settings-canvas">
    <section class="header">
      <div>
        <h1>设置</h1>
        <p>导入规则、导出格式与模板匹配的全局配置。</p>
      </div>
      <div class="header-actions">
        <button class="small-btn" @click="emit('back')">← 返回</button>
        <button class="small-btn" @click="emit('home')">首页</button>
      </div>
    </section>

    <section class="card task-panel settings-card">
      <n-tabs v-model:value="activeTab" type="line" size="small">
        <!-- 导入处理 -->
        <n-tab-pane name="import" tab="导入处理">
          <div class="setting-row">
            <span class="field-label">源文件列分隔符</span>
            <n-select v-model:value="form.source_separator" :options="sepOptions" size="small" style="max-width: 160px" />
            <n-input
              v-if="form.source_separator === CUSTOM_SEP"
              v-model:value="customSep"
              size="small"
              maxlength="1"
              placeholder="如 、"
              style="max-width: 90px"
            />
            <span class="helper-inline">csv/txt 源文件用什么符号分隔两列数据；「自动检测」会尝试逗号/分号/Tab/竖线，可选自定义单个字符</span>
          </div>
          <div class="setting-row">
            <span class="field-label">源文件编码</span>
            <n-select v-model:value="form.source_encoding" :options="encOptions" size="small" style="max-width: 220px" />
            <span class="helper-inline">csv/txt 源文件的文字编码；「自动检测」按 BOM → UTF-8 → GBK → UTF-16 依次尝试</span>
          </div>
          <n-divider />
          <div class="setting-row">
            <span class="field-label">校验手机号合法性</span>
            <n-switch v-model:value="form.phone_validate" size="small" />
            <span class="helper-inline">开启后检查是否为 11 位有效手机号，不合格的在预览与导出中标记出来</span>
          </div>
          <div class="setting-row">
            <span class="field-label">非法手机号标红</span>
            <n-switch v-model:value="form.phone_highlight" size="small" />
            <span class="helper-inline">导出的表格里，非法手机号用红底标示，方便回头排查</span>
          </div>
          <div class="setting-row">
            <span class="field-label">同姓名多手机号合并</span>
            <n-switch v-model:value="form.phone_merge" size="small" />
            <span class="helper-inline">开启后同一人的多个号码合并进一个单元格（仅 xlsx/xls）；关闭则每个号码单独一行</span>
          </div>
          <div class="setting-row">
            <span class="field-label">多手机号连接符</span>
            <n-input v-model:value="form.phone_separators" size="small" style="max-width: 260px" />
            <span class="helper-inline">合并时多个号码之间用什么符号连接；填多个则逐个尝试拆分（逗号分隔）</span>
          </div>
          <div class="setting-row">
            <span class="field-label">跳过首行声明</span>
            <n-switch v-model:value="form.declaration_detect" size="small" />
            <span class="helper-inline">企查查/天眼查等平台导出的表格首行常是版权声明，开启后自动跳过这一行</span>
          </div>
          <div class="setting-row">
            <span class="field-label">声明行关键词</span>
            <n-input v-model:value="form.declaration_keywords" size="small" style="max-width: 260px" />
            <span class="helper-inline">首行包含任一关键词即视为声明行跳过；逗号分隔多个</span>
          </div>
        </n-tab-pane>

        <!-- 导出格式 -->
        <n-tab-pane name="export" tab="导出格式">
          <div class="setting-row">
            <span class="field-label">csv 分隔符</span>
            <n-select
              v-model:value="form.csv_separator"
              :options="[
                { label: '逗号 ,（标准）', value: ',' },
                { label: '分号 ;', value: ';' },
                { label: '制表符 Tab', value: '\t' },
                { label: '竖线 |', value: '|' },
                { label: '自定义…', value: '__csv_custom__' },
              ]"
              size="small"
              style="max-width: 200px"
            />
            <n-input
              v-if="form.csv_separator === '__csv_custom__'"
              v-model:value="form.csv_separator_custom"
              size="small"
              maxlength="1"
              placeholder="如 、"
              style="max-width: 90px"
            />
            <span class="helper-inline">导出的 csv 文件中列与列的分隔符号</span>
          </div>
          <div class="setting-row">
            <span class="field-label">txt 分隔符</span>
            <n-select v-model:value="form.txt_separator" :options="txtSepOptions" size="small" style="max-width: 200px" />
            <n-input
              v-if="form.txt_separator === '__custom__'"
              v-model:value="form.txt_separator_custom"
              size="small"
              style="max-width: 100px"
              placeholder="自定义"
            />
          </div>
          <div class="setting-row">
            <span class="field-label">txt 编码</span>
            <n-select v-model:value="form.txt_encoding" :options="exportEncOptions" size="small" style="max-width: 200px" />
          </div>
          <n-divider />
          <p class="helper">
            示例：在「内置列」新增「职位」（vCard 可选 TITLE，也可自填任意属性）并勾选下方导出 → 列映射里添加并映射源列。也可在列映射页直接「自定义 vcf 字段」。
          </p>
          <div class="setting-row">
            <span class="field-label">vcf 导出字段</span>
            <div class="checkbox-group">
              <n-checkbox
                v-for="b in form.builtins.filter((x) => x.key.trim())"
                :key="b.key"
                :value="b.key"
                :checked="form.vcf_fields.includes(b.key)"
                size="small"
                @update:checked="(checked: boolean) => toggleVcfField(b.key, checked)"
              >{{ b.label || b.key }}</n-checkbox>
            </div>
          </div>
          <div class="setting-row">
            <span class="field-label">姓名前缀</span>
            <n-input v-model:value="form.vcf_name_prefix" size="small" style="max-width: 200px" />
          </div>
          <div class="setting-row">
            <span class="field-label">姓名后缀</span>
            <n-input v-model:value="form.vcf_name_suffix" size="small" style="max-width: 200px" />
          </div>
          <div class="setting-row">
            <span class="field-label">附加时间戳</span>
            <n-switch v-model:value="form.vcf_timestamp" size="small" />
            <span class="helper-inline">年月日</span>
          </div>
          <div class="setting-row">
            <span class="field-label">时间戳位置</span>
            <n-select v-model:value="form.vcf_timestamp_position" :options="tsPosOptions" size="small" style="max-width: 200px" />
          </div>
          <div class="setting-row">
            <span class="field-label">vCard 导入指南</span>
            <n-switch v-model:value="form.vcf_show_import_guide" size="small" />
            <span class="helper-inline">vcf 导出成功后弹出导入手机通讯录说明（默认开启）</span>
          </div>
        </n-tab-pane>

        <!-- 内置列 -->
        <n-tab-pane name="builtin" tab="内置列">
          <p class="helper">
            自动匹配按「显示名 / 别名」。自定义列可指定 vCard 属性（导出 vcf 时使用）。
            预设为常用项；列表没有的可在框内直接输入（如 NICKNAME、X-DEPT）。
            示例：显示名「职位」、vCard=TITLE → 保存后勾选导出字段即可。
          </p>
          <div v-for="(b, bi) in form.builtins" :key="bi" class="setting-row builtin-row">
            <n-input v-model:value="b.key" size="small" style="max-width: 100px" placeholder="语义键" :disabled="CORE_VCF_KEYS.has(b.key)" />
            <n-input v-model:value="b.label" size="small" style="max-width: 100px" placeholder="显示名" />
            <n-input v-model:value="b.aliases" size="small" style="max-width: 200px" placeholder="匹配别名（逗号分隔）" />
            <n-select
              v-if="!CORE_VCF_KEYS.has(b.key)"
              :value="b.vcf_prop"
              :options="vcfPropOptionsFor(b.vcf_prop)"
              filterable
              tag
              size="small"
              style="width: 180px"
              placeholder="选择或输入属性"
              @update:value="(v: string) => { b.vcf_prop = normalizeVcfProp(v || 'NOTE'); }"
            />
            <n-tag v-else size="small" :bordered="false">{{ coreVcfPropLabel[b.key] || b.vcf_prop }}</n-tag>
            <n-button size="tiny" type="error" quaternary @click="confirmBuiltinDelete(bi)">删除</n-button>
          </div>
          <n-button size="small" quaternary @click="form.builtins.push({ key: '', label: '', aliases: '', vcf_prop: 'NOTE' })">+ 新增内置列</n-button>
        </n-tab-pane>

        <!-- 日志 -->
        <n-tab-pane name="log" tab="日志">
          <div class="row log-actions">
            <n-button size="small" :loading="logsLoading" @click="loadLogs">刷新日志</n-button>
            <n-button size="small" @click="exportLogs">导出日志文件</n-button>
            <span class="helper-inline">最近 200 行（logs/ 目录最新日志文件）</span>
          </div>
          <n-divider />
          <div class="setting-row">
            <span class="field-label">调试日志</span>
            <n-switch v-model:value="form.log_debug" size="small" />
            <span class="helper-inline">记录 DEBUG 级别明细</span>
          </div>
          <div class="setting-row">
            <span class="field-label">自动清理过期日志</span>
            <n-switch v-model:value="form.log_auto_clean" size="small" />
          </div>
          <div class="setting-row">
            <span class="field-label">保留天数</span>
            <n-input-number v-model:value="form.log_retain_days" size="small" :min="1" :max="365" style="max-width: 120px" />
          </div>
          <n-code v-if="logs.length > 0" :code="logs.join('\n')" language="text" word-wrap class="log-code" />
          <p v-else class="helper">暂无日志。</p>
        </n-tab-pane>

        <!-- 更新 -->
        <n-tab-pane name="update" tab="更新">
          <div class="setting-row">
            <span class="field-label">当前版本</span>
            <span class="version-text">{{ aboutVersion }}</span>
          </div>
          <div class="setting-row">
            <span class="field-label">启动时自动检查更新</span>
            <n-switch v-model:value="form.auto_update_enabled" size="small" />
          </div>
          <div class="setting-row">
            <span class="field-label">使用系统代理</span>
            <n-switch v-model:value="form.use_system_proxy" size="small" />
          </div>
          <div class="setting-row">
            <span class="field-label">下载目录</span>
            <n-input v-model:value="form.download_dir" size="small" style="max-width: 320px" placeholder="更新包保存目录" />
          </div>
          <div class="setting-row">
            <span class="field-label">安装目录</span>
            <n-input v-model:value="form.install_dir" size="small" style="max-width: 320px" placeholder="应用安装目录（覆盖升级目标）" />
          </div>
          <n-divider />
          <div class="setting-row">
            <span class="field-label">检查更新</span>
            <n-button size="small" :loading="updateLoading" @click="checkUpdate">检查</n-button>
            <n-tag v-if="updateInfo && updateInfo.has_update" type="success" size="small" :bordered="false">
              发现新版本 {{ updateInfo.latest }}
            </n-tag>
            <n-tag v-else-if="updateInfo && !updateInfo.has_update && !updateInfo.error" size="small" :bordered="false">
              已是最新版本
            </n-tag>
            <span v-if="updateInfo?.error" class="helper-inline error-text">{{ updateInfo.error }}</span>
          </div>
          <template v-if="updateInfo?.has_update">
            <div v-if="dlProgress === null || dlProgress.status === 'idle' || dlProgress.status === 'done'" class="setting-row">
              <span class="field-label">获取更新</span>
              <n-button size="small" type="primary" secondary @click="startDownload">下载更新包</n-button>
              <n-button
                v-if="dlProgress?.status === 'done' && dlProgress.file_path"
                size="small"
                @click="openDownloadFolder"
              >打开所在目录</n-button>
              <n-button
                v-if="dlProgress?.status === 'done'"
                size="small"
                @click="openInstallFolder"
              >打开安装目录</n-button>
            </div>
            <div v-if="dlProgress && dlProgress.status === 'downloading'" class="dl-progress">
              <n-progress type="line" :percentage="Math.min(100, Math.round(dlProgress.percent))" indicator-placement="inside" processing />
              <span class="helper-inline">正在下载更新包…</span>
            </div>
            <n-alert v-if="dlProgress?.status === 'error'" type="error" :show-icon="true" class="warn-alert">
              {{ dlProgress.error }}
            </n-alert>
            <n-alert v-if="dlProgress?.status === 'done' && dlProgress.guide" type="success" :show-icon="true" class="warn-alert">
              <div>更新包已就绪：{{ dlProgress.file_path }}</div>
              <div style="white-space: pre-line; margin-top: 4px">{{ dlProgress.guide }}</div>
            </n-alert>
          </template>
        </n-tab-pane>

        <!-- 关于 -->
        <n-tab-pane name="about" tab="关于">
          <div class="about-box">
            <div class="about-logo">W</div>
            <div class="about-name">WPS Enhancer</div>
            <div class="about-version">版本 v{{ aboutVersion }}</div>
            <p class="helper about-desc">
              针对 WPS Office 中 Word 和 Excel 的本地增强工具。<br />
              所有文件均在本机处理，不会上传到网络。
            </p>
          </div>

          <n-divider />
          <div class="setting-row">
            <span class="field-label">卸载</span>
            <n-button size="small" type="error" secondary @click="openUninstall">卸载 WPS 增强工具…</n-button>
            <span class="helper-inline">卸载会删除选中的内容（默认仅应用本体与日志）</span>
          </div>
        </n-tab-pane>
      </n-tabs>

    </section>

    <!-- 操作栏：在滚动区域之外，固定于页面底部 -->
    <section class="task-actions settings-actions">
      <n-button secondary @click="resetDefaults">恢复默认设置</n-button>
      <n-button type="primary" :loading="saving" @click="save">保存</n-button>
    </section>

    <!-- 卸载弹窗 -->
    <n-modal v-model:show="uninstallShow" preset="dialog" title="卸载 WPS 增强工具"
      :positive-text="uninstalling ? '删除中…' : '删除选中项'" negative-text="取消"
      :positive-button-props="{ type: 'error', disabled: uninstalling }"
      @positive-click="confirmUninstall">
      <div class="modal-body">
        <p class="helper" style="margin-bottom: 8px">选择要删除的内容：</p>
        <div v-for="it in uninstallItems" :key="it.key" class="setting-row">
          <n-checkbox
            v-model:checked="uninstallChecked[it.key]"
            :disabled="it.key === 'app'"
            size="small"
          >
            {{ it.label }}<span v-if="it.risky">　⚠️ 高风险（用户数据）</span>
            <span v-if="!it.exists" class="helper-inline">（无残留）</span>
          </n-checkbox>
        </div>
        <p class="helper" style="margin-top: 8px">
          提示：应用本体删除需在完全退出本应用后进行；若删除失败，请退出后手动删除
          （macOS：/Applications/WPS增强工具.app；Windows：安装目录）。
        </p>
      </div>
    </n-modal>
  </div>
</template>
