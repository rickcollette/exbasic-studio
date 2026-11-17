<script lang="ts">
  // The workbench shell: title bar, activity bar, side bar, editors, panel
  // and status bar, plus the global keyboard handler and resize handles.
  import { wb } from '../lib/workbench.svelte';
  import TitleBar from './TitleBar.svelte';
  import ActivityBar from './ActivityBar.svelte';
  import Sidebar from './Sidebar.svelte';
  import EditorGroup from './editor/EditorGroup.svelte';
  import Panel from './panel/Panel.svelte';
  import StatusBar from './StatusBar.svelte';
  import Palette from './Palette.svelte';
  import Dialog from './Dialog.svelte';

  let dragging = $state<'sidebar' | 'panel' | null>(null);

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape' && wb.palette.open) { wb.closePalette(); event.preventDefault(); return; }
    wb.handleKey(event);
  }

  function startResize(kind: 'sidebar' | 'panel', event: PointerEvent) {
    dragging = kind;
    (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
    event.preventDefault();
  }

  function resize(event: PointerEvent) {
    if (dragging === 'sidebar') wb.ui.setPrimaryWidth(event.clientX - 48);
    else if (dragging === 'panel') wb.ui.setPanelHeight(window.innerHeight - event.clientY - 22);
  }

  /** Keyboard resizing for the separators (arrow keys, 16px steps). */
  function resizeKey(kind: 'sidebar' | 'panel', event: KeyboardEvent) {
    const step = event.shiftKey ? 48 : 16;
    if (kind === 'sidebar' && (event.key === 'ArrowLeft' || event.key === 'ArrowRight')) {
      wb.ui.setPrimaryWidth(wb.ui.primaryWidth + (event.key === 'ArrowRight' ? step : -step));
      event.preventDefault();
    }
    if (kind === 'panel' && (event.key === 'ArrowUp' || event.key === 'ArrowDown')) {
      wb.ui.setPanelHeight(wb.ui.panelHeight + (event.key === 'ArrowUp' ? step : -step));
      event.preventDefault();
    }
  }
</script>

<svelte:window onkeydown={keydown} onblur={() => wb.editors.focusLost()} />

<div class={`workbench syntax-${wb.prefs.values.syntax}`} class:high-contrast={wb.prefs.theme === 'high-contrast'} class:theme-light={wb.prefs.theme === 'light'} class:resizing={dragging !== null} style={`--editor-font-size:${wb.prefs.values.fontSize}px`}>
  <TitleBar />
  <div class="work-area">
    <ActivityBar />
    {#if wb.ui.primaryOpen}
      <aside class="primary-sidebar" style={`width:${wb.ui.primaryWidth}px`} aria-label="Side bar">
        <Sidebar />
      </aside>
      <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
      <div class="sash vertical" role="separator" aria-orientation="vertical" aria-label="Resize side bar" aria-valuenow={Math.round(wb.ui.primaryWidth)} aria-valuemin={160} aria-valuemax={900} tabindex="0"
        onpointerdown={event => startResize('sidebar', event)} onpointermove={resize} onpointerup={() => (dragging = null)} onkeydown={event => resizeKey('sidebar', event)}></div>
    {/if}
    <main class="editor-area">
      <div class="editor-group" class:hidden={wb.ui.panelOpen && wb.ui.panelMaximized}>
        <EditorGroup />
      </div>
      {#if wb.ui.panelOpen}
        {#if !wb.ui.panelMaximized}
          <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
          <div class="sash horizontal" role="separator" aria-orientation="horizontal" aria-label="Resize panel" aria-valuenow={Math.round(wb.ui.panelHeight)} aria-valuemin={80} aria-valuemax={1200} tabindex="0"
            onpointerdown={event => startResize('panel', event)} onpointermove={resize} onpointerup={() => (dragging = null)} onkeydown={event => resizeKey('panel', event)}></div>
        {/if}
        <section class="panel" class:maximized={wb.ui.panelMaximized} style={wb.ui.panelMaximized ? '' : `height:${wb.ui.panelHeight}px`} aria-label="Panel">
          <Panel />
        </section>
      {/if}
    </main>
  </div>
  <StatusBar />
  {#if wb.palette.open}<Palette />{/if}
  {#if wb.ui.dialog}<Dialog />{/if}
</div>
