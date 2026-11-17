<script lang="ts">
  // Interactive terminal sessions rendered with xterm.js. Each session is a
  // real shell on a pseudo-terminal in the backend.
  import { onMount } from 'svelte';
  import { Terminal } from '@xterm/xterm';
  import { FitAddon } from '@xterm/addon-fit';
  import { wb } from '../../lib/workbench.svelte';

  interface View { terminal: Terminal; fit: FitAddon; element: HTMLDivElement; detach: () => void }
  const views = new Map<string, View>();
  let container = $state<HTMLDivElement>();
  let observer: ResizeObserver | undefined;

  function theme() {
    const style = getComputedStyle(document.documentElement);
    const value = (name: string, fallback: string) => style.getPropertyValue(name).trim() || fallback;
    return { background: value('--panel-bg', '#181818'), foreground: value('--fg', '#cccccc'), cursor: value('--editor-cursor', '#aeafad'), selectionBackground: value('--editor-selection', '#264f78') };
  }

  function ensure(id: string): View {
    let view = views.get(id);
    if (view) return view;
    const element = document.createElement('div');
    element.className = 'terminal-view';
    container!.append(element);
    const terminal = new Terminal({ fontFamily: 'Consolas, "Cascadia Code", "DejaVu Sans Mono", monospace', fontSize: 13, cursorBlink: true, scrollback: 5000, theme: theme(), screenReaderMode: true });
    const fit = new FitAddon();
    terminal.loadAddon(fit);
    terminal.open(element);
    // Ctrl+C with a selection copies; otherwise it interrupts the shell.
    terminal.attachCustomKeyEventHandler(event => {
      if (event.type === 'keydown' && event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 'c' && terminal.hasSelection()) {
        void navigator.clipboard?.writeText(terminal.getSelection());
        terminal.clearSelection();
        return false;
      }
      // Let the workbench have its panel and palette shortcuts.
      if (event.ctrlKey && (event.key === '`' || event.key.toLowerCase() === 'j' || (event.shiftKey && event.key.toLowerCase() === 'p'))) return false;
      return true;
    });
    terminal.onData(data => wb.terminal.input(id, data));
    terminal.onResize(size => wb.terminal.resize(id, size.cols, size.rows));
    const detach = wb.terminal.attach(id, data => terminal.write(data));
    view = { terminal, fit, element, detach };
    views.set(id, view);
    return view;
  }

  function layout() {
    const active = views.get(wb.terminal.activeId);
    if (!active || !container || container.offsetParent === null) return;
    try { active.fit.fit(); } catch { /* not laid out yet */ }
  }

  // Create views for new sessions, show the active one, drop closed ones.
  $effect(() => {
    const ids = new Set(wb.terminal.sessions.map(session => session.id));
    if (!container) return;
    for (const id of ids) ensure(id);
    for (const [id, view] of views) {
      if (!ids.has(id)) { view.detach(); view.terminal.dispose(); view.element.remove(); views.delete(id); continue; }
      view.element.style.display = id === wb.terminal.activeId ? '' : 'none';
    }
    queueMicrotask(layout);
  });

  // Refit when the panel becomes visible or changes size.
  $effect(() => { void wb.ui.panelTab; void wb.ui.panelHeight; void wb.ui.panelMaximized; void wb.ui.panelOpen; queueMicrotask(layout); });

  onMount(() => {
    observer = new ResizeObserver(layout);
    observer.observe(container!);
    wb.focusTerminal = () => queueMicrotask(() => { layout(); views.get(wb.terminal.activeId)?.terminal.focus(); });
    return () => {
      observer?.disconnect();
      for (const view of views.values()) { view.detach(); view.terminal.dispose(); }
      views.clear();
      wb.focusTerminal = () => {};
    };
  });
</script>

<div class="terminal-panel">
  <div class="terminal-toolbar" role="toolbar" aria-label="Terminals">
    <div class="tab-list" role="tablist" aria-label="Terminal sessions">
      {#each wb.terminal.sessions as session (session.id)}
        <button role="tab" aria-selected={wb.terminal.activeId === session.id} class:active={wb.terminal.activeId === session.id} class:exited={session.exited}
          onclick={() => { wb.terminal.activeId = session.id; wb.focusTerminal(); }}>
          <i aria-hidden="true" class="codicon codicon-terminal"></i> {session.title}{#if session.exited} (exited {session.exitCode}){/if}
        </button>
      {/each}
    </div>
    <span class="tab-spacer"></span>
    <button class="icon-button" title="New terminal (Ctrl+Shift+`)" aria-label="New terminal" disabled={!wb.workspace.project} onclick={() => wb.execute('terminal.new')}><i aria-hidden="true" class="codicon codicon-add"></i></button>
    <button class="icon-button" title="Kill terminal" aria-label="Kill active terminal" disabled={!wb.terminal.activeId} onclick={() => wb.execute('terminal.kill')}><i aria-hidden="true" class="codicon codicon-trash"></i></button>
  </div>
  <div class="terminal-stack" bind:this={container}></div>
  {#if wb.terminal.sessions.length === 0}
    <div class="empty-state terminal-empty">
      {#if !wb.workspace.project}Open a folder to use the terminal.
      {:else if wb.workspace.restricted}The terminal is disabled in restricted mode. <button class="inline" onclick={() => wb.execute('trust')}>Trust Folder…</button>
      {:else}<button class="primary" onclick={() => wb.execute('terminal.new')}><i aria-hidden="true" class="codicon codicon-terminal"></i> New Terminal</button>{/if}
    </div>
  {/if}
</div>