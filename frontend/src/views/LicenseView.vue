<script setup lang="ts">
// 激活与授权页：订阅体系专用 UI，不嵌入任何业务功能逻辑。
import { computed, onMounted, ref } from "vue";
import { NButton, NInput, NTag, useMessage } from "naive-ui";
import { App } from "../../bindings/wps-enhancer-go/internal/app/index.js";
import { useAppMeta } from "../composables/useAppMeta";

const message = useMessage();
const emit = defineEmits<{ back: []; home: [] }>();
const meta = useAppMeta();

const loading = ref(false);
const key = ref("");

const statusLine = computed(() => {
  if (!meta.isPro.value) return "当前为免费版，输入激活码升级 Pro 版。";
  const exp = meta.expiresAt.value
    ? new Date(meta.expiresAt.value).toLocaleDateString("zh-CN")
    : "-";
  return `授权类型：${meta.licenseType.value || "pro"} ｜ 到期时间：${exp}`;
});

async function activate() {
  if (!key.value.trim()) {
    message.warning("请输入激活码");
    return;
  }
  loading.value = true;
  try {
    const result = await App.LicenseActivate(key.value.trim());
    if (result.ok) {
      message.success(result.message || "激活成功");
      key.value = "";
    } else {
      message.error(result.message || `激活失败（${result.code}）`);
    }
    await meta.refresh();
  } catch (e: any) {
    message.error("激活异常：" + (e?.message ?? e));
  } finally {
    loading.value = false;
  }
}

async function deactivate() {
  loading.value = true;
  try {
    await App.LicenseDeactivate();
    message.success("解绑成功，已恢复免费版");
    await meta.refresh();
  } catch (e: any) {
    message.error("解绑失败：" + (e?.message ?? e));
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  void meta.refresh();
});
</script>

<template>
  <div class="canvas">
    <section class="header">
      <div>
        <h1>激活与授权</h1>
        <p>激活码购买后获得，一码一设备；本机验签优先，联网验证兜底。</p>
      </div>
      <div class="header-actions">
        <button class="small-btn" @click="emit('back')">← 返回</button>
        <button class="small-btn" @click="emit('home')">首页</button>
      </div>
    </section>

    <section class="card task-panel">
      <div class="license-head">
        <h2>当前授权状态</h2>
        <n-tag v-if="meta.isPro.value" type="warning" size="small" :bordered="false">Pro 版</n-tag>
        <n-tag v-else size="small" :bordered="false">免费版</n-tag>
      </div>
      <p class="helper">{{ statusLine }}</p>
    </section>

    <section class="card task-panel">
      <h2>激活码激活</h2>
      <div class="field-row">
        <n-input v-model:value="key" placeholder="WPS-…" clearable style="flex: 1" @keyup.enter="activate" />
        <n-button type="primary" :loading="loading" @click="activate">激活</n-button>
      </div>
      <p class="helper">激活码由购买渠道发放，格式为 WPS- 开头。</p>
    </section>

    <section v-if="meta.isPro.value" class="card task-panel">
      <h2>解绑本机</h2>
      <p class="helper">解绑后可在新设备上重新激活（解绑有频率限制，频繁操作会被拒绝）。</p>
      <n-button size="small" type="error" secondary :loading="loading" @click="deactivate">解绑并恢复免费版</n-button>
    </section>
  </div>
</template>
