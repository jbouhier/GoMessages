<script lang="ts">
  // The list toggle (⌘B) and New chat (⌘N), in the list header, or in the
  // conversation header while the list is hidden.
  import { app } from "../lib/app.svelte";
  import { keys } from "../lib/platform";
  import Icon from "./Icon.svelte";

  let { vertical = false }: { vertical?: boolean } = $props();
  const verb = $derived(app.settings.sidebar_visible ? "Hide list" : "Show list");
</script>

<div class="buttons" class:vertical>
  <button class="icon-button" title="{verb}  {keys('⌘B')}" aria-label={verb} onclick={() => app.act("toggle-list")}>
    <Icon name="panel-left" />
  </button>
  <button class="icon-button" title="New chat  {keys('⌘N')}" aria-label="New chat" onclick={() => app.act("new-chat")}>
    <Icon name="new-chat" />
  </button>
</div>

<style>
  .buttons {
    display: flex;
    gap: 2px;
  }
  .vertical {
    flex-direction: column;
    align-items: center;
  }
</style>
