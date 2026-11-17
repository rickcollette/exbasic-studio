// Drawing frames on canvases: the pixel editor, frame thumbnails and the
// animation preview share these.
import { TRANSPARENT, type SpriteSheet } from './model';
import { colorMap } from './image';

export interface DrawOptions {
  scale: number;
  grid?: boolean;
  /** Rows drawn faintly underneath (onion skin). */
  under?: string[] | null;
  /** Only pixels that set this plane are drawn; others are dimmed. */
  plane?: number | null;
  background?: string;
}

const CHECKER_A = '#1b2130';
const CHECKER_B = '#232a3b';

export function drawRows(canvas: HTMLCanvasElement, sheet: SpriteSheet, rows: string[], options: DrawOptions): void {
  const { scale } = options;
  const width = sheet.width * scale;
  const height = sheet.height * scale;
  if (canvas.width !== width) canvas.width = width;
  if (canvas.height !== height) canvas.height = height;
  const context = canvas.getContext('2d');
  if (!context) return;
  context.imageSmoothingEnabled = false;
  // Checkerboard shows transparency.
  const cell = Math.max(scale, 4);
  for (let y = 0; y < height; y += cell) {
    for (let x = 0; x < width; x += cell) {
      context.fillStyle = options.background ?? (((x / cell + y / cell) & 1) ? CHECKER_B : CHECKER_A);
      context.fillRect(x, y, cell, cell);
    }
  }
  const colors = new Map([...colorMap(sheet.palette)].map(([symbol, [r, g, b]]) => [symbol, `rgb(${r},${g},${b})`]));
  const planes = new Map(sheet.palette.map(color => [color.symbol, color.planes]));
  if (options.under) {
    context.globalAlpha = 0.28;
    options.under.forEach((row, y) => [...row].forEach((symbol, x) => {
      if (symbol === TRANSPARENT) return;
      context.fillStyle = colors.get(symbol) ?? '#ff00ff';
      context.fillRect(x * scale, y * scale, scale, scale);
    }));
    context.globalAlpha = 1;
  }
  rows.forEach((row, y) => [...row].forEach((symbol, x) => {
    if (symbol === TRANSPARENT || x >= sheet.width) return;
    const onPlane = options.plane == null || (planes.get(symbol) ?? []).includes(options.plane);
    context.globalAlpha = onPlane ? 1 : 0.12;
    context.fillStyle = options.plane != null && onPlane ? '#e8ecf3' : colors.get(symbol) ?? '#ff00ff';
    context.fillRect(x * scale, y * scale, scale, scale);
  }));
  context.globalAlpha = 1;
  if (options.grid && scale >= 6) {
    context.strokeStyle = 'rgba(255,255,255,0.09)';
    context.lineWidth = 1;
    context.beginPath();
    for (let x = 0; x <= sheet.width; x++) { context.moveTo(x * scale + 0.5, 0); context.lineTo(x * scale + 0.5, height); }
    for (let y = 0; y <= sheet.height; y++) { context.moveTo(0, y * scale + 0.5); context.lineTo(width, y * scale + 0.5); }
    context.stroke();
    // Byte boundaries of wide sprites, every 8 pixels.
    if (sheet.width > 8) {
      context.strokeStyle = 'rgba(255,200,80,0.35)';
      context.beginPath();
      for (let x = 8; x < sheet.width; x += 8) { context.moveTo(x * scale + 0.5, 0); context.lineTo(x * scale + 0.5, height); }
      context.stroke();
    }
  }
}

/** Svelte action: keeps a canvas showing `rows` at `scale`. */
export function frameCanvas(canvas: HTMLCanvasElement, params: { sheet: SpriteSheet; rows: string[]; scale: number }) {
  const draw = (p: typeof params) => drawRows(canvas, p.sheet, p.rows, { scale: p.scale });
  draw(params);
  return { update: draw };
}

/** Frames sharing a name stem with `name`, e.g. walk_0..walk_3 for walk_2. */
export function animationGroup(sheet: SpriteSheet, name: string): number[] {
  const stem = (value: string) => value.replace(/[_-]?\d+$/, '');
  const target = stem(name);
  const group = sheet.frames.flatMap((frame, index) => (stem(frame.name) === target ? [index] : []));
  return group.length ? group : [sheet.frames.findIndex(frame => frame.name === name)].filter(index => index >= 0);
}
