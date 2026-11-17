<script lang="ts">
  // Editor tabs, breadcrumbs and the content area: the text editor, the
  // Sprite Studio, the emulator screen, the merge conflict view or the
  // welcome page.
  import { wb } from '../../lib/workbench.svelte';
  import { EMULATOR_TAB } from '../../lib/stores/editors.svelte';
  import { fileIcon, baseName } from '../../lib/stores/workspace.svelte';
  import EditorPane from './EditorPane.svelte';
  import EmulatorView from './EmulatorView.svelte';
  import ConflictView from './ConflictView.svelte';
  import SpriteStudio from './SpriteStudio.svelte';

  const editors = wb.editors;
  const showEmulator = $derived(editors.active === EMULATOR_TAB);
  const tab = $derived(editors.activeTab);
  const sprite = $derived(wb.sprites.doc(editors.active));
  let tabList = $state<HTMLElement>();

  $effect(() => { wb.emulator.setVisible(showEmulator); });

  /** All tab ids in display order, for arrow-key navigation. */
  const order = $derived([...editors.tabs.map(item => item.path), ...wb.sprites.docs.map(doc => doc.id), ...(editors.emulatorOpen ? [EMULATOR_TAB] : [])]);

  function activate(id: string) { if (id === EMULATOR_TAB) editors.openEmulator(); else editors.activate(id); }

  async function closeTab(id: string) {
    const doc = wb.sprites.doc(id);
    if (id === EMULATOR_TAB) { await wb.emulator.stop(); editors.closeEmulator(); } else if (doc) await wb.sprites.close(doc); else await editors.close(id);
  }

  function tabKey(event: KeyboardEvent, id: string) {
    const index = order.indexOf(id);
    let next = -1;
    if (event.key === 'ArrowRight') next = (index + 1) % order.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + order.length) % order.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = order.length - 1;
    else if (event.key === 'Delete') { event.preventDefault(); void closeTab(id); return; }
    else return;
    event.preventDefault();
    activate(order[next]);
    queueMicrotask(() => tabList?.querySelector<HTMLElement>('[role="tab"][aria-selected="true"]')?.focus());
  }

  function middleClick(event: MouseEvent, id: string) { if (event.button === 1) { event.preventDefault(); void closeTab(id); } }
</script>

<div class="editor-tabs">
  <!-- A tab list must contain tabs, so the role is present only with open editors. -->
  <div class="tab-list" role={order.length ? 'tablist' : undefined} aria-label={order.length ? 'Open editors' : undefined} bind:this={tabList}>
    {#each editors.tabs as item (item.path)}
      <div class="editor-tab" role="presentation" class:active={editors.active === item.path} class:dirty={item.dirty}>
        <button role="tab" id={`tab-${item.path}`} aria-selected={editors.active === item.path} aria-controls="editor-content" tabindex={editors.active === item.path ? 0 : -1} title={item.path}
          onclick={() => activate(item.path)} onkeydown={event => tabKey(event, item.path)} onauxclick={event => middleClick(event, item.path)}>
          <i aria-hidden="true" class={`codicon codicon-${fileIcon(item.path)}`}></i><span>{baseName(item.path)}</span>
          {#if item.external}<i aria-hidden="true" class="codicon codicon-warning changed" title="Changed on disk"></i>{/if}
          {#if item.dirty}<b class="dirty-dot" aria-label="unsaved changes">●</b>{/if}
        </button>
        <button class="close-tab" tabindex="-1" aria-hidden="true" title={`Close ${baseName(item.path)}`} aria-label={`Close ${baseName(item.path)}`} onclick={() => closeTab(item.path)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
      </div>
    {/each}
    {#each wb.sprites.docs as doc (doc.id)}
      <div class="editor-tab" role="presentation" class:active={editors.active === doc.id} class:dirty={doc.dirty}>
        <button role="tab" id={`tab-${doc.id}`} aria-selected={editors.active === doc.id} aria-controls="editor-content" tabindex={editors.active === doc.id ? 0 : -1} title={`${doc.path} (Sprite Studio)`}
          onclick={() => activate(doc.id)} onkeydown={event => tabKey(event, doc.id)} onauxclick={event => middleClick(event, doc.id)}>
          <i aria-hidden="true" class="codicon codicon-paintcan"></i><span>{baseName(doc.path)}</span>
          {#if doc.external}<i aria-hidden="true" class="codicon codicon-warning changed" title="Changed on disk"></i>{/if}
          {#if doc.dirty}<b class="dirty-dot" aria-label="unsaved changes">●</b>{/if}
        </button>
        <button class="close-tab" tabindex="-1" aria-hidden="true" title={`Close ${baseName(doc.path)}`} aria-label={`Close ${baseName(doc.path)}`} onclick={() => closeTab(doc.id)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
      </div>
    {/each}
    {#if editors.emulatorOpen}
      <div class="editor-tab" role="presentation" class:active={showEmulator}>
        <button role="tab" id="tab-emulator" aria-selected={showEmulator} aria-controls="editor-content" tabindex={showEmulator ? 0 : -1} onclick={() => activate(EMULATOR_TAB)} onkeydown={event => tabKey(event, EMULATOR_TAB)}>
          <i aria-hidden="true" class="codicon codicon-device-desktop"></i><span>Atari800</span>{#if wb.emulator.running}<i role="img" class="codicon codicon-circle-filled running" aria-label="running"></i>{/if}
        </button>
        <button class="close-tab" tabindex="-1" aria-hidden="true" title="Stop and close Atari800" aria-label="Stop and close Atari800" onclick={() => closeTab(EMULATOR_TAB)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
      </div>
    {/if}
  </div>
  <span class="tab-spacer"></span>
  {#if !editors.emulatorOpen}<button class="icon-button" title="Open Atari800 screen" aria-label="Open Atari800 screen" onclick={() => wb.execute('emulator.open')}><i aria-hidden="true" class="codicon codicon-device-desktop"></i></button>{/if}
  <button class="icon-button" title="Claude Code: open it in the terminal for this project" aria-label="Open Claude Code" disabled={!wb.commandEnabled('claude.open')} onclick={() => wb.execute('claude.open')}><i aria-hidden="true" class="codicon codicon-sparkle"></i></button>
  <button class="icon-button" title="Run without debugging (Ctrl+F5)" aria-label="Run without debugging" disabled={!wb.commandEnabled('run')} onclick={() => wb.execute('run')}><i aria-hidden="true" class="codicon codicon-play"></i></button>
  <button class="icon-button" title="Toggle editor split (Ctrl+\)" aria-label="Toggle editor split" aria-pressed={editors.splitOpen} disabled={!wb.commandEnabled('editor.split')} onclick={() => wb.execute('editor.split')}><i aria-hidden="true" class="codicon codicon-split-horizontal"></i></button>
</div>

<div class="breadcrumbs" aria-label="Location">
  {#if showEmulator}<i aria-hidden="true" class="codicon codicon-device-desktop"></i><span>Atari800</span>
  {:else if sprite}<i aria-hidden="true" class="codicon codicon-paintcan"></i>{#each sprite.path.split('/') as part, index}{#if index > 0}<i aria-hidden="true" class="codicon codicon-chevron-right"></i>{/if}<span>{part}</span>{/each}<span>(Sprite Studio)</span>
  {:else if tab}{#each tab.path.split('/') as part, index}{#if index > 0}<i aria-hidden="true" class="codicon codicon-chevron-right"></i>{/if}<span>{part}</span>{/each}
  {:else}<span>exBASIC Studio</span>{/if}
</div>

{#if tab?.external}
  <div class="banner" role="alert">
    <i aria-hidden="true" class="codicon codicon-warning"></i>
    {#if tab.external === 'deleted'}
      <span>{tab.path} was deleted or moved outside the editor. Your text is kept here; save to write it back.</span>
      <button onclick={() => editors.save(tab.path)}>Save</button><button onclick={() => editors.resolveExternal(tab.path, 'keep')}>Dismiss</button>
    {:else}
      <span>{tab.path} changed on disk and you have unsaved changes.</span>
      <button onclick={() => editors.resolveExternal(tab.path, 'reload')}>Reload From Disk</button><button onclick={() => editors.resolveExternal(tab.path, 'keep')}>Keep My Changes</button>
    {/if}
  </div>
{/if}

<div class="editor-content" id="editor-content" role="tabpanel" aria-labelledby={showEmulator ? 'tab-emulator' : sprite ? `tab-${sprite.id}` : tab ? `tab-${tab.path}` : undefined}>
  <!-- The text editor stays mounted so tab switches keep their state. -->
  <div class="pane" class:hidden={!tab || showEmulator || !!wb.git.conflict}><EditorPane /></div>
  {#if wb.git.conflict && !showEmulator}
    <div class="pane"><ConflictView /></div>
  {:else if showEmulator}
    <div class="pane"><EmulatorView /></div>
  {:else if sprite}
    {#key sprite.id}<div class="pane"><SpriteStudio doc={sprite} /></div>{/key}
  {:else if !tab}
    <div class="pane welcome">
      <img src="/exbasic-logo-icon.png" alt="" draggable="false" />
      <h1>exBASIC Studio</h1>
      {#if !wb.ready}
        <p role="status"><i aria-hidden="true" class="codicon codicon-loading codicon-modifier-spin"></i> Starting…</p>
      {:else if wb.workspace.project}
        <p>Choose a file in the Explorer, or press <kbd>Ctrl+P</kbd> to go to a file.</p>
      {:else}
        <p>Open a folder or create a project to begin.</p>
        <div class="welcome-actions">
          <button class="primary" onclick={() => wb.chooseFolder()}><i aria-hidden="true" class="codicon codicon-folder-opened"></i> Open Folder…</button>
          <button onclick={() => wb.ui.showActivity('manual')}><i aria-hidden="true" class="codicon codicon-book"></i> exBASIC Manual</button>
        </div>
      {/if}
      {#if wb.workspace.env && !wb.workspace.env.toolsAvailable}<p class="warning" role="alert"><i aria-hidden="true" class="codicon codicon-warning"></i> {wb.workspace.env.toolsNote}</p>{/if}
      <dl class="shortcuts">
        <dt><kbd>Ctrl+Shift+P</kbd></dt><dd>Command palette</dd>
        <dt><kbd>F5</kbd></dt><dd>Start debugging</dd>
        <dt><kbd>Ctrl+F5</kbd></dt><dd>Run without debugging</dd>
        <dt><kbd>F6</kbd></dt><dd>Build</dd>
      </dl>
    </div>
  {/if}
</div>
