<script lang="ts">
  import { app } from "../lib/app.svelte";
  import { fallbackColor } from "../lib/match";
  import { mediaURL } from "../lib/types";

  let {
    name,
    color = "",
    group = false,
    id = "",
    size = 42,
  }: { name: string; color?: string; group?: boolean; id?: string; size?: number } = $props();

  $effect(() => app.wantAvatar(id));
  const photo = $derived(app.avatarPath(id));
  const initial = $derived(([...name].find((c) => /[\p{L}\p{N}]/u.test(c)) ?? "#").toUpperCase());
  const bg = $derived(/^#[0-9a-f]{6}$/i.test(color) ? color : fallbackColor(name));
</script>

<div class="avatar" style:width="{size}px" style:height="{size}px" style:background={photo ? "transparent" : bg}>
  {#if photo}
    <img src={mediaURL(photo)} alt="" draggable="false" />
  {:else if group}
    <span style:font-size="{size * 0.5}px">👥</span>
  {:else}
    <span style:font-size="{size * 0.45}px">{initial}</span>
  {/if}
</div>

<style>
  .avatar {
    flex: none;
    border-radius: 50%;
    display: grid;
    place-items: center;
    overflow: hidden;
    color: white;
  }
  img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
</style>
