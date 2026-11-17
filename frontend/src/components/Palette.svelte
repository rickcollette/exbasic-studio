<script lang="ts">
  // Command palette, quick open and workspace symbols. A combobox with a
  // listbox popup: arrow keys move the selection, Enter runs it, Escape
  // closes and restores focus.
  import { onMount, tick } from 'svelte';
  import { wb } from '../lib/workbench.svelte';
  import { trapFocus } from '../lib/actions';

  let input = $state<HTMLInputElement>();
  let list = $state<HTMLElement>();
  const items = $derived(wb.paletteItems);
  const placeholder = $derived(wb.palette.mode === 'commands' ? 'Type the name of a command' : wb.palette.mode === 'files' ? 'Search files by name' : 'Search symbols in the workspace');

  onMount(() => input?.focus());

  $effect(() => {
    // Keep the selection valid and visible as the list changes.
    const index = Math.min(wb.palette.index, Math.max(0, items.length - 1));
    void tick().then(() => list?.querySelector<HTMLElement>(`[data-index="${index}"]`)?.scrollIntoView({ block: 'nearest' }));
  });

  function keydown(event: KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown': event.preventDefault(); wb.movePalette(1); break;
      case 'ArrowUp': event.preventDefault(); wb.movePalette(-1); break;
      case 'Enter': event.preventDefault(); wb.runPaletteSelection(); break;
      case 'Escape': event.preventDefault(); event.stopPropagation(); wb.closePalette(); break;
    }
  }

  function typed() { wb.palette = { ...wb.palette, query: input?.value ?? '', index: 0 }; }
  const selected = $derived(Math.min(wb.palette.index, Math.max(0, items.length - 1)));
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="palette-backdrop" onmousedown={event => { if (event.target === event.currentTarget) wb.closePalette(); }}>
  <div class="palette" use:trapFocus>
    <div class="palette-input">
      <span aria-hidden="true">{wb.palette.mode === 'commands' ? '>' : wb.palette.mode === 'symbols' ? '@' : ''}</span>
      <input bind:this={input} value={wb.palette.query} oninput={typed} onkeydown={keydown} {placeholder} role="combobox" aria-expanded="true" aria-controls="palette-list"
        aria-activedescendant={items.length ? `palette-item-${selected}` : undefined} aria-label={placeholder} autocomplete="off" spellcheck="false" />
    </div>
    <div class="palette-results" id="palette-list" role="listbox" aria-label="Results" bind:this={list}>
      {#each items as item, index (item.key)}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div id={`palette-item-${index}`} data-index={index} role="option" tabindex="-1" aria-selected={index === selected} aria-disabled={item.disabled ? 'true' : undefined} class:selected={index === selected} class:disabled={item.disabled}
          onclick={() => { if (!item.disabled) item.run(); }} onmousemove={() => { if (wb.palette.index !== index && !item.disabled) wb.palette = { ...wb.palette, index }; }}>
          <i aria-hidden="true" class={`codicon codicon-${item.icon}`}></i><span class="label">{item.label}</span>{#if item.detail}<span class="detail">{item.detail}</span>{/if}<kbd>{item.shortcut ?? ''}</kbd>
        </div>
      {/each}
      {#if items.length === 0}<div class="empty-state" role="status">No matching {wb.palette.mode === 'commands' ? 'commands' : wb.palette.mode}.</div>{/if}
    </div>
  </div>
</div>