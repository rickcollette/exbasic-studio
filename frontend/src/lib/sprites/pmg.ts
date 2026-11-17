// Player/missile frame banks. A bank holds the frames one after another; a
// frame holds its planes one after another (player 0 first), and a plane
// holds `height` rows of `ceil(width / 8)` bytes, leftmost pixel in bit 7,
// so a program copies a plane straight into PMG memory.
import { TRANSPARENT, maskPlanes, planesMask, transparentColor, DEFAULT_COLORS, SYMBOL_POOL, planeLabel, type IndexStyle, type SpriteColor, type SpriteFrame, type SpriteSheet } from './model';

export interface BankLayout { width: number; height: number; planes: number }

export function bytesPerRow(layout: BankLayout): number { return Math.ceil(layout.width / 8); }
export function planeBytes(layout: BankLayout): number { return layout.height * bytesPerRow(layout); }
export function frameBytes(layout: BankLayout): number { return planeBytes(layout) * layout.planes; }

/** Plane bit mask of every palette symbol. */
export function symbolMasks(palette: SpriteColor[]): Map<string, number> {
  return new Map(palette.map(color => [color.symbol, color.symbol === TRANSPARENT ? 0 : planesMask(color.planes)]));
}

export function encodeFrame(sheet: SpriteSheet, frame: SpriteFrame, masks = symbolMasks(sheet.palette)): Uint8Array {
  const stride = bytesPerRow(sheet);
  const out = new Uint8Array(frameBytes(sheet));
  for (let plane = 0; plane < sheet.planes; plane++) {
    const base = plane * planeBytes(sheet);
    for (let y = 0; y < sheet.height; y++) {
      const pixels = [...(frame.rows[y] ?? '')];
      for (let x = 0; x < sheet.width; x++) {
        const mask = masks.get(pixels[x] ?? TRANSPARENT) ?? 0;
        if (mask & (1 << plane)) out[base + y * stride + (x >> 3)] |= 0x80 >> (x & 7);
      }
    }
  }
  return out;
}

export function encodeBank(sheet: SpriteSheet): Uint8Array {
  const masks = symbolMasks(sheet.palette);
  const size = frameBytes(sheet);
  const out = new Uint8Array(size * sheet.frames.length);
  sheet.frames.forEach((frame, index) => out.set(encodeFrame(sheet, frame, masks), index * size));
  return out;
}

/** The index object, with a fixed key order so regenerating it is stable. */
export function buildIndex(sheet: SpriteSheet, style: IndexStyle): Record<string, Record<string, number>> {
  const plane = planeBytes(sheet);
  const size = frameBytes(sheet);
  const index: Record<string, Record<string, number>> = {};
  sheet.frames.forEach((frame, n) => {
    const offset = n * size;
    if (style === 'simple') {
      index[frame.name] = { offset, length: size, width: sheet.width, height: sheet.height };
      return;
    }
    const entry: Record<string, number> = { offset };
    for (let p = 0; p < sheet.planes; p++) entry[`player${p}_offset`] = offset + p * plane;
    Object.assign(entry, { bytes_per_plane: plane, frame_length: size, height: sheet.height, width: sheet.width });
    index[frame.name] = entry;
  });
  return index;
}

export function indexText(sheet: SpriteSheet, style: IndexStyle): string {
  return JSON.stringify(buildIndex(sheet, style), null, 2) + '\n';
}

// ---- Reading banks -----------------------------------------------------------

export interface IndexEntry { name: string; offset: number; planeOffsets: number[]; width: number; height: number }
export interface ParsedIndex { entries: IndexEntry[]; layout: BankLayout; style: IndexStyle }

/** Reads an index in either layout. Returns null for other JSON. */
export function parseIndex(text: string): ParsedIndex | null {
  let data: unknown;
  try { data = JSON.parse(text); } catch { return null; }
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null;
  const entries: IndexEntry[] = [];
  let style: IndexStyle = 'simple';
  for (const [name, raw] of Object.entries(data as Record<string, unknown>)) {
    if (!raw || typeof raw !== 'object') return null;
    const item = raw as Record<string, unknown>;
    const offset = Number(item.offset);
    const height = Number(item.height);
    const width = Number(item.width ?? 8);
    if (!Number.isInteger(offset) || !Number.isInteger(height) || height < 1 || !Number.isInteger(width) || width < 1) return null;
    const planeOffsets: number[] = [];
    for (let p = 0; `player${p}_offset` in item; p++) planeOffsets.push(Number(item[`player${p}_offset`]));
    if (planeOffsets.length) style = 'planes';
    else planeOffsets.push(offset);
    entries.push({ name, offset, planeOffsets, width, height });
  }
  if (!entries.length) return null;
  return { entries, style, layout: { width: Math.max(...entries.map(entry => entry.width)), height: Math.max(...entries.map(entry => entry.height)), planes: Math.max(...entries.map(entry => entry.planeOffsets.length)) } };
}

/** Bytes an index needs from its bank. */
export function indexExtent(index: ParsedIndex): number {
  return Math.max(...index.entries.map(entry => Math.max(...entry.planeOffsets) + entry.height * Math.ceil(entry.width / 8)));
}

/** A palette with one symbol for every plane combination. */
export function planePalette(planes: number): SpriteColor[] {
  const palette = [transparentColor()];
  if (planes === 1) { palette.push({ symbol: '#', name: 'P0', color: '#d0d6e0', planes: [0] }); return palette; }
  for (let mask = 1; mask < 1 << planes; mask++) palette.push({ symbol: SYMBOL_POOL[mask - 1], name: planeLabel(maskPlanes(mask)), color: DEFAULT_COLORS[(mask - 1) % DEFAULT_COLORS.length], planes: maskPlanes(mask) });
  return palette;
}

/**
 * Turns bank bytes back into frames. Each pixel becomes the palette symbol
 * whose planes match exactly; plane combinations the palette lacks get a
 * new symbol, which is added to the returned palette.
 */
export function decodeBank(bytes: Uint8Array, index: IndexEntry[], layout: BankLayout, palette: SpriteColor[] = planePalette(layout.planes)): { frames: SpriteFrame[]; palette: SpriteColor[] } {
  const out = palette.map(color => ({ ...color, planes: [...color.planes] }));
  const byMask = new Map<number, string>();
  for (const color of out) if (!byMask.has(color.symbol === TRANSPARENT ? 0 : planesMask(color.planes))) byMask.set(color.symbol === TRANSPARENT ? 0 : planesMask(color.planes), color.symbol);
  if (!byMask.has(0)) byMask.set(0, TRANSPARENT);
  const symbolFor = (mask: number) => {
    let symbol = byMask.get(mask);
    if (symbol === undefined) {
      const used = new Set(out.map(color => color.symbol));
      symbol = [...SYMBOL_POOL].find(candidate => !used.has(candidate))!;
      out.push({ symbol, name: planeLabel(maskPlanes(mask)), color: DEFAULT_COLORS[out.length % DEFAULT_COLORS.length], planes: maskPlanes(mask) });
      byMask.set(mask, symbol);
    }
    return symbol;
  };
  const frames = index.map(entry => {
    const stride = Math.ceil(entry.width / 8);
    const rows: string[] = [];
    for (let y = 0; y < layout.height; y++) {
      let row = '';
      for (let x = 0; x < layout.width; x++) {
        let mask = 0;
        if (y < entry.height && x < entry.width) {
          entry.planeOffsets.forEach((offset, plane) => {
            const value = bytes[offset + y * stride + (x >> 3)] ?? 0;
            if (value & (0x80 >> (x & 7))) mask |= 1 << plane;
          });
        }
        row += symbolFor(mask);
      }
      rows.push(row);
    }
    return { name: entry.name, rows };
  });
  return { frames, palette: out };
}

/** Index entries for a bank that has no index file: equal-sized frames. */
export function uniformIndex(bankSize: number, layout: BankLayout, prefix = 'frame'): IndexEntry[] {
  const size = frameBytes(layout);
  const count = Math.floor(bankSize / size);
  return Array.from({ length: count }, (_, n) => ({
    name: `${prefix}_${n}`, offset: n * size, width: layout.width, height: layout.height,
    planeOffsets: Array.from({ length: layout.planes }, (_, p) => n * size + p * planeBytes(layout)),
  }));
}

// ---- exBASIC include -----------------------------------------------------------

const PER_LINE = 16;

function dataLines(name: string, type: 'BYTE' | 'WORD', values: string[]): string[] {
  if (!values.length) values = ['0'];
  const lines: string[] = [];
  const head = `DATA ${name}() ${type} = `;
  const cont = `DATA ${' '.repeat(name.length + 2)} ${type} = `;
  for (let i = 0; i < values.length; i += PER_LINE) {
    const chunk = values.slice(i, i + PER_LINE).join(', ');
    const last = i + PER_LINE >= values.length;
    lines.push(`${i === 0 ? head : cont}${chunk}${last ? '' : ','}`);
  }
  return lines;
}

const hex = (value: number) => '$' + value.toString(16).toUpperCase().padStart(2, '0');

/**
 * An exBASIC source with the bank as BYTE DATA and the offset of every
 * frame (and of every plane) as WORD DATA, for programs that INCLUDE it
 * instead of loading the bank with BYTE FILE.
 */
export function includeText(sheet: SpriteSheet, name: string, sourceName: string): string {
  const bank = encodeBank(sheet);
  const size = frameBytes(sheet);
  const lines = [
    `' ${name}: ${sheet.frames.length} frame(s), ${sheet.width}x${sheet.height}, ${sheet.planes} plane(s), ${size} bytes per frame.`,
    `' Generated by exBASIC Studio from ${sourceName}. Edit the sprite sheet, not this file.`,
    `' Frame n starts at ${name}Frames(n); plane p of a frame starts ${planeBytes(sheet)} * p bytes later.`,
    ...sheet.frames.map((frame, n) => `'   ${String(n).padStart(3)}  ${frame.name}  offset ${n * size}`),
    ...dataLines(`${name}Frames`, 'WORD', sheet.frames.map((_, n) => String(n * size))),
    ...dataLines(name, 'BYTE', Array.from(bank, hex)),
  ];
  return lines.join('\n') + '\n';
}
