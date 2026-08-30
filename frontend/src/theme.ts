// 设计系统 tokens → Naive UI themeOverrides（依据 WPS-Enhancer-UI-Design-System v1.0）
import type { GlobalThemeOverrides } from "naive-ui";

export const tokens = {
  brand: "#E5484D",
  brandHover: "#D63B42",
  brandSoft: "#FFF1F2",
  word: "#2563EB",
  wordSoft: "#EFF6FF",
  excel: "#16A34A",
  excelSoft: "#F0FDF4",
  bg: "#F6F6F7",
  surface: "#FFFFFF",
  surfaceSubtle: "#FAFAFA",
  border: "#E5E7EB",
  borderStrong: "#D4D4D8",
  text: "#18181B",
  textSecondary: "#52525B",
  textTertiary: "#71717A",
  textMuted: "#A1A1AA",
  success: "#16A34A",
  warning: "#D97706",
  danger: "#DC2626",
} as const;

// 字号体系（设计规范 §7.1）
export const type = {
  pageTitle: "22px",
  taskTitle: "20px",
  sectionTitle: "13px",
  body: "12px",
  secondary: "11px",
  caption: "10px",
} as const;

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: tokens.brand,
    primaryColorHover: tokens.brandHover,
    primaryColorPressed: tokens.brandHover,
    primaryColorSuppl: tokens.brand,
    primaryColorFaded: tokens.brandSoft,
    infoColor: tokens.word,
    infoColorHover: tokens.word,
    successColor: tokens.excel,
    warningColor: tokens.warning,
    errorColor: tokens.danger,
    bodyColor: tokens.bg,
    cardColor: tokens.surface,
    modalColor: tokens.surface,
    popoverColor: tokens.surface,
    tableColor: tokens.surface,
    inputColor: tokens.surface,
    borderColor: tokens.border,
    dividerColor: tokens.border,
    textColorBase: tokens.text,
    textColor1: tokens.text,
    textColor2: tokens.textSecondary,
    textColor3: tokens.textTertiary,
    borderRadius: "7px",
    borderRadiusSmall: "6px",
    borderRadiusMedium: "7px",
    borderRadiusLarge: "10px",
    fontFamily:
      '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif',
    // 设计规范 §7.1：Body 12px 为基线，Naive 组件字号与手写 CSS 一致
    fontSize: "12px",
  },
  Button: {
    heightMedium: "36px",
    fontSizeMedium: "12px",
    fontWeight: "600",
    borderRadiusMedium: "7px",
    borderRadiusLarge: "7px",
    borderRadiusSmall: "7px",
  },
  Card: {
    borderRadius: "12px",
  },
  Steps: {
    fontSize: "12px",
    titleFontSizeMedium: "13px",
    titleFontSizeSmall: "12px",
  },
  Tag: {
    borderRadius: "6px",
    fontSizeSmall: "10px",
  },
  Table: {
    fontSize: "12px",
    thFontWeight: "600",
  },
};
