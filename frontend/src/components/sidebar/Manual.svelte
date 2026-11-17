<script lang="ts">
  // The bundled handbook, rendered as sanitized Markdown with anchors,
  // cross links, search highlighting and back/forward navigation.
  import { tick } from 'svelte';
  import { wb } from '../../lib/workbench.svelte';
  import { renderManual, highlightMatches, chapterTitle } from '../../lib/markdown';

  const manual = wb.manual;
  let view = $state<HTMLElement>();
  const rendered = $derived(manual.markdown ? renderManual(manual.markdown) : null);

  // After a chapter renders: highlight the search term and scroll to the
  // requested heading (or the top).
  $effect(() => {
    const location = manual.current;
    void rendered;
    if (!location) return;
    void tick().then(() => {
      if (!view) return;
      highlightMatches(view, manual.query);
      const target = location.anchor ? view.querySelector<HTMLElement>(`#${CSS.escape(location.anchor)}`) : null;
      if (target) target.scrollIntoView({ block: 'start' });
      else if (manual.query.trim().length >= 2) view.querySelector('mark.search-hit')?.scrollIntoView({ block: 'center' });
      else view.scrollTop = 0;
    });
  });

  /** Follows in-manual links; chapter names are validated before use. */
  function click(event: MouseEvent) {
    const link = (event.target as Element | null)?.closest<HTMLAnchorElement>('a[data-anchor], a[data-chapter]');
    if (!link) return;
    event.preventDefault();
    const chapter = decodeURIComponent(link.dataset.chapter ?? '') || manual.current?.chapter || '';
    const anchor = decodeURIComponent(link.dataset.anchor ?? '');
    if (manual.chapters.includes(chapter)) void manual.open(chapter, anchor);
  }
</script>

<form class="sidebar-form" onsubmit={event => { event.preventDefault(); void manual.search(); }}>
  <div class="input-row">
    <button class="icon-button" type="button" title="Back" aria-label="Back in manual" disabled={!manual.historyState.back} onclick={() => manual.go(-1)}><i aria-hidden="true" class="codicon codicon-arrow-left"></i></button>
    <button class="icon-button" type="button" title="Forward" aria-label="Forward in manual" disabled={!manual.historyState.forward} onclick={() => manual.go(1)}><i aria-hidden="true" class="codicon codicon-arrow-right"></i></button>
    <input aria-label="Search manual" placeholder="Search manual" bind:value={manual.query} autocomplete="off" spellcheck="false" />
    <button class="icon-button" type="submit" title="Search manual" aria-label="Search manual"><i aria-hidden="true" class="codicon codicon-search"></i></button>
  </div>
  <select aria-label="Manual chapter" value={manual.current?.chapter ?? ''} onchange={event => { const value = (event.currentTarget as HTMLSelectElement).value; if (value) void manual.open(value); }}>
    <option value="">Choose a chapter</option>
    {#each manual.chapters as chapter (chapter)}<option value={chapter}>{chapterTitle(chapter)}</option>{/each}
  </select>
</form>
{#if manual.hits.length}
  <div class="result-list compact" role="group" aria-label="Manual search results">
    {#each manual.hits as hit, index (index)}
      <button onclick={() => manual.open(hit.file)}><strong>{hit.title}</strong><small>{hit.snippet}</small></button>
    {/each}
  </div>
{/if}
{#if rendered}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions, a11y_no_noninteractive_tabindex -->
  <article class="manual" bind:this={view} onclick={click} tabindex="0" aria-label={`Manual: ${chapterTitle(manual.current?.chapter ?? '')}`}>{@html rendered.html}</article>
{:else}
  <div class="empty-state">Choose a chapter, or search the manual. Hover a keyword in the editor and choose "Open in manual" to jump straight to its section.</div>
{/if}