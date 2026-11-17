// Pixel operations on frame rows. Rows are strings of one symbol per pixel;
// every operation returns new rows and leaves its input untouched.
import { TRANSPARENT } from './model';

export type Rows = string[];

function grid(rows: Rows): string[][] { return rows.map(row => [...row]); }
function join(cells: string[][]): Rows { return cells.map(row => row.join('')); }

export function getPixel(rows: Rows, x: number, y: number): string {
  return [...(rows[y] ?? '')][x] ?? TRANSPARENT;
}

export function setPixels(rows: Rows, points: [number, number][], symbol: string): Rows {
  const cells = grid(rows);
  let changed = false;
  for (const [x, y] of points) {
    if (y < 0 || y >= cells.length || x < 0 || x >= cells[y].length || cells[y][x] === symbol) continue;
    cells[y][x] = symbol;
    changed = true;
  }
  return changed ? join(cells) : rows;
}

/** Points of a straight line, Bresenham style, both ends included. */
export function linePoints(x0: number, y0: number, x1: number, y1: number): [number, number][] {
  const points: [number, number][] = [];
  const dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0);
  const sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
  let error = dx + dy;
  for (;;) {
    points.push([x0, y0]);
    if (x0 === x1 && y0 === y1) return points;
    const e2 = 2 * error;
    if (e2 >= dy) { error += dy; x0 += sx; }
    if (e2 <= dx) { error += dx; y0 += sy; }
  }
}

export function rectPoints(x0: number, y0: number, x1: number, y1: number, filled: boolean): [number, number][] {
  const points: [number, number][] = [];
  const [left, right] = [Math.min(x0, x1), Math.max(x0, x1)];
  const [top, bottom] = [Math.min(y0, y1), Math.max(y0, y1)];
  for (let y = top; y <= bottom; y++) {
    for (let x = left; x <= right; x++) if (filled || y === top || y === bottom || x === left || x === right) points.push([x, y]);
  }
  return points;
}

/** Four-way flood fill of the region around (x, y). */
export function floodFill(rows: Rows, x: number, y: number, symbol: string): Rows {
  const cells = grid(rows);
  const target = cells[y]?.[x];
  if (target === undefined || target === symbol) return rows;
  const stack: [number, number][] = [[x, y]];
  while (stack.length) {
    const [px, py] = stack.pop()!;
    if (cells[py]?.[px] !== target) continue;
    cells[py][px] = symbol;
    stack.push([px + 1, py], [px - 1, py], [px, py + 1], [px, py - 1]);
  }
  return join(cells);
}

export function flipHorizontal(rows: Rows): Rows { return rows.map(row => [...row].reverse().join('')); }
export function flipVertical(rows: Rows): Rows { return [...rows].reverse(); }

/** Moves the image by (dx, dy); pixels pushed off one edge wrap to the other. */
export function shift(rows: Rows, dx: number, dy: number): Rows {
  const cells = grid(rows);
  const height = cells.length;
  if (!height) return rows;
  const width = cells[0].length;
  const out = cells.map((_, y) => cells[(((y - dy) % height) + height) % height]);
  return out.map(row => row.map((_, x) => row[(((x - dx) % width) + width) % width]).join(''));
}

export function clear(rows: Rows): Rows { return rows.map(row => TRANSPARENT.repeat([...row].length)); }

/** Replaces every use of one symbol with another. */
export function replaceSymbol(rows: Rows, from: string, to: string): Rows {
  return rows.map(row => [...row].map(symbol => (symbol === from ? to : symbol)).join(''));
}
