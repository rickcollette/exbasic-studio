// Merging sheets into one bank. Sizes are unified (frames grow to the largest
// width and height, never shrink), palettes are combined so every pixel keeps
// its planes and colour, and clashing frame names get the sheet's name.
import { TRANSPARENT, planesMask, resizeRows, transparentColor, unusedSymbol, type SpriteColor, type SpriteFrame, type SpriteSheet } from './model';

export interface MergeSource { label: string; sheet: SpriteSheet }
export interface MergeOptions {
  /** Where shorter frames sit when padded to the tallest: top keeps feet aligned to row 0. */
  anchor?: 'top' | 'bottom';
  /** Prefix every frame name with its sheet label, not only clashing ones. */
  prefixAll?: boolean;
}
export interface MergeResult { sheet: SpriteSheet; notes: string[] }

function slug(label: string): string {
  return label.split('/').pop()!.replace(/\.sprites\.json$|\.[^.]*$/i, '').replace(/[^A-Za-z0-9]+/g, '_').replace(/^_+|_+$/g, '').toLowerCase() || 'sheet';
}

export function mergeSheets(sources: MergeSource[], options: MergeOptions = {}): MergeResult {
  if (!sources.length) throw new Error('choose at least one sheet to merge');
  const notes: string[] = [];
  const width = Math.max(...sources.map(source => source.sheet.width));
  const height = Math.max(...sources.map(source => source.sheet.height));
  const planes = Math.max(...sources.map(source => source.sheet.planes));
  const palette: SpriteColor[] = [transparentColor()];
  const frames: SpriteFrame[] = [];
  const names = new Set<string>();

  for (const { label, sheet } of sources) {
    if (sheet.width !== width || sheet.height !== height) notes.push(`${label}: ${sheet.width}x${sheet.height} frames padded to ${width}x${height}.`);
    // Map every symbol of this sheet to a symbol of the merged palette.
    const remap = new Map<string, string>([[TRANSPARENT, TRANSPARENT]]);
    for (const color of sheet.palette) {
      if (color.symbol === TRANSPARENT) continue;
      const mask = planesMask(color.planes);
      const same = palette.find(other => other.symbol === color.symbol && planesMask(other.planes) === mask);
      if (same) { remap.set(color.symbol, same.symbol); continue; }
      const twin = palette.find(other => other.symbol !== TRANSPARENT && planesMask(other.planes) === mask && other.color.toLowerCase() === color.color.toLowerCase());
      if (twin) { remap.set(color.symbol, twin.symbol); continue; }
      const symbol = palette.some(other => other.symbol === color.symbol) ? unusedSymbol(palette) : color.symbol;
      if (symbol !== color.symbol) notes.push(`${label}: symbol "${color.symbol}" (${color.name || 'unnamed'}) renamed to "${symbol}" because the merged palette already uses "${color.symbol}" differently.`);
      palette.push({ ...color, symbol, planes: [...color.planes] });
      remap.set(color.symbol, symbol);
    }
    const prefix = slug(label);
    for (const frame of sheet.frames) {
      let name = options.prefixAll ? `${prefix}_${frame.name}` : frame.name;
      if (names.has(name)) {
        const renamed = options.prefixAll ? name : `${prefix}_${frame.name}`;
        let candidate = renamed;
        for (let n = 2; names.has(candidate); n++) candidate = `${renamed}_${n}`;
        notes.push(`${label}: frame "${frame.name}" renamed to "${candidate}" to keep names unique.`);
        name = candidate;
      }
      names.add(name);
      const rows = frame.rows.map(row => [...row].map(symbol => remap.get(symbol) ?? symbol).join(''));
      frames.push({ name, rows: resizeRows(rows, width, height, options.anchor ?? 'top') });
    }
  }
  return { sheet: { width, height, planes, palette, frames, outputs: {} }, notes };
}
