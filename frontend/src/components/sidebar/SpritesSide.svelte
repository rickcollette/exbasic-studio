<script lang="ts">
  // Sprite Studio side bar: the sprite files of the project, new sheets and
  // merging several sheets or banks into one.
  import { wb } from '../../lib/workbench.svelte';
  import { fileIcon } from '../../lib/stores/workspace.svelte';
  import { spriteId } from '../../lib/stores/sprites.svelte';

  const sprites = wb.sprites;
  const groups = [
    { kind: 'sheet', label: 'Sprite sheets' },
    { kind: 'json', label: 'Frame files' },
    { kind: 'bank', label: 'PMG banks' },
    { kind: 'image', label: 'Images' },
  ] as const;

  let target = $state('merged.sprites.json');
  let anchor = $state<'top' | 'bottom'>('top');
  let prefixAll = $state(false);
  const mergeable = $derived(sprites.assets.filter(asset => asset.kind !== 'image'));

  function toggle(path: string, on: boolean) {
    if (!sprites.merging) return;
    sprites.merging.paths = on ? [...sprites.merging.paths, path] : sprites.merging.paths.filter(item => item !== path);
  }

  function move(path: string, delta: number) {
    if (!sprites.merging) return;
    const list = [...sprites.merging.paths];
    const from = list.indexOf(path);
    const to = from + delta;
    if (from < 0 || to < 0 || to >= list.length) return;
    [list[from], list[to]] = [list[to], list[from]];
    sprites.merging.paths = list;
  }

  function startMerge() {
    const directory = sprites.assets.find(asset => asset.kind === 'sheet' || asset.kind === 'bank')?.path.split('/').slice(0, -1).join('/') ?? '';
    target = `${directory ? directory + '/' : ''}merged.sprites.json`;
    sprites.merging = { paths: [] };
  }
</script>

{#if wb.workspace.project}
  <div class="section-header">
    <strong>Sprites</strong>
    <span class="section-actions">
      <button class="icon-button" title="New sprite sheet" aria-label="New sprite sheet" onclick={() => wb.execute('sprites.new')}><i aria-hidden="true" class="codicon codicon-new-file"></i></button>
      <button class="icon-button" title="Merge sprite sheets" aria-label="Merge sprite sheets" aria-pressed={!!sprites.merging} onclick={() => (sprites.merging ? (sprites.merging = null) : startMerge())}><i aria-hidden="true" class="codicon codicon-merge"></i></button>
      <button class="icon-button" title="Refresh" aria-label="Refresh sprite files" onclick={() => wb.workspace.refresh()}><i aria-hidden="true" class="codicon codicon-refresh"></i></button>
    </span>
  </div>

  {#if sprites.merging}
    <div class="merge">
      <p>Tick the files to merge, in order. The result is a new sheet holding every frame; generate it to get one combined PMG bank and index.</p>
      <ol class="merge-list">
        {#each mergeable as asset (asset.path)}
          {@const order = sprites.merging.paths.indexOf(asset.path)}
          <li>
            <label class="check"><input type="checkbox" checked={order >= 0} onchange={event => toggle(asset.path, event.currentTarget.checked)} />
              {#if order >= 0}<b class="order">{order + 1}</b>{/if}<span title={asset.path}>{asset.path}</span></label>
            {#if order >= 0}
              <button class="icon-button" title="Earlier" aria-label={`Move ${asset.path} earlier`} disabled={order === 0} onclick={() => move(asset.path, -1)}><i aria-hidden="true" class="codicon codicon-arrow-up"></i></button>
              <button class="icon-button" title="Later" aria-label={`Move ${asset.path} later`} disabled={order === sprites.merging.paths.length - 1} onclick={() => move(asset.path, 1)}><i aria-hidden="true" class="codicon codicon-arrow-down"></i></button>
            {/if}
          </li>
        {/each}
      </ol>
      {#if !mergeable.length}<p class="muted">No sheets, frame files or banks in this project yet.</p>{/if}
      <label for="merge-target">New sheet</label>
      <input id="merge-target" bind:value={target} spellcheck="false" />
      <label for="merge-anchor-side">Shorter frames</label>
      <select id="merge-anchor-side" bind:value={anchor}><option value="top">Pad below (align tops)</option><option value="bottom">Pad above (align feet)</option></select>
      <label class="check"><input type="checkbox" bind:checked={prefixAll} /> Prefix every frame with its file name</label>
      <div class="merge-actions">
        <button onclick={() => (sprites.merging = null)}>Cancel</button>
        <button class="primary" disabled={!sprites.merging.paths.length || !target.trim()} onclick={() => sprites.merge(sprites.merging?.paths ?? [], target, { anchor, prefixAll })}>Merge {sprites.merging.paths.length || ''}</button>
      </div>
    </div>
  {/if}

  {#each groups as group (group.kind)}
    {@const items = sprites.assets.filter(asset => asset.kind === group.kind)}
    {#if items.length}
      <h3 class="group">{group.label}</h3>
      <ul class="assets">
        {#each items as asset (asset.path)}
          <li><button class:active={wb.editors.active === spriteId(asset.path)} title={asset.path} onclick={() => sprites.open(asset.path)}>
            <i aria-hidden="true" class={`codicon codicon-${fileIcon(asset.path)}`}></i><span>{asset.path}</span>
            {#if sprites.doc(spriteId(asset.path))?.dirty}<b class="dirty-dot" aria-label="unsaved changes">●</b>{/if}
          </button></li>
        {/each}
      </ul>
    {/if}
  {/each}
  {#if !sprites.assets.length}
    <div class="empty-state">No sprite files yet. Create a sheet, or open a PNG, a PMG bank or a frames JSON (an object of frame names to row strings) from the Explorer.</div>
  {/if}
  <div class="help">
    <p><b>Sheets</b> (<code>*.sprites.json</code>) keep frames, the palette, which player planes each colour sets, and the files Generate writes: a PMG bank, its index, an exBASIC include and a PNG atlas.</p>
    <p>A sheet can keep its frames in a separate frames JSON (set <code>"source"</code>), so existing tools that read that file keep working.</p>
    <p>Any frames JSON opens in the text editor too: <i>Sprites: Open Sheet as JSON Text</i>.</p>
  </div>
{:else}
  <div class="empty-state">Open a folder to work with sprites.</div>
{/if}

<style>
  .group { margin: 10px 12px 2px; font-size: 11px; text-transform: uppercase; color: var(--muted); font-weight: 600; }
  .assets { list-style: none; margin: 0; padding: 0; }
  .assets button { width: 100%; justify-content: flex-start; border: none; border-radius: 0; background: none; padding: 3px 12px; text-align: left; }
  .assets button.active { background: var(--list-active); }
  .assets span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .merge { margin: 6px 10px; padding: 8px; border: 1px solid var(--accent); border-radius: 3px; }
  .merge p { margin: 0 0 6px; line-height: 1.4; }
  .merge-list { list-style: none; margin: 0; padding: 0; max-height: 240px; overflow: auto; }
  .merge-list li { display: flex; align-items: center; gap: 2px; }
  .merge-list label { flex: 1; min-width: 0; margin: 2px 0; }
  .merge-list span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .order { background: var(--accent); color: var(--accent-fg); border-radius: 8px; padding: 0 5px; font-size: 11px; }
  .merge-actions { display: flex; justify-content: flex-end; gap: 6px; margin-top: 8px; }
  .muted { color: var(--muted); }
  .help { margin: 14px 12px; color: var(--muted); font-size: 12px; line-height: 1.45; }
  .help p { margin: 0 0 8px; }
</style>
