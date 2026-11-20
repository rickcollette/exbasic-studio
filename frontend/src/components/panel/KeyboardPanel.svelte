<script lang="ts">
  // A PC or Mac keyboard with the Atari key each key produces in the
  // emulator. Keys held on the emulator screen light up.
  import { wb } from '../../lib/workbench.svelte';
  import { layouts, atariKey, LAYOUT_WIDTH, LAYOUT_HEIGHT, type KeyboardLayout, type KeyKind } from '../../lib/keyboardMap';

  const emulator = wb.emulator;
  const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent);
  let layout = $state<KeyboardLayout>(isMac ? 'mac' : 'pc');
  let width = $state(0);
  let height = $state(0);
  // One key unit, in pixels: as large as the panel allows, within limits
  // that keep the legends legible. Smaller panels scroll.
  const unit = $derived(Math.max(26, Math.min(58, Math.floor(Math.min((width - 24) / LAYOUT_WIDTH, (height - 8) / LAYOUT_HEIGHT)))));

  const keys = $derived(layouts[layout].keys.map(cap => ({ cap, atari: atariKey(cap, emulator.joystickMode) })));

  const legend: { kind: KeyKind; label: string }[] = [
    { kind: 'char', label: 'Same character' },
    { kind: 'atari', label: 'Atari key' },
    { kind: 'console', label: 'Console key' },
    { kind: 'modifier', label: 'Modifier' },
    { kind: 'joystick', label: 'Joystick 0' },
    { kind: 'none', label: 'Not used' },
  ];

  function tooltip(label: string, atari: { label: string; detail?: string; kind: KeyKind }): string {
    const name = label || 'Space';
    if (atari.kind === 'none') return `${name}: ${atari.detail ?? 'not used by the Atari'}`;
    return `${name} → ${atari.detail ?? atari.label}`;
  }
</script>

<div class="keyboard-panel">
  <div class="keyboard-bar">
    <div class="segmented" role="radiogroup" aria-label="Keyboard layout">
      {#each Object.entries(layouts) as [id, entry] (id)}
        <button role="radio" aria-checked={layout === id} class:active={layout === id} onclick={() => (layout = id as KeyboardLayout)}>{entry.label}</button>
      {/each}
    </div>
    <button class="joystick-toggle" aria-pressed={emulator.joystickMode} title="Q W E / A D / Z X C, the arrow keys and the numeric keypad move joystick 0; Space, Numpad 0 and Ctrl fire"
      onclick={() => emulator.setJoystickMode(!emulator.joystickMode)}>
      <i aria-hidden="true" class="codicon codicon-game"></i> Keyboard joystick {emulator.joystickMode ? 'on' : 'off'}
    </button>
    <ul class="keyboard-legend" aria-label="Legend">
      {#each legend as item (item.kind)}<li><span class={`swatch kind-${item.kind}`}></span>{item.label}</li>{/each}
    </ul>
  </div>
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div class="keyboard-scroll" tabindex="0" aria-label="Keyboard map" bind:clientWidth={width} bind:clientHeight={height}>
    <div class="keyboard" role="img" aria-label={`${layouts[layout].label} keyboard with the Atari key of each key`}
      style={`--cols:${LAYOUT_WIDTH * 4}; --rows:${LAYOUT_HEIGHT}; --unit:${unit}px`}>
      {#each keys as { cap, atari } (`${cap.code}-${cap.x}-${cap.y}`)}
        <div class={`key kind-${atari.kind}`} class:held={cap.code !== '' && emulator.held.includes(cap.code)} title={tooltip(cap.label, atari)}
          style={`grid-column:${cap.x * 4 + 1} / span ${cap.w * 4}; grid-row:${cap.y + 1} / span ${cap.h}`}>
          <span class="pc">{cap.shift && atari.kind !== 'char' ? `${cap.label} ${cap.shift}` : cap.label}</span>
          {#if atari.kind !== 'char' && atari.kind !== 'none'}<span class="atari">{atari.label}</span>{/if}
        </div>
      {/each}
    </div>
  </div>
  <p class="keyboard-note">
    Unmarked keys type their own character, as your keyboard layout produces it (Shift+2 is <kbd>@</kbd> on a US keyboard).
    <kbd>{layout === 'mac' ? 'control' : 'Ctrl'}</kbd>+letter types Atari control graphics; Restart in the emulator toolbar stands in for RESET.
    {#if emulator.joystickMode}While the keyboard joystick is on, its keys do not reach the Atari keyboard.{/if}
  </p>
</div>

<style>
  .keyboard-panel { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .keyboard-bar { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 16px; padding: 6px 12px; }
  .segmented { display: inline-flex; border: 1px solid var(--input-border); border-radius: 4px; overflow: hidden; }
  .segmented button { border: 0; border-radius: 0; background: transparent; color: var(--fg); padding: 2px 10px; }
  .segmented button.active { background: var(--accent); color: var(--accent-fg); }
  .joystick-toggle { display: inline-flex; align-items: center; gap: 6px; }
  .joystick-toggle[aria-pressed='true'] { border-color: var(--success); color: var(--success); }
  .keyboard-legend { display: flex; flex-wrap: wrap; gap: 4px 12px; margin: 0; padding: 0; list-style: none; color: var(--muted); font-size: 11px; }
  .keyboard-legend li { display: inline-flex; align-items: center; gap: 4px; }
  .swatch, .key { --key-bg: var(--input-bg); --key-edge: var(--input-border); --key-fg: var(--fg); }
  .swatch { width: 12px; height: 12px; border-radius: 3px; border: 1px solid var(--key-edge); background: var(--key-bg); }
  .keyboard-scroll { flex: 1; min-height: 0; overflow: auto; padding: 4px 12px; }
  .keyboard {
    display: grid; grid-template-columns: repeat(var(--cols), calc(var(--unit) / 4)); grid-template-rows: repeat(var(--rows), var(--unit));
    gap: 0; width: max-content; user-select: none;
  }
  .key {
    margin: 2px; padding: calc(var(--unit) * 0.06) calc(var(--unit) * 0.08); border-radius: 5px; border: 1px solid var(--key-edge); background: var(--key-bg);
    box-shadow: inset 0 -2px 0 rgba(0, 0, 0, 0.25);
    display: flex; flex-direction: column; justify-content: space-between; overflow: hidden; min-width: 0;
  }
  .pc { font-size: calc(var(--unit) * 0.2); line-height: 1.15; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .kind-char .pc { font-size: calc(var(--unit) * 0.3); color: var(--fg); }
  .atari { font-size: calc(var(--unit) * 0.19); font-weight: 700; line-height: 1.1; letter-spacing: -0.02em; color: var(--key-fg); }
  .kind-atari { --key-edge: var(--accent); --key-fg: var(--link); }
  .kind-console { --key-edge: var(--warning); --key-fg: var(--warning); }
  .kind-modifier { --key-edge: var(--syntax-control); --key-fg: var(--syntax-control); }
  .kind-joystick { --key-edge: var(--success); --key-fg: var(--success); }
  .kind-joystick .atari { font-size: calc(var(--unit) * 0.34); line-height: 1; }
  .kind-none { --key-bg: transparent; --key-edge: var(--border); border-style: dashed; box-shadow: none; }
  .key.held { background: var(--key-edge); }
  .key.held .pc, .key.held .atari { color: var(--bg); }
  .key.held.kind-atari .pc, .key.held.kind-atari .atari, .key.held.kind-char .pc { color: var(--accent-fg); }
  .key.held.kind-char { background: var(--accent); }
  .keyboard-note { margin: 0; padding: 4px 12px 8px; color: var(--muted); font-size: 11px; line-height: 1.5; }
  kbd { font-family: var(--mono); border: 1px solid var(--input-border); border-radius: 3px; padding: 0 3px; }
</style>
