<script setup lang="ts">
// 版本与授权展示块：纯产品元信息，不依赖任何业务功能。
import { onMounted } from "vue";
import { useAppMeta } from "../composables/useAppMeta";

const emit = defineEmits<{ license: [] }>();
const { version, isPro, editionLabel, ensureLoaded } = useAppMeta();

onMounted(() => {
  void ensureLoaded();
});
</script>

<template>
  <div class="box version-license-panel">
    <div class="box-head">
      <strong>版本与授权</strong>
      <span>产品信息</span>
    </div>
    <div class="recent-row">
      <div class="version-icon">v</div>
      <div>
        <div class="recent-name">WPS Enhancer v{{ version || "—" }}</div>
        <div class="recent-time">
          <span class="dot" :style="{ background: isPro ? '#D97706' : '#22C55E' }"></span>
          {{ editionLabel }}
          <button type="button" class="link-btn" @click="emit('license')">管理授权 ›</button>
        </div>
      </div>
    </div>
  </div>
</template>
