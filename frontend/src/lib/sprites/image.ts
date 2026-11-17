// Pixels in and out of RGBA buffers: frame previews, PNG sheets, and
// importing artwork by mapping each pixel to the nearest palette colour.
import { TRANSPARENT, SYMBOL_POOL, maskPlanes, planeLabel, transparentColor, type SpriteColor, type SpriteFrame, type SpriteSheet } from './model';

export interface RGBAImage { width: number; height: number; data: Uint8ClampedArray }

export function parseHex(color: string): [number, number, number] {
  const match = /^#?([0-9a-f]{6})$/i.exec(color.trim());
  const value = match ? parseInt(match[1], 16) : 0xffffff;
  return [(value >> 16) & 255, (value >> 8) & 255, value & 255];
}

export function toHex(r: number, g: number, b: number): string {
  return '#' + [r, g, b].map(part => part.toString(16).padStart(2, '0')).join('');
}

export function colorMap(palette: SpriteColor[]): Map<string, [number, number, number]> {
  return new Map(palette.map(color => [color.symbol, parseHex(color.color)]));
}

/**
 * Draws frames into one RGBA image, `columns` per row, each pixel `scale`
 * pixels square. Transparent pixels stay fully transparent.
 */
export function renderSheet(sheet: SpriteSheet, options: { columns?: number; scale?: number; frames?: SpriteFrame[] } = {}): RGBAImage {
  const frames = options.frames ?? sheet.frames;
  const scale = Math.max(1, Math.floor(options.scale ?? 1));
  const columns = Math.max(1, Math.min(options.columns ?? frames.length, frames.length || 1));
  const rows = Math.max(1, Math.ceil(frames.length / columns));
  const width = columns * sheet.width * scale;
  const height = rows * sheet.height * scale;
  const data = new Uint8ClampedArray(width * height * 4);
  const colors = colorMap(sheet.palette);
  frames.forEach((frame, n) => {
    const ox = (n % columns) * sheet.width * scale;
    const oy = Math.floor(n / columns) * sheet.height * scale;
    frame.rows.forEach((row, y) => {
      [...row].forEach((symbol, x) => {
        if (symbol === TRANSPARENT || x >= sheet.width || y >= sheet.height) return;
        const [r, g, b] = colors.get(symbol) ?? [255, 0, 255];
        for (let dy = 0; dy < scale; dy++) {
          let at = ((oy + y * scale + dy) * width + ox + x * scale) * 4;
          for (let dx = 0; dx < scale; dx++, at += 4) { data[at] = r; data[at + 1] = g; data[at + 2] = b; data[at + 3] = 255; }
        }
      });
    });
  });
  return { width, height, data };
}

export interface CellGrid { cellWidth: number; cellHeight: number; columns: number; rows: number; offsetX?: number; offsetY?: number; gapX?: number; gapY?: number }

/** The grid that tiles an image with cells of the given size. */
export function gridFor(image: { width: number; height: number }, cellWidth: number, cellHeight: number): CellGrid {
  return { cellWidth, cellHeight, columns: Math.max(1, Math.floor(image.width / cellWidth)), rows: Math.max(1, Math.floor(image.height / cellHeight)) };
}

function pixel(image: RGBAImage, x: number, y: number): [number, number, number, number] {
  if (x < 0 || y < 0 || x >= image.width || y >= image.height) return [0, 0, 0, 0];
  const at = (y * image.width + x) * 4;
  return [image.data[at], image.data[at + 1], image.data[at + 2], image.data[at + 3]];
}

/**
 * The distinct opaque colours of an image, most frequent first, as a palette
 * whose symbols take distinct plane combinations (so a 3-colour sprite uses
 * P0, P1 and P0+P1). Returns at most 15 colours, the most a 4-plane sprite
 * can show; rarer colours map to their nearest neighbour.
 */
export function paletteFromImage(image: RGBAImage, limit = 15): { palette: SpriteColor[]; planes: number } {
  const counts = new Map<string, number>();
  for (let at = 0; at < image.data.length; at += 4) {
    if (image.data[at + 3] < 128) continue;
    const key = toHex(image.data[at], image.data[at + 1], image.data[at + 2]);
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }
  const colors = [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, Math.max(1, Math.min(15, limit))).map(([color]) => color);
  let planes = 1;
  while ((1 << planes) - 1 < colors.length && planes < 4) planes++;
  const palette = [transparentColor(), ...colors.map((color, n) => ({ symbol: SYMBOL_POOL[n], name: planeLabel(maskPlanes(n + 1)), color, planes: maskPlanes(n + 1) }))];
  return { palette, planes };
}

/** Maps every pixel of every cell to the nearest palette colour. */
export function framesFromImage(image: RGBAImage, grid: CellGrid, palette: SpriteColor[], options: { prefix?: string; skipEmpty?: boolean } = {}): SpriteFrame[] {
  const opaque = palette.filter(color => color.symbol !== TRANSPARENT).map(color => ({ symbol: color.symbol, rgb: parseHex(color.color) }));
  const cache = new Map<number, string>();
  const nearest = (r: number, g: number, b: number) => {
    const key = (r << 16) | (g << 8) | b;
    const hit = cache.get(key);
    if (hit !== undefined) return hit;
    let best = opaque[0]?.symbol ?? TRANSPARENT;
    let bestDistance = Infinity;
    for (const color of opaque) {
      // Weighted RGB distance: close to perceived difference, cheap to compute.
      const dr = r - color.rgb[0], dg = g - color.rgb[1], db = b - color.rgb[2];
      const distance = 2 * dr * dr + 4 * dg * dg + 3 * db * db;
      if (distance < bestDistance) { bestDistance = distance; best = color.symbol; }
    }
    cache.set(key, best);
    return best;
  };
  const frames: SpriteFrame[] = [];
  const prefix = options.prefix ?? 'frame';
  for (let cy = 0; cy < grid.rows; cy++) {
    for (let cx = 0; cx < grid.columns; cx++) {
      const x0 = (grid.offsetX ?? 0) + cx * (grid.cellWidth + (grid.gapX ?? 0));
      const y0 = (grid.offsetY ?? 0) + cy * (grid.cellHeight + (grid.gapY ?? 0));
      let empty = true;
      const rows: string[] = [];
      for (let y = 0; y < grid.cellHeight; y++) {
        let row = '';
        for (let x = 0; x < grid.cellWidth; x++) {
          const [r, g, b, a] = pixel(image, x0 + x, y0 + y);
          if (a < 128) { row += TRANSPARENT; continue; }
          empty = false;
          row += nearest(r, g, b);
        }
        rows.push(row);
      }
      if (empty && options.skipEmpty) continue;
      frames.push({ name: `${prefix}_${frames.length}`, rows });
    }
  }
  return frames;
}
