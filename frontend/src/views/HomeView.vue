<script setup lang="ts">
// 首页：仅产品切换与业务入口；版本/授权由独立组件承载，本页不直接调 License/Version API。
import { ref } from "vue";
import VersionLicensePanel from "../components/VersionLicensePanel.vue";
import { SHOW_SUBSCRIPTION_UI } from "../featureFlags";

const office = ref<"excel" | "word">("excel");

const isMac = /Mac|iPhone|iPad/.test(navigator.platform);
const emit = defineEmits<{
  start: [];
  license: [];
  settings: [];
}>();

const tools = [
  { icon: "↔", name: "批量导入通讯录", desc: "文件 → 模板 → 列映射 → 预览 → 导出" },
  { icon: "▦", name: "模板系统", desc: "当前功能内部使用的模板能力" },
  { icon: "↓", name: "多格式导出", desc: "XLSX / CSV / VCF / TXT" },
];
</script>

<template>
  <div class="canvas">
    <section class="header">
      <div>
        <h1>选择你要增强的 Office 工具</h1>
        <p>围绕 WPS Word 与 Excel，提供更直接的本地操作。</p>
      </div>
      <div class="header-actions">
        <button v-if="SHOW_SUBSCRIPTION_UI" class="small-btn" @click="emit('license')">激活与授权</button>
        <button class="small-btn" :title="isMac ? '快捷键 ⌘,' : '快捷键 Ctrl,'" @click="emit('settings')">设置 ⌘,</button>
      </div>
    </section>

    <section class="switch-row">
      <div class="tabs">
        <button class="tab" :class="{ active: office === 'excel' }" @click="office = 'excel'">Excel</button>
        <button class="tab word" :class="{ active: office === 'word' }" @click="office = 'word'">Word</button>
      </div>
      <span class="context">
        {{ office === "excel" ? "Excel · 1 个已实现功能" : "Word · 当前暂无已实现功能" }}
      </span>
    </section>

    <section v-if="office === 'excel'">
      <div class="workspace">
        <article class="primary-task excel">
          <span class="task-label excel">EXCEL · 已实现</span>
          <div class="task-icon excel">X</div>
          <h2>批量导入通讯录</h2>
          <p>选择源表格与模板，完成列映射，预览处理结果，并导出为需要的通讯录格式。</p>
          <div class="meta">
            <span>XLSX / XLS / CSV</span>
            <span>VCF / TXT / CSV / XLSX</span>
            <span>本地处理</span>
          </div>
          <button class="start" @click="emit('start')">开始使用 →</button>
        </article>

        <aside class="tools">
          <div class="tools-title">
            <strong>Excel 工具</strong>
            <span>当前可用</span>
          </div>
          <div v-for="t in tools" :key="t.name" class="tool" @click="emit('start')">
            <div class="tool-icon">{{ t.icon }}</div>
            <div class="tool-text">
              <div class="tool-name">{{ t.name }}</div>
              <div class="tool-desc">{{ t.desc }}</div>
            </div>
            <div class="arrow">›</div>
          </div>
        </aside>
      </div>
    </section>

    <section v-else>
      <div class="workspace">
        <article class="primary-task word">
          <span class="task-label empty">WORD · 功能待加入</span>
          <div class="task-icon empty">W</div>
          <h2>Word 增强</h2>
          <p>当前项目资料只明确了 Word 作为目标产品范围，尚未提供已经实现的 Word 增强功能，因此这里不虚构具体功能。</p>
          <div class="meta">
            <span>功能规划中</span>
            <span>保持现有设计体系</span>
          </div>
          <button class="start disabled">暂未提供功能</button>
        </article>

        <aside class="tools">
          <div class="tools-title">
            <strong>Word 工具</strong>
            <span>暂无已实现功能</span>
          </div>
          <div class="empty-tools">
            <div>
              <div class="empty-icon">W</div>
              当前没有可展示的 Word 功能<br />新增功能后会沿用同一套 UI 规范
            </div>
          </div>
        </aside>
      </div>
    </section>

    <section class="bottom">
      <div class="box">
        <div class="box-head">
          <strong>最近使用</strong>
          <span>辅助信息</span>
        </div>
        <div class="recent-row">
          <div class="recent-icon">X</div>
          <div>
            <div class="recent-name">Excel 批量导入通讯录</div>
            <div class="recent-time">当前功能</div>
          </div>
        </div>
      </div>
      <VersionLicensePanel @license="emit('license')" />
    </section>

    <div class="note">WPS Enhancer · 为 WPS Office 提供本地增强能力</div>
  </div>
</template>
