<script lang="ts">
  // Project explorer: recent projects and project creation when nothing is
  // open; otherwise the file tree, inline create inputs and the outline.
  import { tick } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import { fileIcon, parentOf } from '../../lib/stores/workspace.svelte';
  import { rowNavigation } from '../../lib/actions';
  import { outline, isExbasicSource } from '../../lib/language';

  let folderPath = $state('');
  let projectName = $state('helloatari');
  let createInput = $state<HTMLInputElement>();
  let outlineOpen = $state(true);

  $effect(() => { if (!folderPath && wb.workspace.env) folderPath = wb.workspace.env.workingDir; });
  $effect(() => {
    if (wb.creating) void tick().then(() => { createInput?.focus(); const value = createInput?.value ?? ''; createInput?.setSelectionRange(value.lastIndexOf('/') + 1, value.length); });
  });

  const tree = $derived(wb.workspace.tree);
  const symbols = $derived.by(() => {
    const tab = wb.editors.activeTab;
    if (!tab || !isExbasicSource(tab.path)) return [];
    void tab.version; void tab.epoch;
    return outline(wb.editors.text(tab.path));
  });

  function startDrag(event: DragEvent, path: string) {
    event.dataTransfer?.setData('application/x-exbasic-project-path', path);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = 'copyMove';
  }

  async function drop(event: DragEvent, destination: string) {
    event.preventDefault();
    event.stopPropagation();
    const source = event.dataTransfer?.getData('application/x-exbasic-project-path');
    if (source) {
      const name = source.split('/').pop() ?? source;
      await wb.movePath(source, destination ? `${destination}/${name}` : name);
      return;
    }
    const entry = event.dataTransfer?.getData('application/x-exbasic-atr-path');
    if (entry) await wb.atr.extractToProject(entry, destination);
  }

  function rowKey(event: KeyboardEvent, path: string, directory: boolean) {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      if (directory) wb.workspace.toggleFolder(path); else void wb.openFile(path);
    } else if (event.key === 'Delete') { event.preventDefault(); wb.workspace.selected = path; void wb.deleteSelected(); }
    else if (event.key === 'F2') { event.preventDefault(); event.stopPropagation(); wb.workspace.selected = path; void wb.renameSelected(); }
  }

  function collapse(row: HTMLElement) { const path = row.dataset.path!; if (row.dataset.dir && !wb.workspace.collapsed.has(path)) wb.workspace.toggleFolder(path); }
  function expand(row: HTMLElement) { const path = row.dataset.path!; if (row.dataset.dir && wb.workspace.collapsed.has(path)) wb.workspace.toggleFolder(path); }

  function recentName(path: string) { return path.replaceAll('\\', '/').split('/').filter(Boolean).pop() ?? path; }
</script>

{#if wb.workspace.project}
  <div class="section-header">
    <strong>{wb.workspace.project.config.name}</strong>
    <span class="section-actions">
      <button class="icon-button" title="New file" aria-label="New file" onclick={() => wb.beginCreate('file')}><i aria-hidden="true" class="codicon codicon-new-file"></i></button>
      <button class="icon-button" title="New folder" aria-label="New folder" onclick={() => wb.beginCreate('folder')}><i aria-hidden="true" class="codicon codicon-new-folder"></i></button>
      <button class="icon-button" title="Refresh explorer" aria-label="Refresh explorer" onclick={() => wb.workspace.refresh()}><i aria-hidden="true" class="codicon codicon-refresh"></i></button>
      <button class="icon-button" title="Close folder" aria-label="Close folder" onclick={() => wb.execute('file.close')}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </span>
  </div>
  {#if wb.workspace.restricted}
    <div class="notice">
      <i aria-hidden="true" class="codicon codicon-shield"></i>
      <span>Restricted mode. You can browse and edit; building, running, the terminal and Git hooks are disabled.</span>
      <button onclick={() => wb.execute('trust')}>Trust Folder…</button>
    </div>
  {/if}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="tree" role="tree" tabindex="-1" aria-label="Project files" id="project-drop-root" data-file-drop-target use:rowNavigation={{ onLeft: collapse, onRight: expand }}
    ondragover={event => event.preventDefault()} ondrop={event => drop(event, '')}>
    {#each tree as entry (entry.path)}
      <div role="treeitem" data-row data-path={entry.path} data-dir={entry.directory ? '1' : undefined} tabindex="-1" aria-level={entry.depth + 1}
        aria-selected={wb.workspace.selected === entry.path} aria-expanded={entry.directory ? !wb.workspace.collapsed.has(entry.path) : undefined}
        class="tree-row" class:active={wb.editors.active === entry.path} style={`padding-left:${8 + entry.depth * 14}px`} title={entry.path}
        id={entry.directory ? `project-drop-${encodeURIComponent(entry.path)}` : undefined} data-file-drop-target={entry.directory ? '' : undefined} draggable="true"
        ondragstart={event => startDrag(event, entry.path)} ondragover={event => event.preventDefault()} ondrop={event => drop(event, entry.directory ? entry.path : parentOf(entry.path))}
        onclick={() => { wb.workspace.selected = entry.path; if (entry.directory) wb.workspace.toggleFolder(entry.path); else void wb.openFile(entry.path); }}
        onkeydown={event => rowKey(event, entry.path, entry.directory)}>
        {#if entry.directory}
          <i aria-hidden="true" class={`codicon codicon-chevron-${wb.workspace.collapsed.has(entry.path) ? 'right' : 'down'}`}></i>
          <i aria-hidden="true" class={`codicon codicon-folder${wb.workspace.collapsed.has(entry.path) ? '' : '-opened'}`}></i>
        {:else}
          <i aria-hidden="true" class="codicon tree-spacer"></i><i aria-hidden="true" class={`codicon codicon-${fileIcon(entry.path)}`}></i>
        {/if}
        <span class="name">{entry.name}</span>
        {#if wb.editors.tab(entry.path)?.dirty}<b class="dirty-dot" aria-label="unsaved changes">●</b>{/if}
        {#if wb.workspace.selected === entry.path}
          <span class="tree-row-actions">
            <button class="icon-button" tabindex="-1" title="Rename (F2)" aria-label={`Rename ${entry.name}`} onclick={event => { event.stopPropagation(); void wb.renameSelected(); }}><i aria-hidden="true" class="codicon codicon-edit"></i></button>
            <button class="icon-button" tabindex="-1" title="Delete (Del)" aria-label={`Delete ${entry.name}`} onclick={event => { event.stopPropagation(); void wb.deleteSelected(); }}><i aria-hidden="true" class="codicon codicon-trash"></i></button>
          </span>
        {/if}
      </div>
    {/each}
    {#if tree.length === 0}<div class="empty-state">This folder is empty. Create a file to begin.</div>{/if}
  </div>
  {#if wb.creating}
    <form class="inline-create" onsubmit={event => { event.preventDefault(); void wb.commitCreate(); }}>
      <i aria-hidden="true" class={`codicon codicon-new-${wb.creating.kind}`}></i>
      <input bind:this={createInput} bind:value={wb.creating.value} aria-label={`New ${wb.creating.kind} path`} autocomplete="off" spellcheck="false"
        onkeydown={event => { if (event.key === 'Escape') { event.stopPropagation(); wb.creating = null; } }} />
      <button class="icon-button" type="submit" title={`Create ${wb.creating.kind}`} aria-label={`Create ${wb.creating.kind}`}><i aria-hidden="true" class="codicon codicon-check"></i></button>
      <button class="icon-button" type="button" title="Cancel" aria-label="Cancel" onclick={() => (wb.creating = null)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </form>
  {/if}
  <button class="collapsible" aria-expanded={outlineOpen} onclick={() => (outlineOpen = !outlineOpen)}>
    <i aria-hidden="true" class={`codicon codicon-chevron-${outlineOpen ? 'down' : 'right'}`}></i> Outline
  </button>
  {#if outlineOpen}
    <div class="outline" role="group" aria-label="Document outline">
      {#each symbols as symbol (symbol.kind + symbol.name + symbol.line)}
        <button title={symbol.detail} onclick={() => wb.editors.open(wb.editors.active, { line: symbol.line, column: symbol.column })}>
          <i aria-hidden="true" class={`codicon codicon-${symbol.kind === 'function' ? 'symbol-method' : symbol.kind === 'include' ? 'file-symlink-file' : 'symbol-array'}`}></i>
          <span>{symbol.name}</span><small>{symbol.line}</small>
        </button>
      {/each}
      {#if symbols.length === 0}<div class="empty-state">{wb.editors.activeTab ? 'No symbols in this file.' : 'Open an exBASIC file to see its outline.'}</div>{/if}
    </div>
  {/if}
{:else}
  <div class="welcome-card">
    <p>You have not opened a folder yet.</p>
    <label for="folder-path">Folder path</label>
    <div class="input-row">
      <input id="folder-path" bind:value={folderPath} onkeydown={event => { if (event.key === 'Enter') { event.preventDefault(); void wb.openFolder(folderPath); } }} autocomplete="off" spellcheck="false" />
      <button class="icon-button" title="Choose a folder to open" aria-label="Choose a folder to open" disabled={wb.workspace.opening} onclick={() => wb.chooseFolder()}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
    </div>
    <div class="divider"><span>or create a project</span></div>
    <label for="project-name">Project name</label>
    <input id="project-name" bind:value={projectName} autocomplete="off" spellcheck="false" />
    <button disabled={wb.workspace.opening || !folderPath.trim() || !projectName.trim()} onclick={() => wb.createProject(folderPath, projectName)}>Create Project</button>
    {#if wb.workspace.recents.length}
      <div class="divider"><span>recent</span></div>
      <ul class="recent-list" aria-label="Recent projects">
        {#each wb.workspace.recents as recent (recent.path)}
          <li>
            <button class="recent" title={recent.path} onclick={() => wb.openFolder(recent.path)}><strong>{recent.name || recentName(recent.path)}</strong><small>{recent.path}</small></button>
            <button class="icon-button" title="Remove from recent projects" aria-label={`Remove ${recent.name || recentName(recent.path)} from recent projects`} onclick={() => wb.workspace.forgetRecent(recent.path)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
          </li>
        {/each}
      </ul>
    {/if}
  </div>
{/if}
