<script setup lang="ts">
// PreviewPanel — 数据预览公共组件（表格/文本两种模式，全格式共用）。
// 行数上限（默认 20，×4 递增，上限 480）与收起/展开状态内聚在组件内部，
// 避免父组件重复维护导致状态不一致。
import { computed, ref, watch } from "vue";
import { NButton, NCode, NTable, NSpin } from "naive-ui";

const props = defineProps<{
  /** 展示模式：table=结构化表格（xlsx），text=纯文本（vcf/csv/txt） */
  mode: "table" | "text";
  /** 表格模式：列名（与 rows[].values 顺序一致） */
  columns?: string[];
  /** 表格模式：数据行 */
  rows?: { values: string[]; phoneValid?: boolean }[];
  /** 文本模式：按行数上限拉取预览内容 */
  fetchText?: (limit: number) => Promise<string>;
  /** 总行数（文本模式由父组件提供业务行数） */
  total: number;
  /** 刷新信号：该值变化时重新拉取文本预览（如切换导出格式） */
  refreshKey?: string | number;
  emptyText?: string;
}>();

const LIMIT_MAX = 480;
const limit = ref(20); // 预览行数上限（所有格式统一）
const collapsed = ref(false);
const text = ref("");
const textLoading = ref(false);

const tableRows = computed(() => props.rows ?? []);
const shown = computed(() => {
  if (props.mode === "table") return Math.min(limit.value, tableRows.value.length);
  const lines = text.value ? text.value.split("\n").length : 0;
  return Math.min(limit.value, lines);
});
const canMore = computed(() => props.total > 0 && shown.value < props.total && limit.value < LIMIT_MAX);

async function refreshText() {
  if (props.mode !== "text" || !props.fetchText) {
    text.value = "";
    return;
  }
  textLoading.value = true;
  try {
    text.value = await props.fetchText(limit.value);
  } catch {
    text.value = ""; // 错误已由父层提示，这里保持空态
  } finally {
    textLoading.value = false;
  }
}

watch(() => props.fetchText, refreshText, { immediate: true });
watch(() => limit.value, refreshText);
watch(() => props.refreshKey, refreshText);

function moreRows() {
  limit.value = Math.min(limit.value * 4, LIMIT_MAX);
}
</script>

<template>
  <section class="preview-panel" :class="{ collapsed }">
    <div class="preview-head">
      <span class="preview-title">
        数据预览 · 共 {{ total }} 行
        <span class="helper-inline">（当前显示前 {{ shown }} 行）</span>
      </span>
      <div class="preview-tools">
        <n-button size="tiny" quaternary :disabled="!canMore || textLoading" @click="moreRows">显示更多</n-button>
        <n-button size="tiny" quaternary @click="collapsed = !collapsed">{{ collapsed ? "展开" : "收起" }}</n-button>
      </div>
    </div>
    <div v-show="!collapsed" class="preview-body">
      <n-spin :show="textLoading">
        <n-table v-if="mode === 'table'" :bordered="false" size="small" class="preview-table">
          <thead>
            <tr>
              <th v-for="c in columns" :key="c">{{ c }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in tableRows.slice(0, limit)" :key="i">
              <td
                v-for="(c, ci) in columns"
                :key="ci"
                :style="r.phoneValid === false ? 'color:#DC2626' : undefined"
              >{{ r.values?.[ci] ?? "" }}</td>
            </tr>
            <tr v-if="tableRows.length === 0">
              <td :colspan="columns?.length || 1" class="preview-empty">{{ emptyText || "暂无数据" }}</td>
            </tr>
          </tbody>
        </n-table>
        <n-code
          v-else
          :code="text || (textLoading ? '生成预览中…' : emptyText || '暂无预览')"
          language="text"
          word-wrap
          class="preview-code"
        />
      </n-spin>
    </div>
  </section>
</template>

<style scoped>
.preview-panel {
  height: 380px;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--line);
  border-radius: 8px;
  background: var(--surface);
  margin: 10px 0 14px;
  overflow: hidden;
}
.preview-panel.collapsed { height: auto; }
.preview-head {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 7px 12px;
  border-bottom: 1px solid var(--line);
  background: var(--surface-subtle);
}
.preview-title { font-size: 12px; font-weight: 600; }
.preview-tools { display: flex; gap: 4px; }
.preview-body { flex: 1; overflow: auto; }
.preview-body :deep(.n-spin-container),
.preview-body :deep(.n-spin-content) { height: 100%; }
.preview-empty { text-align: center; color: var(--text-muted); }
.preview-panel .preview-code { border: none; border-radius: 0; max-height: none; height: 100%; }
.preview-panel :deep(.preview-table) { max-height: none; }
</style>
