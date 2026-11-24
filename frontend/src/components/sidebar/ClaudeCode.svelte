<script lang="ts">
  import { onMount } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import { errorMessage, native, type ClaudeCodeInfo } from '../../lib/native';

  let status = $state<ClaudeCodeInfo | null>(null);
  let checking = $state(true);

  async function refresh() {
    checking = true;
    try { status = await native().studio.ClaudeCodeStatus(); }
    catch (error) { status = { available: false, path: '', message: errorMessage(error) }; }
    finally { checking = false; }
  }

  async function open() {
    await wb.execute('claude.open');
  }

  function focus() {
    const id = wb.terminal.activeClaudeId;
    if (!id) return;
    wb.terminal.activeId = id;
    wb.ui.showPanel('terminal');
    wb.focusTerminal();
  }

  onMount(() => { void refresh(); });
</script>

<div class="claude-side">
  <div class="claude-mark" aria-hidden="true"><i class="codicon codicon-robot"></i></div>
  <h3>Claude Code</h3>
  <p>Work with Claude directly in the open project. It runs in an integrated terminal and uses your existing Claude Code login.</p>

  {#if checking}
    <p class="hint"><i class="codicon codicon-loading codicon-modifier-spin" aria-hidden="true"></i> Checking the local CLI…</p>
  {:else if status?.available}
    <p class="claude-ready"><i class="codicon codicon-pass-filled" aria-hidden="true"></i> Ready</p>
    <p class="hint cli-path" title={status.path}>{status.path}</p>
  {:else}
    <p class="claude-missing"><i class="codicon codicon-warning" aria-hidden="true"></i> Claude Code not found</p>
    <p class="hint">{status?.message}</p>
    <button onclick={refresh}><i class="codicon codicon-refresh" aria-hidden="true"></i> Check Again</button>
  {/if}

  <div class="claude-actions">
    {#if wb.terminal.activeClaudeId}
      <button class="primary" onclick={focus}><i class="codicon codicon-terminal" aria-hidden="true"></i> Focus Claude</button>
    {/if}
    <button class:primary={!wb.terminal.activeClaudeId} disabled={!status?.available || !wb.workspace.project} onclick={open}>
      <i class="codicon codicon-add" aria-hidden="true"></i> New Claude Session
    </button>
  </div>

  {#if !wb.workspace.project}
    <p class="hint">Open a project folder to start Claude Code.</p>
  {:else if wb.workspace.restricted}
    <p class="hint">Claude Code needs a trusted folder because it can read, edit, and run project files.</p>
  {/if}

  <p class="claude-privacy"><i class="codicon codicon-lock" aria-hidden="true"></i> Credentials stay with the Claude CLI; exBASIC Studio does not read or store them.</p>
</div>
