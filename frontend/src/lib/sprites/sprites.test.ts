import { describe, expect, it } from 'vitest';
import { inferSheet, newSheet, parseSpriteJson, serializeSheet, serializeFrames, validateSheet, resizeRows, dataNameFor, uniqueFrameName, type SpriteSheet } from './model';
import { encodeBank, buildIndex, indexText, parseIndex, decodeBank, uniformIndex, includeText, indexExtent } from './pmg';
import { framesFromImage, paletteFromImage, renderSheet, gridFor, parseHex, type RGBAImage } from './image';
import { floodFill, flipHorizontal, flipVertical, linePoints, rectPoints, setPixels, shift } from './edit';
import { mergeSheets } from './merge';

/** A two-plane sheet: P0 = H B F, P1 = S F; F is the overlap colour. */
function hero(): SpriteSheet {
  return {
    width: 8, height: 4, planes: 2,
    palette: [
      { symbol: '.', name: 'transparent', color: '#0a0f1a', planes: [] },
      { symbol: 'H', name: 'hat', color: '#c1802c', planes: [0] },
      { symbol: 'B', name: 'boots', color: '#c1802c', planes: [0] },
      { symbol: 'S', name: 'shirt', color: '#237eae', planes: [1] },
      { symbol: 'F', name: 'skin', color: '#eeac70', planes: [0, 1] },
    ],
    frames: [
      { name: 'idle_0', rows: ['..HH....', '..FFFF..', '.SSSSSF.', 'BB....BB'] },
      { name: 'idle_1', rows: ['..HH....', '..F.FF..', 'FSSSSSFF', '.BB..BB.'] },
    ],
    outputs: { indexStyle: 'planes' },
  };
}

/** The same encoding written out independently, one plane at a time. */
function pythonBank(sheet: SpriteSheet): number[] {
  const bits = (row: string, symbols: string) => [...row].reduce((value, ch) => (value << 1) | (symbols.includes(ch) ? 1 : 0), 0);
  return sheet.frames.flatMap(frame => [...frame.rows.map(row => bits(row, 'HBF')), ...frame.rows.map(row => bits(row, 'SF'))]);
}

describe('PMG banks', () => {
  it('encodes two-plane frames plane by plane, leftmost pixel in bit 7', () => {
    const sheet = hero();
    expect([...encodeBank(sheet)]).toEqual(pythonBank(sheet));
  });

  it('writes the multi-plane index in a fixed key order', () => {
    const text = indexText(hero(), 'planes');
    expect(text).toBe(`{
  "idle_0": {
    "offset": 0,
    "player0_offset": 0,
    "player1_offset": 4,
    "bytes_per_plane": 4,
    "frame_length": 8,
    "height": 4,
    "width": 8
  },
  "idle_1": {
    "offset": 8,
    "player0_offset": 8,
    "player1_offset": 12,
    "bytes_per_plane": 4,
    "frame_length": 8,
    "height": 4,
    "width": 8
  }
}
`);
  });

  it('writes the single-plane index in a fixed key order', () => {
    const sheet = newSheet(8, 16, 1);
    sheet.frames.push({ name: 'snake_1', rows: sheet.frames[0].rows });
    expect(Object.keys(buildIndex(sheet, 'simple').snake_1)).toEqual(['offset', 'length', 'width', 'height']);
    expect(buildIndex(sheet, 'simple').snake_1).toEqual({ offset: 16, length: 16, width: 8, height: 16 });
  });

  it('round-trips a bank through its index', () => {
    const sheet = hero();
    const bank = encodeBank(sheet);
    const index = parseIndex(indexText(sheet, 'planes'))!;
    expect(index.style).toBe('planes');
    expect(index.layout).toEqual({ width: 8, height: 4, planes: 2 });
    expect(indexExtent(index)).toBe(bank.length);
    const decoded = decodeBank(bank, index.entries, index.layout, sheet.palette);
    // H and B share P0, so B pixels come back as the first P0 symbol, H.
    expect(decoded.frames.map(frame => frame.rows)).toEqual(sheet.frames.map(frame => frame.rows.map(row => row.replaceAll('B', 'H'))));
  });

  it('decodes a single-plane bank and invents symbols for unknown plane combinations', () => {
    const bank = Uint8Array.from([0x18, 0x3c, 0x81, 0x00]);
    const { frames, palette } = decodeBank(bank, uniformIndex(bank.length, { width: 8, height: 2, planes: 1 }), { width: 8, height: 2, planes: 1 });
    expect(frames).toEqual([{ name: 'frame_0', rows: ['...##...', '..####..'] }, { name: 'frame_1', rows: ['#......#', '........'] }]);
    expect(palette.map(color => color.symbol)).toEqual(['.', '#']);
    const two = decodeBank(Uint8Array.from([0x80, 0x80]), [{ name: 'x', offset: 0, planeOffsets: [0, 1], width: 8, height: 1 }], { width: 8, height: 1, planes: 2 }, palette);
    expect(two.palette.at(-1)!.planes).toEqual([0, 1]);
  });

  it('packs wide sprites leftmost pixel first across bytes', () => {
    const sheet = newSheet(12, 1, 1);
    sheet.frames[0].rows = ['A..........A'];
    expect([...encodeBank(sheet)]).toEqual([0x80, 0x10]);
  });

  it('generates an exBASIC include with continuation lines', () => {
    const sheet = hero();
    sheet.frames.push({ name: 'idle_2', rows: sheet.frames[0].rows }, { name: 'idle_3', rows: sheet.frames[1].rows });
    const text = includeText(sheet, 'Hero', 'hero.sprites.json');
    expect(text).toContain('DATA HeroFrames() WORD = 0, 8, 16, 24\n');
    const data = text.split('\n').filter(line => line.startsWith('DATA ') && !line.includes('Frames'));
    expect(data[0]).toMatch(/^DATA Hero\(\) BYTE = \$30, /);
    expect(data[0].endsWith(',')).toBe(true);
    // Continuation lines line BYTE up under the first line's BYTE.
    expect(data[1]).toMatch(/^DATA {8}BYTE = /);
    expect(data.at(-1)!.endsWith(',')).toBe(false);
    const values = data.flatMap(line => line.split('=')[1].split(',').map(value => value.trim()).filter(Boolean)).map(value => parseInt(value.slice(1), 16));
    expect(values).toEqual(pythonBank(sheet));
  });
});

describe('sheet files', () => {
  it('round-trips a sheet with inline frames', () => {
    const sheet = hero();
    sheet.outputs = { bank: 'G.PMG', index: 'g_index.json', indexStyle: 'planes', include: '' };
    const parsed = parseSpriteJson(serializeSheet(sheet, null));
    expect(parsed.kind).toBe('sheet');
    if (parsed.kind !== 'sheet') return;
    expect(parsed.source).toBeNull();
    expect(parsed.sheet).toEqual({ ...sheet, outputs: { bank: 'G.PMG', index: 'g_index.json', indexStyle: 'planes' } });
  });

  it('keeps frames out of a sheet that names a source file', () => {
    const text = serializeSheet(hero(), 'hero_pixels.json');
    expect(JSON.parse(text).frames).toBeUndefined();
    const parsed = parseSpriteJson(text);
    expect(parsed.kind === 'sheet' && parsed.source).toBe('hero_pixels.json');
  });

  it('reads plain frame files', () => {
    const parsed = parseSpriteJson(serializeFrames(hero().frames));
    expect(parsed).toEqual({ kind: 'frames', frames: hero().frames });
  });

  it('rejects JSON that is not sprites', () => {
    expect(() => parseSpriteJson('{"empty": 32, "brick": 33}')).toThrow(/not a sprite file/);
    expect(() => parseSpriteJson('{')).toThrow(/not valid JSON/);
  });

  it('infers a sheet that keeps every colour distinct on hardware', () => {
    const sheet = inferSheet(hero().frames);
    // Four symbols need three planes to stay four distinct colours.
    expect(sheet).toMatchObject({ width: 8, height: 4, planes: 3 });
    const masks = sheet.palette.filter(color => color.symbol !== '.').map(color => color.planes.join('+'));
    expect(masks).toEqual(['0', '1', '0+1', '2']);
    expect(validateSheet(sheet).filter(issue => issue.severity === 'error')).toEqual([]);
  });

  it('reports problems a build would trip over', () => {
    const sheet = hero();
    sheet.frames[1].rows[0] = '..HX...';
    sheet.frames.push({ ...sheet.frames[0] });
    sheet.palette[1].planes = [3];
    const messages = validateSheet(sheet).map(issue => issue.message).join('\n');
    expect(messages).toMatch(/row 0 has 7 pixels/);
    expect(messages).toMatch(/missing from the palette: "X"/);
    expect(messages).toMatch(/"idle_0" is used twice/);
    expect(messages).toMatch(/plane the sheet does not have/);
  });

  it('warns about colours that look the same on the Atari', () => {
    const sheet = hero();
    sheet.palette[2].color = '#ffffff';
    expect(validateSheet(sheet).some(issue => issue.severity === 'warning' && issue.message.includes('"H" and "B"'))).toBe(true);
  });

  it('names things sensibly', () => {
    expect(dataNameFor('game/assets/hero24.sprites.json')).toBe('Hero24');
    expect(dataNameFor('9lives.pmg')).toBe('S9lives');
    expect(uniqueFrameName(hero(), 'idle_0')).toBe('idle_1'.replace('1', '2'));
    expect(resizeRows(['AB'], 3, 3, 'bottom')).toEqual(['...', '...', 'AB.']);
  });
});

describe('images', () => {
  function image(width: number, height: number, pixels: (string | null)[]): RGBAImage {
    const data = new Uint8ClampedArray(width * height * 4);
    pixels.forEach((color, n) => {
      if (!color) return;
      const [r, g, b] = parseHex(color);
      data.set([r, g, b, 255], n * 4);
    });
    return { width, height, data };
  }

  it('renders a sheet with transparent background', () => {
    const sheet = hero();
    const out = renderSheet(sheet, { columns: 1, scale: 2 });
    expect([out.width, out.height]).toEqual([16, 16]);
    const at = (x: number, y: number) => [...out.data.slice((y * out.width + x) * 4, (y * out.width + x) * 4 + 4)];
    expect(at(0, 0)).toEqual([0, 0, 0, 0]);
    expect(at(4, 0)).toEqual([0xc1, 0x80, 0x2c, 255]);
    expect(at(5, 1)).toEqual([0xc1, 0x80, 0x2c, 255]);
  });

  it('imports a PNG sheet into the nearest palette colours', () => {
    const img = image(4, 2, ['#c0802c', null, '#ffffff', '#ffffff', null, '#2580b0', null, null]);
    const frames = framesFromImage(img, gridFor(img, 2, 2), hero().palette, { prefix: 'walk' });
    expect(frames.map(frame => frame.name)).toEqual(['walk_0', 'walk_1']);
    expect(frames[0].rows).toEqual(['H.', '.S']);
    expect(frames[1].rows[0]).toBe('FF');
    expect(framesFromImage(image(2, 2, [null, null, null, null]), gridFor({ width: 2, height: 2 }, 2, 2), hero().palette, { skipEmpty: true })).toEqual([]);
  });

  it('builds a palette from an image, most used colour first', () => {
    const { palette, planes } = paletteFromImage(image(3, 1, ['#ff0000', '#00ff00', '#00ff00']));
    expect(planes).toBe(2);
    expect(palette.slice(1).map(color => [color.color, color.planes])).toEqual([['#00ff00', [0]], ['#ff0000', [1]]]);
  });
});

describe('pixel editing', () => {
  it('draws, fills, flips and shifts', () => {
    const rows = ['....', '....', '....'];
    expect(setPixels(rows, linePoints(0, 0, 3, 2), 'A')).toEqual(['A...', '.AA.', '...A']);
    expect(setPixels(rows, rectPoints(3, 2, 1, 0, false), 'B')).toEqual(['.BBB', '.B.B', '.BBB']);
    expect(floodFill(['A..', 'AA.', '..A'], 2, 0, 'C')).toEqual(['ACC', 'AAC', '..A']);
    expect(flipHorizontal(['AB.'])).toEqual(['.BA']);
    expect(flipVertical(['A', 'B'])).toEqual(['B', 'A']);
    expect(shift(['AB.', '...'], 1, 1)).toEqual(['...', '.AB']);
    expect(shift(['AB.'], -1, 0)).toEqual(['B.A']);
    expect(setPixels(rows, [[9, 9]], 'A')).toBe(rows);
  });
});

describe('merging', () => {
  it('combines sheets of different sizes and planes', () => {
    const enemies = newSheet(8, 2, 1);
    enemies.palette[1] = { symbol: 'H', name: 'enemy', color: '#ffffff', planes: [0] };
    enemies.frames = [{ name: 'idle_0', rows: ['HHHHHHHH', 'H......H'] }, { name: 'bat', rows: ['H.H.H.H.', '........'] }];
    const { sheet, notes } = mergeSheets([{ label: 'game/hero.sprites.json', sheet: hero() }, { label: 'game/enemies.sprites.json', sheet: enemies }]);
    expect(sheet).toMatchObject({ width: 8, height: 4, planes: 2 });
    expect(sheet.frames.map(frame => frame.name)).toEqual(['idle_0', 'idle_1', 'enemies_idle_0', 'bat']);
    // Same symbol and planes but another colour: the first sheet's H is reused.
    expect(sheet.frames[2].rows).toEqual(['HHHHHHHH', 'H......H', '........', '........']);
    expect(notes.join('\n')).toMatch(/padded to 8x4/);
    expect(validateSheet(sheet).filter(issue => issue.severity === 'error')).toEqual([]);
  });

  it('renames symbols that clash', () => {
    const other = newSheet(8, 1, 2);
    other.palette = [other.palette[0], { symbol: 'H', name: 'helmet', color: '#ff0000', planes: [1] }];
    other.frames = [{ name: 'x', rows: ['HHHHHHHH'] }];
    const { sheet, notes } = mergeSheets([{ label: 'a', sheet: hero() }, { label: 'b', sheet: other }], { anchor: 'bottom', prefixAll: true });
    const helmet = sheet.palette.find(color => color.name === 'helmet')!;
    expect(helmet.symbol).not.toBe('H');
    expect(sheet.frames.at(-1)).toEqual({ name: 'b_x', rows: ['........', '........', '........', helmet.symbol.repeat(8)] });
    expect(notes.join('\n')).toMatch(/renamed to/);
    // Encoding keeps the planes of both sources.
    expect(encodeBank(sheet).slice(-8)).toEqual(Uint8Array.from([0, 0, 0, 0, 0, 0, 0, 0xff]));
  });
});
