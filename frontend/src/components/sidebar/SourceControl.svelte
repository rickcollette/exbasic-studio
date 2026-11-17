<script lang="ts">
  // Local Git: status, staging, commit, branches, remotes, explicit network
  // operations and merge conflict resolution. Rebase, cherry-pick, bisect,
  // submodules and cloning are left to the integrated terminal.
  import { wb } from '../../lib/workbench.svelte';
  import { fileIcon } from '../../lib/stores/workspace.svelte';
  import type { GitChange } from '../../lib/native';

  const git = wb.git;
  let branchesOpen = $state(false);
  let remotesOpen = $state(false);

  $effect(() => { if (wb.workspace.project && !git.status) void git.autoRefresh(); });

  const code = (change: GitChange) => (change.untracked ? 'U' : change.conflicted ? '!' : (change.staged ? change.index : change.worktree).replace('.', ''));
  const describe = (change: GitChange) => (change.origPath ? `${change.origPath} → ${change.path}` : change.path);

  async function newBranch() {
    const name = await wb.ui.prompt({ title: 'New branch', message: 'Create a branch from the current commit and switch to it.', label: 'Branch name', placeholder: 'feature/name', confirmLabel: 'Create' });
    if (name?.trim()) await git.createBranch(name.trim(), true);
  }

  async function deleteBranch(name: string) {
    if (!(await wb.ui.confirm({ title: 'Delete branch?', message: `Delete the branch ${name}?`, detail: 'Git refuses to delete a branch with unmerged commits unless you force it.', confirmLabel: 'Delete', danger: true }))) return;
    if (await git.deleteBranch(name, false)) return;
    if (await wb.ui.confirm({ title: 'Force delete?', message: `${name} has commits that are not merged.`, detail: 'Force deleting discards those commits.', confirmLabel: 'Force Delete', danger: true })) await git.deleteBranch(name, true);
  }

  async function addRemote() {
    const name = await wb.ui.prompt({ title: 'Add remote', message: 'Name for the new remote.', label: 'Remote name', value: git.remotes.length ? '' : 'origin', confirmLabel: 'Next' });
    if (!name?.trim()) return;
    const url = await wb.ui.prompt({ title: 'Add remote', message: `URL of ${name.trim()}. Authentication uses your existing Git credential helper or SSH agent; exBASIC Studio never asks for credentials.`, label: 'Remote URL', placeholder: 'https://… or git@host:owner/repo.git', confirmLabel: 'Add' });
    if (url?.trim()) await git.addRemote(name.trim(), url.trim());
  }

  async function editRemote(name: string, current: string) {
    const url = await wb.ui.prompt({ title: 'Edit remote', message: `New URL for ${name}.`, label: 'Remote URL', value: current, confirmLabel: 'Save' });
    if (url?.trim() && url.trim() !== current) await git.setRemoteURL(name, url.trim());
  }

  async function removeRemote(name: string) {
    if (await wb.ui.confirm({ title: 'Remove remote?', message: `Remove the remote ${name}?`, detail: 'Its remote-tracking branches are removed too. Nothing on the server changes.', confirmLabel: 'Remove', danger: true })) await git.removeRemote(name);
  }

  async function abortMerge() {
    if (await wb.ui.confirm({ title: 'Abort merge?', message: 'Abort the merge in progress?', detail: 'Conflict resolutions made so far are discarded and the files return to their state before the merge.', confirmLabel: 'Abort Merge', danger: true })) await git.mergeAbort();
  }
</script>

{#if !wb.workspace.project}
  <div class="empty-state">Open a folder to use source control.</div>
{:else if git.blocked}
  <div class="notice">
    <i aria-hidden="true" class="codicon codicon-shield"></i>
    <span>Restricted mode: Git has not been run in this folder. A repository's own configuration can run programs even for a status check.</span>
    <button onclick={() => git.allow()}>Show Repository Status</button>
    <button onclick={() => wb.execute('trust')}>Trust Folder…</button>
  </div>
{:else if !git.status}
  <div class="empty-state" role="status"><i aria-hidden="true" class="codicon codicon-loading codicon-modifier-spin"></i> Reading repository…</div>
{:else if !git.status.available}
  <div class="empty-state">This folder is not a Git repository.</div>
  <button class="primary wide" onclick={() => git.init()}>Initialize Repository</button>
{:else}
  <div class="scm-compose">
    <textarea aria-label="Commit message" rows="3" placeholder="Message (Ctrl+Enter to commit)" bind:value={git.message}
      onkeydown={event => { if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') { event.preventDefault(); void git.commit(); } }}></textarea>
    <button class="primary" disabled={!git.message.trim() || git.staged.length === 0 || git.conflicts.length > 0} onclick={() => git.commit()}
      title={git.staged.length === 0 ? 'Stage changes to commit them' : 'Commit staged changes'}>Commit{git.staged.length ? ` (${git.staged.length})` : ''}</button>
  </div>
  <div class="scm-toolbar" role="toolbar" aria-label="Repository actions">
    <button class="icon-button" title="Refresh" aria-label="Refresh source control" onclick={() => git.refresh()}><i aria-hidden="true" class="codicon codicon-refresh"></i></button>
    <button disabled={git.networkBusy || git.remotes.length === 0} onclick={() => git.network('fetch')} title="Fetch from the default remote"><i aria-hidden="true" class="codicon codicon-cloud-download"></i> Fetch</button>
    <button disabled={git.networkBusy || !git.status.upstream} onclick={() => git.network('pull')} title={git.status.upstream ? `Pull from ${git.status.upstream}` : 'This branch has no upstream'}><i aria-hidden="true" class="codicon codicon-arrow-down"></i> Pull{git.status.behind ? ` ${git.status.behind}` : ''}</button>
    <button disabled={git.networkBusy || git.remotes.length === 0 || git.status.unborn} onclick={() => git.network('push')} title="Push the current branch"><i aria-hidden="true" class="codicon codicon-arrow-up"></i> Push{git.status.ahead ? ` ${git.status.ahead}` : ''}</button>
    {#if git.networkBusy}<button onclick={() => git.cancelNetwork()}><i aria-hidden="true" class="codicon codicon-debug-stop"></i> Cancel</button>{/if}
  </div>

  {#if git.status.merging}
    <div class="notice merge">
      <i aria-hidden="true" class="codicon codicon-git-merge"></i>
      <span>{git.conflicts.length ? `Merge in progress: ${git.conflicts.length} conflict${git.conflicts.length === 1 ? '' : 's'} to resolve.` : 'All conflicts are resolved. Continue to complete the merge.'}</span>
      <button class="primary" disabled={git.conflicts.length > 0} onclick={() => git.mergeContinue()}>Continue Merge</button>
      <button onclick={abortMerge}>Abort</button>
    </div>
  {/if}

  {#if git.conflicts.length}
    <h3 class="group-title">Merge Conflicts <span>{git.conflicts.length}</span></h3>
    <ul class="change-list" aria-label="Merge conflicts">
      {#each git.conflicts as change (change.path)}
        <li>
          <button class="change-file" title={`Resolve ${change.path}`} onclick={() => git.openConflict(change.path)}><i aria-hidden="true" class="codicon codicon-warning"></i><span>{change.path}</span><b class="conflict">!</b></button>
          <button class="icon-button" title="Open in editor to resolve manually" aria-label={`Open ${change.path} in editor`} onclick={() => wb.editors.open(change.path)}><i aria-hidden="true" class="codicon codicon-go-to-file"></i></button>
        </li>
      {/each}
    </ul>
  {/if}

  <h3 class="group-title">Staged Changes <span>{git.staged.length}</span>
    {#if git.staged.length}<button class="icon-button" title="Unstage all" aria-label="Unstage all changes" onclick={() => git.unstageAll()}><i aria-hidden="true" class="codicon codicon-remove"></i></button>{/if}
  </h3>
  <ul class="change-list" aria-label="Staged changes">
    {#each git.staged as change (change.path)}
      <li>
        <button class="change-file" title={`View staged diff of ${describe(change)}`} onclick={() => git.showDiff(change.path, true)}><i aria-hidden="true" class={`codicon codicon-${fileIcon(change.path)}`}></i><span>{describe(change)}</span><b>{code(change)}</b></button>
        <button class="icon-button" title="Unstage" aria-label={`Unstage ${change.path}`} onclick={() => git.unstage(change.path)}><i aria-hidden="true" class="codicon codicon-remove"></i></button>
      </li>
    {/each}
  </ul>

  <h3 class="group-title">Changes <span>{git.unstaged.length}</span>
    {#if git.unstaged.length}<button class="icon-button" title="Stage all" aria-label="Stage all changes" onclick={() => git.stageAll()}><i aria-hidden="true" class="codicon codicon-add"></i></button>{/if}
  </h3>
  <ul class="change-list" aria-label="Changes">
    {#each git.unstaged as change (change.path)}
      <li>
        <button class="change-file" title={`View diff of ${change.path}`} onclick={() => (change.untracked ? wb.editors.open(change.path) : git.showDiff(change.path, false))}><i aria-hidden="true" class={`codicon codicon-${fileIcon(change.path)}`}></i><span>{change.path}</span><b>{change.untracked ? 'U' : change.worktree}</b></button>
        <button class="icon-button" title="Stage" aria-label={`Stage ${change.path}`} onclick={() => git.stage(change.path)}><i aria-hidden="true" class="codicon codicon-add"></i></button>
      </li>
    {/each}
  </ul>
  {#if !(git.status.changes ?? []).length}<div class="empty-state">There are no pending changes.</div>{/if}

  {#if git.diff}
    <div class="diff-title"><span>{git.diff.staged ? 'Staged diff' : 'Diff'}: {git.diff.path}</span><button class="icon-button" title="Close diff" aria-label="Close diff" onclick={() => (git.diff = null)}><i aria-hidden="true" class="codicon codicon-close"></i></button></div>
    <!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre class="diff" tabindex="0" aria-label={`Diff of ${git.diff.path}`}>{#each (git.diff.text || 'No textual changes.').split('\n') as line}<span class:add={line.startsWith('+') && !line.startsWith('+++')} class:del={line.startsWith('-') && !line.startsWith('---')} class:hunk={line.startsWith('@@')}>{line}
</span>{/each}</pre>
  {/if}

  <button class="collapsible" aria-expanded={branchesOpen} onclick={() => (branchesOpen = !branchesOpen)}><i aria-hidden="true" class={`codicon codicon-chevron-${branchesOpen ? 'down' : 'right'}`}></i> Branches <span>{git.branches.length}</span></button>
  {#if branchesOpen}
    <ul class="ref-list" aria-label="Branches">
      {#each git.branches as branch (branch.name)}
        <li class:current={branch.current}>
          <span class="ref-name" title={branch.upstream ? `Tracks ${branch.upstream}` : 'No upstream'}><i aria-hidden="true" class={`codicon codicon-${branch.current ? 'check' : 'git-branch'}`}></i> {branch.name}</span>
          {#if !branch.current}
            <button class="icon-button" title={`Switch to ${branch.name}`} aria-label={`Switch to ${branch.name}`} onclick={() => git.switchBranch(branch.name)}><i aria-hidden="true" class="codicon codicon-arrow-swap"></i></button>
            <button class="icon-button" title={`Delete ${branch.name}`} aria-label={`Delete ${branch.name}`} onclick={() => deleteBranch(branch.name)}><i aria-hidden="true" class="codicon codicon-trash"></i></button>
          {/if}
        </li>
      {/each}
      {#if git.branches.length === 0}<li class="empty-state">No branches yet. Make the first commit.</li>{/if}
    </ul>
    <button class="wide" disabled={git.status.unborn} onclick={newBranch}><i aria-hidden="true" class="codicon codicon-add"></i> New Branch…</button>
  {/if}

  <button class="collapsible" aria-expanded={remotesOpen} onclick={() => (remotesOpen = !remotesOpen)}><i aria-hidden="true" class={`codicon codicon-chevron-${remotesOpen ? 'down' : 'right'}`}></i> Remotes <span>{git.remotes.length}</span></button>
  {#if remotesOpen}
    <ul class="ref-list" aria-label="Remotes">
      {#each git.remotes as remote (remote.name)}
        <li>
          <span class="ref-name" title={remote.fetchUrl}><i aria-hidden="true" class="codicon codicon-cloud"></i> {remote.name} <small>{remote.fetchUrl}</small></span>
          <button class="icon-button" title={`Fetch ${remote.name}`} aria-label={`Fetch ${remote.name}`} disabled={git.networkBusy} onclick={() => git.network('fetch', remote.name)}><i aria-hidden="true" class="codicon codicon-cloud-download"></i></button>
          <button class="icon-button" title={`Edit ${remote.name}`} aria-label={`Edit ${remote.name}`} onclick={() => editRemote(remote.name, remote.fetchUrl)}><i aria-hidden="true" class="codicon codicon-edit"></i></button>
          <button class="icon-button" title={`Remove ${remote.name}`} aria-label={`Remove ${remote.name}`} onclick={() => removeRemote(remote.name)}><i aria-hidden="true" class="codicon codicon-trash"></i></button>
        </li>
      {/each}
      {#if git.remotes.length === 0}<li class="empty-state">No remotes configured.</li>{/if}
    </ul>
    <button class="wide" onclick={addRemote}><i aria-hidden="true" class="codicon codicon-add"></i> Add Remote…</button>
    <p class="hint">Network operations run only when you start them, and use your existing Git credentials.</p>
  {/if}
{/if}
