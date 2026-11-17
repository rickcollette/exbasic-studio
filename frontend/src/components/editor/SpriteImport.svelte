<script lang="ts">
  // Import panel of the Sprite Studio: cut a PNG into frames, read a PMG
  // bank with a chosen layout, or append the frames of other sprite files.
  import { untrack } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import type { SpriteDoc } from '../../lib/stores/sprites.svelte';
  import type { RGBAImage } from '../../lib/sprites/image';
  import { gridFor } from '../../lib/sprites/image';
  import { MAX_PLANES } from '../../lib/sprites/model';
  import { baseName } from '../../lib/stores/workspace.svelte';

  let { doc, onclose }: { doc: SpriteDoc; onclose: () => void } = $props();

  type Mode = 'image' | 'bank' | 'files';
  // The panel opens for one document; its fields start from that document.
  const start = untrack(() => {
    const pending = doc.pendingImage;
    const empty = !doc.sheet.frames.length;
    return {
      mode: (pending || empty ? 'image' : 'files') as Mode,
      imagePath: pending?.path ?? '', image: pending?.image ?? null,
      cellWidth: pending ? 8 : doc.sheet.width, cellHeight: pending ? Math.min(pending.image.height, 16) : doc.sheet.height,
      paletteMode: (pending || empty ? 'image' : 'sheet') as 'image' | 'sheet',
      replace: empty || !!pending,
      prefix: pending ? baseName(pending.path).replace(/\.[^.]*$/, '').toLowerCase() : 'frame',
      bankPath: doc.bank?.bank ?? '', indexPath: doc.bank?.index ?? '', bankReplace: !!doc.bank,
    };
  });
  let mode = $state<Mode>(start.mode);
  const files = $derived((wb.workspace.project?.files ?? []).map(file => file.path));
  const own = $derived(new Set([doc.path, ...doc.backingPaths]));

  // ---- Image --------------------------------------------------------------------
  let imagePath = $state(start.imagePath);
  let image = $state<RGBAImage | null>(start.image);
  let cellWidth = $state(start.cellWidth);
  let cellHeight = $state(start.cellHeight);
  let paletteMode = $state<'image' | 'sheet'>(start.paletteMode);
  let replace = $state(start.replace);
  let prefix = $state(start.prefix);
  let skipEmpty = $state(true);
  let imageCanvas = $state<HTMLCanvasElement>();
  const pngs = $derived(files.filter(path => /\.png$/i.test(path)));
  const grid = $derived(image && cellWidth > 0 && cellHeight > 0 ? gridFor(image, cellWidth, cellHeight) : null);

  async function chooseImage(path: string) {
    imagePath = path;
    image = null;
    if (!path) return;
    try {
      image = await wb.sprites.readImage(path);
      prefix = baseName(path).replace(/\.[^.]*$/, '').toLowerCase();
      if (cellHeight > image.height) cellHeight = image.height;
    } catch (error) { wb.sprites.fail(`Reading ${path}`, error); }
  }

  $effect(() => {
    if (!imageCanvas || !image) return;
    const zoom = Math.max(1, Math.min(8, Math.floor(480 / Math.max(image.width, image.height))));
    imageCanvas.width = image.width * zoom;
    imageCanvas.height = image.height * zoom;
    const context = imageCanvas.getContext('2d')!;
    context.imageSmoothingEnabled = false;
    const source = document.createElement('canvas');
    source.width = image.width; source.height = image.height;
    source.getContext('2d')!.putImageData(new ImageData(new Uint8ClampedArray(image.data), image.width, image.height), 0, 0);
    context.clearRect(0, 0, imageCanvas.width, imageCanvas.height);
    context.drawImage(source, 0, 0, imageCanvas.width, imageCanvas.height);
    if (grid) {
      context.strokeStyle = 'rgba(255, 210, 80, 0.8)';
      context.lineWidth = 1;
      for (let row = 0; row < grid.rows; row++) for (let column = 0; column < grid.columns; column++) {
        context.strokeRect(column * cellWidth * zoom + 0.5, row * cellHeight * zoom + 0.5, cellWidth * zoom - 1, cellHeight * zoom - 1);
      }
    }
  });

  function importImage() {
    if (!image) return;
    const count = wb.sprites.importImage(doc, image, { cellWidth, cellHeight, palette: paletteMode, replace, prefix, skipEmpty });
    wb.sprites.status(count ? `Imported ${count} frame${count === 1 ? '' : 's'} from ${imagePath}` : 'Every cell of the image is empty');
    if (count) onclose();
  }

  // ---- Bank -------------------------------------------------------------------------
  const banks = $derived(files.filter(path => /\.pmg$/i.test(path)));
  const jsons = $derived(files.filter(path => /\.json$/i.test(path) && !/\.sprites\.json$/i.test(path)));
  let bankPath = $state(start.bankPath);
  let layoutMode = $state<'auto' | 'index' | 'manual'>('auto');
  let indexPath = $state(start.indexPath);
  let bankWidth = $state(8);
  let bankHeight = $state(16);
  let bankPlanes = $state(1);
  let bankReplace = $state(start.bankReplace);

  async function importBank() {
    if (!bankPath) return;
    const layout = layoutMode === 'auto' ? null : layoutMode === 'index' ? { width: 8, height: 1, planes: 1, index: indexPath } : { width: bankWidth, height: bankHeight, planes: bankPlanes, index: null };
    const count = await wb.sprites.importBank(doc, bankPath, layout, bankReplace);
    if (count) { wb.sprites.status(`Imported ${count} frames from ${bankPath}`); onclose(); }
  }

  // ---- Other sprite files -------------------------------------------------------------
  const sources = $derived(wb.sprites.assets.filter(asset => asset.kind !== 'image' && !own.has(asset.path)));
  let picked = $state<string[]>([]);
  let anchor = $state<'top' | 'bottom'>('top');
  let prefixAll = $state(false);

  async function append() {
    if (!picked.length) return;
    await wb.sprites.appendFrom(doc, picked, { anchor, prefixAll });
    onclose();
  }

  function keydown(event: KeyboardEvent) { if (event.key === 'Escape') { event.stopPropagation(); onclose(); } }
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="import-backdrop" role="presentation" onkeydown={keydown}>
  <div class="import-panel" role="dialog" aria-modal="true" aria-labelledby="sprite-import-title">
    <header>
      <h2 id="sprite-import-title">Import Frames</h2>
      <button class="icon-button" title="Close" aria-label="Close" onclick={onclose}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </header>
    <div class="tabs" role="tablist" aria-label="Import from">
      <button role="tab" aria-selected={mode === 'image'} onclick={() => (mode = 'image')}><i aria-hidden="true" class="codicon codicon-file-media"></i> PNG image</button>
      <button role="tab" aria-selected={mode === 'bank'} onclick={() => (mode = 'bank')}><i aria-hidden="true" class="codicon codicon-symbol-color"></i> PMG bank</button>
      <button role="tab" aria-selected={mode === 'files'} onclick={() => (mode = 'files')}><i aria-hidden="true" class="codicon codicon-merge"></i> Sprite files</button>
    </div>

    <div class="body" role="tabpanel">
      {#if mode === 'image'}
        <label for="import-png">Image</label>
        <select id="import-png" value={imagePath} onchange={event => chooseImage(event.currentTarget.value)}>
          <option value="">Choose a PNG in the project…</option>
          {#each pngs as path}<option value={path}>{path}</option>{/each}
        </select>
        {#if image}
          <div class="grid-fields">
            <div><label for="import-cw">Cell width</label><input id="import-cw" type="number" min="1" max="32" bind:value={cellWidth} /></div>
            <div><label for="import-ch">Cell height</label><input id="import-ch" type="number" min="1" max="256" bind:value={cellHeight} /></div>
            <div><label for="import-prefix">Frame names</label><input id="import-prefix" bind:value={prefix} spellcheck="false" /></div>
          </div>
          <p class="hint">{image.width}x{image.height} pixels: {grid ? `${grid.columns} x ${grid.rows} cells` : 'choose a cell size'}. Transparent pixels (alpha below 50%) stay transparent; others take the nearest palette colour.</p>
          <div class="image-preview"><canvas bind:this={imageCanvas} aria-label="Image with the cell grid"></canvas></div>
          <fieldset>
            <legend>Colours</legend>
            <label class="check"><input type="radio" bind:group={paletteMode} value="image" /> Build the palette from the image (each colour gets its own plane combination)</label>
            <label class="check"><input type="radio" bind:group={paletteMode} value="sheet" disabled={!doc.sheet.palette.some(color => color.symbol !== '.')} /> Map to this sheet's palette</label>
          </fieldset>
          <label class="check"><input type="checkbox" bind:checked={replace} disabled={!doc.sheet.frames.length} /> Replace the frames of this sheet (otherwise append)</label>
          <label class="check"><input type="checkbox" bind:checked={skipEmpty} /> Skip empty cells</label>
        {/if}
        <footer><button onclick={onclose}>Cancel</button><button class="primary" disabled={!image || !grid} onclick={importImage}>Import</button></footer>
      {:else if mode === 'bank'}
        <label for="import-bank">PMG bank</label>
        <select id="import-bank" bind:value={bankPath}>
          <option value="">Choose a bank in the project…</option>
          {#each banks as path}<option value={path}>{path}</option>{/each}
        </select>
        <fieldset>
          <legend>Layout</legend>
          <label class="check"><input type="radio" bind:group={layoutMode} value="auto" /> Find the index file next to the bank</label>
          <label class="check"><input type="radio" bind:group={layoutMode} value="index" /> Use an index file</label>
          {#if layoutMode === 'index'}
            <select aria-label="Index file" bind:value={indexPath}>
              <option value="">Choose an index…</option>
              {#each jsons as path}<option value={path}>{path}</option>{/each}
            </select>
          {/if}
          <label class="check"><input type="radio" bind:group={layoutMode} value="manual" /> Equal frames of this size</label>
          {#if layoutMode === 'manual'}
            <div class="grid-fields">
              <div><label for="bank-w">Width</label><input id="bank-w" type="number" min="1" max="32" bind:value={bankWidth} /></div>
              <div><label for="bank-h">Height</label><input id="bank-h" type="number" min="1" max="256" bind:value={bankHeight} /></div>
              <div><label for="bank-p">Planes</label><input id="bank-p" type="number" min="1" max={MAX_PLANES} bind:value={bankPlanes} /></div>
            </div>
          {/if}
        </fieldset>
        <label class="check"><input type="checkbox" bind:checked={bankReplace} /> Replace the frames of this sheet (otherwise append)</label>
        <footer><button onclick={onclose}>Cancel</button><button class="primary" disabled={!bankPath || (layoutMode === 'index' && !indexPath)} onclick={importBank}>Import</button></footer>
      {:else}
        <p class="hint">Append the frames of other sheets, frame files or banks. Frames grow to the largest size, palettes are combined, and clashing names get the file's name as a prefix.</p>
        {#if sources.length}
          <ul class="sources">
            {#each sources as asset (asset.path)}
              <li><label class="check"><input type="checkbox" checked={picked.includes(asset.path)} onchange={event => (picked = event.currentTarget.checked ? [...picked, asset.path] : picked.filter(path => path !== asset.path))} /> {asset.path}</label></li>
            {/each}
          </ul>
        {:else}<p class="empty-state">No other sprite files in this project.</p>{/if}
        <div class="grid-fields">
          <div><label for="merge-anchor">Shorter frames</label>
            <select id="merge-anchor" bind:value={anchor}><option value="top">Pad below</option><option value="bottom">Pad above</option></select></div>
        </div>
        <label class="check"><input type="checkbox" bind:checked={prefixAll} /> Prefix every new frame with its file name</label>
        <footer><button onclick={onclose}>Cancel</button><button class="primary" disabled={!picked.length} onclick={append}>Append {picked.length || ''} File{picked.length === 1 ? '' : 's'}</button></footer>
      {/if}
    </div>
  </div>
</div>

<style>
  .import-backdrop { position: absolute; inset: 0; background: rgba(0, 0, 0, 0.45); display: grid; place-items: center; z-index: 20; padding: 16px; }
  .import-panel { background: var(--popup-bg); border: 1px solid var(--border); border-radius: 6px; width: min(640px, 100%); max-height: 100%; display: flex; flex-direction: column; box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5); }
  header { display: flex; align-items: center; padding: 8px 12px; border-bottom: 1px solid var(--border); }
  header h2 { flex: 1; margin: 0; font-size: 14px; }
  .tabs { display: flex; gap: 4px; padding: 8px 12px 0; }
  .tabs [aria-selected="true"] { border-color: var(--accent); background: var(--list-active); }
  .body { padding: 4px 12px 12px; overflow: auto; }
  .grid-fields { display: grid; grid-template-columns: repeat(3, 1fr); gap: 8px; }
  .hint { color: var(--muted); font-size: 12px; line-height: 1.4; }
  .image-preview { max-height: 300px; overflow: auto; border: 1px solid var(--border); background: repeating-conic-gradient(#333 0% 25%, #222 0% 50%) 50% / 16px 16px; }
  .image-preview canvas { display: block; image-rendering: pixelated; }
  fieldset { border: 1px solid var(--border); border-radius: 3px; margin: 8px 0; padding: 2px 10px 6px; }
  legend { color: var(--muted); font-size: 12px; padding: 0 4px; }
  input[type="radio"] { width: auto; accent-color: var(--accent); }
  .sources { list-style: none; margin: 0; padding: 0; max-height: 220px; overflow: auto; border: 1px solid var(--border); border-radius: 3px; }
  .sources li { padding: 0 8px; }
  footer { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
