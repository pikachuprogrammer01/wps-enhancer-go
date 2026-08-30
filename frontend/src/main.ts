// WPS Enhancer — Vue 3 + Naive UI 入口
import { createApp } from "vue";
import App from "./App.vue";
import "./../style.css";

// 注：macOS 红绿灯让位已由 CSS 无条件处理（.brand padding-left: 86px），
// 不依赖运行时平台检测（Wails environment 注入时机不可靠）。

createApp(App).mount("#app");
