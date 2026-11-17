<script lang="ts">
  // Run and Debug: build/run/debug actions, breakpoints, watches and the
  // symbol browser. Watches and memory are read-only in this release.
  import { wb } from '../../lib/workbench.svelte';
  import { hex } from '../../lib/actions';

  const debug = wb.debug;
  let watchText = $state('');
  let symbolFilter = $state('');
  let symbolsOpen = $state(false);

  const symbols = $derived(debug.symbols.filter(symbol => !symbolFilter.trim() || symbol.name.toLowerCase().includes(symbolFilter.trim().toLowerCase())).slice(0, 200));

  function addWatch() { debug.addWatch(watchText); watchText = ''; }
</script>

<div class="run-actions">
  <button class="primary" disabled={!wb.commandEnabled('debug.start') || debug.starting} onclick={() => wb.execute('debug.start')}><i aria-hidden="true" class="codicon codicon-debug-start"></i> Start Debugging <kbd>F5</kbd></button>
  <button disabled={!wb.commandEnabled('run')} onclick={() => wb.execute('run')}><i aria-hidden="true" class="codicon codicon-play"></i> Run Without Debugging</button>
  <button disabled={!wb.commandEnabled('build')} onclick={() => wb.execute('build')}><i aria-hidden="true" class="codicon codicon-tools"></i> Build{wb.workspace.project?.config.atr.enabled ? ' + ATR' : ''} <kbd>F6</kbd></button>
  {#if wb.operations.isRunning('build')}<button onclick={() => wb.execute('build.cancel')}><i aria-hidden="true" class="codicon codicon-debug-stop"></i> Cancel Build</button>{/if}
</div>
{#if !wb.workspace.project}
  <div class="empty-state">Open a folder to build, run and debug.</div>
{:else if wb.workspace.restricted}
  <div class="notice"><i aria-hidden="true" class="codicon codicon-shield"></i><span>Building and debugging are disabled in restricted mode.</span><button onclick={() => wb.execute('trust')}>Trust Folder…</button></div>
{/if}
{#if wb.lastBuild?.success && !wb.lastBuild.debugAvailable && wb.lastBuild.debugNote}
  <p class="hint" role="note"><i aria-hidden="true" class="codicon codicon-info"></i> {wb.lastBuild.debugNote}</p>
{/if}

<h3 class="group-title">Breakpoints <span>{debug.breakpoints.length}</span>
  {#if debug.breakpoints.length}<button class="icon-button" title="Remove all breakpoints" aria-label="Remove all breakpoints" onclick={() => debug.removeAll()}><i aria-hidden="true" class="codicon codicon-close-all"></i></button>{/if}
</h3>
<ul class="change-list" aria-label="Breakpoints">
  {#each debug.breakpoints as bp (bp.path + ':' + bp.line)}
    <li>
      <input type="checkbox" checked={bp.enabled} aria-label={`Enable breakpoint at ${bp.path} line ${bp.line}`} onchange={event => debug.setEnabled(bp.path, bp.line, (event.currentTarget as HTMLInputElement).checked)} />
      <button class="change-file" title={debug.active && !bp.verified ? 'No code at this line in the running build' : `Go to ${bp.path}:${bp.line}`} onclick={() => wb.editors.open(bp.path, { line: bp.line })}>
        <span>{bp.path}</span><b>{bp.line}{#if debug.active && bp.verified && bp.resolvedLine !== bp.line} → {bp.resolvedLine}{/if}</b>
      </button>
      <button class="icon-button" title="Remove breakpoint" aria-label={`Remove breakpoint at ${bp.path} line ${bp.line}`} onclick={() => debug.toggleBreakpoint(bp.path, bp.line)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </li>
  {/each}
  {#if debug.breakpoints.length === 0}<li class="empty-state">Click the gutter of an exBASIC file, or press F9, to add a breakpoint.</li>{/if}
</ul>

<h3 class="group-title">Watch <span>{debug.watches.length}</span></h3>
<form class="sidebar-form" onsubmit={event => { event.preventDefault(); addWatch(); }}>
  <div class="input-row">
    <input aria-label="Watch expression" placeholder="SCORE, $D40A, NAME$ + 1" bind:value={watchText} autocomplete="off" spellcheck="false" />
    <button class="icon-button" type="submit" title="Add watch" aria-label="Add watch"><i aria-hidden="true" class="codicon codicon-add"></i></button>
  </div>
</form>
<ul class="watch-list" aria-label="Watch expressions">
  {#each debug.watches as watch (watch.expression)}
    <li>
      <div><strong>{watch.expression}</strong>{#if watch.type}<small>{watch.type}</small>{/if}</div>
      <code class:error={!!watch.error}>{watch.error || watch.value || (debug.active ? '…' : 'not debugging')}</code>
      <button class="icon-button" title="Remove watch" aria-label={`Remove watch ${watch.expression}`} onclick={() => debug.removeWatch(watch.expression)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </li>
  {/each}
</ul>

{#if debug.active}
  <button class="collapsible" aria-expanded={symbolsOpen} onclick={() => (symbolsOpen = !symbolsOpen)}><i aria-hidden="true" class={`codicon codicon-chevron-${symbolsOpen ? 'down' : 'right'}`}></i> Symbols <span>{debug.symbols.length}</span></button>
  {#if symbolsOpen}
    <div class="sidebar-form"><input aria-label="Filter symbols" placeholder="Filter symbols" bind:value={symbolFilter} autocomplete="off" spellcheck="false" /></div>
    <ul class="symbol-list" aria-label="Program symbols">
      {#each symbols as symbol (symbol.name)}
        <li>
          <button title="Add to watch" onclick={() => debug.addWatch(symbol.name)}><span>{symbol.name}</span><small>{symbol.type}</small><code>${hex(symbol.address, 4)}</code></button>
        </li>
      {/each}
      {#if symbols.length === 0}<li class="empty-state">No symbols match.</li>{/if}
    </ul>
  {/if}
{/if}