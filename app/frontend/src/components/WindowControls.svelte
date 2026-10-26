<script lang="ts">
  // Windows and Linux: the window has no system title bar (customTitleBar in
  // window.go). The top strip of each pane stands in for it, in the theme's
  // colors, and these are its minimize, maximize and close buttons.
  import { Window } from "@wailsio/runtime";
  import { app } from "../lib/app.svelte";
  import { overHTTP } from "../lib/go";

  let maximised = $state(false);

  // Only Go knows if the window is maximised; it changes size either way.
  async function readMaximised() {
    if (overHTTP) return;
    try {
      maximised = await Window.IsMaximised();
    } catch {
      // Not in the app window: nothing to read.
    }
  }
  void readMaximised();

  function run(fn: () => Promise<void>) {
    if (!overHTTP) void fn().catch(() => {});
  }

  // A double-click on the title strip maximises, as on any title bar. Wails
  // does this itself on macOS only.
  function onDoubleClick(e: MouseEvent) {
    const target = e.target as Element | null;
    if (!target || target.closest("button, input, textarea, a")) return;
    if (getComputedStyle(target).getPropertyValue("--wails-draggable").trim() !== "drag") return;
    run(() => Window.ToggleMaximise());
  }
</script>

<svelte:window onresize={readMaximised} ondblclick={onDoubleClick} />

<!-- The photo viewer is dark in every theme: the buttons go light over it. -->
<div class="controls no-drag" class:over-viewer={!!app.viewer} role="group" aria-label="Window">
  <button title="Minimize" aria-label="Minimize" onclick={() => run(() => Window.Minimise())}>
    <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 5.5h10" /></svg>
  </button>
  <button
    title={maximised ? "Restore" : "Maximize"}
    aria-label={maximised ? "Restore" : "Maximize"}
    onclick={() => run(() => Window.ToggleMaximise())}>
    {#if maximised}
      <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M2.5 2.5V.5h7v7h-2" /><rect x=".5" y="2.5" width="7" height="7" /></svg>
    {:else}
      <svg viewBox="0 0 10 10" aria-hidden="true"><rect x=".5" y=".5" width="9" height="9" /></svg>
    {/if}
  </button>
  <button class="close" title="Close" aria-label="Close" onclick={() => run(() => Window.Close())}>
    <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M.5.5l9 9M9.5.5l-9 9" /></svg>
  </button>
</div>

<style>
  /* Above the panes and overlays, so the window can always be closed. */
  .controls {
    position: fixed;
    top: 0;
    right: 0;
    z-index: 80;
    display: flex;
    height: var(--top-inset);
  }
  button {
    width: 46px;
    height: 100%;
    display: grid;
    place-items: center;
    color: var(--text);
  }
  button:hover {
    background: var(--hover);
  }
  button:focus-visible {
    outline-offset: -2px;
  }
  .over-viewer button {
    color: #f1f3f4;
  }
  .over-viewer button:hover {
    background: rgba(255, 255, 255, 0.12);
  }
  .close:hover,
  .over-viewer .close:hover {
    background: var(--danger);
    color: var(--bg);
  }
  svg {
    width: 10px;
    height: 10px;
    fill: none;
    stroke: currentColor;
    stroke-width: 1;
    shape-rendering: crispEdges;
  }
</style>
