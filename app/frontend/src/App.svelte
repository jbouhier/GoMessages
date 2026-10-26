<script lang="ts">
  import { app } from "./lib/app.svelte";
  import { commandKey, customTitleBar, isMac } from "./lib/platform";
  import { setTitle } from "./lib/go";
  import { backgroundOf, paneOpacity } from "./lib/themes";
  import { Go } from "./lib/go";
  import Backdrop from "./components/Backdrop.svelte";
  import LinkCard from "./components/LinkCard.svelte";
  import NewChat from "./components/NewChat.svelte";
  import Pairing from "./components/Pairing.svelte";
  import SettingsPage from "./components/SettingsPage.svelte";
  import Shortcuts from "./components/Shortcuts.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import Switcher from "./components/Switcher.svelte";
  import ThemeButton from "./components/ThemeButton.svelte";
  import Info from "./components/Info.svelte";
  import Thread from "./components/Thread.svelte";
  import Viewer from "./components/Viewer.svelte";
  import WindowControls from "./components/WindowControls.svelte";

  app.start();

  // The theme and its light or dark half, and how much shows through. Transparency
  // fades the panes and the theme's background together, down to what's
  // behind the window: the blurred desktop where the system blurs (macOS;
  // Windows uses Acrylic), the theme's plain color elsewhere.
  $effect(() => {
    const root = document.documentElement;
    const { theme, dark, gradient } = app;
    const t = Math.min(Math.max(app.settings.transparency || 0, 0), 100) / 100;
    const blur = app.blurSupported && t > 0;
    root.dataset.theme = theme;
    root.dataset.mode = dark ? "dark" : "light";
    root.classList.toggle("dark", dark);
    // Sky's dark half: white glass on blue.
    root.classList.toggle("sky", theme === "sky" && dark);
    root.classList.toggle("blur", blur);
    // Something shows behind the panes: fields and cards turn to glass.
    root.classList.toggle("see-through", t > 0 || !!backgroundOf(theme, dark, gradient));
    const [pane, list] = paneOpacity(theme, dark, gradient);
    root.style.setProperty("--pane-alpha", `${pane * (1 - t)}%`);
    root.style.setProperty("--list-alpha", `${list * (1 - t)}%`);
    root.style.setProperty("--background-opacity", String(1 - t));
    // Rows go translucent with the panes. Inline, so they outrank the
    // theme rules.
    const rows: Record<string, string> = {
      "--hover": "color-mix(in srgb, var(--text) 8%, transparent)",
      "--selected": "color-mix(in srgb, var(--accent) 22%, transparent)",
      "--divider": "color-mix(in srgb, var(--text) 12%, transparent)",
    };
    for (const [k, v] of Object.entries(rows)) {
      if (t > 0) root.style.setProperty(k, v);
      else root.style.removeProperty(k);
    }
    if (app.blurSupported) void Go.SetBlur(blur);
    root.classList.toggle("mac", isMac);
    root.classList.toggle("titlebar", customTitleBar);
  });

  // The window's own light or dark (traffic lights, scroll bars, and the
  // system blur's tint) matches the theme's. Following the system, the
  // window is left to it: a forced appearance would also force the page's
  // prefers-color-scheme, so the system's own change would never be seen.
  // Once the window follows again, read the system's mode afresh.
  let appearance: string | null = null;
  $effect(() => {
    const next = app.settings.mode === "system" ? "" : app.dark ? "dark" : "light";
    if (next === appearance) return;
    appearance = next;
    void Go.SetAppearance(next).then(() => {
      if (next === "") app.readSystemDark();
    });
  });

  $effect(() => {
    const n = app.unreadCount;
    setTitle(n ? `Messages (${n})` : "Messages");
  });

  const overlay = $derived(!!(app.switcher || app.showShortcuts || app.viewer));
  const inApp = $derived(app.link === "connected" || app.link === "phone_offline" || app.link === "connecting");

  // Keys the menu bar doesn't carry, or can't catch: everything with ⌘/Ctrl
  // lives in the menu, which works on every platform and shows the shortcut.
  function onKey(e: KeyboardEvent) {
    if (overlay || !inApp) return;
    const target = e.target as HTMLElement;
    const editing = target?.tagName === "TEXTAREA" || target?.tagName === "INPUT";
    const composing = editing && (target as HTMLTextAreaElement).value.length > 0;
    const arrow = e.key === "ArrowDown" ? 1 : e.key === "ArrowUp" ? -1 : 0;
    if (e.ctrlKey && e.key === "Tab") {
      app.act(e.shiftKey ? "previous" : "next");
    } else if (arrow && e.altKey && !e.metaKey && !e.ctrlKey) {
      // The menu carries ⌥↓/⌥↑ too, but on macOS the web view takes the key
      // first (caret move, scroll) and the menu never sees it.
      app.act(e.shiftKey ? (arrow > 0 ? "next-unread" : "previous-unread") : arrow > 0 ? "next" : "previous");
    } else if (e.key === "PageUp" && app.selected) {
      app.scrollThread("page-up");
    } else if (e.key === "PageDown" && app.selected) {
      app.scrollThread("page-down");
    } else if (e.key === "Home" && app.selected && (commandKey(e) || !composing)) {
      app.scrollThread("top");
    } else if (e.key === "End" && app.selected && (commandKey(e) || !composing)) {
      app.scrollThread("bottom");
    } else if (e.key === "Escape" && app.info && app.current && !app.settingsOpen && !app.newChat) {
      app.info = false;
    } else if (e.key === "Escape" && !editing && !app.settingsOpen && !app.newChat) {
      app.focusComposer++;
    } else return;
    e.preventDefault();
  }
</script>

<svelte:window onkeydown={onKey} />

<Backdrop />
<main>
  {#if app.link === "starting"}
    <div class="center"><div class="spinner"></div></div>
  {:else if app.link === "unpaired" || app.link === "pairing"}
    <Pairing />
  {:else}
    {#if app.settings.sidebar_visible}<Sidebar />{/if}
    {#if app.settingsOpen}
      <SettingsPage />
    {:else if app.newChat}
      <NewChat />
    {:else if app.current}
      <Thread conv={app.current} />
      {#if app.info}<Info conv={app.current} />{/if}
    {:else}
      <div class="center drag">
        <div class="corner"><ThemeButton /></div>
        <div>
          <p>Pick a conversation</p>
          <p class="subtle">Your chats appear on the left.</p>
        </div>
      </div>
    {/if}
  {/if}
</main>

{#if app.switcher}<Switcher />{/if}
{#if app.showShortcuts}<Shortcuts />{/if}
{#if app.viewer}<Viewer />{/if}
<LinkCard />
{#if app.toast}<div class="toast" role="status">{app.toast}</div>{/if}
{#if customTitleBar}<WindowControls />{/if}

<style>
  main {
    position: relative;
    z-index: 1;
    display: flex;
    height: 100vh;
  }
  .center {
    flex: 1;
    background: var(--pane);
    display: grid;
    place-items: center;
    text-align: center;
  }
  .center:has(.corner) {
    position: relative;
  }
  .corner {
    position: absolute;
    top: calc(var(--top-inset) + 14px);
    right: 16px;
  }
  .toast {
    position: fixed;
    left: 50%;
    bottom: 90px;
    transform: translateX(-50%);
    z-index: 70;
    padding: 10px 16px;
    border-radius: 8px;
    background: var(--text);
    color: var(--bg);
  }
</style>
