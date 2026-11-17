<script lang="ts">
  // Frameless window title bar with the application menu. The menu bar
  // follows the ARIA menubar pattern: arrow keys move between and within
  // menus, Enter/Space activate, Escape closes and returns focus.
  import { tick } from 'svelte';
  import { wb, menus, commands, type CommandID } from '../lib/workbench.svelte';
  import { native } from '../lib/native';

  const names = Object.keys(menus);
  let open = $state('');
  let active = $state(0);
  let bar = $state<HTMLElement>();
  let popup = $state<HTMLElement>();

  const label = (id: CommandID) => commands.find(command => command.id === id)!;
  const itemLabel = (id: CommandID) => label(id).label.replace(/^[^:]+:\s*/, '');

  async function show(name: string, focusFirst = true) {
    open = name;
    active = 0;
    await tick();
    if (focusFirst) focusItem(0, 1);
  }

  function close(restore = true) {
    const name = open;
    open = '';
    if (restore && name) bar?.querySelector<HTMLElement>(`[data-menu="${name}"]`)?.focus();
  }

  /** Focuses the enabled item at or after index, searching in direction. */
  function focusItem(index: number, direction: 1 | -1) {
    const items = Array.from(popup?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? []);
    if (!items.length) return;
    for (let step = 0; step < items.length; step++) {
      const candidate = (index + step * direction + items.length * 2) % items.length;
      if (!items[candidate].disabled) { active = candidate; items[candidate].focus(); return; }
    }
  }

  function barKey(event: KeyboardEvent, name: string) {
    const index = names.indexOf(name);
    if (event.key === 'ArrowRight' || event.key === 'ArrowLeft') {
      const next = names[(index + (event.key === 'ArrowRight' ? 1 : -1) + names.length) % names.length];
      event.preventDefault();
      if (open) void show(next);
      else bar?.querySelector<HTMLElement>(`[data-menu="${next}"]`)?.focus();
    } else if (event.key === 'ArrowDown' || event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      void show(name);
    }
  }

  function menuKey(event: KeyboardEvent) {
    const index = names.indexOf(open);
    switch (event.key) {
      case 'ArrowDown': event.preventDefault(); focusItem(active + 1, 1); break;
      case 'ArrowUp': event.preventDefault(); focusItem(active - 1, -1); break;
      case 'Home': event.preventDefault(); focusItem(0, 1); break;
      case 'End': event.preventDefault(); focusItem(-1, -1); break;
      case 'ArrowRight': event.preventDefault(); void show(names[(index + 1) % names.length]); break;
      case 'ArrowLeft': event.preventDefault(); void show(names[(index - 1 + names.length) % names.length]); break;
      case 'Escape': event.preventDefault(); event.stopPropagation(); close(); break;
      case 'Tab': close(false); break;
    }
  }

  function run(id: CommandID) {
    close(false);
    void wb.execute(id);
  }

  function outside(event: MouseEvent) {
    if (open && !(event.target as Element | null)?.closest('.menubar')) close(false);
  }
</script>

<svelte:window onclick={outside} />

<header class="titlebar">
  <div class="title-drag"><img class="app-logo" src="/exbasic-logo-icon.png" alt="" draggable="false" /></div>
  <div class="menubar" role="menubar" aria-label="Application menu" bind:this={bar}>
    {#each names as name, index}
      <div class="menu-wrap">
        <button role="menuitem" aria-haspopup="menu" aria-expanded={open === name} data-menu={name} tabindex={index === 0 || open === name ? 0 : -1} class:open={open === name}
          onclick={() => (open === name ? close() : void show(name, false))} onkeydown={event => barKey(event, name)} onmouseenter={() => { if (open && open !== name) void show(name, false); }}>{name}</button>
        {#if open === name}
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="menu-popup" role="menu" aria-label={name} tabindex="-1" bind:this={popup} onkeydown={menuKey}>
            {#each menus[name] as id}
              <button role="menuitem" tabindex="-1" disabled={!wb.commandEnabled(id)} onclick={() => run(id)}>
                <span>{itemLabel(id)}</span><kbd>{label(id).shortcut ?? ''}</kbd>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </div>
  <button class="icon-button" title="Go back (Alt+Left)" aria-label="Go back" disabled={!wb.editors.historyState.back} onclick={() => wb.execute('nav.back')}><i aria-hidden="true" class="codicon codicon-arrow-left"></i></button>
  <button class="icon-button" title="Go forward (Alt+Right)" aria-label="Go forward" disabled={!wb.editors.historyState.forward} onclick={() => wb.execute('nav.forward')}><i aria-hidden="true" class="codicon codicon-arrow-right"></i></button>
  <button class="command-center" onclick={() => wb.showPalette('commands')} title="Open the command palette (Ctrl+Shift+P)">
    <i aria-hidden="true" class="codicon codicon-search"></i><span>{wb.workspace.project?.config.name ?? 'exBASIC Studio'}</span><span class="sr-only">, open the command palette</span>
  </button>
  <div class="title-actions">
    <button class="icon-button" title="Toggle side bar (Ctrl+B)" aria-label="Toggle side bar" aria-pressed={wb.ui.primaryOpen} onclick={() => wb.execute('view.sidebar')}><i aria-hidden="true" class="codicon codicon-layout-sidebar-left"></i></button>
    <button class="icon-button" title="Toggle panel (Ctrl+J)" aria-label="Toggle panel" aria-pressed={wb.ui.panelOpen} onclick={() => wb.execute('view.panel')}><i aria-hidden="true" class="codicon codicon-layout-panel"></i></button>
  </div>
  <div class="window-controls">
    <button aria-label="Minimize" onclick={() => native().window.minimise()}><i aria-hidden="true" class="codicon codicon-chrome-minimize"></i></button>
    <button aria-label="Maximize" onclick={() => native().window.toggleMaximise()}><i aria-hidden="true" class="codicon codicon-chrome-maximize"></i></button>
    <button class="close" aria-label="Close" onclick={() => native().window.close()}><i aria-hidden="true" class="codicon codicon-chrome-close"></i></button>
  </div>
</header>
