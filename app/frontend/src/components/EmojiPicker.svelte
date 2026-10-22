<script lang="ts">
  // Every emoji, for a reaction beyond the quick six: search, categories,
  // skin tones, and the ones used most at the top. It's emoji-picker-element
  // with its data bundled in the app, so it works offline.
  //
  // Keys: it opens in the search box. Typing filters, ↑ ↓ pick a result and
  // Enter chooses it. Tab reaches the skin tones, categories and the grid
  // (arrows inside each), and wraps around inside the picker. Escape closes.
  import Picker from "emoji-picker-element/picker";
  import dataSource from "emoji-picker-element-data/en/emojibase/data.json?url";
  import { onMount } from "svelte";
  import { focusOrigin } from "../lib/popup";

  let { onpick, onclose }: { onpick: (emoji: string) => void; onclose: () => void } = $props();

  let host = $state<HTMLElement>();
  let picker: Picker | undefined;
  const returnTo = focusOrigin();

  function focusSearch() {
    picker?.shadowRoot?.querySelector<HTMLInputElement>("#search")?.focus();
  }

  /** Shift+Tab past the search box: the last stop inside the picker. */
  function focusLast() {
    const stops = [...(picker?.shadowRoot?.querySelectorAll<HTMLElement>("input, button:not([disabled]), [tabindex]:not([tabindex='-1'])") ?? [])].filter(
      (el) => {
        const r = el.getBoundingClientRect();
        if (!r.width || !r.height) return false;
        return typeof el.checkVisibility === "function" ? el.checkVisibility({ opacityProperty: true, visibilityProperty: true }) : true;
      },
    );
    (stops.at(-1) ?? null)?.focus();
  }

  onMount(() => {
    // Made here rather than in the markup so it never fetches the default
    // data from a CDN before it learns about the bundled copy.
    const el = new Picker({ dataSource });
    el.classList.add("light");
    picker = el;
    host!.replaceChildren(el);
    const pick = (e: Event) => onpick((e as CustomEvent<{ unicode?: string }>).detail.unicode ?? "");
    el.addEventListener("emoji-click", pick);
    // The search box exists once the picker has rendered.
    let tries = 0;
    const wait = () => {
      if (el.shadowRoot?.querySelector("#search")) focusSearch();
      else if (tries++ < 60) requestAnimationFrame(wait);
    };
    wait();
    return () => {
      el.removeEventListener("emoji-click", pick);
      const active = document.activeElement;
      if ((!active || active === document.body) && returnTo?.isConnected) returnTo.focus({ preventScroll: true });
    };
  });

  function onKey(e: KeyboardEvent) {
    // The picker handles its own keys; none of them reach the app's shortcuts.
    e.stopPropagation();
    if (e.key === "Escape") {
      e.preventDefault();
      onclose();
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="emoji-picker" role="dialog" aria-label="Choose a reaction" tabindex="-1" onkeydown={onKey}>
  <!-- Tab past either end wraps around inside the picker. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <span tabindex="0" onfocus={focusLast}></span>
  <div class="host" bind:this={host}></div>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <span tabindex="0" onfocus={focusSearch}></span>
</div>

<style>
  .emoji-picker {
    border-radius: 14px;
    overflow: hidden;
    border: 1px solid var(--divider);
    box-shadow: 0 8px 28px var(--shadow);
    background: var(--bg);
  }
  .host :global(emoji-picker) {
    display: block;
    width: 352px;
    height: 400px;
    max-width: calc(100vw - 16px);
    max-height: calc(100vh - 16px);
    /* The theme's colors, light or dark alike. */
    --background: var(--bg);
    --border-color: var(--divider);
    --border-size: 0;
    --button-active-background: var(--selected);
    --button-hover-background: var(--hover);
    --category-font-color: var(--subtle);
    --indicator-color: var(--accent);
    --input-border-color: var(--divider);
    --input-border-radius: 18px;
    --input-font-color: var(--text);
    --input-placeholder-color: var(--subtle);
    --input-padding: 0.35rem 0.75rem;
    --outline-color: var(--accent);
    --emoji-size: 1.5rem;
    --emoji-padding: 0.4rem;
  }
</style>
