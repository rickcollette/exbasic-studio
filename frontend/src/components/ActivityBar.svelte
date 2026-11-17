<script lang="ts">
  import { wb } from '../lib/workbench.svelte';
  import { activities, type Activity } from '../lib/stores/ui.svelte';

  function badge(id: Activity): number {
    if (id === 'source') return wb.git.status?.changes?.length ?? 0;
    if (id === 'run') return wb.debug.breakpoints.length;
    return 0;
  }

  /** Dragging between the project and ATR trees switches the visible view. */
  function dragEnter(event: DragEvent, id: Activity) {
    const types = Array.from(event.dataTransfer?.types ?? []);
    if ((id === 'atr' && types.includes('application/x-exbasic-project-path')) || (id === 'explorer' && types.includes('application/x-exbasic-atr-path'))) wb.ui.showActivity(id);
  }

  function select(id: Activity) {
    wb.ui.toggleActivity(id);
    if (id === 'source' && wb.ui.primaryOpen) void wb.git.autoRefresh();
  }
</script>

<nav class="activitybar" aria-label="Views">
  <div class="activity-top">
    {#each activities as item}
      <button class:active={wb.ui.primaryOpen && wb.ui.activity === item.id} title={item.label} aria-pressed={wb.ui.primaryOpen && wb.ui.activity === item.id}
        ondragenter={event => dragEnter(event, item.id)} onclick={() => select(item.id)}>
        <i aria-hidden="true" class={`codicon codicon-${item.icon}`}></i>
        <span class="sr-only">{item.label}</span>
        {#if badge(item.id)}<span class="badge"><span class="sr-only">, </span>{badge(item.id)}<span class="sr-only"> {item.id === 'source' ? 'changes' : 'breakpoints'}</span></span>{/if}
      </button>
    {/each}
  </div>
  <div class="activity-bottom">
    {#if wb.workspace.restricted}
      <button class="restricted" title="Restricted mode: manage folder trust" aria-label="Restricted mode: manage folder trust" onclick={() => wb.execute('trust')}><i aria-hidden="true" class="codicon codicon-shield"></i></button>
    {/if}
    <button class:active={wb.ui.primaryOpen && wb.ui.activity === 'settings'} title="Settings" aria-label="Settings" aria-pressed={wb.ui.primaryOpen && wb.ui.activity === 'settings'} onclick={() => wb.ui.toggleActivity('settings')}><i aria-hidden="true" class="codicon codicon-gear"></i></button>
  </div>
</nav>