<script setup lang="ts">
// WPS Enhancer — 应用壳：Titlebar + 视图切换（首页/导入任务/激活/设置）
// 导航：双向链表记录视图与导入步骤；容量随宿主环境变化（约默认 100 步 / ~20KB）
import { defineComponent, onMounted, onUnmounted, ref } from "vue";
import {
  NConfigProvider,
  NDialogProvider,
  NMessageProvider,
  NNotificationProvider,
  dateZhCN,
  useNotification,
  zhCN,
} from "naive-ui";
import { Events } from "@wailsio/runtime";
import { themeOverrides } from "./theme";
import { NavHistory, suggestNavHistoryLimit, type AppView, type NavEntry } from "./navHistory";
import HomeView from "./views/HomeView.vue";
import ImportView from "./views/ImportView.vue";
import LicenseView from "./views/LicenseView.vue";
import SettingsView from "./views/SettingsView.vue";
import { SHOW_SUBSCRIPTION_UI } from "./featureFlags";

const history = new NavHistory("home", suggestNavHistoryLimit());
const view = ref<AppView>("home");
/** 导入会话计数：从首页「开始」进入时递增；从历史返回不递增以保留进度 */
const importSession = ref(0);
/** 由导航历史驱动的导入向导步骤（ImportView 同步） */
const importNavStep = ref(1);

function applyEntry(e: NavEntry) {
  // 订阅 UI 隐藏时不允许停在激活页（历史回退等）
  if (!SHOW_SUBSCRIPTION_UI && e.view === "license") {
    view.value = "home";
    return;
  }
  view.value = e.view;
  if (e.view === "import") {
    importNavStep.value = e.step ?? 1;
  }
}

function navigate(to: AppView, step?: number) {
  if (!SHOW_SUBSCRIPTION_UI && to === "license") return;
  const entry: NavEntry =
    to === "import"
      ? { view: to, step: step ?? (importNavStep.value || 1) }
      : { view: to };
  applyEntry(history.push(entry));
}

function goBack() {
  const prev = history.back();
  if (prev) applyEntry(prev);
}

/** 回到首页：首页无「返回」，不入栈，直接清空历史只留首页 */
function goHome() {
  if (view.value === "home") return;
  history.reset("home");
  applyEntry({ view: "home" });
}

function startImport() {
  importSession.value += 1;
  importNavStep.value = 1;
  navigate("import", 1);
}

/** 导入向导步骤变化时入栈（与「上一步/返回」共用链表） */
function onImportStep(step: number) {
  if (view.value !== "import") return;
  importNavStep.value = step;
  history.push({ view: "import", step });
}

// 启动自动检查更新结果通知（后端发 update:available 事件）
let notify: ((opts: any) => void) | null = null;

const ApiBinder = defineComponent({
  setup() {
    const n = useNotification();
    onMounted(() => {
      notify = (opts: any) => n.create(opts);
    });
    return () => null;
  },
});

let offUpdate: (() => void) | null = null;

const isMac = /Mac|iPhone|iPad/.test(navigator.platform);

function onGlobalKeydown(e: KeyboardEvent) {
  const mod = isMac ? e.metaKey : e.ctrlKey;
  if (mod && e.key === ",") {
    e.preventDefault();
    navigate("settings");
  }
}

onMounted(() => {
  window.addEventListener("keydown", onGlobalKeydown);
  offUpdate = Events.On("update:available", (e: any) => {
    let payload = e?.data;
    if (Array.isArray(payload)) payload = payload[0];
    if (!notify || !payload?.latest) return;
    notify({
      title: "发现新版本",
      content: `最新版本 v${payload.latest} 已发布${payload.current ? `（当前 v${payload.current}）` : ""}，可前往「设置 → 更新」下载。`,
      meta: "点击前往",
      duration: 10000,
      action: () => navigate("settings"),
    });
  });
});

onUnmounted(() => {
  window.removeEventListener("keydown", onGlobalKeydown);
  offUpdate?.();
});
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <n-notification-provider placement="top-right">
          <ApiBinder />
          <div class="app-window">
          <header class="titlebar">
            <div class="brand">
              <div class="logo">W</div>
              <span class="brand-name">WPS Enhancer</span>
              <span class="brand-type">Office 增强工具</span>
            </div>
          </header>

          <main class="main">
            <HomeView
              v-show="view === 'home'"
              @start="startImport"
              @license="navigate('license')"
              @settings="navigate('settings')"
            />
            <ImportView
              v-if="importSession > 0"
              v-show="view === 'import'"
              :key="importSession"
              :nav-step="importNavStep"
              @step="onImportStep"
              @back="goBack"
              @home="goHome"
            />
            <LicenseView v-if="SHOW_SUBSCRIPTION_UI && view === 'license'" @back="goBack" @home="goHome" />
            <SettingsView v-if="view === 'settings'" @back="goBack" @home="goHome" />
          </main>
        </div>
        </n-notification-provider>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
