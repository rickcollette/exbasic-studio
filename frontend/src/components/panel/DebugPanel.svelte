<script lang="ts">
  // Debugger panel: execution controls, registers, return stack, nearby
  // disassembly and a read-only memory viewer.
  import { wb } from '../../lib/workbench.svelte';
  import { hex } from '../../lib/actions';

  const debug = wb.debug;
  const stop = $derived(debug.stop);
  let addressText = $state('$0080');

  const flags = (p: number) => 'NV-BDIZC'.split('').map((flag, index) => ((p >> (7 - index)) & 1 ? flag : '·')).join('');

  function parseAddress(text: string): number | null {
    const clean = text.trim();
    const value = clean.startsWith('$') ? parseInt(clean.slice(1), 16) : /^0x/i.test(clean) ? parseInt(clean.slice(2), 16) : parseInt(clean, 10);
    return Number.isFinite(value) && value >= 0 && value <= 0xFFFF ? value : null;
  }

  function showMemory() {
    const symbol = debug.symbols.find(item => item.name.toLowerCase() === addressText.trim().toLowerCase());
    const address = symbol ? symbol.address : parseAddress(addressText);
    if (address === null) { wb.ui.status = 'Enter an address such as $0600, 1536, or a symbol name'; return; }
    void debug.readMemory(address);
  }

  const rows = $derived.by(() => {
    const memory = debug.memory;
    if (!memory) return [];
    const out: { address: number; bytes: number[] }[] = [];
    for (let offset = 0; offset < memory.bytes.length; offset += 16) out.push({ address: memory.address + offset, bytes: memory.bytes.slice(offset, offset + 16) });
    return out;
  });
  const ascii = (bytes: number[]) => bytes.map(byte => { const c = byte & 0x7F; return c >= 0x20 && c < 0x7F ? String.fromCharCode(c) : '.'; }).join('');
</script>

<div class="debug-panel">
  <div class="debug-toolbar" role="toolbar" aria-label="Debugger controls">
    {#if debug.active}
      {#if debug.stopped}
        <button class="icon-button" title="Continue (F5)" aria-label="Continue" onclick={() => wb.execute('debug.continue')}><i aria-hidden="true" class="codicon codicon-debug-continue"></i></button>
      {:else}
        <button class="icon-button" title="Pause (F8)" aria-label="Pause" onclick={() => wb.execute('debug.pause')}><i aria-hidden="true" class="codicon codicon-debug-pause"></i></button>
      {/if}
      <button class="icon-button" title="Step to next source line (F10)" aria-label="Step to next source line" disabled={!debug.stopped} onclick={() => wb.execute('debug.stepLine')}><i aria-hidden="true" class="codicon codicon-debug-step-over"></i></button>
      <button class="icon-button" title="Step one 6502 instruction (F11)" aria-label="Step one instruction" disabled={!debug.stopped} onclick={() => wb.execute('debug.stepInstruction')}><i aria-hidden="true" class="codicon codicon-debug-step-into"></i></button>
      <button class="icon-button" title="Step over JSR (Shift+F10)" aria-label="Step over subroutine call" disabled={!debug.stopped} onclick={() => wb.execute('debug.stepOver')}><i aria-hidden="true" class="codicon codicon-debug-step-back"></i></button>
      <button class="icon-button" title="Step out (Shift+F11)" aria-label="Step out" disabled={!debug.stopped} onclick={() => wb.execute('debug.stepOut')}><i aria-hidden="true" class="codicon codicon-debug-step-out"></i></button>
      <button class="icon-button" title="Stop debugging (Shift+F5)" aria-label="Stop debugging" onclick={() => wb.execute('debug.stop')}><i aria-hidden="true" class="codicon codicon-debug-stop"></i></button>
      <span class="state" role="status">{debug.stopped ? (stop?.path ? `${stop.reason === 'breakpoint' ? 'Breakpoint' : 'Stopped'} at ${stop.path}:${stop.line}` : `Stopped at $${hex(stop?.registers.pc ?? 0, 4)} (no source line)`) : 'Running'}</span>
    {:else}
      <button class="primary" disabled={!wb.commandEnabled('debug.start') || debug.starting} onclick={() => wb.execute('debug.start')}><i aria-hidden="true" class="codicon codicon-debug-start"></i> Start Debugging</button>
      <span class="state">{debug.starting ? 'Building and starting…' : 'Not debugging. Set a breakpoint in an exBASIC file and press F5.'}</span>
    {/if}
  </div>
  {#if debug.active && stop}
    <div class="debug-grid">
      <section aria-label="Registers">
        <h3>Registers</h3>
        <dl class="registers">
          <dt>PC</dt><dd>${hex(stop.registers.pc, 4)}</dd><dt>A</dt><dd>${hex(stop.registers.a, 2)}</dd><dt>X</dt><dd>${hex(stop.registers.x, 2)}</dd>
          <dt>Y</dt><dd>${hex(stop.registers.y, 2)}</dd><dt>S</dt><dd>${hex(stop.registers.s, 2)}</dd><dt>P</dt><dd>${hex(stop.registers.p, 2)} <span title="N V - B D I Z C">{flags(stop.registers.p)}</span></dd>
        </dl>
        <p class="hint">{stop.bytecode ? `Interpreting bytecode at $${hex(stop.location, 4)}` : 'Running 6502 machine code'} · read-only</p>
      </section>
      <section aria-label="Call stack">
        <h3>Call Stack</h3>
        <ol class="frames">
          {#each stop.frames ?? [] as frame}
            <li>{#if frame.path}<button class="inline" onclick={() => wb.editors.open(frame.path!, { line: frame.line! })}>{frame.path}:{frame.line}</button>{:else}<span>${hex(frame.call, 4)}</span>{/if} <small>JSR ${hex(frame.target, 4)} → returns to ${hex(frame.return, 4)}</small></li>
          {/each}
          {#if (stop.frames ?? []).length === 0}<li class="empty-state">No subroutine calls on the 6502 stack.</li>{/if}
        </ol>
      </section>
      <section aria-label="Disassembly">
        <h3>Disassembly</h3>
        <table class="disassembly"><tbody>
          {#each stop.disassembly ?? [] as instruction, index}
            <tr class:current={index === 0}><td>${hex(instruction.address, 4)}</td><td>{instruction.bytes}</td><td>{instruction.text}</td></tr>
          {/each}
        </tbody></table>
      </section>
    </div>
  {/if}
  {#if debug.active}
    <section class="memory" aria-label="Memory">
      <form class="input-row" onsubmit={event => { event.preventDefault(); showMemory(); }}>
        <label for="memory-address">Memory</label>
        <input id="memory-address" bind:value={addressText} placeholder="$0600, 1536 or a symbol" autocomplete="off" spellcheck="false" />
        <button type="submit">Show</button>
        {#if debug.memory}
          <button type="button" class="icon-button" title="Previous page" aria-label="Previous memory page" onclick={() => debug.readMemory(debug.memory!.address - 256)}><i aria-hidden="true" class="codicon codicon-chevron-up"></i></button>
          <button type="button" class="icon-button" title="Next page" aria-label="Next memory page" onclick={() => debug.readMemory(debug.memory!.address + 256)}><i aria-hidden="true" class="codicon codicon-chevron-down"></i></button>
        {/if}
      </form>
      {#if debug.memory}
        <!-- svelte-ignore a11y_no_noninteractive_tabindex --><pre class="memory-dump" tabindex="0" aria-label={`Memory from $${hex(debug.memory.address, 4)}`}>{#each rows as row}{hex(row.address, 4)}  {row.bytes.map(byte => hex(byte, 2)).join(' ')}  {ascii(row.bytes)}
{/each}</pre>
      {/if}
    </section>
  {/if}
</div>