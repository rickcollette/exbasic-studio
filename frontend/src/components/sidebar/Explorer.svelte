<script lang="ts">
  // Project explorer: recent projects and project creation when nothing is
  // open; otherwise the file tree, inline create inputs and the outline.
  import { onMount, tick } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import { baseName, fileIcon, parentOf } from '../../lib/stores/workspace.svelte';
  import { rowNavigation } from '../../lib/actions';
  import { outline, isExbasicSource } from '../../lib/language';
  import { native } from '../../lib/native';

  let folderPath = $state('');
  let projectName = $state('helloatari');
  let createInput = $state<HTMLInputElement>();
  let outlineOpen = $state(true);
  let renameInput = $state<HTMLInputElement>();
  let contextMenu = $state<HTMLElement>();
  let renaming = $state<{ path: string; value: string } | null>(null);
  let context = $state<{ x: number; y: number; path: string; directory: boolean } | null>(null);
  let revealedActive = '';

  $effect(() => { if (!folderPath && wb.workspace.env) folderPath = wb.workspace.env.workingDir; });
  $effect(() => {
    if (wb.creating) void tick().then(() => { createInput?.focus(); const value = createInput?.value ?? ''; createInput?.setSelectionRange(value.lastIndexOf('/') + 1, value.length); });
  });
  $effect(() => {
    if (renaming) void tick().then(() => { renameInput?.focus(); renameInput?.select(); });
  });
  $effect(() => {
    if (context) void tick().then(() => contextMenu?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus());
  });
  $effect(() => {
    const path = wb.editors.activeTab?.path ?? '';
    if (path && path !== revealedActive) { revealedActive = path; wb.workspace.reveal(path); }
  });

  onMount(() => {
    const close = (event: Event) => { if (context && !contextMenu?.contains(event.target as Node)) context = null; };
    const escape = (event: KeyboardEvent) => { if (event.key === 'Escape' && context) { event.preventDefault(); context = null; } };
    // Reconcile after the application regains focus as a fallback for native
    // watcher queue overflows while the machine or app was suspended.
    const refreshOnFocus = () => { if (wb.workspace.project) void wb.workspace.refresh(); };
    window.addEventListener('pointerdown', close);
    window.addEventListener('keydown', escape);
    window.addEventListener('blur', close);
    window.addEventListener('focus', refreshOnFocus);
    window.addEventListener('resize', close);
    return () => { window.removeEventListener('pointerdown', close); window.removeEventListener('keydown', escape); window.removeEventListener('blur', close); window.removeEventListener('focus', refreshOnFocus); window.removeEventListener('resize', close); };
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
    else if (event.key === 'F2') { event.preventDefault(); event.stopPropagation(); startRename(path); }
    else if (event.key === 'ContextMenu' || (event.shiftKey && event.key === 'F10')) {
      event.preventDefault();
      const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
      showContext(path, directory, rect.left + 24, rect.top + 18);
    }
  }

  function collapse(row: HTMLElement) { const path = row.dataset.path!; if (row.dataset.dir && !wb.workspace.collapsed.has(path)) wb.workspace.toggleFolder(path); }
  function expand(row: HTMLElement) { const path = row.dataset.path!; if (row.dataset.dir && wb.workspace.collapsed.has(path)) wb.workspace.toggleFolder(path); }

  function recentName(path: string) { return path.replaceAll('\\', '/').split('/').filter(Boolean).pop() ?? path; }

  function startRename(path: string): void {
    wb.workspace.selected = path;
    renaming = { path, value: baseName(path) };
    context = null;
  }

  async function commitRename(): Promise<void> {
    const request = renaming;
    if (!request) return;
    const name = request.value.trim();
    if (!name || name.includes('/') || name.includes('\\')) { renaming = null; wb.ui.status = 'A name cannot be empty or contain a slash'; return; }
    renaming = null;
    await wb.movePath(request.path, parentOf(request.path) ? `${parentOf(request.path)}/${name}` : name);
  }

  function showContext(path: string, directory: boolean, x: number, y: number): void {
    if (path) wb.workspace.selected = path;
    context = { x: Math.min(x, window.innerWidth - 230), y: Math.min(y, window.innerHeight - 330), path, directory };
  }

  function startCreate(kind: 'file' | 'folder', directory: string): void {
    wb.workspace.selected = directory;
    context = null;
    wb.beginCreate(kind, directory);
  }

  function openContextFile(): void {
    const path = context?.path ?? '';
    context = null;
    if (path) void wb.openFile(path);
  }

  async function copyPath(path: string, absolute: boolean): Promise<void> {
    const root = wb.workspace.root.replace(/[\\/]+$/, '');
    const separator = root.includes('\\') ? '\\' : '/';
    const value = absolute ? `${root}${separator}${path.replaceAll('/', separator)}` : path;
    await native().clipboard.setText(value);
    wb.ui.status = `Copied ${absolute ? 'path' : 'relative path'}`;
    context = null;
  }

  async function download(path: string): Promise<void> {
    context = null;
    const destination = await native().dialogs.save({ title: `Download ${baseName(path)}`, filename: baseName(path) });
    if (!destination) return;
    if (await wb.workspace.download(path, destination)) wb.ui.status = `Downloaded ${baseName(path)}`;
  }

  function contextKey(event: KeyboardEvent): void {
    if (!contextMenu) return;
    const items = Array.from(contextMenu.querySelectorAll<HTMLButtonElement>('[role="menuitem"]:not(:disabled)'));
    const index = items.indexOf(document.activeElement as HTMLButtonElement);
    let next = -1;
    if (event.key === 'ArrowDown') next = (index + 1) % items.length;
    else if (event.key === 'ArrowUp') next = (index - 1 + items.length) % items.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = items.length - 1;
    else return;
    event.preventDefault();
    items[next]?.focus();
  }
</script>

{#if wb.workspace.project}
  <div class="section-header">
    <strong>{wb.workspace.project.config.name}</strong>
    <span class="section-actions">
      <button class="icon-button" title="New file" aria-label="New file" onclick={() => wb.beginCreate('file')}><i aria-hidden="true" class="codicon codicon-new-file"></i></button>
      <button class="icon-button" title="New folder" aria-label="New folder" onclick={() => wb.beginCreate('folder')}><i aria-hidden="true" class="codicon codicon-new-folder"></i></button>
      <button class="icon-button" title="Refresh explorer" aria-label="Refresh explorer" onclick={() => wb.workspace.refresh()}><i aria-hidden="true" class="codicon codicon-refresh"></i></button>
      <button class="icon-button" title="Collapse folders" aria-label="Collapse folders" onclick={() => wb.workspace.collapseAll()}><i aria-hidden="true" class="codicon codicon-collapse-all"></i></button>
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
    ondragover={event => event.preventDefault()} ondrop={event => drop(event, '')}
    oncontextmenu={event => { if (!(event.target as HTMLElement).closest('[data-row]')) { event.preventDefault(); showContext('', true, event.clientX, event.clientY); } }}>
    {#each tree as entry (entry.path)}
      <div role="treeitem" data-row data-path={entry.path} data-dir={entry.directory ? '1' : undefined} tabindex="-1" aria-level={entry.depth + 1}
        aria-selected={wb.workspace.selected === entry.path} aria-expanded={entry.directory ? !wb.workspace.collapsed.has(entry.path) : undefined}
        class="tree-row" class:active={wb.editors.active === entry.path} style={`padding-left:${8 + entry.depth * 14}px`} title={entry.path}
        id={entry.directory ? `project-drop-${encodeURIComponent(entry.path)}` : undefined} data-file-drop-target={entry.directory ? '' : undefined} draggable="true"
        ondragstart={event => startDrag(event, entry.path)} ondragover={event => event.preventDefault()} ondrop={event => drop(event, entry.directory ? entry.path : parentOf(entry.path))}
        onclick={() => { wb.workspace.selected = entry.path; if (entry.directory) wb.workspace.toggleFolder(entry.path); else void wb.openFile(entry.path); }}
        oncontextmenu={event => { event.preventDefault(); event.stopPropagation(); showContext(entry.path, entry.directory, event.clientX, event.clientY); }}
        onkeydown={event => rowKey(event, entry.path, entry.directory)}>
        {#if entry.directory}
          <i aria-hidden="true" class={`codicon codicon-chevron-${wb.workspace.collapsed.has(entry.path) ? 'right' : 'down'}`}></i>
          <i aria-hidden="true" class={`codicon codicon-folder${wb.workspace.collapsed.has(entry.path) ? '' : '-opened'}`}></i>
        {:else}
          <i aria-hidden="true" class="codicon tree-spacer"></i><i aria-hidden="true" class={`codicon codicon-${fileIcon(entry.path)}`}></i>
        {/if}
        {#if renaming?.path === entry.path}
          <form class="inline-rename" onsubmit={event => { event.preventDefault(); void commitRename(); }}>
            <input bind:this={renameInput} bind:value={renaming.value} aria-label={`Rename ${entry.name}`} onclick={event => event.stopPropagation()} onkeydown={event => { event.stopPropagation(); if (event.key === 'Escape') { event.preventDefault(); renaming = null; } }} onblur={() => { if (renaming?.path === entry.path) void commitRename(); }} />
          </form>
        {:else}<span class="name">{entry.name}</span>{/if}
        {#if wb.editors.tab(entry.path)?.dirty}<b class="dirty-dot" aria-label="unsaved changes">●</b>{/if}
      </div>
    {/each}
    {#if tree.length === 0}<div class="empty-state">This folder is empty. Create a file to begin.</div>{/if}
  </div>
  {#if context}
    <div class="explorer-context" role="menu" aria-label="Explorer context menu" tabindex="-1" bind:this={contextMenu} style={`left:${context.x}px;top:${context.y}px`} onkeydown={contextKey}>
      {#if context.path && !context.directory}
        <button role="menuitem" onclick={openContextFile}><span><i aria-hidden="true" class="codicon codicon-go-to-file"></i> Open</span><kbd>Enter</kbd></button>
        {#if /\.png$/i.test(context.path)}<button role="menuitem" onclick={() => { const path = context?.path ?? ''; context = null; void wb.sprites.open(path); }}><span><i aria-hidden="true" class="codicon codicon-paintcan"></i> Open in Sprite Studio</span></button>{/if}
        <button role="menuitem" onclick={() => download(context?.path ?? '')}><span><i aria-hidden="true" class="codicon codicon-cloud-download"></i> Download…</span></button>
        <div class="menu-separator" role="separator"></div>
      {/if}
      {#if context.directory}
        <button role="menuitem" onclick={() => startCreate('file', context?.path ?? '')}><span><i aria-hidden="true" class="codicon codicon-new-file"></i> New File…</span></button>
        <button role="menuitem" onclick={() => startCreate('folder', context?.path ?? '')}><span><i aria-hidden="true" class="codicon codicon-new-folder"></i> New Folder…</span></button>
        <div class="menu-separator" role="separator"></div>
      {/if}
      {#if context.path}
        <button role="menuitem" onclick={() => startRename(context?.path ?? '')}><span><i aria-hidden="true" class="codicon codicon-edit"></i> Rename</span><kbd>F2</kbd></button>
        <button role="menuitem" onclick={() => { context = null; void wb.deleteSelected(); }}><span><i aria-hidden="true" class="codicon codicon-trash"></i> Delete</span><kbd>Del</kbd></button>
        <div class="menu-separator" role="separator"></div>
        <button role="menuitem" onclick={() => copyPath(context?.path ?? '', false)}><span><i aria-hidden="true" class="codicon codicon-copy"></i> Copy Relative Path</span></button>
        <button role="menuitem" onclick={() => copyPath(context?.path ?? '', true)}><span><i aria-hidden="true" class="codicon codicon-clippy"></i> Copy Path</span></button>
        <div class="menu-separator" role="separator"></div>
      {/if}
      <button role="menuitem" onclick={() => { context = null; void wb.workspace.refresh(); }}><span><i aria-hidden="true" class="codicon codicon-refresh"></i> Refresh Explorer</span></button>
      <button role="menuitem" onclick={() => { context = null; wb.workspace.collapseAll(); }}><span><i aria-hidden="true" class="codicon codicon-collapse-all"></i> Collapse Folders</span></button>
    </div>
  {/if}
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
