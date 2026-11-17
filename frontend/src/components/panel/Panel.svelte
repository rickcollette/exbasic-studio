<script lang="ts">
  import { wb } from '../../lib/workbench.svelte';
  import type { PanelTab } from '../../lib/stores/ui.svelte';
  import Problems from './Problems.svelte';
  import TerminalPanel from './TerminalPanel.svelte';
  import DebugPanel from './DebugPanel.svelte';
  import KeyboardPanel from './KeyboardPanel.svelte';

  const tabs: { id: PanelTab; label: string }[] = [
    { id: 'problems', label: 'Problems' }, { id: 'output', label: 'Output' }, { id: 'debug', label: 'Debug' }, { id: 'terminal', label: 'Terminal' }, { id: 'keyboard', label: 'Keyboard' },
  ];
  let outputView = $state<HTMLElement>();
  let tabList = $state<HTMLElement>();

  // Keep the Output channel scrolled to the newest text.
  $effect(() => { void wb.ui.output; if (outputView) outputView.scrollTop = outputView.scrollHeight; });

  function tabKey(event: KeyboardEvent, id: PanelTab) {
    const index = tabs.findIndex(tab => tab.id === id);
    let next = -1;
    if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
    else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
    else return;
    event.preventDefault();
    wb.ui.showPanel(tabs[next].id);
    queueMicrotask(() => tabList?.querySelector<HTMLElement>('[aria-selected="true"]')?.focus());
  }
</script>

<div class="panel-tabs">
  <div class="tab-list" role="tablist" aria-label="Panel" bind:this={tabList}>
    {#each tabs as tab}
      <button role="tab" id={`panel-tab-${tab.id}`} aria-selected={wb.ui.panelTab === tab.id} aria-controls="panel-content" tabindex={wb.ui.panelTab === tab.id ? 0 : -1} class:active={wb.ui.panelTab === tab.id}
        onclick={() => wb.ui.showPanel(tab.id)} onkeydown={event => tabKey(event, tab.id)}>
        {tab.label}{#if tab.id === 'problems' && wb.problems.all.length}<span class="count">{wb.problems.all.length}</span>{/if}
      </button>
    {/each}
  </div>
  <div class="panel-actions">
    {#if wb.ui.panelTab === 'output'}<button class="icon-button" title="Clear output" aria-label="Clear output" onclick={() => wb.ui.clearOutput()}><i aria-hidden="true" class="codicon codicon-clear-all"></i></button>{/if}
    <button class="icon-button" title={wb.ui.panelMaximized ? 'Restore panel' : 'Maximize panel'} aria-label={wb.ui.panelMaximized ? 'Restore panel' : 'Maximize panel'} onclick={() => (wb.ui.panelMaximized = !wb.ui.panelMaximized)}><i aria-hidden="true" class={`codicon codicon-chevron-${wb.ui.panelMaximized ? 'down' : 'up'}`}></i></button>
    <button class="icon-button" title="Close panel" aria-label="Close panel" onclick={() => wb.execute('view.panel')}><i aria-hidden="true" class="codicon codicon-close"></i></button>
  </div>
</div>
<div class="panel-content" id="panel-content" role="tabpanel" aria-labelledby={`panel-tab-${wb.ui.panelTab}`}>
  {#if wb.ui.panelTab === 'problems'}<Problems />
  {:else if wb.ui.panelTab === 'output'}<!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre class="output-view" bind:this={outputView} tabindex="0" aria-label="Output" aria-live="off">{wb.ui.output}</pre>
  {:else if wb.ui.panelTab === 'debug'}<DebugPanel />
  {:else if wb.ui.panelTab === 'keyboard'}<KeyboardPanel />{/if}
  <!-- Terminals stay mounted so their scrollback survives tab switches. -->
  <div class="terminal-host" class:hidden={wb.ui.panelTab !== 'terminal'}><TerminalPanel /></div>
</div>