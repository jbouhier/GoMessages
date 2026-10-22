// Keyboard and placement for popups: menus, pickers and confirmations.
//
// - Focus moves into the popup when it opens: to [data-autofocus] or the
//   first button. It goes back to what had it when the popup closes.
// - Tab and Shift+Tab cycle inside the popup (focus never leaks behind it).
// - Arrows move between buttons. In a menu, ← → walk the buttons in order
//   and ↑ ↓ go to the row above or below (the reaction row, then the items).
//   In a dialog every arrow walks the buttons in order.
// - Home and End jump to the first and last button.
// - Enter and Space press the focused button (the browser's own behavior).
// - Escape closes.
// Keys handled here stop at the popup, so the app's own shortcuts behind it
// (Home, End, Escape to the message box) don't fire as well.

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

const returns = new WeakMap<HTMLElement, HTMLElement | null>();

/** Where focus should go back to when a popup opening now closes: what has
 *  focus, or, from inside another popup, what that one came from. */
export function focusOrigin(): HTMLElement | null {
  const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
  const owner = active?.closest<HTMLElement>("[data-popup]");
  return owner ? (returns.get(owner) ?? null) : active;
}

export interface PopupOptions {
  /** Escape, or anything else that should close the popup. */
  onclose: () => void;
  /** "menu": ↑ ↓ move by row. "dialog": all arrows move in order. */
  kind?: "menu" | "dialog";
}

export function popup(node: HTMLElement, options: PopupOptions) {
  let opts = options;
  // Opened from inside another popup on its way out (Delete in a menu
  // opening its confirmation): hand focus back to what that one came from.
  const returnTo = focusOrigin();
  node.dataset.popup = "";
  returns.set(node, returnTo);

  const targets = () => [...node.querySelectorAll<HTMLElement>(FOCUSABLE)];

  const first = node.querySelector<HTMLElement>("[data-autofocus]") ?? targets()[0];
  first?.focus();

  function move(to: HTMLElement | undefined) {
    to?.focus();
  }

  /** The nearest button on the next row down (or up), wrapping around. */
  function byRow(list: HTMLElement[], cur: HTMLElement, dir: 1 | -1): HTMLElement | undefined {
    const r = cur.getBoundingClientRect();
    const mid = r.left + r.width / 2;
    const rows = list.filter((el) => {
      const o = el.getBoundingClientRect();
      return dir > 0 ? o.top >= r.bottom - 2 : o.bottom <= r.top + 2;
    });
    const pool = rows.length ? rows : list.filter((el) => el !== cur);
    if (!pool.length) return undefined;
    // The closest row that way, or, past the edge, the first (last) row.
    const edge = (el: HTMLElement) => el.getBoundingClientRect().top;
    const want = dir > 0 ? Math.min(...pool.map(edge)) : Math.max(...pool.map(edge));
    const row = pool.filter((el) => Math.abs(edge(el) - want) < 2);
    // In that row, the button under the current one.
    return row.reduce((best, el) => {
      const d = (x: HTMLElement) => {
        const o = x.getBoundingClientRect();
        return Math.abs(o.left + o.width / 2 - mid);
      };
      return d(el) < d(best) ? el : best;
    });
  }

  function onKey(e: KeyboardEvent) {
    const list = targets();
    const cur = document.activeElement as HTMLElement;
    const i = list.indexOf(cur);
    const at = (n: number) => list[(n + list.length) % list.length];
    const menu = (opts.kind ?? "menu") === "menu";
    switch (e.key) {
      case "Escape":
        opts.onclose();
        break;
      case "Tab":
        move(at(i < 0 ? 0 : i + (e.shiftKey ? -1 : 1)));
        break;
      case "ArrowRight":
        move(at(i + 1));
        break;
      case "ArrowLeft":
        move(at(i - 1));
        break;
      case "ArrowDown":
        move(menu && i >= 0 ? byRow(list, cur, 1) : at(i + 1));
        break;
      case "ArrowUp":
        move(menu && i >= 0 ? byRow(list, cur, -1) : at(i - 1));
        break;
      case "Home":
        move(list[0]);
        break;
      case "End":
        move(list[list.length - 1]);
        break;
      case "Enter":
      case " ":
        // The focused button handles it; only keep it from the app.
        e.stopPropagation();
        return;
      default:
        return;
    }
    e.preventDefault();
    e.stopPropagation();
  }

  node.addEventListener("keydown", onKey);

  return {
    update(next: PopupOptions) {
      opts = next;
    },
    destroy() {
      node.removeEventListener("keydown", onKey);
      // Give focus back unless something else took it on the way out (the
      // message box after Reply, say).
      const active = document.activeElement;
      const lost = !active || active === document.body || node.contains(active);
      if (lost && returnTo?.isConnected) returnTo.focus({ preventScroll: true });
    },
  };
}

export interface Anchor {
  /** The button or point the popup hangs from. */
  rect: { left: number; top: number; right: number; bottom: number };
  /** Open below (the default) or above the anchor; flips if it doesn't fit. */
  side?: "below" | "above";
  /** Line up the popup's start, end or center with the anchor's. */
  align?: "start" | "end" | "center";
}

const MARGIN = 8;
const GAP = 4;

/** Places a fixed popup next to its anchor, inside the window. */
export function anchored(node: HTMLElement, anchor: Anchor) {
  function place(a: Anchor) {
    const w = node.offsetWidth;
    const h = node.offsetHeight;
    const { rect } = a;
    let left = a.align === "end" ? rect.right - w : a.align === "center" ? (rect.left + rect.right) / 2 - w / 2 : rect.left;
    const below = rect.bottom + GAP;
    const above = rect.top - GAP - h;
    let top = (a.side ?? "below") === "below" ? below : above;
    if (top + h > window.innerHeight - MARGIN && above >= MARGIN) top = above;
    else if (top < MARGIN && below + h <= window.innerHeight - MARGIN) top = below;
    left = Math.max(MARGIN, Math.min(left, window.innerWidth - w - MARGIN));
    top = Math.max(MARGIN, Math.min(top, window.innerHeight - h - MARGIN));
    node.style.left = `${left}px`;
    node.style.top = `${top}px`;
  }
  place(anchor);
  return { update: place };
}

/** An anchor at a point: where a right-click happened. */
export const atPoint = (x: number, y: number): Anchor => ({ rect: { left: x, top: y, right: x, bottom: y }, side: "below", align: "start" });
