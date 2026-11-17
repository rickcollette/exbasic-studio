<script lang="ts">
  // Modal dialog for confirmations, prompts and choices. Focus is trapped
  // while it is open and returned to the opener when it closes.
  import { onMount } from 'svelte';
  import { wb } from '../lib/workbench.svelte';
  import { trapFocus } from '../lib/actions';

  const dialog = $derived(wb.ui.dialog!);
  let text = $state(wb.ui.dialog?.input?.value ?? '');
  let input = $state<HTMLInputElement>();
  let box = $state<HTMLElement>();

  onMount(() => {
    if (input) { input.focus(); input.select(); }
    else {
      // Focus the safe choice (the first button) unless a primary action
      // exists; destructive buttons never get initial focus.
      const primary = box?.querySelector<HTMLButtonElement>('button.primary') ?? box?.querySelector<HTMLButtonElement>('.dialog-buttons button');
      primary?.focus();
    }
  });

  function answer(value: string) { wb.ui.closeDialog(value, text); }

  function keydown(event: KeyboardEvent) {
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); answer(dialog.cancelValue); }
    else if (event.key === 'Enter' && dialog.input && (event.target as HTMLElement).tagName === 'INPUT') {
      event.preventDefault();
      answer(dialog.buttons.find(button => button.kind === 'primary')?.value ?? dialog.cancelValue);
    }
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop" onmousedown={event => { if (event.target === event.currentTarget) answer(dialog.cancelValue); }} onkeydown={keydown}>
  <div class="dialog" role="alertdialog" aria-modal="true" aria-labelledby="dialog-title" aria-describedby="dialog-message" bind:this={box} use:trapFocus>
    <h2 id="dialog-title">{dialog.title}</h2>
    <p id="dialog-message">{dialog.message}</p>
    {#if dialog.detail}<pre class="dialog-detail">{dialog.detail}</pre>{/if}
    {#if dialog.input}
      <label for="dialog-input">{dialog.input.label}</label>
      <input id="dialog-input" bind:this={input} bind:value={text} placeholder={dialog.input.placeholder ?? ''} autocomplete="off" spellcheck="false" />
    {/if}
    <div class="dialog-buttons">
      {#each dialog.buttons as button}
        <button class={button.kind ?? 'default'} onclick={() => answer(button.value)}>{button.label}</button>
      {/each}
    </div>
  </div>
</div>