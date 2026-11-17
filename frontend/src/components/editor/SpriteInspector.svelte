<script lang="ts">
  // Right-hand side of the Sprite Studio: palette and plane mapping, the
  // animation preview, sheet size and the generated files.
  import { wb } from '../../lib/workbench.svelte';
  import type { SpriteDoc } from '../../lib/stores/sprites.svelte';
  import { resolveFrom } from '../../lib/stores/sprites.svelte';
  import { DEFAULT_COLORS, MAX_HEIGHT, MAX_PLANES, MAX_WIDTH, TRANSPARENT, dataNameFor, maskPlanes, planeLabel, planesMask, resizeRows, unusedSymbol, type SheetOutputs } from '../../lib/sprites/model';
  import { replaceSymbol } from '../../lib/sprites/edit';
  import { frameBytes } from '../../lib/sprites/pmg';
  import { animationGroup, drawRows } from '../../lib/sprites/canvas';

  let { doc, symbol = $bindable() }: { doc: SpriteDoc; symbol: string } = $props();

  const sheet = $derived(doc.sheet);
  const issues = $derived(doc.issues);
  const used = $derived(new Set(sheet.frames.flatMap(frame => frame.rows.join('').split(''))));

  // ---- Palette ------------------------------------------------------------------

  function setColor(index: number, change: Partial<{ color: string; name: string; planes: number[] }>, key: string) {
    doc.edit(next => { Object.assign(next.palette[index], change); }, `${key}:${index}`);
  }

  function togglePlane(index: number, plane: number) {
    const planes = sheet.palette[index].planes;
    setColor(index, { planes: planes.includes(plane) ? planes.filter(item => item !== plane) : [...planes, plane].sort() }, 'planes');
    doc.seal();
  }

  function renameSymbol(index: number, input: HTMLInputElement) {
    const old = sheet.palette[index].symbol;
    const value = [...input.value.trim()][0] ?? '';
    if (!value || value === old) { input.value = old; return; }
    if (old === TRANSPARENT || value === TRANSPARENT || /\s/.test(value) || sheet.palette.some(color => color.symbol === value)) {
      wb.sprites.status(`"${value}" cannot be used: symbols must be unique, visible, and "." stays transparent`);
      input.value = old;
      return;
    }
    doc.edit(next => {
      next.palette[index].symbol = value;
      for (const frame of next.frames) frame.rows = replaceSymbol(frame.rows, old, value);
    });
    if (symbol === old) symbol = value;
  }

  function addColor() {
    const masks = new Set(sheet.palette.map(color => planesMask(color.planes)));
    let mask = 1;
    while (masks.has(mask) && mask < (1 << sheet.planes) - 1) mask++;
    const next = unusedSymbol(sheet.palette);
    doc.edit(draft => { draft.palette.push({ symbol: next, name: planeLabel(maskPlanes(mask)), color: DEFAULT_COLORS[draft.palette.length % DEFAULT_COLORS.length], planes: maskPlanes(mask) }); });
    symbol = next;
  }

  async function removeColor(index: number) {
    const color = sheet.palette[index];
    if (color.symbol === TRANSPARENT) return;
    if (used.has(color.symbol) && !(await wb.ui.confirm({ title: 'Remove colour?', message: `"${color.symbol}" (${color.name || 'unnamed'}) is used in this sheet.`, detail: 'Its pixels become transparent. You can undo this.', confirmLabel: 'Remove', danger: true }))) return;
    doc.edit(next => {
      next.palette.splice(index, 1);
      for (const frame of next.frames) frame.rows = replaceSymbol(frame.rows, color.symbol, TRANSPARENT);
    });
  }

  // ---- Sheet size ------------------------------------------------------------------

  function resize(field: 'width' | 'height' | 'planes', input: HTMLInputElement) {
    const limit = field === 'width' ? MAX_WIDTH : field === 'height' ? MAX_HEIGHT : MAX_PLANES;
    const value = Math.round(Number(input.value));
    if (!Number.isFinite(value) || value < 1 || value > limit || value === sheet[field]) { input.value = String(sheet[field]); return; }
    doc.edit(next => {
      next[field] = value;
      if (field === 'planes') for (const color of next.palette) color.planes = color.planes.filter(plane => plane < value);
      else for (const frame of next.frames) frame.rows = resizeRows(frame.rows, next.width, next.height);
    });
  }

  // ---- Outputs --------------------------------------------------------------------

  function setOutput<K extends keyof SheetOutputs>(key: K, value: SheetOutputs[K]) {
    doc.edit(next => {
      if (value === '' || value === undefined) delete next.outputs[key]; else next.outputs[key] = value;
    }, `output:${key}`);
  }

  const outputFields: { key: 'bank' | 'index' | 'include' | 'atlas'; label: string; placeholder: string; help: string }[] = [
    { key: 'bank', label: 'PMG bank', placeholder: 'SPRITES.PMG', help: 'Raw frame bytes. Load it with DATA Name() BYTE FILE "…".' },
    { key: 'index', label: 'Index JSON', placeholder: 'sprites_index.json', help: 'Byte offset of every frame and plane.' },
    { key: 'include', label: 'exBASIC include', placeholder: 'SPRITES.BAS', help: 'The bank and frame offsets as DATA statements, for INCLUDE.' },
    { key: 'atlas', label: 'PNG atlas', placeholder: '../previews/sprites.png', help: 'All frames in one PNG, for previews and other tools.' },
  ];

  function resolved(rel: string | undefined): string {
    if (!rel) return '';
    try { return resolveFrom(doc.baseDir, rel); } catch (error) { return (error as Error).message; }
  }

  // ---- Animation preview -------------------------------------------------------------

  let playing = $state(true);
  let fps = $state(6);
  let scope = $state<'group' | 'all'>('group');
  let tick = $state(0);
  let previewCanvas = $state<HTMLCanvasElement>();
  const frames = $derived(scope === 'all' ? sheet.frames.map((_, index) => index) : animationGroup(sheet, sheet.frames[doc.frame]?.name ?? ''));
  const shown = $derived(frames.length ? frames[tick % frames.length] : -1);
  const previewScale = $derived(Math.max(1, Math.floor(96 / Math.max(sheet.width, sheet.height))));

  $effect(() => {
    if (!playing || frames.length < 2) return;
    const timer = setInterval(() => { tick++; }, 1000 / Math.max(1, Math.min(60, fps)));
    return () => clearInterval(timer);
  });
  $effect(() => {
    if (!previewCanvas || shown < 0 || !sheet.frames[shown]) return;
    drawRows(previewCanvas, sheet, sheet.frames[shown].rows, { scale: previewScale, background: '#000000' });
  });
</script>

<aside class="sprite-inspector" aria-label="Sprite settings">
  <section>
    <div class="section-header"><strong>Palette</strong>
      <span class="section-actions"><button class="icon-button" title="Add colour" aria-label="Add colour" onclick={addColor}><i aria-hidden="true" class="codicon codicon-add"></i></button></span>
    </div>
    <p class="hint">Click a colour to draw with it (keys 0-9). Each colour sets the player planes ticked; pixels in two planes show the GTIA overlap colour, so set the preview colour to match.</p>
    <ul class="palette" aria-label="Colours">
      {#each sheet.palette as color, index (index)}
        <li class:selected={symbol === color.symbol}>
          <button class="swatch-pick" title={`Draw with "${color.symbol}"${index < 10 ? ` (key ${index})` : ''}`} aria-label={`Draw with ${color.name || color.symbol}`} aria-pressed={symbol === color.symbol} onclick={() => (symbol = color.symbol)}>
            <span class="swatch" class:transparent={color.symbol === TRANSPARENT} style={color.symbol === TRANSPARENT ? '' : `background:${color.color}`}></span>
          </button>
          <input class="symbol" value={color.symbol} maxlength="2" aria-label="Symbol" title="The character used for this colour in the JSON rows" disabled={color.symbol === TRANSPARENT}
            onchange={event => renameSymbol(index, event.currentTarget)} />
          <input class="name" value={color.name} aria-label="Colour name" placeholder="name" oninput={event => setColor(index, { name: event.currentTarget.value }, 'name')} onblur={() => doc.seal()} />
          {#if color.symbol !== TRANSPARENT}
            <input class="color" type="color" value={color.color} aria-label={`Preview colour of ${color.symbol}`} oninput={event => setColor(index, { color: event.currentTarget.value }, 'color')} onchange={() => doc.seal()} />
            <span class="planes">
              {#each Array.from({ length: sheet.planes }, (_, n) => n) as plane}
                <label class="plane" title={`Sets player ${plane}`}><input type="checkbox" checked={color.planes.includes(plane)} onchange={() => togglePlane(index, plane)} />P{plane}</label>
              {/each}
            </span>
            <button class="icon-button" title="Remove colour" aria-label={`Remove ${color.symbol}`} onclick={() => removeColor(index)}><i aria-hidden="true" class="codicon codicon-close"></i></button>
          {:else}
            <span class="planes muted">transparent</span>
          {/if}
        </li>
      {/each}
    </ul>
  </section>

  <section>
    <div class="section-header"><strong>Preview</strong>
      <span class="section-actions">
        <button class="icon-button" title={playing ? 'Pause' : 'Play'} aria-label={playing ? 'Pause animation' : 'Play animation'} onclick={() => (playing = !playing)}><i aria-hidden="true" class={`codicon codicon-${playing ? 'debug-pause' : 'play'}`}></i></button>
      </span>
    </div>
    <div class="preview">
      <canvas bind:this={previewCanvas} aria-label="Animation preview"></canvas>
      <div class="preview-controls">
        <label for="sprite-fps">Frames per second</label>
        <input id="sprite-fps" type="number" min="1" max="60" bind:value={fps} />
        <label for="sprite-scope">Animate</label>
        <select id="sprite-scope" bind:value={scope}>
          <option value="group">Frames named like this one</option>
          <option value="all">All frames</option>
        </select>
        <small>{frames.length} frame{frames.length === 1 ? '' : 's'}{shown >= 0 ? `: ${sheet.frames[shown]?.name}` : ''}</small>
      </div>
    </div>
  </section>

  <section>
    <div class="section-header"><strong>Sheet</strong></div>
    <div class="fields three">
      <div><label for="sprite-width">Width</label><input id="sprite-width" type="number" min="1" max={MAX_WIDTH} value={sheet.width} onchange={event => resize('width', event.currentTarget)} /></div>
      <div><label for="sprite-height">Height</label><input id="sprite-height" type="number" min="1" max={MAX_HEIGHT} value={sheet.height} onchange={event => resize('height', event.currentTarget)} /></div>
      <div><label for="sprite-planes" title="Players overlaid per frame">Planes</label><input id="sprite-planes" type="number" min="1" max={MAX_PLANES} value={sheet.planes} onchange={event => resize('planes', event.currentTarget)} /></div>
    </div>
    <p class="hint">{frameBytes(sheet)} bytes per frame, {frameBytes(sheet) * sheet.frames.length} bytes in the bank.
      {#if doc.sheetPath}Saved in {doc.sheetPath}{doc.framesPath ? `, frames in ${doc.framesPath}` : ''}.{:else if doc.framesPath}Frames saved in {doc.framesPath}; settings are not saved.{:else if doc.bank}Saving rewrites {doc.bank.bank}{doc.bank.index ? ` and ${doc.bank.index}` : ''}.{:else}Not saved yet.{/if}
    </p>
  </section>

  <section>
    <div class="section-header"><strong>Outputs</strong></div>
    <p class="hint">Paths are relative to {doc.baseDir || 'the project folder'}. Generate writes every file set here.</p>
    {#each outputFields as field (field.key)}
      <label for={`sprite-out-${field.key}`} title={field.help}>{field.label}</label>
      <input id={`sprite-out-${field.key}`} value={sheet.outputs[field.key] ?? ''} placeholder={field.placeholder} spellcheck="false" title={resolved(sheet.outputs[field.key]) || field.help}
        oninput={event => setOutput(field.key, event.currentTarget.value.trim())} onblur={() => doc.seal()} />
    {/each}
    <div class="fields two">
      <div><label for="sprite-index-style">Index layout</label>
        <select id="sprite-index-style" value={sheet.outputs.indexStyle ?? (sheet.planes > 1 ? 'planes' : 'simple')} onchange={event => setOutput('indexStyle', event.currentTarget.value as 'planes' | 'simple')}>
          <option value="planes">Per plane (playerN_offset)</option>
          <option value="simple">Simple (offset, length)</option>
        </select></div>
      <div><label for="sprite-data-name">DATA name</label>
        <input id="sprite-data-name" value={sheet.outputs.dataName ?? ''} placeholder={dataNameFor(doc.sheetPath ?? doc.path)} spellcheck="false" oninput={event => setOutput('dataName', event.currentTarget.value.trim())} onblur={() => doc.seal()} /></div>
    </div>
    <div class="fields two">
      <div><label for="sprite-atlas-scale">Atlas scale</label><input id="sprite-atlas-scale" type="number" min="1" max="16" value={sheet.outputs.atlasScale ?? 1} onchange={event => setOutput('atlasScale', Math.max(1, Math.min(16, Math.round(Number(event.currentTarget.value)) || 1)))} /></div>
      <div><label for="sprite-atlas-columns">Atlas columns</label><input id="sprite-atlas-columns" type="number" min="0" value={sheet.outputs.atlasColumns ?? 0} title="0 puts every frame in one row" onchange={event => setOutput('atlasColumns', Math.max(0, Math.round(Number(event.currentTarget.value)) || 0) || undefined)} /></div>
    </div>
  </section>

  <section>
    <div class="section-header"><strong>Problems ({issues.length})</strong></div>
    {#if issues.length}
      <ul class="issues">
        {#each issues as issue}
          <li class={issue.severity}><i aria-hidden="true" class={`codicon codicon-${issue.severity}`}></i>
            {#if issue.frame}<button class="inline" onclick={() => (doc.frame = Math.max(0, sheet.frames.findIndex(frame => frame.name === issue.frame)))}>{issue.message}</button>{:else}<span>{issue.message}</span>{/if}
          </li>
        {/each}
      </ul>
    {:else}<p class="hint">No problems. The sheet is ready to generate.</p>{/if}
  </section>
</aside>

<style>
  .sprite-inspector { border-left: 1px solid var(--border); overflow: auto; background: var(--sidebar-bg); min-height: 0; padding-bottom: 12px; }
  section { border-bottom: 1px solid var(--border); padding-bottom: 8px; }
  .section-header { padding-top: 6px; }
  .hint { margin: 4px 12px; color: var(--muted); font-size: 12px; line-height: 1.4; }
  .palette { list-style: none; margin: 4px 0; padding: 0 8px; display: flex; flex-direction: column; gap: 2px; }
  .palette li { display: grid; grid-template-columns: 26px 26px minmax(0, 1fr) 30px auto 24px; align-items: center; gap: 4px; padding: 2px; border-radius: 3px; border: 1px solid transparent; }
  .palette li.selected { border-color: var(--accent); background: var(--list-active); }
  .swatch-pick { padding: 0; border: none; background: none; }
  .swatch { display: block; width: 22px; height: 22px; border: 1px solid var(--border); border-radius: 2px; }
  .swatch.transparent { background: repeating-conic-gradient(#555 0% 25%, #222 0% 50%) 50% / 10px 10px; }
  .symbol { text-align: center; font-family: var(--mono); padding: 2px; }
  .name { padding: 2px 4px; }
  .color { padding: 0; height: 22px; border: none; background: none; }
  .planes { display: inline-flex; gap: 4px; font-size: 11px; }
  .planes.muted { color: var(--muted); grid-column: span 3; }
  .plane { display: inline-flex; align-items: center; gap: 1px; margin: 0; color: var(--fg); font-size: 11px; }
  .preview { display: flex; gap: 10px; padding: 4px 12px; align-items: flex-start; }
  .preview canvas { image-rendering: pixelated; border: 1px solid var(--border); flex: none; }
  .preview-controls { flex: 1; min-width: 0; }
  .preview-controls label:first-child { margin-top: 0; }
  .preview-controls small { display: block; color: var(--muted); margin-top: 4px; }
  .fields { display: grid; gap: 6px; padding: 0 12px; }
  .fields.three { grid-template-columns: repeat(3, 1fr); }
  .fields.two { grid-template-columns: repeat(2, 1fr); }
  section > label, section > input { margin-left: 12px; margin-right: 12px; width: calc(100% - 24px); }
  .issues { list-style: none; margin: 4px 0; padding: 0 12px; display: flex; flex-direction: column; gap: 4px; font-size: 12px; }
  .issues li { display: flex; gap: 6px; align-items: flex-start; line-height: 1.4; }
  .issues li.error i { color: var(--danger); }
  .issues li.warning i { color: var(--warning); }
  .issues button.inline { text-align: left; color: inherit; }
</style>
