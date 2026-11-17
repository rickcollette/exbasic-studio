// Sprite Studio workflows: the real stores against the mock backend.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { setNative } from '../native';
import { createMockNative, type MockNative } from '../mockNative';
import { createWorkbench, type Workbench } from '../workbench.svelte';
import { spriteId, resolveFrom, relativeFrom } from './sprites.svelte';
import { base64ToBytes, bytesToBase64 } from '../sprites/browser';
import { setPixels } from '../sprites/edit';

let mock: MockNative;
let wb: Workbench;

async function settle(rounds = 10) {
  for (let i = 0; i < rounds; i++) { await vi.advanceTimersByTimeAsync(1); await Promise.resolve(); }
  flushSync();
}

async function answer(value: string, text = '') {
  await settle();
  expect(wb.ui.dialog, `expected a dialog to answer with "${value}"`).not.toBeNull();
  wb.ui.closeDialog(value, text);
  await settle();
}

async function write(files: Record<string, string>) {
  for (const [path, content] of Object.entries(files)) await mock.studio.SaveFile(path, content);
  await wb.workspace.refresh();
}

const FRAMES = JSON.stringify({ idle_0: ['..HH....', '..FFFF..', '.SSSSSF.', 'BB....BB'], idle_1: ['..HH....', '..F.FF..', 'FSSSSSFF', '.BB..BB.'] });
const SHEET = JSON.stringify({
  format: 'exbasic-sprites', version: 1, width: 8, height: 4, planes: 2,
  palette: [
    { symbol: '.', name: 'transparent', color: '#0a0f1a', planes: [] },
    { symbol: 'H', name: 'hat', color: '#c1802c', planes: [0] },
    { symbol: 'B', name: 'boots', color: '#c1802c', planes: [0] },
    { symbol: 'S', name: 'shirt', color: '#237eae', planes: [1] },
    { symbol: 'F', name: 'skin', color: '#eeac70', planes: [0, 1] },
  ],
  source: 'hero_pixels.json',
  outputs: { bank: 'HERO.PMG', index: 'hero_index.json', indexStyle: 'planes', include: 'HERO.BAS', dataName: 'Hero' },
});

beforeEach(async () => {
  vi.useFakeTimers();
  mock = createMockNative();
  setNative(mock);
  wb = createWorkbench();
  await wb.start();
  await settle();
  const pending = wb.createProject('/home/tester', 'demo');
  await settle();
  await pending;
  await settle();
});

describe('paths', () => {
  it('resolves and relativises inside the project only', () => {
    expect(resolveFrom('game/assets', '../previews/a.png')).toBe('game/previews/a.png');
    expect(() => resolveFrom('game', '../../x')).toThrow(/outside the project/);
    expect(relativeFrom('game/assets', 'game/previews/a.png')).toBe('../previews/a.png');
    expect(relativeFrom('', 'a/b.json')).toBe('a/b.json');
    expect(relativeFrom('a', 'a/b.json')).toBe('b.json');
  });
});

describe('opening and saving', () => {
  it('edits a plain frames file and keeps its format', async () => {
    await write({ 'gfx/hero_pixels.json': FRAMES });
    expect(await wb.openFile('gfx/hero_pixels.json')).toBe(true);
    // Plain JSON opens as text from the explorer; the command opens it as sprites.
    expect(wb.editors.active).toBe('gfx/hero_pixels.json');
    await wb.execute('sprites.open');
    await settle();
    const doc = wb.sprites.active!;
    expect(doc.id).toBe(spriteId('gfx/hero_pixels.json'));
    expect(doc.sheetPath).toBeNull();
    expect(doc.notes[0]).toMatch(/holds only frames/);
    doc.edit(sheet => { sheet.frames[0].rows = setPixels(sheet.frames[0].rows, [[0, 0]], 'H'); });
    expect(doc.dirty).toBe(true);
    await wb.execute('file.save');
    expect(doc.dirty).toBe(false);
    const saved = JSON.parse(await mock.studio.ReadFile('gfx/hero_pixels.json'));
    expect(saved.idle_0[0]).toBe('H.HH....');
    expect(Object.keys(saved)).toEqual(['idle_0', 'idle_1']);
  });

  it('opens the sheet that owns a frames file and generates its outputs', async () => {
    await write({ 'gfx/hero_pixels.json': FRAMES, 'gfx/hero.sprites.json': SHEET });
    await wb.sprites.open('gfx/hero_pixels.json');
    await settle();
    const doc = wb.sprites.active!;
    expect(doc.path).toBe('gfx/hero.sprites.json');
    expect(doc.framesPath).toBe('gfx/hero_pixels.json');
    expect(await wb.sprites.generate(doc)).toBe(true);
    const bank = base64ToBytes(await mock.studio.ReadBinaryFile('gfx/HERO.PMG'));
    expect([...bank.slice(0, 8)]).toEqual([0x30, 0x3c, 0x02, 0xc3, 0x00, 0x3c, 0x7e, 0x00]);
    const index = JSON.parse(await mock.studio.ReadFile('gfx/hero_index.json'));
    expect(index.idle_1).toEqual({ offset: 8, player0_offset: 8, player1_offset: 12, bytes_per_plane: 4, frame_length: 8, height: 4, width: 8 });
    expect(await mock.studio.ReadFile('gfx/HERO.BAS')).toContain('DATA HeroFrames() WORD = 0, 8');
    // The sheet keeps its frames in the source file.
    doc.edit(sheet => { sheet.palette[1].name = 'hat brim'; });
    await wb.sprites.save(doc);
    expect(JSON.parse(await mock.studio.ReadFile('gfx/hero.sprites.json')).frames).toBeUndefined();
    expect(JSON.parse(await mock.studio.ReadFile('gfx/hero.sprites.json')).palette[1].name).toBe('hat brim');
  });

  it('refuses to generate a sheet with errors', async () => {
    await write({ 'gfx/hero_pixels.json': FRAMES, 'gfx/hero.sprites.json': SHEET });
    await wb.sprites.open('gfx/hero.sprites.json');
    const doc = wb.sprites.active!;
    doc.edit(sheet => { sheet.frames[0].rows[0] = '..ZZ....'; });
    expect(await wb.sprites.generate(doc)).toBe(false);
    await expect(mock.studio.ReadBinaryFile('gfx/HERO.PMG')).rejects.toThrow();
  });

  it('opens a PMG bank through the index beside it and saves both', async () => {
    const bank = Uint8Array.from([0x18, 0x3c, 0x7e, 0xff, 0x81, 0x42, 0x24, 0x18]);
    await mock.studio.SaveBinaryFile('gfx/ENEMY.PMG', bytesToBase64(bank));
    await write({ 'gfx/enemy_index.json': JSON.stringify({ a: { offset: 0, length: 4, width: 8, height: 4 }, b: { offset: 4, length: 4, width: 8, height: 4 } }) });
    expect(await wb.openFile('gfx/ENEMY.PMG')).toBe(true);
    const doc = wb.sprites.active!;
    expect(doc.bank).toEqual({ bank: 'gfx/ENEMY.PMG', index: 'gfx/enemy_index.json', style: 'simple' });
    expect(doc.sheet.frames.map(frame => frame.name)).toEqual(['a', 'b']);
    expect(doc.sheet.frames[0].rows[0]).toBe('...##...');
    doc.edit(sheet => { sheet.frames[1].name = 'bat'; sheet.frames[1].rows = setPixels(sheet.frames[1].rows, [[1, 0]], '#'); });
    await wb.execute('file.save');
    expect([...base64ToBytes(await mock.studio.ReadBinaryFile('gfx/ENEMY.PMG'))]).toEqual([0x18, 0x3c, 0x7e, 0xff, 0xc1, 0x42, 0x24, 0x18]);
    expect(Object.keys(JSON.parse(await mock.studio.ReadFile('gfx/enemy_index.json')))).toEqual(['a', 'bat']);
  });

  it('turns a bank into a sheet file that generates the same bank', async () => {
    await mock.studio.SaveBinaryFile('ENEMY.PMG', bytesToBase64(Uint8Array.from([1, 2, 3, 4])));
    await wb.workspace.refresh();
    await wb.openFile('ENEMY.PMG');
    const doc = wb.sprites.active!;
    expect(doc.notes[0]).toMatch(/No index file/);
    const pending = wb.sprites.createSheetFile(doc);
    await answer('confirm', 'enemy.sprites.json');
    expect(await pending).toBe(true);
    expect(doc.path).toBe('enemy.sprites.json');
    expect(wb.editors.active).toBe(spriteId('enemy.sprites.json'));
    const sheet = JSON.parse(await mock.studio.ReadFile('enemy.sprites.json'));
    expect(sheet.outputs).toMatchObject({ bank: 'ENEMY.PMG', indexStyle: 'simple' });
    expect(Object.keys(sheet.frames)).toEqual(['frame_0']);
  });
});

describe('tabs', () => {
  it('creates a sheet, asks before closing it dirty, and saves it with Save All', async () => {
    const pending = wb.execute('sprites.new');
    await answer('confirm', 'art/player.sprites.json');
    await pending;
    await settle();
    const doc = wb.sprites.doc(spriteId('art/player.sprites.json'))!;
    expect(doc).toBeDefined();
    expect(doc.sheet.outputs).toMatchObject({ bank: 'PLAYER.PMG', index: 'player_index.json', dataName: 'Player' });
    doc.edit(sheet => { sheet.frames[0].rows = setPixels(sheet.frames[0].rows, [[3, 3]], 'A'); });
    expect(wb.commandEnabled('file.saveAll')).toBe(true);
    const closing = wb.execute('editor.close');
    await answer('cancel');
    await closing;
    expect(wb.sprites.docs).toHaveLength(1);
    await wb.execute('file.saveAll');
    await settle();
    expect(doc.dirty).toBe(false);
    expect(JSON.parse(await mock.studio.ReadFile('art/player.sprites.json')).frames.frame_0[3]).toBe('...A....');
    await wb.execute('editor.close');
    await settle();
    expect(wb.sprites.docs).toHaveLength(0);
  });

  it('undoes a pen stroke as one step', async () => {
    await write({ 'hero_pixels.json': FRAMES });
    await wb.sprites.open('hero_pixels.json');
    const doc = wb.sprites.active!;
    for (const x of [0, 1, 2]) doc.edit(sheet => { sheet.frames[0].rows = setPixels(sheet.frames[0].rows, [[x, 3]], 'S'); }, 'stroke:1');
    doc.seal();
    expect(doc.sheet.frames[0].rows[3]).toBe('SSS...BB');
    await wb.execute('editor.undo');
    expect(doc.sheet.frames[0].rows[3]).toBe('BB....BB');
    expect(doc.dirty).toBe(false);
    await wb.execute('editor.redo');
    expect(doc.sheet.frames[0].rows[3]).toBe('SSS...BB');
  });

  it('includes sprite tabs when the folder is closed and when it is reopened', async () => {
    await write({ 'hero_pixels.json': FRAMES });
    await wb.sprites.open('hero_pixels.json');
    await vi.advanceTimersByTimeAsync(1000);
    await settle();
    wb.sprites.active!.edit(sheet => { sheet.frames[0].name = 'stand'; });
    const closing = wb.closeFolder();
    await answer('discard');
    await closing;
    await settle();
    expect(wb.workspace.project).toBeNull();
    expect(wb.sprites.docs).toHaveLength(0);
    const reopening = wb.openFolder('/home/tester/demo');
    await settle(20);
    await reopening;
    await settle(20);
    expect(wb.sprites.docs.map(doc => doc.path)).toEqual(['hero_pixels.json']);
  });
});

describe('merging and outside changes', () => {
  it('merges sheets into a new sheet and opens it', async () => {
    await write({ 'gfx/hero_pixels.json': FRAMES, 'gfx/hero.sprites.json': SHEET });
    await mock.studio.SaveBinaryFile('gfx/ENEMY.PMG', bytesToBase64(Uint8Array.from([0xff, 0, 0, 0xff])));
    await wb.workspace.refresh();
    expect(wb.sprites.assets.map(asset => `${asset.kind}:${asset.path}`)).toEqual(['bank:gfx/ENEMY.PMG', 'sheet:gfx/hero.sprites.json', 'json:gfx/hero_pixels.json']);
    expect(await wb.sprites.merge(['gfx/hero.sprites.json', 'gfx/ENEMY.PMG'], 'gfx/all')).toBe(true);
    const doc = wb.sprites.active!;
    expect(doc.path).toBe('gfx/all.sprites.json');
    expect(doc.sheet).toMatchObject({ width: 8, height: 4, planes: 2 });
    expect(doc.sheet.frames.map(frame => frame.name)).toEqual(['idle_0', 'idle_1', 'frame_0']);
    expect(doc.sheet.outputs).toMatchObject({ bank: 'ALL.PMG', index: 'all_index.json', indexStyle: 'planes' });
    expect(await wb.sprites.generate(doc)).toBe(true);
    expect(base64ToBytes(await mock.studio.ReadBinaryFile('gfx/ALL.PMG'))).toHaveLength(24);
  });

  it('reloads a clean sheet changed elsewhere and flags a dirty one', async () => {
    await write({ 'hero_pixels.json': FRAMES });
    await wb.sprites.open('hero_pixels.json');
    const doc = wb.sprites.active!;
    const changed = JSON.stringify({ idle_0: ['HHHHHHHH', '........', '........', '........'] });
    await mock.studio.SaveFile('hero_pixels.json', changed);
    await wb.sprites.handleExternal({ root: '', overflow: false, changes: [{ path: 'hero_pixels.json', kind: 'write', directory: false, size: changed.length }] });
    expect(doc.sheet.frames.map(frame => frame.name)).toEqual(['idle_0']);
    expect(doc.dirty).toBe(false);
    // Our own save does not count as an outside change.
    doc.edit(sheet => { sheet.frames[0].name = 'stand'; });
    await wb.sprites.save(doc);
    await wb.sprites.handleExternal({ root: '', overflow: false, changes: [{ path: 'hero_pixels.json', kind: 'write', directory: false, size: 1 }] });
    expect(doc.external).toBeNull();
    doc.edit(sheet => { sheet.frames[0].name = 'idle'; });
    await mock.studio.SaveFile('hero_pixels.json', FRAMES);
    await wb.sprites.handleExternal({ root: '', overflow: false, changes: [{ path: 'hero_pixels.json', kind: 'write', directory: false, size: 1 }] });
    expect(doc.external).toBe('modified');
    expect(doc.sheet.frames[0].name).toBe('idle');
  });

  it('follows renames', async () => {
    await write({ 'gfx/hero_pixels.json': FRAMES, 'gfx/hero.sprites.json': SHEET });
    await wb.sprites.open('gfx/hero.sprites.json');
    await wb.movePath('gfx', 'art');
    await settle();
    const doc = wb.sprites.docs[0];
    expect(doc.path).toBe('art/hero.sprites.json');
    expect(doc.framesPath).toBe('art/hero_pixels.json');
    expect(wb.editors.active).toBe(doc.id);
  });
});
