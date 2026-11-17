<script lang="ts">
  import { wb } from '../lib/workbench.svelte';
  import { EMULATOR_TAB } from '../lib/stores/editors.svelte';

  const lineEnding = $derived.by(() => {
    const tab = wb.editors.activeTab;
    void tab?.version;
    return tab && wb.editors.text(tab.path).includes('\r\n') ? 'CRLF' : 'LF';
  });
  const busy = $derived(wb.ui.busy.length > 0 || wb.operations.running.length > 0);
</script>

<footer class="statusbar">
  <div class="status-left">
    {#if wb.workspace.restricted}
      <button class="restricted" onclick={() => wb.execute('trust')} title="Building, running, the terminal and Git hooks are disabled until you trust this folder"><i aria-hidden="true" class="codicon codicon-shield"></i> Restricted Mode</button>
    {/if}
    <button onclick={() => wb.execute('view.source')} title="Source control"><i aria-hidden="true" class="codicon codicon-source-control"></i> {wb.git.status?.branch || '--'}{#if wb.git.status?.ahead || wb.git.status?.behind}<span class="sync"> {wb.git.status.behind}↓ {wb.git.status.ahead}↑</span>{/if}</button>
    <button onclick={() => wb.ui.showPanel('problems')} title="Show problems">
      <i class="codicon codicon-error" aria-hidden="true"></i> {wb.problems.errorCount}<span class="sr-only"> errors,</span> <i class="codicon codicon-warning" aria-hidden="true"></i> {wb.problems.warningCount}<span class="sr-only"> warnings</span>
    </button>
    {#if wb.debug.active}<span class="debugging"><i aria-hidden="true" class="codicon codicon-debug-alt"></i> {wb.debug.stopped ? 'Paused' : 'Debugging'}</span>{/if}
  </div>
  <div class="status-message" role="status" aria-live="polite">
    {#if busy}<i class="codicon codicon-sync codicon-modifier-spin" aria-hidden="true"></i>{/if} {wb.ui.status}
    {#if wb.operations.isRunning('build')}<button class="inline" onclick={() => wb.execute('build.cancel')}>Cancel</button>{/if}
    {#if wb.git.networkBusy}<button class="inline" onclick={() => wb.git.cancelNetwork()}>Cancel</button>{/if}
  </div>
  <div class="status-right">
    {#if wb.editors.activeTab}
      <span>Ln {wb.editors.cursor.line}, Col {wb.editors.cursor.column}</span><span>Spaces: 2</span><span>UTF-8</span><span>{lineEnding}</span>
    {:else if wb.editors.active === EMULATOR_TAB}
      <span>Frame {wb.emulator.frame}</span>
    {:else if wb.sprites.active}
      {@const sheet = wb.sprites.active.sheet}
      <span>Frame {wb.sprites.active.frame + 1} of {sheet.frames.length}</span><span>{sheet.width}x{sheet.height}, {sheet.planes} plane{sheet.planes === 1 ? '' : 's'}</span>
    {/if}
    <span>{wb.workspace.project?.config.target ?? 'Atari 8-bit'}</span>
    <span title={`exBASIC Studio ${wb.workspace.env?.version ?? ''}`}>{wb.workspace.env?.version ?? ''}</span>
  </div>
</footer>