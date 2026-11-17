<script lang="ts">
  // Three-way merge conflict view: base, current (ours) and incoming
  // (theirs) side by side, with accept actions and manual resolution.
  import { wb } from '../../lib/workbench.svelte';

  const git = wb.git;
  const conflict = $derived(git.conflict!);

  async function manual() {
    const path = conflict.path;
    git.conflict = null;
    await wb.editors.open(path);
  }

  async function accept(choice: 'current' | 'incoming' | 'both') {
    const path = conflict.path;
    if (await git.resolve(path, choice)) await wb.editors.resolveExternal(path, 'reload');
  }
</script>

<section class="conflict" aria-label={`Merge conflict in ${conflict.path}`}>
  <header>
    <h2><i aria-hidden="true" class="codicon codicon-git-merge"></i> {conflict.path}</h2>
    <div class="conflict-actions" role="toolbar" aria-label="Resolve conflict">
      <button onclick={() => accept('current')} disabled={conflict.binary && !conflict.hasCurrent}>Accept Current</button>
      <button onclick={() => accept('incoming')} disabled={conflict.binary && !conflict.hasIncoming}>Accept Incoming</button>
      <button onclick={() => accept('both')} disabled={conflict.binary || !conflict.hasCurrent || !conflict.hasIncoming}>Accept Both</button>
      <button onclick={manual} disabled={conflict.binary}>Resolve in Editor</button>
      <button onclick={() => git.stageResolved(conflict.path)} title="Stage the file as resolved after editing it">Mark Resolved</button>
      <button class="icon-button" title="Close" aria-label="Close conflict view" onclick={() => (git.conflict = null)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
    </div>
  </header>
  {#if conflict.binary}
    <p class="empty-state">This file is not text, so its contents cannot be compared. Choose which side to keep.</p>
  {:else}
    <div class="conflict-columns">
      <div><h3>Base <small>common ancestor</small></h3><!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre tabindex="0" aria-label="Base version">{conflict.hasBase ? conflict.base : '(the file did not exist)'}</pre></div>
      <div><h3>Current <small>your branch</small></h3><!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre tabindex="0" aria-label="Current version">{conflict.hasCurrent ? conflict.current : '(deleted on your branch)'}</pre></div>
      <div><h3>Incoming <small>being merged</small></h3><!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre tabindex="0" aria-label="Incoming version">{conflict.hasIncoming ? conflict.incoming : '(deleted on the incoming branch)'}</pre></div>
    </div>
  {/if}
</section>