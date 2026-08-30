// 应用元信息（版本 + 授权）：与具体业务解耦的唯一前端入口。
// 业务页不应直接调用 App.Version / App.License*；需要时用本 composable 或 VersionLicensePanel。
import { computed, ref } from "vue";
import { App } from "../../bindings/wps-enhancer-go/internal/app/index.js";

const version = ref("");
const isPro = ref(false);
const licenseType = ref("");
const expiresAt = ref<number | undefined>(undefined);
const loaded = ref(false);
const loading = ref(false);

let loadPromise: Promise<void> | null = null;

/** 拉取版本与授权状态（并发安全，可重复调用以刷新）。 */
async function refresh(): Promise<void> {
  if (loading.value && loadPromise) {
    await loadPromise;
    return;
  }
  loading.value = true;
  loadPromise = (async () => {
    try {
      try {
        version.value = (await App.Version()) || "";
      } catch {
        /* 版本读取失败静默 */
      }
      try {
        const st = await App.LicenseStatus();
        isPro.value = !!st?.is_pro;
        licenseType.value = st?.type ?? "";
        expiresAt.value = st?.expires_at;
      } catch {
        /* 授权读取失败静默 */
      }
      loaded.value = true;
    } finally {
      loading.value = false;
      loadPromise = null;
    }
  })();
  await loadPromise;
}

/** 确保至少加载过一次（首页/关于页挂载时调用）。 */
async function ensureLoaded(): Promise<void> {
  if (loaded.value) return;
  await refresh();
}

const editionLabel = computed(() => (isPro.value ? "Pro 版" : "免费版"));

/**
 * 应用元信息（单例状态）。
 * 业务功能（导入通讯录等）不要依赖本模块做流程编排；仅展示或显式能力门禁时使用。
 */
export function useAppMeta() {
  return {
    version,
    isPro,
    licenseType,
    expiresAt,
    loaded,
    loading,
    editionLabel,
    refresh,
    ensureLoaded,
  };
}
