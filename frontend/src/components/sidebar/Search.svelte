<script lang="ts">
  import { wb } from '../../lib/workbench.svelte';
</script>

{#if wb.workspace.project}
  <form class="sidebar-form" onsubmit={event => { event.preventDefault(); void wb.runSearch(); }}>
    <div class="input-row">
      <input aria-label="Search project" placeholder="Search" bind:value={wb.search.query} autocomplete="off" spellcheck="false" />
      <button class="icon-button" type="submit" title="Search" aria-label="Search"><i aria-hidden="true" class="codicon codicon-search"></i></button>
    </div>
    <small role="status">{wb.search.ran ? `${wb.search.results.length}${wb.search.results.length >= 200 ? '+' : ''} result${wb.search.results.length === 1 ? '' : 's'}` : 'Type at least two characters and press Enter.'}</small>
  </form>
  <div class="result-list" role="group" aria-label="Search results">
    {#each wb.search.results as result (result.path + ':' + result.line)}
      <button onclick={() => wb.editors.open(result.path, { line: result.line })}>
        <strong>{result.path}<span>:{result.line}</span></strong><small>{result.text}</small>
      </button>
    {/each}
    {#if wb.search.ran && wb.search.results.length === 0}<div class="empty-state">No results found.</div>{/if}
  </div>
{:else}
  <div class="empty-state">Open a folder to search it.</div>
{/if}