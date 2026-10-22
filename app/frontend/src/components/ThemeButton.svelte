<script lang="ts">
  // The theme menu, top right: light, dark or system, and the theme, without
  // going to Settings. Hovering or focusing a swatch shows it on the chats
  // behind; a click keeps it, Escape or leaving puts the saved one back.
  import { app } from "../lib/app.svelte";
  import { keys } from "../lib/platform";
  import { anchored, popup, type Anchor } from "../lib/popup";
  import { THEMES } from "../lib/themes";
  import Icon from "./Icon.svelte";

  type Mode = "system" | "light" | "dark";
  const MODES: [Mode, string][] = [
    ["light", "Light"],
    ["dark", "Dark"],
    ["system", "System"],
  ];

  let anchor = $state<Anchor | null>(null);
  let grid = $state<HTMLElement>();
  const saved = $derived(app.settings.theme);

  function open(e: MouseEvent) {
    e.stopPropagation();
    if (anchor) return close();
    const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
    anchor = { rect: r, side: "below", align: "end" };
  }

  function close() {
    app.previewTheme = null;
    anchor = null;
  }

  function setMode(mode: Mode) {
    if (app.settings.mode === mode) return;
    app.restyle(() => {
      app.settings.mode = mode;
      app.saveSettings();
    });
  }

  function preview(id: string) {
    app.previewTheme = id === saved ? null : id;
  }

  function choose(id: string) {
    // Already on screen through the preview: no crossfade needed.
    app.previewTheme = null;
    if (app.settings.theme === id) return;
    app.settings.theme = id;
    app.saveSettings();
  }

  function leaveGrid(e: FocusEvent | PointerEvent) {
    const to = (e as FocusEvent).relatedTarget as Node | null;
    if (e.type === "focusout" && to && grid?.contains(to)) return;
    if (e.type === "pointerleave" && grid?.contains(document.activeElement)) return;
    app.previewTheme = null;
  }
</script>

<svelte:window onclick={() => anchor && close()} onblur={() => anchor && close()} onresize={() => anchor && close()} />

<button
  class="icon-button"
  class:open={!!anchor}
  title="Appearance  ({keys('⌘⇧L')} flips light / dark)"
  aria-label="Appearance"
  aria-haspopup="true"
  aria-expanded={!!anchor}
  onclick={open}>
  <Icon name="palette" size={17} />
</button>

{#if anchor}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
  <div
    class="menu theme-menu"
    role="dialog"
    tabindex="-1"
    aria-label="Appearance"
    use:anchored={anchor}
    use:popup={{ onclose: close }}
    onclick={(e) => e.stopPropagation()}>
    <div class="segmented" role="radiogroup" aria-label="Mode">
      {#each MODES as [value, label] (value)}
        <button
          class:on={app.settings.mode === value}
          role="radio"
          aria-checked={app.settings.mode === value}
          data-autofocus={app.settings.mode === value ? "" : undefined}
          onclick={() => setMode(value)}>{label}</button>
      {/each}
    </div>
    <div
      class="swatches"
      role="radiogroup"
      aria-label="Theme"
      tabindex="-1"
      bind:this={grid}
      onpointerleave={leaveGrid}
      onfocusout={leaveGrid}>
      {#each THEMES as t (t.id)}
        <button
          class="swatch"
          class:on={saved === t.id}
          role="radio"
          aria-checked={saved === t.id}
          aria-label={t.name}
          title={t.credit ? `${t.name}: ${t.credit}` : t.name}
          style:--light={t.light.accent}
          style:--dark={t.dark.accent}
          onpointerenter={() => preview(t.id)}
          onfocus={() => preview(t.id)}
          onclick={() => choose(t.id)}>
          <span class="dot"></span>
          <span class="name">{t.name}</span>
        </button>
      {/each}
    </div>
    <div class="sep"></div>
    <button
      class="item more"
      onclick={() => {
        close();
        app.act("settings");
      }}>More in Settings…</button>
  </div>
{/if}

<style>
  .icon-button.open {
    background: var(--hover);
    color: var(--text);
  }
  .theme-menu {
    width: 280px;
    padding: 10px;
  }
  /* Nearly solid: messages behind would read through the labels. */
  :global(:root.see-through) .theme-menu {
    background: color-mix(in srgb, var(--bg) 97%, transparent);
  }
  .sep {
    height: 1px;
    margin: 0 4px 4px;
    background: var(--divider);
  }
  .segmented {
    display: flex;
    gap: 2px;
    padding: 2px;
    border-radius: 18px;
    background: var(--hover);
  }
  .segmented button {
    flex: 1;
    padding: 6px 0;
    border-radius: 16px;
    font-size: 13px;
  }
  .segmented button.on {
    background: var(--bg);
    box-shadow: 0 1px 3px var(--shadow);
    font-weight: 600;
  }
  .swatches {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 2px;
    margin: 10px 0 6px;
  }
  .swatch {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 5px;
    padding: 7px 0 5px;
    border-radius: var(--row-radius);
    font-size: 12px;
    color: var(--subtle);
  }
  .swatch:hover,
  .swatch:focus-visible {
    background: var(--hover);
  }
  /* The theme's accent on its light and dark sides. */
  .dot {
    width: 26px;
    height: 26px;
    border-radius: 50%;
    background: linear-gradient(135deg, var(--light) 50%, var(--dark) 50%);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--text) 18%, transparent);
  }
  .swatch.on .dot {
    outline: 2px solid var(--text);
    outline-offset: 2px;
  }
  .swatch.on .name {
    color: var(--text);
    font-weight: 600;
  }
  .more {
    justify-content: center;
    padding: 7px 12px;
    font-size: 13px;
    color: var(--subtle);
  }
</style>
