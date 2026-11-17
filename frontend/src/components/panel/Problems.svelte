<script lang="ts">
  // Structured diagnostics from live linting and the last build.
  import { wb } from '../../lib/workbench.svelte';
  import { rowNavigation } from '../../lib/actions';

  const problems = $derived(wb.problems.all);
  const icon = (severity: string) => (severity === 'warning' ? 'warning' : severity === 'info' ? 'info' : 'error');
</script>

{#if problems.length === 0}
  <div class="empty-state" role="status">{wb.workspace.restricted ? 'Live diagnostics are off in restricted mode. Trust the folder to check your code as you type.' : 'No problems have been detected.'}</div>
{:else}
  <div class="problems" role="group" aria-label="Problems" use:rowNavigation>
    {#each problems as problem, index (index)}
      <button data-row class={problem.severity} onclick={() => wb.editors.open(problem.path, { line: problem.line, column: problem.column })}>
        <i role="img" class={`codicon codicon-${icon(problem.severity)}`} aria-label={problem.severity}></i>
        <span class="message">{problem.message}</span>
        {#if problem.code}<code>{problem.code}</code>{/if}
        <span class="location">{problem.path}:{problem.line}:{problem.column}</span>
      </button>
    {/each}
  </div>
{/if}