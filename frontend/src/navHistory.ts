// 应用级导航历史（双向链表）：视图 + 导入向导步骤均可入栈；
// 「返回」沿 prev 回退；容量按宿主环境估算并裁掉最旧节点。

export type AppView = "home" | "import" | "license" | "settings";

/** 一条导航记录。import 可带向导步骤 1–3。 */
export interface NavEntry {
  view: AppView;
  /** 仅 view=import 时有意义 */
  step?: number;
}

export interface NavNode {
  entry: NavEntry;
  prev: NavNode | null;
  next: NavNode | null;
}

/** 单节点粗算占用（含对象头、字符串、指针），用于说明默认 100 步量级。 */
export const NAV_NODE_BYTES_ESTIMATE = 200;

/**
 * 按宿主环境给出历史容量上限。
 * - navigator.deviceMemory（GiB，Chromium/WebKit 部分环境）
 * - 否则用 hardwareConcurrency 兜底
 * - 默认目标约 100 步（约 100×200B ≈ 20KB，可忽略）
 */
export function suggestNavHistoryLimit(
  env: { deviceMemory?: number; hardwareConcurrency?: number } = typeof navigator !== "undefined" ? navigator : {},
): number {
  const mem = typeof env.deviceMemory === "number" ? env.deviceMemory : undefined;
  const cores = env.hardwareConcurrency || 4;
  if (mem !== undefined) {
    if (mem <= 2) return 32;
    if (mem <= 4) return 64;
    if (mem <= 8) return 100;
    return Math.min(256, 100 + Math.floor((mem - 8) * 12));
  }
  if (cores <= 2) return 48;
  if (cores <= 4) return 80;
  return 100;
}

function sameEntry(a: NavEntry, b: NavEntry): boolean {
  return a.view === b.view && (a.step ?? 0) === (b.step ?? 0);
}

function cloneEntry(e: NavEntry): NavEntry {
  return e.step === undefined ? { view: e.view } : { view: e.view, step: e.step };
}

/** 双向链表导航历史（有容量上限）。 */
export class NavHistory {
  private head: NavNode;
  private current: NavNode;
  private size: number;
  readonly maxSize: number;

  constructor(initial: NavEntry | AppView = "home", maxSize?: number) {
    const entry = typeof initial === "string" ? { view: initial } : cloneEntry(initial);
    this.head = { entry, prev: null, next: null };
    this.current = this.head;
    this.size = 1;
    this.maxSize = Math.max(2, maxSize ?? suggestNavHistoryLimit());
  }

  get view(): AppView {
    return this.current.entry.view;
  }

  get entry(): NavEntry {
    return cloneEntry(this.current.entry);
  }

  get length(): number {
    return this.size;
  }

  canBack(): boolean {
    return this.current.prev != null;
  }

  canForward(): boolean {
    return this.current.next != null;
  }

  /**
   * 前进到新记录：切断 forward 链后接到当前节点之后。
   * 首页无「返回」按钮 → 不入栈，改为 reset，释放整段历史以省内存。
   * 与当前相同则不入栈。超出 maxSize 时丢掉最旧节点。
   */
  push(input: NavEntry | AppView): NavEntry {
    const entry = typeof input === "string" ? { view: input } : cloneEntry(input);
    if (entry.view === "home") {
      this.reset("home");
      return this.entry;
    }
    if (sameEntry(this.current.entry, entry)) return cloneEntry(this.current.entry);
    this.dropForward();
    const node: NavNode = { entry, prev: this.current, next: null };
    this.current.next = node;
    this.current = node;
    this.size += 1;
    this.trimOldest();
    return cloneEntry(this.current.entry);
  }

  /**
   * 清空历史，只保留无「返回」的首页节点（或指定起点）。
   * 用于「首页」按钮与抵达首页时的内存回收。
   */
  reset(initial: NavEntry | AppView = "home"): NavEntry {
    this.dropEntireChain();
    const entry = typeof initial === "string" ? { view: initial } : cloneEntry(initial);
    this.head = { entry, prev: null, next: null };
    this.current = this.head;
    this.size = 1;
    return cloneEntry(this.current.entry);
  }

  /** 回退一步；回到首页时清空 forward 并折叠为单节点。 */
  back(): NavEntry | null {
    if (!this.current.prev) return null;
    this.current = this.current.prev;
    if (this.current.entry.view === "home") {
      this.dropForward();
      this.head = this.current;
      this.current.prev = null;
      this.size = 1;
    }
    return cloneEntry(this.current.entry);
  }

  /** 前进（redo）一步；无 forward 返回 null。 */
  forward(): NavEntry | null {
    if (!this.current.next) return null;
    this.current = this.current.next;
    return cloneEntry(this.current.entry);
  }

  /** 估算当前链表占用字节（说明用）。 */
  estimatedBytes(): number {
    return this.size * NAV_NODE_BYTES_ESTIMATE;
  }

  private dropEntireChain(): void {
    let n: NavNode | null = this.head;
    while (n) {
      const next = n.next;
      n.prev = null;
      n.next = null;
      n = next;
    }
    this.size = 0;
  }

  private dropForward(): void {
    let n = this.current.next;
    let dropped = 0;
    while (n) {
      dropped += 1;
      const next = n.next;
      n.prev = null;
      n.next = null;
      n = next;
    }
    this.current.next = null;
    this.size -= dropped;
  }

  /** 从链表头丢掉最旧节点，直到 size ≤ maxSize（不丢当前节点）。 */
  private trimOldest(): void {
    while (this.size > this.maxSize && this.head !== this.current && this.head.next) {
      const old = this.head;
      this.head = old.next;
      this.head.prev = null;
      old.next = null;
      this.size -= 1;
    }
  }
}
