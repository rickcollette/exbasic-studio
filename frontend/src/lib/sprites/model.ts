// Sprite sheets: frames of symbol rows, a palette that maps each symbol to a
// preview colour and to the Atari player planes it sets, and the files a
// sheet generates. Everything here is pure data so it can be tested without
// a browser and used by the store, the editor and the merge tool alike.

/** The symbol of an empty pixel. It never sets a plane. */
export const TRANSPARENT = '.';

export const MAX_WIDTH = 32;
export const MAX_HEIGHT = 256;
/** GTIA has four players; a sprite overlays up to four of them. */
export const MAX_PLANES = 4;
export const MAX_FRAMES = 1024;

export interface SpriteColor {
  /** One printable character used in the frame rows. */
  symbol: string;
  name: string;
  /** Preview colour, #rrggbb. */
  color: string;
  /** Player planes (0-based) this symbol sets. Empty means transparent. */
  planes: number[];
}

export interface SpriteFrame { name: string; rows: string[] }

/**
 * Index layout of generated banks. "planes" lists every plane's offset
 * (offset, player0_offset, player1_offset, bytes_per_plane, frame_length,
 * height, width); "simple" is (offset, length, width, height).
 */
export type IndexStyle = 'planes' | 'simple';

/** Generated files, relative to the folder of the sheet. */
export interface SheetOutputs {
  bank?: string;
  index?: string;
  indexStyle?: IndexStyle;
  /** A PNG strip of every frame at 1:1, for tools and other engines. */
  atlas?: string;
  atlasScale?: number;
  atlasColumns?: number;
  /** An exBASIC source file with the bank and its frame offsets as DATA. */
  include?: string;
  /** Base name of the DATA arrays in the include file. */
  dataName?: string;
}

export interface SpriteSheet {
  width: number;
  height: number;
  planes: number;
  palette: SpriteColor[];
  frames: SpriteFrame[];
  outputs: SheetOutputs;
}

export const SHEET_FORMAT = 'exbasic-sprites';
export const SHEET_EXTENSION = '.sprites.json';

/** Preview colours handed out to new symbols. */
export const DEFAULT_COLORS = ['#c1802c', '#237eae', '#eeac70', '#d0d6e0', '#5b9c65', '#ed5b37', '#a77bd2', '#f4b846', '#52b6cf', '#8b6743', '#e2d8b7', '#795b64', '#7ccdc3', '#ffdf69', '#9a93b4'];
/** Symbols handed out to new colours, in order. */
export const SYMBOL_POOL = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789#@*+=%&$!?~^<>[]{}()|/:;,_-';

export function isSheetPath(path: string): boolean { return path.toLowerCase().endsWith(SHEET_EXTENSION); }
export function isPmgPath(path: string): boolean { return /\.pmg$/i.test(path); }
export function isPngPath(path: string): boolean { return /\.png$/i.test(path); }
/** Files the Sprite Studio opens by default. */
export function isSpritePath(path: string): boolean { return isSheetPath(path) || isPmgPath(path) || isPngPath(path); }

export function transparentColor(): SpriteColor {
  return { symbol: TRANSPARENT, name: 'transparent', color: '#0a0f1a', planes: [] };
}

export function blankRows(width: number, height: number): string[] {
  return Array.from({ length: height }, () => TRANSPARENT.repeat(width));
}

/** A new sheet with one blank frame and one colour per plane. */
export function newSheet(width = 8, height = 16, planes = 1): SpriteSheet {
  const palette = [transparentColor()];
  for (let mask = 1; mask < 1 << planes; mask++) {
    palette.push({ symbol: SYMBOL_POOL[mask - 1], name: planeLabel(maskPlanes(mask)), color: DEFAULT_COLORS[(mask - 1) % DEFAULT_COLORS.length], planes: maskPlanes(mask) });
  }
  return { width, height, planes, palette, frames: [{ name: 'frame_0', rows: blankRows(width, height) }], outputs: {} };
}

export function maskPlanes(mask: number): number[] {
  const planes: number[] = [];
  for (let plane = 0; plane < MAX_PLANES; plane++) if (mask & (1 << plane)) planes.push(plane);
  return planes;
}

export function planesMask(planes: number[]): number {
  return planes.reduce((mask, plane) => mask | (1 << plane), 0);
}

export function planeLabel(planes: number[]): string {
  return planes.length ? planes.map(plane => `P${plane}`).join('+') : 'none';
}

export function cloneSheet(sheet: SpriteSheet): SpriteSheet {
  return {
    width: sheet.width, height: sheet.height, planes: sheet.planes,
    palette: sheet.palette.map(color => ({ ...color, planes: [...color.planes] })),
    frames: sheet.frames.map(frame => ({ name: frame.name, rows: [...frame.rows] })),
    outputs: { ...sheet.outputs },
  };
}

/** A frame name that is not used yet, based on `base`. */
export function uniqueFrameName(sheet: SpriteSheet, base: string): string {
  const names = new Set(sheet.frames.map(frame => frame.name));
  const clean = base.trim() || 'frame';
  if (!names.has(clean)) return clean;
  const stem = clean.replace(/_\d+$/, '');
  for (let n = 1; ; n++) if (!names.has(`${stem}_${n}`)) return `${stem}_${n}`;
}

export function unusedSymbol(palette: SpriteColor[]): string {
  const used = new Set(palette.map(color => color.symbol));
  const symbol = [...SYMBOL_POOL].find(candidate => !used.has(candidate));
  if (!symbol) throw new Error('every palette symbol is in use');
  return symbol;
}

// ---- Files ------------------------------------------------------------------

export interface SheetFile {
  format: typeof SHEET_FORMAT;
  version: 1;
  width: number;
  height: number;
  planes: number;
  palette: SpriteColor[];
  /** Plain frames JSON holding the frames, relative to the sheet. */
  source?: string;
  frames?: Record<string, string[]>;
  outputs: SheetOutputs;
}

export type ParsedSprites =
  | { kind: 'sheet'; sheet: SpriteSheet; source: string | null }
  | { kind: 'frames'; frames: SpriteFrame[] };

function isRows(value: unknown): value is string[] {
  return Array.isArray(value) && value.length > 0 && value.every(row => typeof row === 'string');
}

/** Reads frames stored as { "name": ["row", ...], ... }. */
export function framesFromObject(value: unknown): SpriteFrame[] | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const entries = Object.entries(value as Record<string, unknown>);
  if (!entries.length || !entries.every(([, rows]) => isRows(rows))) return null;
  return entries.map(([name, rows]) => ({ name, rows: [...(rows as string[])] }));
}

export function framesToObject(frames: SpriteFrame[]): Record<string, string[]> {
  return Object.fromEntries(frames.map(frame => [frame.name, [...frame.rows]]));
}

/** Parses a sprite sheet file or a plain frames file. */
export function parseSpriteJson(text: string): ParsedSprites {
  let data: unknown;
  try { data = JSON.parse(text); } catch (error) { throw new Error(`not valid JSON: ${(error as Error).message}`); }
  if (data && typeof data === 'object' && (data as { format?: unknown }).format === SHEET_FORMAT) {
    const file = data as Partial<SheetFile>;
    if (file.version !== 1) throw new Error(`unsupported sprite sheet version ${String(file.version)}`);
    const palette = Array.isArray(file.palette) ? file.palette.map(color => ({
      symbol: String(color.symbol ?? ''), name: String(color.name ?? ''), color: String(color.color ?? '#ffffff'),
      planes: Array.isArray(color.planes) ? color.planes.map(Number) : [],
    })) : [];
    if (!palette.some(color => color.symbol === TRANSPARENT)) palette.unshift(transparentColor());
    const source = typeof file.source === 'string' && file.source.trim() ? file.source.trim() : null;
    let frames: SpriteFrame[] = [];
    if (!source) {
      if (file.frames !== undefined && Object.keys(file.frames ?? {}).length) {
        const parsed = framesFromObject(file.frames);
        if (!parsed) throw new Error('"frames" must map each frame name to a list of row strings');
        frames = parsed;
      }
    }
    const sheet: SpriteSheet = {
      width: Number(file.width) || 8, height: Number(file.height) || 16, planes: Number(file.planes) || 1,
      palette, frames, outputs: { ...(file.outputs ?? {}) },
    };
    return { kind: 'sheet', sheet, source };
  }
  const frames = framesFromObject(data);
  if (!frames) throw new Error(`not a sprite file: expected a "${SHEET_FORMAT}" sheet or an object mapping frame names to lists of row strings`);
  return { kind: 'frames', frames };
}

/** Serialises a sheet; with `source`, the frames are left to that file. */
export function serializeSheet(sheet: SpriteSheet, source: string | null): string {
  const outputs: SheetOutputs = {};
  for (const [key, value] of Object.entries(sheet.outputs) as [keyof SheetOutputs, unknown][]) {
    if (value !== undefined && value !== '' && value !== null) (outputs as Record<string, unknown>)[key] = value;
  }
  const file: SheetFile = {
    format: SHEET_FORMAT, version: 1, width: sheet.width, height: sheet.height, planes: sheet.planes,
    palette: sheet.palette.map(color => ({ symbol: color.symbol, name: color.name, color: color.color, planes: [...color.planes].sort((a, b) => a - b) })),
    ...(source ? { source } : { frames: framesToObject(sheet.frames) }),
    outputs,
  };
  return JSON.stringify(file, null, 2) + '\n';
}

export function serializeFrames(frames: SpriteFrame[]): string {
  return JSON.stringify(framesToObject(frames), null, 2) + '\n';
}

/**
 * A sheet for frames that came without settings. Width and height come from
 * the frames, each symbol gets a colour, and symbols are given distinct
 * plane combinations so that every colour survives on hardware.
 */
export function inferSheet(frames: SpriteFrame[]): SpriteSheet {
  const height = Math.max(1, ...frames.map(frame => frame.rows.length));
  const width = Math.max(1, ...frames.flatMap(frame => frame.rows.map(row => [...row].length)));
  const symbols: string[] = [];
  for (const frame of frames) for (const row of frame.rows) for (const symbol of row) if (symbol !== TRANSPARENT && !symbols.includes(symbol)) symbols.push(symbol);
  let planes = 1;
  while ((1 << planes) - 1 < symbols.length && planes < MAX_PLANES) planes++;
  const palette = [transparentColor(), ...symbols.map((symbol, index) => ({
    symbol, name: symbol, color: DEFAULT_COLORS[index % DEFAULT_COLORS.length],
    planes: maskPlanes(Math.min(index + 1, (1 << planes) - 1)),
  }))];
  return { width, height, planes, palette, frames: normalizeFrames(frames, width, height), outputs: {} };
}

/** Pads or crops every frame to the sheet size. */
export function normalizeFrames(frames: SpriteFrame[], width: number, height: number): SpriteFrame[] {
  return frames.map(frame => ({ name: frame.name, rows: resizeRows(frame.rows, width, height) }));
}

export function resizeRows(rows: string[], width: number, height: number, anchor: 'top' | 'bottom' = 'top'): string[] {
  const fitted = rows.map(row => {
    const pixels = [...row];
    return pixels.length >= width ? pixels.slice(0, width).join('') : pixels.join('') + TRANSPARENT.repeat(width - pixels.length);
  });
  const blank = TRANSPARENT.repeat(width);
  if (fitted.length >= height) return anchor === 'bottom' ? fitted.slice(fitted.length - height) : fitted.slice(0, height);
  const padding = Array.from({ length: height - fitted.length }, () => blank);
  return anchor === 'bottom' ? [...padding, ...fitted] : [...fitted, ...padding];
}

// ---- Validation -------------------------------------------------------------

export interface SheetIssue { severity: 'error' | 'warning'; message: string; frame?: string }

export function validateSheet(sheet: SpriteSheet): SheetIssue[] {
  const issues: SheetIssue[] = [];
  const error = (message: string, frame?: string) => issues.push({ severity: 'error', message, frame });
  const warn = (message: string, frame?: string) => issues.push({ severity: 'warning', message, frame });
  if (!Number.isInteger(sheet.width) || sheet.width < 1 || sheet.width > MAX_WIDTH) error(`Width must be 1 to ${MAX_WIDTH} pixels.`);
  if (!Number.isInteger(sheet.height) || sheet.height < 1 || sheet.height > MAX_HEIGHT) error(`Height must be 1 to ${MAX_HEIGHT} rows.`);
  if (!Number.isInteger(sheet.planes) || sheet.planes < 1 || sheet.planes > MAX_PLANES) error(`Planes must be 1 to ${MAX_PLANES}.`);
  if (sheet.width > 8) warn(`A player is 8 pixels wide; ${sheet.width}-pixel frames need ${Math.ceil(sheet.width / 8)} bytes per row and your program must place them side by side.`);

  const symbols = new Set<string>();
  const byMask = new Map<number, SpriteColor>();
  for (const color of sheet.palette) {
    if ([...color.symbol].length !== 1 || /\s/.test(color.symbol)) error(`Palette symbol "${color.symbol}" must be a single visible character.`);
    if (symbols.has(color.symbol)) error(`Palette symbol "${color.symbol}" is used twice.`);
    symbols.add(color.symbol);
    if (!/^#[0-9a-f]{6}$/i.test(color.color)) error(`Colour of "${color.symbol}" must be #rrggbb.`);
    if (color.planes.some(plane => !Number.isInteger(plane) || plane < 0 || plane >= sheet.planes)) error(`"${color.symbol}" uses a plane the sheet does not have (planes: ${sheet.planes}).`);
    if (color.symbol === TRANSPARENT) { if (color.planes.length) error('The transparent symbol "." cannot set planes.'); continue; }
    if (!color.planes.length) warn(`"${color.symbol}" (${color.name || 'unnamed'}) sets no plane, so it is invisible on the Atari.`);
    const mask = planesMask(color.planes);
    const twin = byMask.get(mask);
    if (twin && twin.color.toLowerCase() !== color.color.toLowerCase()) warn(`"${twin.symbol}" and "${color.symbol}" both set ${planeLabel(color.planes)}, so they show the same colour on the Atari.`);
    if (!twin) byMask.set(mask, color);
  }
  if (!symbols.has(TRANSPARENT)) error('The palette needs the transparent symbol ".".');

  if (!sheet.frames.length) warn('The sheet has no frames.');
  if (sheet.frames.length > MAX_FRAMES) error(`At most ${MAX_FRAMES} frames are supported.`);
  const names = new Set<string>();
  for (const frame of sheet.frames) {
    if (!frame.name.trim()) error('A frame has no name.');
    if (names.has(frame.name)) error(`Frame name "${frame.name}" is used twice.`, frame.name);
    names.add(frame.name);
    if (frame.rows.length !== sheet.height) error(`Frame "${frame.name}" has ${frame.rows.length} rows, expected ${sheet.height}.`, frame.name);
    const unknown = new Set<string>();
    frame.rows.forEach((row, index) => {
      if ([...row].length !== sheet.width) error(`Frame "${frame.name}" row ${index} has ${[...row].length} pixels, expected ${sheet.width}.`, frame.name);
      for (const symbol of row) if (!symbols.has(symbol)) unknown.add(symbol);
    });
    if (unknown.size) error(`Frame "${frame.name}" uses symbols missing from the palette: ${[...unknown].map(symbol => `"${symbol}"`).join(', ')}.`, frame.name);
  }
  return issues;
}

/** Identifier for exBASIC DATA arrays derived from a file name. */
export function dataNameFor(path: string): string {
  const base = path.split('/').pop()!.replace(/\.sprites\.json$|\.[^.]*$/i, '');
  const words = base.split(/[^A-Za-z0-9]+/).filter(Boolean);
  const name = words.map(word => word[0].toUpperCase() + word.slice(1)).join('') || 'Sprites';
  return /^[A-Za-z]/.test(name) ? name : `S${name}`;
}
