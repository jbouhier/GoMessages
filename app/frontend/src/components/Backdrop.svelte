<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { mountBackdrop } from "../lib/backdrop";
  import { backgroundOf } from "../lib/themes";

  // The theme's own background under the panes: a CSS painting, with a
  // shader over it for the themes that have one. Transparency fades it.
  let host = $state<HTMLDivElement>();
  const background = $derived(backgroundOf(app.theme, app.dark, app.gradient));

  $effect(() => {
    if (!host || !background?.shader) return;
    return mountBackdrop(host, background.shader);
  });
</script>

{#if background}
  <div class="backdrop" bind:this={host} style:background={background.css} aria-hidden="true"></div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 0;
    pointer-events: none;
    opacity: var(--background-opacity, 1);
  }
  .backdrop :global(.layer) {
    position: absolute;
    inset: 0;
  }
</style>
