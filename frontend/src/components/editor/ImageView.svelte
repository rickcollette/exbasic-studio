<script lang="ts">
  import { wb } from '../../lib/workbench.svelte';
  import { baseName } from '../../lib/stores/workspace.svelte';

  let { path, src }: { path: string; src: string } = $props();
  let zoom = $state(100);
  let fit = $state(true);
  let width = $state(0);
  let height = $state(0);

  function changeZoom(value: number) { fit = false; zoom = Math.max(10, Math.min(800, value)); }
  function loaded(event: Event): void {
    const image = event.currentTarget as HTMLImageElement;
    width = image.naturalWidth;
    height = image.naturalHeight;
  }
</script>

<div class="image-viewer">
  <div class="image-toolbar" aria-label="Image controls">
    <button class="icon-button" title="Zoom out" aria-label="Zoom out" onclick={() => changeZoom(zoom - 25)}><i aria-hidden="true" class="codicon codicon-zoom-out"></i></button>
    <button class="zoom-value" aria-label="Reset image zoom" title="Actual size" onclick={() => { fit = false; zoom = 100; }}>{fit ? 'Fit' : `${zoom}%`}</button>
    <button class="icon-button" title="Zoom in" aria-label="Zoom in" onclick={() => changeZoom(zoom + 25)}><i aria-hidden="true" class="codicon codicon-zoom-in"></i></button>
    <button class="icon-button" class:active={fit} title="Fit image to editor" aria-label="Fit image to editor" aria-pressed={fit} onclick={() => (fit = true)}><i aria-hidden="true" class="codicon codicon-screen-full"></i></button>
    <span class="image-info">{width && height ? `${width} × ${height}` : baseName(path)}</span>
    {#if /\.png$/i.test(path)}
      <button title="Open this PNG with the sprite overlay and import tools" onclick={() => wb.sprites.open(path)}><i aria-hidden="true" class="codicon codicon-paintcan"></i> Open in Sprite Studio</button>
    {/if}
  </div>
  <div class="image-stage" class:fit>
    <img {src} alt={baseName(path)} draggable="false" style:width={fit || !width ? undefined : `${width * zoom / 100}px`} onload={loaded} />
  </div>
</div>
