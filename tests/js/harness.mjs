// Runs the scripts embedded in the Rust sources inside a fake browser, so
// tests exercise the exact strings the app ships (no copies to drift).
// Zero dependencies: node:test + node:vm.

import { readFileSync } from "node:fs";
import vm from "node:vm";

const ROOT = new URL("../../", import.meta.url);

function rustRawString(file, constName) {
  const src = readFileSync(new URL(file, ROOT), "utf8");
  const start = src.indexOf(`const ${constName}: &str = r##"`);
  if (start < 0) throw new Error(`${constName} not found in ${file}`);
  const body = src.slice(src.indexOf('r##"', start) + 4);
  return body.slice(0, body.indexOf('"##;'));
}

export const INIT_JS = rustRawString("src/bin/gomessages/webview.rs", "INIT_JS");
export const SETTINGS_TEMPLATE = rustRawString(
  "src/bin/gomessages/settings.rs",
  "SETTINGS_TEMPLATE",
);
export const NOTIFY_FIXTURE = JSON.parse(
  readFileSync(new URL("tests/fixtures/notify_payloads.json", ROOT), "utf8"),
);

/** Manual clock: nothing runs until the test calls tick(). */
export class FakeTimers {
  constructor() {
    this.now = 0;
    this.seq = 0;
    this.tasks = new Map();
  }
  add(fn, ms, repeat) {
    const id = ++this.seq;
    this.tasks.set(id, { fn, at: this.now + (ms || 0), every: repeat ? ms || 1 : 0 });
    return id;
  }
  clear(id) {
    this.tasks.delete(id);
  }
  tick(ms) {
    const end = this.now + ms;
    for (;;) {
      let next = null;
      for (const [id, t] of this.tasks) {
        if (t.at <= end && (!next || t.at < next[1].at)) next = [id, t];
      }
      if (!next) break;
      const [id, t] = next;
      this.now = t.at;
      if (t.every) t.at += t.every;
      else this.tasks.delete(id);
      t.fn();
    }
    this.now = end;
  }
}

class FakeElement {
  // Real DOM coerces these to booleans; `el.checked = undefined` is false.
  get checked() {
    return this._checked;
  }
  set checked(v) {
    this._checked = !!v;
  }
  get disabled() {
    return this._disabled;
  }
  set disabled(v) {
    this._disabled = !!v;
  }
  get hidden() {
    return this._hidden;
  }
  set hidden(v) {
    this._hidden = !!v;
  }
  constructor(tag, attrs = {}) {
    this.tagName = tag;
    this.style = {};
    this.children = [];
    this.parent = null;
    this.hidden = false;
    this.disabled = false;
    this.checked = false;
    this.textContent = "";
    this.innerHTML = "";
    this.attrs = { ...attrs };
    this.id = attrs.id || "";
    const classes = new Set((attrs.class || "").split(/\s+/).filter(Boolean));
    this.classList = {
      toggle: (c, on) => {
        const want = on === undefined ? !classes.has(c) : !!on;
        want ? classes.add(c) : classes.delete(c);
        return want;
      },
      contains: (c) => classes.has(c),
      add: (c) => classes.add(c),
      remove: (c) => classes.delete(c),
    };
  }
  setAttribute(k, v) {
    this.attrs[k] = String(v);
    if (k === "style") this.style.cssText = String(v);
  }
  getAttribute(k) {
    return this.attrs[k] ?? null;
  }
  appendChild(c) {
    c.parent = this;
    this.children.push(c);
    return c;
  }
  remove() {
    if (this.parent) this.parent.children = this.parent.children.filter((c) => c !== this);
    this.parent = null;
  }
  getBoundingClientRect() {
    return { top: 0, left: 0, width: 0, height: 0 };
  }
}

/**
 * A page context for INIT_JS.
 * opts: host, title, unreadRows (number or () => throws), ipc (false = no
 * window.ipc), dark, permissions (original navigator.permissions).
 */
export function makePage(opts = {}) {
  const timers = new FakeTimers();
  const posts = [];
  const listeners = {};
  const docListeners = {};
  const root = new FakeElement("html");
  const state = { title: opts.title ?? "Messages", unreadRows: opts.unreadRows ?? 0 };
  let focusCalls = 0;

  function SWReg() {}
  SWReg.prototype.showNotification = () => Promise.reject(new Error("original"));
  SWReg.prototype.getNotifications = () => Promise.reject(new Error("original"));

  const permissions = opts.permissions ?? {
    query: (d) => Promise.resolve({ name: d && d.name, state: "prompt", original: true }),
  };

  const ctx = {
    console,
    Promise,
    JSON,
    String,
    Object,
    Array,
    Error,
    parseInt,
    setTimeout: (fn, ms) => timers.add(fn, ms, false),
    clearTimeout: (id) => timers.clear(id),
    setInterval: (fn, ms) => timers.add(fn, ms, true),
    clearInterval: (id) => timers.clear(id),
    location: { hostname: opts.host ?? "messages.google.com" },
    navigator: { permissions },
    ServiceWorkerRegistration: SWReg,
    matchMedia: (q) => ({ matches: !!opts.dark && q.includes("dark") }),
    addEventListener: (type, fn) => (listeners[type] ||= []).push(fn),
    focus: () => {
      focusCalls++;
    },
    document: {
      documentElement: root,
      body: null,
      get title() {
        return state.title;
      },
      createElement: (tag) => new FakeElement(tag),
      getElementById: (id) => root.children.find((c) => c.id === id) || null,
      addEventListener: (type, fn) => (docListeners[type] ||= []).push(fn),
      querySelectorAll: (sel) => {
        if (typeof state.unreadRows === "function") return state.unreadRows(sel);
        return sel === '[data-e2e-is-unread="true"]' ? new Array(state.unreadRows) : [];
      },
    },
  };
  if (opts.ipc !== false) ctx.ipc = { postMessage: (m) => posts.push(String(m)) };
  ctx.window = ctx;
  vm.createContext(ctx);
  vm.runInContext(INIT_JS, ctx, { filename: "INIT_JS" });

  return {
    ctx,
    timers,
    posts,
    root,
    state,
    get focusCalls() {
      return focusCalls;
    },
    /** Run code inside the page (e.g. what Google's script would do). */
    run: (code) => vm.runInContext(code, ctx),
    fire: (type, ev) => (listeners[type] || []).forEach((fn) => fn(ev)),
    key: (key, mods = {}) => {
      const ev = {
        key,
        code: mods.code || "",
        metaKey: !!mods.meta,
        ctrlKey: !!mods.ctrl,
        prevented: false,
        stopped: false,
        preventDefault() {
          this.prevented = true;
        },
        stopImmediatePropagation() {
          this.stopped = true;
        },
      };
      (listeners.keydown || []).forEach((fn) => fn(ev));
      return ev;
    },
    notifyPosts: () =>
      posts.filter((p) => p.startsWith("notify:")).map((p) => JSON.parse(p.slice(7))),
  };
}

/** Settings window: real template + script, fake DOM built from its ids. */
export function makeSettings({ platform = "macos", prefs = {}, zoom = "100,7,16,7" } = {}) {
  const fullPrefs = {
    keep_running: true,
    tray: false,
    hide_dock: false,
    notification_sound: "system_notification",
    notification_popup: true,
    banner_status: "allowed",
    tray_ok: true,
    ...prefs,
  };
  const html = SETTINGS_TEMPLATE.replace("__SOUND_HINT__", "hint")
    .replace("__ZOOM_STATE__", zoom)
    .replace("__PLATFORM__", platform)
    .replace("__PREFS__", JSON.stringify(fullPrefs));

  // Elements with an id, plus their inline handlers.
  const els = {};
  const tagRe = /<(\w+)([^>]*?\bid="([^"]+)"[^>]*)>/g;
  for (const m of html.matchAll(tagRe)) {
    const attrs = {};
    for (const a of m[2].matchAll(/([\w-]+)="([^"]*)"/g)) attrs[a[1]] = a[2];
    if (/\bhidden\b/.test(m[2].replace(/"[^"]*"/g, ""))) attrs.hidden = "";
    const el = new FakeElement(m[1], attrs);
    el.hidden = "hidden" in attrs;
    els[m[3]] = el;
  }
  const sent = [];
  const ctx = {
    console,
    JSON,
    setTimeout: () => 0,
    clearTimeout: () => {},
    ipc: { postMessage: (m) => sent.push(m) },
    addEventListener: () => {},
    document: {
      getElementById: (id) => {
        if (!els[id]) throw new Error(`settings script asked for missing #${id}`);
        return els[id];
      },
    },
  };
  ctx.window = ctx;
  vm.createContext(ctx);
  const script = html.slice(html.indexOf("<script>") + 8, html.indexOf("</script>"));
  vm.runInContext(script, ctx, { filename: "settings" });

  return {
    els,
    sent,
    ctx,
    setPrefs: (p) => ctx.__setPrefs({ ...fullPrefs, ...p }),
    /** Run an element's inline handler (onchange/onclick) as the browser would. */
    trigger: (id, attr) => {
      const code = els[id].attrs[attr].replace(/&quot;/g, '"');
      vm.runInContext(`(function(){${code}})`, ctx).call(els[id]);
    },
  };
}
