<script lang="ts">
  // The Sprite Studio editor: frames on the left, the pixel canvas in the
  // middle, palette/preview/settings on the right. Every change goes through
  // the document so it can be undone and is saved like any other editor.
  import { wb } from '../../lib/workbench.svelte';
  import type { SpriteDoc } from '../../lib/stores/sprites.svelte';
  import { TRANSPARENT, blankRows, uniqueFrameName } from '../../lib/sprites/model';
  import { drawRows, frameCanvas } from '../../lib/sprites/canvas';
  import { floodFill, flipHorizontal, flipVertical, getPixel, linePoints, rectPoints, setPixels, shift, clear } from '../../lib/sprites/edit';
  import { frameBytes } from '../../lib/sprites/pmg';
  import SpriteInspector from './SpriteInspector.svelte';
  import SpriteImport from './SpriteImport.svelte';

  let { doc }: { doc: SpriteDoc } = $props();

  type Tool = 'pencil' | 'eraser' | 'fill' | 'line' | 'rect' | 'fillrect' | 'picker';
  const tools: { id: Tool; icon: string; label: string; key: string }[] = [
    { id: 'pencil', icon: 'pencil', label: 'Pencil', key: 'B' },
    { id: 'eraser', icon: 'circle-slash', label: 'Eraser', key: 'E' },
    { id: 'fill', icon: 'paintcan', label: 'Fill', key: 'G' },
    { id: 'line', icon: 'dash', label: 'Line', key: 'L' },
    { id: 'rect', icon: 'primitive-square', label: 'Rectangle', key: 'R' },
    { id: 'fillrect', icon: 'layout', label: 'Filled rectangle', key: 'Shift+R' },
    { id: 'picker', icon: 'inspect', label: 'Pick colour', key: 'I' },
  ];

  let tool = $state<Tool>('pencil');
  let symbol = $state('');
  let zoom = $state(0);
  let grid = $state(true);
  let onion = $state(false);
  let plane = $state<number | null>(null);
  let importOpen = $state(false);
  let renaming = $state('');
  let canvas = $state<HTMLCanvasElement>();
  let root = $state<HTMLElement>();
  let hover = $state<{ x: number; y: number } | null>(null);
  let preview = $state<string[] | null>(null);

  const sheet = $derived(doc.sheet);
  const frame = $derived(sheet.frames[doc.frame]);
  const scale = $derived(zoom || Math.max(4, Math.min(40, Math.floor(440 / Math.max(sheet.width, sheet.height)))));
  const drawSymbol = $derived(sheet.palette.some(color => color.symbol === symbol) ? symbol : (sheet.palette.find(color => color.symbol !== TRANSPARENT)?.symbol ?? TRANSPARENT));
  const hoverSymbol = $derived(hover && frame ? getPixel(frame.rows, hover.x, hover.y) : '');
  const hoverColor = $derived(sheet.palette.find(color => color.symbol === hoverSymbol));

  // A PNG opened from the explorer goes straight to the import panel.
  $effect(() => { if (doc.pendingImage) importOpen = true; });
  $effect(() => { if (plane !== null && plane >= sheet.planes) plane = null; });

  $effect(() => {
    if (!canvas || !frame) return;
    const under = onion && doc.frame > 0 ? sheet.frames[doc.frame - 1].rows : null;
    drawRows(canvas, sheet, preview ?? frame.rows, { scale, grid, under, plane });
  });

  // ---- Drawing ---------------------------------------------------------------

  let stroke = 0;
  let strokeKey = '';
  let anchor: { x: number; y: number } | null = null;
  let last: { x: number; y: number } | null = null;
  let erasing = false;

  function cellAt(event: PointerEvent): { x: number; y: number } | null {
    if (!canvas) return null;
    const box = canvas.getBoundingClientRect();
    const x = Math.floor(((event.clientX - box.left) / box.width) * sheet.width);
    const y = Math.floor(((event.clientY - box.top) / box.height) * sheet.height);
    return x >= 0 && y >= 0 && x < sheet.width && y < sheet.height ? { x, y } : null;
  }

  function setRows(rows: string[], key?: string) {
    const index = doc.frame;
    if (rows === sheet.frames[index].rows) return;
    doc.edit(next => { next.frames[index].rows = rows; }, key);
  }

  function shapeRows(from: { x: number; y: number }, to: { x: number; y: number }, value: string): string[] {
    const points = tool === 'line' ? linePoints(from.x, from.y, to.x, to.y) : rectPoints(from.x, from.y, to.x, to.y, tool === 'fillrect');
    return setPixels(frame.rows, points, value);
  }

  function pointerDown(event: PointerEvent) {
    if (!frame || (event.button !== 0 && event.button !== 2)) return;
    const cell = cellAt(event);
    if (!cell) return;
    event.preventDefault();
    canvas?.setPointerCapture(event.pointerId);
    erasing = event.button === 2 || tool === 'eraser';
    const value = erasing ? TRANSPARENT : drawSymbol;
    strokeKey = `stroke:${++stroke}`;
    if (tool === 'picker') { symbol = getPixel(frame.rows, cell.x, cell.y); tool = 'pencil'; return; }
    if (tool === 'fill') { setRows(floodFill(frame.rows, cell.x, cell.y, value), strokeKey); doc.seal(); return; }
    if (tool === 'line' || tool === 'rect' || tool === 'fillrect') { anchor = cell; preview = shapeRows(cell, cell, value); return; }
    last = cell;
    setRows(setPixels(frame.rows, [[cell.x, cell.y]], value), strokeKey);
  }

  function pointerMove(event: PointerEvent) {
    const cell = cellAt(event);
    hover = cell;
    if (!cell || !event.buttons) return;
    const value = erasing ? TRANSPARENT : drawSymbol;
    if (anchor) { preview = shapeRows(anchor, cell, value); return; }
    if (!last || (tool !== 'pencil' && tool !== 'eraser')) return;
    setRows(setPixels(frame.rows, linePoints(last.x, last.y, cell.x, cell.y), value), strokeKey);
    last = cell;
  }

  function pointerUp() {
    if (anchor && preview) setRows(preview, strokeKey);
    anchor = null; preview = null; last = null;
    doc.seal();
  }

  // ---- Frame operations ------------------------------------------------------

  function transform(apply: (rows: string[]) => string[]) { if (frame) { setRows(apply(frame.rows)); doc.seal(); } }

  function addFrame(copy: boolean) {
    const at = doc.frame + 1;
    doc.edit(next => {
      const name = uniqueFrameName(next, frame?.name ?? 'frame_0');
      next.frames.splice(at, 0, { name, rows: copy && frame ? [...frame.rows] : blankRows(next.width, next.height) });
    });
    doc.frame = at;
  }

  async function deleteFrame() {
    if (!frame || sheet.frames.length <= 1) return;
    const index = doc.frame;
    doc.edit(next => { next.frames.splice(index, 1); });
    wb.sprites.status(`Deleted frame ${frame.name}. Undo with Ctrl+Z.`);
  }

  function moveFrame(delta: number) {
    const from = doc.frame;
    const to = from + delta;
    if (to < 0 || to >= sheet.frames.length) return;
    doc.edit(next => { const [item] = next.frames.splice(from, 1); next.frames.splice(to, 0, item); });
    doc.frame = to;
  }

  function rename(index: number, value: string) {
    renaming = '';
    const name = value.trim();
    if (!name || name === sheet.frames[index].name) return;
    if (sheet.frames.some(item => item.name === name)) { wb.sprites.status(`A frame called ${name} already exists`); return; }
    doc.edit(next => { next.frames[index].name = name; });
  }

  // ---- Keyboard ----------------------------------------------------------------

  function key(event: KeyboardEvent) {
    const target = event.target as HTMLElement;
    if (target.closest('input, select, textarea')) return;
    const mod = event.ctrlKey || event.metaKey;
    const k = event.key.toLowerCase();
    if (mod && !event.altKey) {
      if (k === 'z' && !event.shiftKey) { event.preventDefault(); doc.undo(); return; }
      if (k === 'y' || (k === 'z' && event.shiftKey)) { event.preventDefault(); doc.redo(); return; }
      if (k === 'd') { event.preventDefault(); addFrame(true); return; }
      return;
    }
    if (event.altKey) return;
    const pick = (next: Tool) => { event.preventDefault(); tool = next; };
    switch (k) {
      case 'b': return pick('pencil');
      case 'e': return pick('eraser');
      case 'g': return pick('fill');
      case 'l': return pick('line');
      case 'r': return pick(event.shiftKey ? 'fillrect' : 'rect');
      case 'i': return pick('picker');
      case '[': case 'pageup': event.preventDefault(); doc.frame = Math.max(0, doc.frame - 1); return;
      case ']': case 'pagedown': event.preventDefault(); doc.frame = Math.min(sheet.frames.length - 1, doc.frame + 1); return;
      case '+': case '=': event.preventDefault(); zoom = Math.min(64, scale + 2); return;
      case '-': event.preventDefault(); zoom = Math.max(2, scale - 2); return;
      case 'arrowup': if (event.shiftKey) { event.preventDefault(); transform(rows => shift(rows, 0, -1)); } return;
      case 'arrowdown': if (event.shiftKey) { event.preventDefault(); transform(rows => shift(rows, 0, 1)); } return;
      case 'arrowleft': if (event.shiftKey) { event.preventDefault(); transform(rows => shift(rows, -1, 0)); } return;
      case 'arrowright': if (event.shiftKey) { event.preventDefault(); transform(rows => shift(rows, 1, 0)); } return;
    }
    if (/^[0-9]$/.test(k)) {
      const color = sheet.palette[Number(k)];
      if (color) { event.preventDefault(); symbol = color.symbol; }
    }
  }

  const bytesPerFrame = $derived(frameBytes(sheet));
</script>

<!-- The editor surface takes tool and frame shortcuts while it has focus. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<section class="sprite-studio" bind:this={root} tabindex="0" role="application" aria-label={`Sprite Studio: ${doc.path}`} onkeydown={key}>
  <div class="sprite-toolbar" role="toolbar" aria-label="Sprite tools">
    <button class="icon-button" title="Save (Ctrl+S)" aria-label="Save" onclick={() => wb.sprites.save(doc)}><i aria-hidden="true" class="codicon codicon-save"></i></button>
    <button class="primary" title="Save, then write the PMG bank, index, include and atlas set in Outputs" onclick={() => wb.sprites.generate(doc)}><i aria-hidden="true" class="codicon codicon-run-all"></i> Generate</button>
    <span class="sep"></span>
    <button class="icon-button" title="Undo (Ctrl+Z)" aria-label="Undo" disabled={!doc.history.undo} onclick={() => doc.undo()}><i aria-hidden="true" class="codicon codicon-discard"></i></button>
    <button class="icon-button" title="Redo (Ctrl+Y)" aria-label="Redo" disabled={!doc.history.redo} onclick={() => doc.redo()}><i aria-hidden="true" class="codicon codicon-redo"></i></button>
    <span class="sep"></span>
    <div class="tool-group" role="radiogroup" aria-label="Drawing tool">
      {#each tools as item (item.id)}
        <button class="icon-button" role="radio" aria-checked={tool === item.id} title={`${item.label} (${item.key})`} aria-label={item.label} onclick={() => (tool = item.id)}><i aria-hidden="true" class={`codicon codicon-${item.icon}`}></i></button>
      {/each}
    </div>
    <span class="sep"></span>
    <button class="icon-button" title="Flip horizontally" aria-label="Flip horizontally" onclick={() => transform(flipHorizontal)}><i aria-hidden="true" class="codicon codicon-mirror"></i></button>
    <button class="icon-button" title="Flip vertically" aria-label="Flip vertically" onclick={() => transform(flipVertical)}><i aria-hidden="true" class="codicon codicon-mirror rotate"></i></button>
    <button class="icon-button" title="Shift left (Shift+Left)" aria-label="Shift left" onclick={() => transform(rows => shift(rows, -1, 0))}><i aria-hidden="true" class="codicon codicon-arrow-left"></i></button>
    <button class="icon-button" title="Shift right (Shift+Right)" aria-label="Shift right" onclick={() => transform(rows => shift(rows, 1, 0))}><i aria-hidden="true" class="codicon codicon-arrow-right"></i></button>
    <button class="icon-button" title="Shift up (Shift+Up)" aria-label="Shift up" onclick={() => transform(rows => shift(rows, 0, -1))}><i aria-hidden="true" class="codicon codicon-arrow-up"></i></button>
    <button class="icon-button" title="Shift down (Shift+Down)" aria-label="Shift down" onclick={() => transform(rows => shift(rows, 0, 1))}><i aria-hidden="true" class="codicon codicon-arrow-down"></i></button>
    <button class="icon-button" title="Clear frame" aria-label="Clear frame" onclick={() => transform(clear)}><i aria-hidden="true" class="codicon codicon-clear-all"></i></button>
    <span class="sep"></span>
    <button class="icon-button" title="Zoom out (-)" aria-label="Zoom out" onclick={() => (zoom = Math.max(2, scale - 2))}><i aria-hidden="true" class="codicon codicon-zoom-out"></i></button>
    <span class="zoom" aria-live="polite">{scale}x</span>
    <button class="icon-button" title="Zoom in (+)" aria-label="Zoom in" onclick={() => (zoom = Math.min(64, scale + 2))}><i aria-hidden="true" class="codicon codicon-zoom-in"></i></button>
    <button class="icon-button" title="Pixel grid" aria-label="Pixel grid" aria-pressed={grid} onclick={() => (grid = !grid)}><i aria-hidden="true" class="codicon codicon-table"></i></button>
    <button class="icon-button" title="Onion skin: show the previous frame faintly" aria-label="Onion skin" aria-pressed={onion} onclick={() => (onion = !onion)}><i aria-hidden="true" class="codicon codicon-layers"></i></button>
    <select class="plane-view" title="Show one player plane as the hardware sees it" aria-label="Plane view" value={plane === null ? '' : String(plane)} onchange={event => (plane = event.currentTarget.value === '' ? null : Number(event.currentTarget.value))}>
      <option value="">All planes</option>
      {#each Array.from({ length: sheet.planes }, (_, n) => n) as n}<option value={String(n)}>Player {n} bits</option>{/each}
    </select>
    <span class="spacer"></span>
    <button title="Import frames from a PNG, a PMG bank or other sprite files" onclick={() => (importOpen = true)}><i aria-hidden="true" class="codicon codicon-cloud-upload"></i> Import</button>
    <button title="Export the frames as a PNG sheet" onclick={() => wb.sprites.exportPng(doc, sheet.outputs.atlasScale ?? 4, Math.min(sheet.frames.length, 8))}><i aria-hidden="true" class="codicon codicon-export"></i> PNG</button>
    <button class="icon-button" title="Open the JSON in the text editor" aria-label="Open as JSON text" disabled={!(doc.sheetPath ?? doc.framesPath ?? doc.bank?.index)} onclick={() => wb.execute('sprites.openText')}><i aria-hidden="true" class="codicon codicon-json"></i></button>
  </div>

  {#if doc.external}
    <div class="banner" role="alert">
      <i aria-hidden="true" class="codicon codicon-warning"></i>
      {#if doc.external === 'deleted'}
        <span>A file of this sprite was deleted or moved. Your frames are kept here; save to write them back.</span>
        <button onclick={() => wb.sprites.save(doc)}>Save</button><button onclick={() => (doc.external = null)}>Dismiss</button>
      {:else}
        <span>This sprite changed on disk and you have unsaved changes.</span>
        <button onclick={() => wb.sprites.reload(doc)}>Reload From Disk</button><button onclick={() => (doc.external = null)}>Keep My Changes</button>
      {/if}
    </div>
  {/if}
  {#each doc.notes as note}
    <div class="sprite-note"><i aria-hidden="true" class="codicon codicon-info"></i><span>{note}</span>
      {#if !doc.sheetPath}<button onclick={() => wb.sprites.createSheetFile(doc)}>Create Sheet File…</button>{/if}
    </div>
  {/each}

  <div class="sprite-body">
    <aside class="sprite-frames" aria-label="Frames">
      <div class="section-header">
        <strong>Frames ({sheet.frames.length})</strong>
        <span class="section-actions">
          <button class="icon-button" title="New blank frame" aria-label="New blank frame" onclick={() => addFrame(false)}><i aria-hidden="true" class="codicon codicon-add"></i></button>
          <button class="icon-button" title="Duplicate frame (Ctrl+D)" aria-label="Duplicate frame" disabled={!frame} onclick={() => addFrame(true)}><i aria-hidden="true" class="codicon codicon-copy"></i></button>
          <button class="icon-button" title="Move frame up" aria-label="Move frame up" disabled={doc.frame === 0} onclick={() => moveFrame(-1)}><i aria-hidden="true" class="codicon codicon-arrow-up"></i></button>
          <button class="icon-button" title="Move frame down" aria-label="Move frame down" disabled={doc.frame >= sheet.frames.length - 1} onclick={() => moveFrame(1)}><i aria-hidden="true" class="codicon codicon-arrow-down"></i></button>
          <button class="icon-button" title="Delete frame" aria-label="Delete frame" disabled={sheet.frames.length <= 1} onclick={deleteFrame}><i aria-hidden="true" class="codicon codicon-trash"></i></button>
        </span>
      </div>
      <ol class="frame-list">
        {#each sheet.frames as item, index (index)}
          <li class:active={index === doc.frame}>
            <button class="frame-row" aria-current={index === doc.frame} title={`${item.name} — offset ${index * bytesPerFrame}. Double-click to rename.`} onclick={() => (doc.frame = index)} ondblclick={() => (renaming = item.name)}>
              <canvas use:frameCanvas={{ sheet, rows: item.rows, scale: Math.max(1, Math.floor(40 / Math.max(sheet.width, sheet.height))) }} aria-hidden="true"></canvas>
              <span class="frame-index">{index}</span>
              {#if renaming === item.name}
                <!-- svelte-ignore a11y_autofocus -->
                <input value={item.name} aria-label="Frame name" autofocus spellcheck="false" onclick={event => event.stopPropagation()}
                  onkeydown={event => { if (event.key === 'Enter') rename(index, event.currentTarget.value); else if (event.key === 'Escape') { event.stopPropagation(); renaming = ''; } }}
                  onblur={event => rename(index, event.currentTarget.value)} />
              {:else}
                <span class="frame-name">{item.name}</span>
              {/if}
            </button>
          </li>
        {/each}
      </ol>
    </aside>

    <div class="sprite-canvas-area">
      {#if frame}
        <div class="canvas-scroll">
          <canvas bind:this={canvas} class="pixel-canvas" class:picker={tool === 'picker'} style={`width:${sheet.width * scale}px;height:${sheet.height * scale}px`}
            aria-label={`Frame ${frame.name}, ${sheet.width} by ${sheet.height} pixels`}
            onpointerdown={pointerDown} onpointermove={pointerMove} onpointerup={pointerUp} onpointercancel={pointerUp} onpointerleave={() => (hover = null)}
            oncontextmenu={event => event.preventDefault()}></canvas>
        </div>
        <div class="canvas-status" aria-live="off">
          <span>{frame.name}</span>
          <span>{sheet.width}x{sheet.height}, {sheet.planes} plane{sheet.planes === 1 ? '' : 's'}, {bytesPerFrame} bytes/frame, offset {doc.frame * bytesPerFrame}</span>
          {#if hover}<span>x {hover.x}, y {hover.y}: "{hoverSymbol}" {hoverColor?.name ?? ''}</span>{/if}
          <span class="hint">Left draws, right erases. [ ] change frame.</span>
        </div>
      {:else}
        <div class="empty-state">This sheet has no frames. <button onclick={() => addFrame(false)}>Add Frame</button> or <button onclick={() => (importOpen = true)}>Import…</button></div>
      {/if}
    </div>

    <SpriteInspector {doc} bind:symbol />
  </div>

  {#if importOpen}
    <SpriteImport {doc} onclose={() => { importOpen = false; doc.pendingImage = null; root?.focus(); }} />
  {/if}
</section>

<style>
  .sprite-studio { display: flex; flex-direction: column; height: 100%; min-height: 0; outline: none; position: relative; background: var(--editor-bg); }
  .sprite-toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 2px; padding: 4px 8px; border-bottom: 1px solid var(--border); flex: none; }
  .sprite-toolbar .sep { width: 1px; height: 18px; background: var(--border); margin: 0 4px; }
  .sprite-toolbar .spacer { flex: 1; }
  .sprite-toolbar .zoom { min-width: 30px; text-align: center; color: var(--muted); font-size: 12px; }
  .sprite-toolbar .plane-view { width: auto; margin-left: 4px; }
  .tool-group { display: inline-flex; gap: 1px; }
  .tool-group [aria-checked="true"] { background: var(--list-active); border-color: var(--accent); }
  .rotate { transform: rotate(90deg); }
  .sprite-note { display: flex; align-items: center; gap: 8px; padding: 5px 12px; border-bottom: 1px solid var(--border); color: var(--muted); flex: none; }
  .sprite-note span { flex: 1; }
  .sprite-body { flex: 1; min-height: 0; display: grid; grid-template-columns: 190px minmax(0, 1fr) 290px; }
  .sprite-frames { border-right: 1px solid var(--border); display: flex; flex-direction: column; min-height: 0; background: var(--sidebar-bg); }
  .frame-list { list-style: none; margin: 0; padding: 0 0 8px; overflow: auto; flex: 1; }
  .frame-row { width: 100%; display: flex; align-items: center; justify-content: flex-start; gap: 8px; padding: 3px 8px; border: none; border-radius: 0; background: none; text-align: left; }
  .frame-list li.active .frame-row { background: var(--list-active); }
  .frame-row canvas { image-rendering: pixelated; flex: none; border: 1px solid var(--border); }
  .frame-index { color: var(--muted); font-family: var(--mono); font-size: 11px; min-width: 18px; text-align: right; }
  .frame-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .frame-row input { padding: 1px 4px; }
  .sprite-canvas-area { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  .canvas-scroll { flex: 1; overflow: auto; display: grid; place-items: center; padding: 16px; }
  .pixel-canvas { image-rendering: pixelated; cursor: crosshair; touch-action: none; box-shadow: 0 0 0 1px var(--border); }
  .pixel-canvas.picker { cursor: copy; }
  .canvas-status { display: flex; flex-wrap: wrap; gap: 4px 16px; padding: 4px 12px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; flex: none; }
  .canvas-status .hint { margin-left: auto; }
  @media (max-width: 900px) { .sprite-body { grid-template-columns: 150px minmax(0, 1fr); } .sprite-body :global(.sprite-inspector) { grid-column: 1 / -1; max-height: 40vh; border-left: none; border-top: 1px solid var(--border); } }
</style>
