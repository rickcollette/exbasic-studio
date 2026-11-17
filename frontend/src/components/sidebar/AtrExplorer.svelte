<script lang="ts">
  // ATR disk image explorer: browse, import, extract, rename and delete
  // entries of SDFS and EOSFS images.
  import { tick } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import { native } from '../../lib/native';
  import { fileIcon, baseName } from '../../lib/stores/workspace.svelte';
  import { atrParent, formatBytes } from '../../lib/stores/services.svelte';
  import { rowNavigation } from '../../lib/actions';

  let newFolder = $state<string | null>(null);
  let folderInput = $state<HTMLInputElement>();
  const atr = wb.atr;

  $effect(() => { if (newFolder !== null) void tick().then(() => { folderInput?.focus(); folderInput?.select(); }); });

  async function choose() {
    const selected = await native().dialogs.open({ title: 'Open ATR Disk Image', directory: wb.workspace.root || undefined, filters: [{ name: 'Atari disk images', pattern: '*.atr' }] });
    if (selected[0]) await atr.open(selected[0]);
  }

  async function chooseImports() {
    const selected = await native().dialogs.open({ title: 'Add Files or Folders to ATR', chooseDirectories: true, chooseFiles: true, multiple: true });
    if (selected.length) await atr.importHost(selected);
  }

  async function extract() {
    const selected = await native().dialogs.open({ title: 'Extract ATR Entry To Folder', chooseDirectories: true, chooseFiles: false });
    if (selected[0]) await atr.extractToDirectory(atr.selected, selected[0]);
  }

  async function rename() {
    const next = await wb.ui.prompt({ title: 'Rename ATR entry', message: `Rename or move ${atr.selected}`, label: 'New path', value: atr.selected, confirmLabel: 'Rename' });
    if (next) await atr.rename(atr.selected, next.trim());
  }

  function startDrag(event: DragEvent, path: string) {
    event.dataTransfer?.setData('application/x-exbasic-atr-path', path);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copyMove';
  }

  async function drop(event: DragEvent, destination: string) {
    event.preventDefault();
    event.stopPropagation();
    const projectEntry = event.dataTransfer?.getData('application/x-exbasic-project-path');
    if (projectEntry) { await atr.importProject([projectEntry], destination); return; }
    const entry = event.dataTransfer?.getData('application/x-exbasic-atr-path');
    if (entry) await atr.rename(entry, destination ? `${destination}/${baseName(entry)}` : baseName(entry));
  }

  function rowKey(event: KeyboardEvent, path: string, directory: boolean) {
    if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); atr.selected = path; if (directory) atr.toggle(path); }
    else if (event.key === 'Delete') { event.preventDefault(); void atr.remove(path); }
    else if (event.key === 'F2') { event.preventDefault(); event.stopPropagation(); atr.selected = path; void rename(); }
  }
</script>

{#if atr.view}
  <div class="section-header">
    <strong title={atr.view.path}>{baseName(atr.view.path.replaceAll('\\', '/'))}</strong>
    <span class="section-actions">
      <button class="icon-button" title="Add files or folders" aria-label="Add files or folders" onclick={chooseImports}><i aria-hidden="true" class="codicon codicon-add"></i></button>
      <button class="icon-button" title="New folder in image" aria-label="New folder in image" onclick={() => (newFolder = 'NEWFOLDER')}><i aria-hidden="true" class="codicon codicon-new-folder"></i></button>
      <button class="icon-button" title="Refresh" aria-label="Refresh image" onclick={() => atr.refresh()}><i aria-hidden="true" class="codicon codicon-refresh"></i></button>
      <button class="icon-button" title="Open another image" aria-label="Open another image" onclick={choose}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
    </span>
  </div>
  <div class="atr-summary"><span>{atr.view.filesystem === 'eos' ? 'EOSFS' : 'SDFS'}</span><span>{atr.view.sectors} × {atr.view.sectorSize}</span><span>{formatBytes(atr.view.freeBytes)} free</span></div>
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="tree" role="tree" tabindex="-1" aria-label="ATR image contents" id="atr-drop-root" data-file-drop-target use:rowNavigation={{ onLeft: row => { const path = row.dataset.path!; if (row.dataset.dir && !atr.collapsed.has(path)) atr.toggle(path); }, onRight: row => { const path = row.dataset.path!; if (row.dataset.dir && atr.collapsed.has(path)) atr.toggle(path); } }}
    ondragover={event => event.preventDefault()} ondrop={event => drop(event, '')}>
    {#each atr.visibleEntries as entry (entry.path)}
      <div role="treeitem" data-row data-path={entry.path} data-dir={entry.directory ? '1' : undefined} tabindex="-1" aria-level={entry.path.split('/').length}
        aria-selected={atr.selected === entry.path} aria-expanded={entry.directory ? !atr.collapsed.has(entry.path) : undefined}
        class="tree-row" style={`padding-left:${8 + (entry.path.split('/').length - 1) * 14}px`} title={`${entry.path} · ${formatBytes(entry.size)}`}
        id={entry.directory ? `atr-drop-${encodeURIComponent(entry.path)}` : undefined} data-file-drop-target={entry.directory ? '' : undefined} draggable="true"
        ondragstart={event => startDrag(event, entry.path)} ondragover={event => event.preventDefault()} ondrop={event => drop(event, entry.directory ? entry.path : atrParent(entry.path))}
        onclick={() => { atr.selected = entry.path; if (entry.directory) atr.toggle(entry.path); }} onkeydown={event => rowKey(event, entry.path, entry.directory)}>
        {#if entry.directory}
          <i aria-hidden="true" class={`codicon codicon-chevron-${atr.collapsed.has(entry.path) ? 'right' : 'down'}`}></i><i aria-hidden="true" class={`codicon codicon-folder${atr.collapsed.has(entry.path) ? '' : '-opened'}`}></i>
        {:else}
          <i aria-hidden="true" class="codicon tree-spacer"></i><i aria-hidden="true" class={`codicon codicon-${fileIcon(entry.path)}`}></i>
        {/if}
        <span class="name">{entry.name}</span>
        {#if entry.boot}<i role="img" class="codicon codicon-debug-start boot" title="Boot file" aria-label="boot file"></i>{/if}
        <small>{entry.directory ? '' : formatBytes(entry.size)}</small>
      </div>
    {/each}
    {#if (atr.view.entries ?? []).length === 0}<div class="empty-state"><i aria-hidden="true" class="codicon codicon-cloud-upload"></i> This image is empty. Drop files here or use Add.</div>{/if}
  </div>
  {#if newFolder !== null}
    <form class="inline-create" onsubmit={event => { event.preventDefault(); void atr.mkdir(newFolder ?? '').then(() => (newFolder = null)); }}>
      <i aria-hidden="true" class="codicon codicon-new-folder"></i>
      <input bind:this={folderInput} bind:value={newFolder} aria-label="New folder name" autocomplete="off" spellcheck="false" onkeydown={event => { if (event.key === 'Escape') { event.stopPropagation(); newFolder = null; } }} />
      <button class="icon-button" type="submit" title="Create folder" aria-label="Create folder"><i aria-hidden="true" class="codicon codicon-check"></i></button>
    </form>
  {/if}
  {#if atr.selected}
    <div class="row-actions">
      <button onclick={extract}><i aria-hidden="true" class="codicon codicon-export"></i> Extract…</button>
      <button onclick={rename}><i aria-hidden="true" class="codicon codicon-edit"></i> Rename</button>
      <button onclick={() => atr.remove(atr.selected)}><i aria-hidden="true" class="codicon codicon-trash"></i> Delete</button>
    </div>
  {/if}
  <p class="hint">Drag project or desktop files in. Drag entries onto the project explorer to extract them.</p>
{:else}
  <div class="welcome-card">
    <p>Open an SDFS or EOSFS ATR image and manage it like a folder.</p>
    <button class="primary" onclick={choose}><i aria-hidden="true" class="codicon codicon-folder-opened"></i> Open ATR Image…</button>
    {#if wb.workspace.project}<button onclick={() => atr.openConfigured()}><i aria-hidden="true" class="codicon codicon-save"></i> Open Project ATR</button>{/if}
    {#if wb.ui.busy.includes('Opening ATR image') || wb.ui.busy.includes('Opening project ATR')}<p role="status"><i aria-hidden="true" class="codicon codicon-loading codicon-modifier-spin"></i> Reading image…</p>{/if}
  </div>
{/if}