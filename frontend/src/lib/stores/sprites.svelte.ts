// The Sprite Studio: open sprite documents, their undo history, and every
// file operation on them (load, save, generate, import, merge).
//
// A document is opened from one file but may be backed by several:
//  - a sheet file (*.sprites.json) with the palette, plane mapping and
//    outputs, holding the frames itself or naming a plain frames file;
//  - a plain frames file ({ "name": ["row", ...] }),
//    which keeps its format so existing tools still read it;
//  - a PMG bank with its index, edited in place when no sheet describes it.
import { native, errorMessage, type WorkspaceBatch } from '../native';
import { parentOf, baseName, normalizeRel } from './workspace.svelte';
import type { UIStore } from './ui.svelte';
import type { WorkspaceStore } from './workspace.svelte';
import { EMULATOR_TAB, type EditorsStore } from './editors.svelte';
import {
  cloneSheet, dataNameFor, inferSheet, isPmgPath, isPngPath, isSheetPath, newSheet, normalizeFrames, parseSpriteJson,
  serializeFrames, serializeSheet, validateSheet, SHEET_EXTENSION, type IndexStyle, type SpriteSheet,
} from '../sprites/model';
import { decodeBank, encodeBank, includeText, indexExtent, indexText, parseIndex, uniformIndex, type BankLayout, type ParsedIndex } from '../sprites/pmg';
import { framesFromImage, gridFor, paletteFromImage, renderSheet, type RGBAImage } from '../sprites/image';
import { mergeSheets, type MergeOptions } from '../sprites/merge';
import { base64ToBytes, bytesToBase64, decodePng, encodePng } from '../sprites/browser';

export const SPRITE_PREFIX = '@sprite:';
export const spriteId = (path: string) => SPRITE_PREFIX + path;
export const spritePath = (id: string) => (id.startsWith(SPRITE_PREFIX) ? id.slice(SPRITE_PREFIX.length) : id);

const UNDO_LIMIT = 200;

/** Resolves `rel` against a project folder; refuses to leave the project. */
export function resolveFrom(directory: string, rel: string): string {
  const parts = directory ? directory.split('/') : [];
  for (const part of normalizeRel(rel).split('/')) {
    if (!part || part === '.') continue;
    if (part === '..') { if (!parts.length) throw new Error(`${rel} points outside the project`); parts.pop(); } else parts.push(part);
  }
  return parts.join('/');
}

/** The path of `target` relative to `directory`. */
export function relativeFrom(directory: string, target: string): string {
  const from = directory ? directory.split('/') : [];
  const to = target.split('/');
  let common = 0;
  while (common < from.length && common < to.length - 1 && from[common] === to[common]) common++;
  return [...from.slice(common).map(() => '..'), ...to.slice(common)].join('/');
}

export interface BankTarget { bank: string; index: string | null; style: IndexStyle }

/** What a document was loaded from, and where saving writes. */
interface Backing {
  sheetPath: string | null;
  framesPath: string | null;
  bank: BankTarget | null;
  sheet: SpriteSheet;
  notes: string[];
  /** Contents read, keyed by path, to tell our own writes from other programs'. */
  contents: Map<string, string>;
}

/** An image waiting to be cut into frames (opening a PNG starts here). */
export interface PendingImage { path: string; image: RGBAImage }

export class SpriteDoc {
  id = $state('');
  path = $state('');
  sheet = $state<SpriteSheet>(newSheet());
  frame = $state(0);
  dirty = $state(false);
  external = $state<'modified' | 'deleted' | null>(null);
  notes = $state<string[]>([]);
  sheetPath = $state<string | null>(null);
  framesPath = $state<string | null>(null);
  bank = $state<BankTarget | null>(null);
  pendingImage = $state<PendingImage | null>(null);
  history = $state({ undo: false, redo: false });
  /** Contents of backing files as last read or written. */
  contents = new Map<string, string>();

  private past: SpriteSheet[] = [];
  private future: SpriteSheet[] = [];
  private lastKey: string | undefined;
  private saved = '';

  constructor(path: string) {
    this.moveTo(path);
  }

  /** Changes the file the tab is identified by (rename, save as). */
  moveTo(path: string): void {
    this.id = spriteId(path);
    this.path = path;
  }

  get issues() { return validateSheet(this.sheet); }
  get errors() { return this.issues.filter(issue => issue.severity === 'error'); }
  /** The folder generated files are relative to. */
  get baseDir(): string { return parentOf(this.sheetPath ?? this.path); }

  /** Every file whose change on disk affects this document. */
  get backingPaths(): string[] {
    return [this.sheetPath, this.framesPath, this.bank?.bank, this.bank?.index].filter((path): path is string => !!path);
  }

  private fingerprint(sheet: SpriteSheet): string { return serializeSheet(sheet, null); }

  load(backing: Backing): void {
    this.sheet = backing.sheet;
    this.sheetPath = backing.sheetPath;
    this.framesPath = backing.framesPath;
    this.bank = backing.bank;
    this.notes = backing.notes;
    this.contents = backing.contents;
    this.frame = Math.min(this.frame, Math.max(0, backing.sheet.frames.length - 1));
    this.external = null;
    this.markSaved();
  }

  markSaved(): void {
    this.saved = this.fingerprint(this.sheet);
    this.dirty = false;
  }

  /**
   * Replaces the sheet. Consecutive changes with the same `key` (one pen
   * stroke, typing in one field) form a single undo step.
   */
  change(next: SpriteSheet, key?: string): void {
    if (key === undefined || key !== this.lastKey) {
      this.past.push(this.sheet);
      if (this.past.length > UNDO_LIMIT) this.past.shift();
    }
    this.lastKey = key;
    this.future = [];
    this.sheet = next;
    this.frame = Math.min(this.frame, Math.max(0, next.frames.length - 1));
    this.afterChange();
  }

  /** Edits a copy of the sheet; see `change`. */
  edit(mutate: (sheet: SpriteSheet) => void, key?: string): void {
    const next = cloneSheet(this.sheet);
    mutate(next);
    this.change(next, key);
  }

  /** Ends the current coalesced step, so the next change starts a new one. */
  seal(): void { this.lastKey = undefined; }

  undo(): void {
    const previous = this.past.pop();
    if (!previous) return;
    this.future.push(this.sheet);
    this.sheet = previous;
    this.lastKey = undefined;
    this.frame = Math.min(this.frame, Math.max(0, previous.frames.length - 1));
    this.afterChange();
  }

  redo(): void {
    const next = this.future.pop();
    if (!next) return;
    this.past.push(this.sheet);
    this.sheet = next;
    this.lastKey = undefined;
    this.afterChange();
  }

  private afterChange(): void {
    this.dirty = this.fingerprint(this.sheet) !== this.saved;
    this.history = { undo: this.past.length > 0, redo: this.future.length > 0 };
  }
}

export class SpritesStore {
  docs = $state<SpriteDoc[]>([]);
  /** The merge dialog, when open: the files ticked so far. */
  merging = $state<{ paths: string[] } | null>(null);

  constructor(private ui: UIStore, private workspace: WorkspaceStore, private editors: EditorsStore) {}

  doc(id: string): SpriteDoc | undefined { return this.docs.find(doc => doc.id === id); }
  get active(): SpriteDoc | undefined { return this.doc(this.editors.active); }

  /** The hooks the editors store uses to treat sprite tabs like its own. */
  readonly host = {
    ids: () => this.docs.map(doc => doc.id),
    open: (id: string) => this.open(spritePath(id)),
    dirty: () => this.docs.filter(doc => doc.dirty).map(doc => doc.id),
    path: (id: string) => spritePath(id),
    save: (id: string) => { const doc = this.doc(id); return doc ? this.save(doc) : Promise.resolve(true); },
  };

  /** Sprite files in the project, for the sidebar and the merge dialog. */
  get assets(): { path: string; kind: 'sheet' | 'bank' | 'image' | 'json' }[] {
    const files = this.workspace.project?.files ?? [];
    const kind = (path: string) => (isSheetPath(path) ? 'sheet' : isPmgPath(path) ? 'bank' : isPngPath(path) ? 'image' : /pixels|frames|sprite/i.test(baseName(path)) && /\.json$/i.test(path) && !/index/i.test(baseName(path)) ? 'json' : null);
    return files.map(file => ({ path: file.path, kind: kind(file.path) })).filter((item): item is { path: string; kind: 'sheet' | 'bank' | 'image' | 'json' } => !!item.kind);
  }

  // ---- Reading files ----------------------------------------------------------

  private async readText(path: string, contents?: Map<string, string>): Promise<string> {
    const text = await native().studio.ReadFile(path);
    contents?.set(path, text);
    return text;
  }

  private async readBinary(path: string, contents?: Map<string, string>): Promise<Uint8Array> {
    const data = await native().studio.ReadBinaryFile(path);
    contents?.set(path, data);
    return base64ToBytes(data);
  }

  private siblings(path: string, test: (candidate: string) => boolean): string[] {
    const directory = parentOf(path);
    return (this.workspace.project?.files ?? []).map(file => file.path).filter(candidate => parentOf(candidate) === directory && candidate !== path && test(candidate));
  }

  /** A sheet in the same folder whose frames or bank is `path`. */
  private async owningSheet(path: string): Promise<string | null> {
    for (const candidate of this.siblings(path, isSheetPath)) {
      try {
        const parsed = parseSpriteJson(await this.readText(candidate));
        if (parsed.kind !== 'sheet') continue;
        const directory = parentOf(candidate);
        if (parsed.source && resolveFrom(directory, parsed.source) === path) return candidate;
        if (parsed.sheet.outputs.bank && resolveFrom(directory, parsed.sheet.outputs.bank) === path) return candidate;
      } catch { /* not a usable sheet */ }
    }
    return null;
  }

  /** The index that describes a bank: the same-folder JSON that covers it exactly, preferring a matching name. */
  private async findIndex(bankPath: string, size: number): Promise<{ path: string; index: ParsedIndex } | null> {
    const stem = baseName(bankPath).replace(/\.[^.]*$/, '').toLowerCase();
    const candidates = this.siblings(bankPath, path => /\.json$/i.test(path) && !isSheetPath(path))
      .sort((a, b) => Number(baseName(b).toLowerCase().includes(stem)) - Number(baseName(a).toLowerCase().includes(stem)));
    for (const path of candidates) {
      try {
        const index = parseIndex(await this.readText(path));
        if (index && indexExtent(index) === size) return { path, index };
      } catch { /* unreadable */ }
    }
    return null;
  }

  private async loadSheetFile(path: string): Promise<Backing> {
    const contents = new Map<string, string>();
    const parsed = parseSpriteJson(await this.readText(path, contents));
    if (parsed.kind === 'frames') return { sheetPath: null, framesPath: path, bank: null, sheet: inferSheet(parsed.frames), notes: [], contents };
    const sheet = parsed.sheet;
    let framesPath: string | null = null;
    if (parsed.source) {
      framesPath = resolveFrom(parentOf(path), parsed.source);
      const frames = parseSpriteJson(await this.readText(framesPath, contents));
      if (frames.kind !== 'frames') throw new Error(`${framesPath} does not hold plain frames`);
      sheet.frames = frames.frames;
    }
    sheet.frames = normalizeFrames(sheet.frames, sheet.width, sheet.height);
    return { sheetPath: path, framesPath, bank: null, sheet, notes: [], contents };
  }

  private async loadBank(path: string, layout?: BankLayout & { index?: string | null }): Promise<Backing> {
    const contents = new Map<string, string>();
    const bytes = await this.readBinary(path, contents);
    let found: { path: string; index: ParsedIndex } | null = null;
    if (layout?.index) {
      const index = parseIndex(await this.readText(layout.index, contents));
      if (!index) throw new Error(`${layout.index} is not a sprite index`);
      found = { path: layout.index, index };
    } else if (!layout) {
      found = await this.findIndex(path, bytes.length);
      if (found) contents.set(found.path, await this.readText(found.path));
    }
    const notes: string[] = [];
    let entries, shape: BankLayout, style: IndexStyle;
    if (found) {
      ({ entries, layout: shape, style } = found.index);
      notes.push(`Frames come from ${found.path}. Saving rewrites ${baseName(path)} and its index.`);
    } else {
      shape = layout ?? { width: 8, height: Math.min(16, Math.max(1, bytes.length)), planes: 1 };
      entries = uniformIndex(bytes.length, shape);
      style = shape.planes > 1 ? 'planes' : 'simple';
      if (!layout) notes.push(`No index file in this folder covers ${bytes.length} bytes, so the bank is shown as ${shape.width}x${shape.height} single-plane frames. Use Import > PMG bank to choose another layout.`);
      if (bytes.length % (Math.ceil(shape.width / 8) * shape.height * shape.planes)) notes.push(`The last ${bytes.length % (Math.ceil(shape.width / 8) * shape.height * shape.planes)} bytes do not fill a whole frame and are not shown.`);
    }
    const decoded = decodeBank(bytes, entries, shape);
    const sheet: SpriteSheet = { ...shape, palette: decoded.palette, frames: decoded.frames, outputs: {} };
    return { sheetPath: null, framesPath: null, bank: { bank: path, index: found?.path ?? null, style }, sheet, notes, contents };
  }

  /** Loads any sprite source as a sheet (for merging and importing). */
  async loadAny(path: string): Promise<SpriteSheet> {
    if (isPmgPath(path)) return (await this.loadBank(path)).sheet;
    if (isPngPath(path)) {
      const image = await decodePng(await native().studio.ReadBinaryFile(path));
      const { palette, planes } = paletteFromImage(image);
      const height = image.height <= 64 ? image.height : 16;
      const frames = framesFromImage(image, gridFor(image, 8, height), palette, { prefix: baseName(path).replace(/\.[^.]*$/, '').toLowerCase(), skipEmpty: true });
      return { width: 8, height, planes, palette, frames, outputs: {} };
    }
    return (await this.loadSheetFile(path)).sheet;
  }

  // ---- Opening and closing ------------------------------------------------------

  /** Opens a file in the Sprite Studio; returns false when it could not. */
  async open(path: string): Promise<boolean> {
    const existing = this.doc(spriteId(path));
    if (existing) { this.editors.activate(existing.id); return true; }
    try {
      // Plain frames and banks that a sheet describes open as that sheet.
      if (!isSheetPath(path) && !isPngPath(path)) {
        const owner = await this.owningSheet(path);
        if (owner) {
          this.ui.status = `Opened ${baseName(owner)}, the sprite sheet that uses ${baseName(path)}`;
          return this.open(owner);
        }
      }
      const doc = new SpriteDoc(path);
      if (isPngPath(path)) {
        const image = await decodePng(await native().studio.ReadBinaryFile(path));
        const { palette, planes } = paletteFromImage(image);
        doc.load({ sheetPath: null, framesPath: null, bank: null, sheet: { width: 8, height: Math.min(image.height, 16), planes, palette, frames: [], outputs: {} }, notes: [], contents: new Map() });
        doc.pendingImage = { path, image };
      } else {
        doc.load(isPmgPath(path) ? await this.loadBank(path) : await this.loadSheetFile(path));
        if (!doc.sheetPath && doc.framesPath) doc.notes = [`${baseName(path)} holds only frames. Palette, planes and outputs are not saved until you create a sheet file for it.`];
      }
      if (this.doc(doc.id)) { this.editors.activate(doc.id); return true; }
      this.docs = [...this.docs, doc];
      this.editors.activate(doc.id);
      return true;
    } catch (error) {
      this.ui.fail(`Opening ${path} in the Sprite Studio`, error);
      return false;
    }
  }

  async close(doc: SpriteDoc): Promise<boolean> {
    if (doc.dirty) {
      const answer = await this.ui.choose({
        title: 'Save changes?', message: `${doc.path} has unsaved sprite changes.`, detail: 'Your changes will be lost if you do not save them.',
        buttons: [{ value: 'cancel', label: 'Cancel' }, { value: 'discard', label: "Don't Save", kind: 'danger' }, { value: 'save', label: 'Save', kind: 'primary' }],
        cancelValue: 'cancel',
      });
      if (answer === 'cancel') return false;
      if (answer === 'save' && !(await this.save(doc))) return false;
    }
    this.drop(doc.id);
    return true;
  }

  private drop(id: string): void {
    const index = this.docs.findIndex(doc => doc.id === id);
    if (index < 0) return;
    this.docs = this.docs.filter(doc => doc.id !== id);
    if (this.editors.active === id) {
      const next = this.docs[Math.min(index, this.docs.length - 1)]?.id ?? this.editors.tabs.at(-1)?.path ?? (this.editors.emulatorOpen ? EMULATOR_TAB : '');
      if (next) this.editors.activate(next); else this.editors.showWelcome();
    }
    this.editors.schedulePersist();
  }

  reset(): void { this.docs = []; this.merging = null; }

  // ---- Saving ---------------------------------------------------------------------

  private async writeText(doc: SpriteDoc | null, path: string, text: string): Promise<void> {
    await native().studio.SaveFile(path, text);
    doc?.contents.set(path, text);
  }

  private async writeBinary(doc: SpriteDoc | null, path: string, bytes: Uint8Array | string): Promise<void> {
    const data = typeof bytes === 'string' ? bytes : bytesToBase64(bytes);
    await native().studio.SaveBinaryFile(path, data);
    doc?.contents.set(path, data);
  }

  async save(doc: SpriteDoc): Promise<boolean> {
    if (!doc.sheetPath && !doc.framesPath && !doc.bank) return this.saveAs(doc);
    try {
      const sheet = doc.sheet;
      if (doc.sheetPath) await this.writeText(doc, doc.sheetPath, serializeSheet(sheet, doc.framesPath ? relativeFrom(parentOf(doc.sheetPath), doc.framesPath) : null));
      if (doc.framesPath) await this.writeText(doc, doc.framesPath, serializeFrames(sheet.frames));
      if (doc.bank && !doc.sheetPath) {
        await this.writeBinary(doc, doc.bank.bank, encodeBank(sheet));
        if (doc.bank.index) await this.writeText(doc, doc.bank.index, indexText(sheet, doc.bank.style));
      }
      doc.markSaved();
      doc.external = null;
      this.ui.status = `Saved ${doc.sheetPath ?? doc.framesPath ?? doc.bank?.bank}`;
      return true;
    } catch (error) {
      this.ui.fail(`Saving ${doc.path}`, error);
      return false;
    }
  }

  /** Saves the document as a new sheet file that holds its frames. */
  async saveAs(doc: SpriteDoc): Promise<boolean> {
    const suggestion = doc.sheetPath ?? `${doc.path.replace(/(\.sprites)?\.[^./]*$/i, '')}${SHEET_EXTENSION}`;
    const answer = await this.ui.prompt({ title: 'Save Sprite Sheet', message: 'Save the frames, palette and outputs as a sprite sheet file.', label: 'Sheet path', value: suggestion, confirmLabel: 'Save' });
    if (answer === null) return false;
    const target = normalizeRel(answer);
    if (!target) return false;
    const path = isSheetPath(target) ? target : `${target.replace(/\.json$/i, '')}${SHEET_EXTENSION}`;
    if (path !== doc.sheetPath && this.workspace.hasFile(path) && !(await this.ui.confirm({ title: 'Replace file?', message: `${path} already exists. Replace it?`, confirmLabel: 'Replace', danger: true }))) return false;
    // Outputs are relative to the sheet, so keep pointing at the same files.
    const oldBase = doc.baseDir;
    const newBase = parentOf(path);
    const outputs = { ...doc.sheet.outputs };
    for (const key of ['bank', 'index', 'atlas', 'include'] as const) {
      if (outputs[key]) outputs[key] = relativeFrom(newBase, resolveFrom(oldBase, outputs[key]!));
    }
    if (doc.bank && !doc.sheetPath) {
      outputs.bank ??= relativeFrom(newBase, doc.bank.bank);
      if (doc.bank.index) outputs.index ??= relativeFrom(newBase, doc.bank.index);
      outputs.indexStyle ??= doc.bank.style;
    }
    if (outputs.bank !== doc.sheet.outputs.bank || outputs.index !== doc.sheet.outputs.index) doc.change({ ...cloneSheet(doc.sheet), outputs });
    doc.sheetPath = path;
    doc.bank = null;
    doc.pendingImage = null;
    doc.notes = [];
    const ok = await this.save(doc);
    if (ok && doc.path !== path) this.retarget(doc, path);
    return ok;
  }

  /** Re-keys an open document to the sheet it is now saved in. */
  private retarget(doc: SpriteDoc, path: string): void {
    const other = this.doc(spriteId(path));
    if (other && other !== doc) this.drop(other.id);
    const wasActive = this.editors.active === doc.id;
    doc.moveTo(path);
    if (wasActive) this.editors.activate(doc.id);
    this.editors.schedulePersist();
  }

  /** Creates a sheet file for frames or a bank that has none. */
  async createSheetFile(doc: SpriteDoc): Promise<boolean> {
    return this.saveAs(doc);
  }

  // ---- Generating ---------------------------------------------------------------

  /** Writes every configured output. Saves the sheet first so source and outputs agree. */
  async generate(doc: SpriteDoc): Promise<boolean> {
    const errors = doc.errors;
    if (errors.length) {
      this.ui.fail('Generating sprites', `${errors.length} problem${errors.length === 1 ? '' : 's'} in the sheet: ${errors.slice(0, 3).map(issue => issue.message).join(' ')}`);
      return false;
    }
    const sheet = doc.sheet;
    const outputs = sheet.outputs;
    if (!outputs.bank && !outputs.index && !outputs.atlas && !outputs.include) {
      this.ui.status = 'Choose output files in the Outputs section before generating';
      this.ui.fail('Generating sprites', 'no output files are set: fill in the Outputs section of the sheet');
      return false;
    }
    if (doc.dirty && !(await this.save(doc))) return false;
    const base = doc.baseDir;
    const written: string[] = [];
    try {
      if (outputs.bank) { const path = resolveFrom(base, outputs.bank); await this.writeBinary(doc, path, encodeBank(sheet)); written.push(`${path} (${encodeBank(sheet).length} bytes)`); }
      if (outputs.index) { const path = resolveFrom(base, outputs.index); await this.writeText(doc, path, indexText(sheet, outputs.indexStyle ?? (sheet.planes > 1 ? 'planes' : 'simple'))); written.push(path); }
      if (outputs.include) {
        const path = resolveFrom(base, outputs.include);
        await this.writeText(doc, path, includeText(sheet, outputs.dataName?.trim() || dataNameFor(doc.sheetPath ?? doc.path), baseName(doc.sheetPath ?? doc.path)));
        written.push(path);
      }
      if (outputs.atlas) {
        const path = resolveFrom(base, outputs.atlas);
        await this.writeBinary(doc, path, await encodePng(renderSheet(sheet, { scale: outputs.atlasScale ?? 1, columns: outputs.atlasColumns || sheet.frames.length })));
        written.push(path);
      }
    } catch (error) {
      this.ui.fail('Generating sprites', error);
      return false;
    }
    this.ui.appendOutput(`Sprite Studio: generated from ${doc.sheetPath ?? doc.path}\n${written.map(line => `  ${line}\n`).join('')}`);
    this.ui.status = `Generated ${written.length} file${written.length === 1 ? '' : 's'}`;
    void this.workspace.refresh();
    return true;
  }

  /** Writes the frames as a PNG sheet (transparent background). */
  async exportPng(doc: SpriteDoc, scale: number, columns: number): Promise<void> {
    const suggestion = `${doc.path.replace(/(\.sprites)?\.[^./]*$/i, '')}_sheet.png`;
    const answer = await this.ui.prompt({ title: 'Export PNG Sheet', message: `${doc.sheet.frames.length} frames, ${columns} per row, ${scale}x scale, transparent background.`, label: 'PNG path', value: suggestion, confirmLabel: 'Export' });
    if (answer === null || !normalizeRel(answer)) return;
    const path = normalizeRel(answer);
    try {
      await this.writeBinary(null, path, await encodePng(renderSheet(doc.sheet, { scale, columns })));
      this.ui.status = `Exported ${path}`;
      void this.workspace.refresh();
    } catch (error) {
      this.ui.fail('Exporting PNG sheet', error);
    }
  }

  // ---- New, import, merge -----------------------------------------------------------

  async newSheet(): Promise<void> {
    if (!this.workspace.project) return;
    const selected = this.workspace.selected;
    const directory = selected && this.workspace.hasDirectory(selected) ? selected : parentOf(selected);
    const answer = await this.ui.prompt({ title: 'New Sprite Sheet', message: 'An 8x16 single-plane sheet. Size, planes and outputs can be changed afterwards.', label: 'Sheet path', value: `${directory ? directory + '/' : ''}sprites${SHEET_EXTENSION}`, confirmLabel: 'Create' });
    if (answer === null || !normalizeRel(answer)) return;
    const target = normalizeRel(answer);
    const path = isSheetPath(target) ? target : `${target.replace(/\.json$/i, '')}${SHEET_EXTENSION}`;
    if (this.workspace.hasFile(path)) { this.ui.fail('New sprite sheet', `${path} already exists`); return; }
    const sheet = newSheet();
    sheet.outputs = defaultOutputs(path, sheet.planes);
    try {
      await native().studio.SaveFile(path, serializeSheet(sheet, null));
    } catch (error) {
      this.ui.fail('New sprite sheet', error);
      return;
    }
    await this.workspace.refresh();
    await this.open(path);
  }

  /** Cuts the pending (or a chosen) image into frames for a document. */
  importImage(doc: SpriteDoc, image: RGBAImage, options: { cellWidth: number; cellHeight: number; palette: 'image' | 'sheet'; replace: boolean; prefix: string; skipEmpty: boolean }): number {
    const grid = gridFor(image, options.cellWidth, options.cellHeight);
    const fromImage = options.palette === 'image' ? paletteFromImage(image) : null;
    const palette = fromImage?.palette ?? doc.sheet.palette;
    const frames = framesFromImage(image, grid, palette, { prefix: options.prefix || 'frame', skipEmpty: options.skipEmpty });
    if (!frames.length) return 0;
    if (options.replace || !doc.sheet.frames.length) {
      doc.change({ ...cloneSheet(doc.sheet), width: options.cellWidth, height: options.cellHeight, planes: fromImage?.planes ?? doc.sheet.planes, palette, frames });
      doc.frame = 0;
    } else {
      const incoming: SpriteSheet = { width: options.cellWidth, height: options.cellHeight, planes: fromImage?.planes ?? doc.sheet.planes, palette, frames, outputs: {} };
      const merged = mergeSheets([{ label: doc.path, sheet: doc.sheet }, { label: options.prefix || 'image', sheet: incoming }]);
      doc.change({ ...merged.sheet, outputs: { ...doc.sheet.outputs } });
      doc.frame = doc.sheet.frames.length - frames.length;
    }
    doc.pendingImage = null;
    return frames.length;
  }

  /** Reads a PNG for importing into a document. */
  async readImage(path: string): Promise<RGBAImage> {
    return decodePng(await native().studio.ReadBinaryFile(path));
  }

  /** Re-reads a PMG bank with an explicit layout or index, replacing the frames or appending them. */
  async importBank(doc: SpriteDoc, path: string, layout: (BankLayout & { index?: string | null }) | null, replace: boolean): Promise<number> {
    try {
      const backing = await this.loadBank(path, layout ?? undefined);
      if (replace) {
        doc.change({ ...backing.sheet, outputs: { ...doc.sheet.outputs } });
        doc.frame = 0;
        if (doc.bank?.bank === path) doc.bank = backing.bank;
      } else {
        const merged = mergeSheets([{ label: doc.path, sheet: doc.sheet }, { label: path, sheet: backing.sheet }]);
        doc.change({ ...merged.sheet, outputs: { ...doc.sheet.outputs } });
        for (const note of merged.notes) this.ui.appendOutput(`Sprite Studio: ${note}\n`);
      }
      return backing.sheet.frames.length;
    } catch (error) {
      this.ui.fail(`Importing ${path}`, error);
      return 0;
    }
  }

  /** Appends the frames of other sprite files to a document. */
  async appendFrom(doc: SpriteDoc, paths: string[], options: MergeOptions = {}): Promise<void> {
    try {
      const sources = [{ label: doc.path, sheet: doc.sheet }];
      for (const path of paths) sources.push({ label: path, sheet: await this.loadAny(path) });
      const merged = mergeSheets(sources, options);
      const before = doc.sheet.frames.length;
      doc.change({ ...merged.sheet, outputs: { ...doc.sheet.outputs } });
      doc.frame = Math.min(before, doc.sheet.frames.length - 1);
      for (const note of merged.notes) this.ui.appendOutput(`Sprite Studio: ${note}\n`);
      this.ui.status = `Added ${merged.sheet.frames.length - before} frames`;
    } catch (error) {
      this.ui.fail('Adding frames', error);
    }
  }

  /** Merges sprite files into a new sheet and opens it. */
  async merge(paths: string[], target: string, options: MergeOptions = {}): Promise<boolean> {
    const clean = normalizeRel(target);
    if (!paths.length || !clean) return false;
    const path = isSheetPath(clean) ? clean : `${clean.replace(/\.json$/i, '')}${SHEET_EXTENSION}`;
    if (this.workspace.hasFile(path) && !(await this.ui.confirm({ title: 'Replace file?', message: `${path} already exists. Replace it with the merged sheet?`, confirmLabel: 'Replace', danger: true }))) return false;
    try {
      const sources = [];
      for (const source of paths) sources.push({ label: source, sheet: await this.loadAny(source) });
      const { sheet, notes } = mergeSheets(sources, options);
      sheet.outputs = defaultOutputs(path, sheet.planes);
      await native().studio.SaveFile(path, serializeSheet(sheet, null));
      this.ui.appendOutput(`Sprite Studio: merged ${paths.length} file${paths.length === 1 ? '' : 's'} (${sheet.frames.length} frames) into ${path}\n${notes.map(note => `  ${note}\n`).join('')}`);
      this.merging = null;
      const open = this.doc(spriteId(path));
      if (open) this.drop(open.id);
      await this.workspace.refresh();
      await this.open(path);
      return true;
    } catch (error) {
      this.ui.fail('Merging sprite sheets', error);
      return false;
    }
  }

  // ---- Files changed elsewhere ---------------------------------------------------------

  async handleExternal(batch: WorkspaceBatch): Promise<void> {
    for (const doc of [...this.docs]) {
      const affected = batch.overflow ? doc.backingPaths : batch.changes.filter(change => !change.directory && doc.backingPaths.includes(change.path)).map(change => change.path);
      if (!affected.length) continue;
      let changed = false;
      let deleted = false;
      for (const path of affected) {
        const binary = path === doc.bank?.bank;
        let now: string | null;
        try { now = binary ? await native().studio.ReadBinaryFile(path) : await native().studio.ReadFile(path); } catch { now = null; }
        if (now === null) deleted = true;
        else if (now !== doc.contents.get(path)) changed = true;
      }
      if (!this.doc(doc.id)) continue;
      if (deleted) { doc.external = 'deleted'; doc.dirty = true; continue; }
      if (!changed) continue;
      if (doc.dirty) doc.external = 'modified';
      else await this.reload(doc);
    }
  }

  async reload(doc: SpriteDoc): Promise<void> {
    try {
      const target = doc.sheetPath ?? doc.framesPath;
      const backing = target ? await this.loadSheetFile(target) : doc.bank ? await this.loadBank(doc.bank.bank) : null;
      if (!backing) { doc.external = null; return; }
      doc.load({ ...backing, notes: doc.notes });
    } catch (error) {
      this.ui.fail(`Reloading ${doc.path}`, error);
    }
  }

  applyRename(oldPath: string, newPath: string): void {
    const moved = (path: string) => (path === oldPath || path.startsWith(oldPath + '/') ? newPath + path.slice(oldPath.length) : path);
    let changed = false;
    for (const doc of this.docs) {
      doc.sheetPath = doc.sheetPath && moved(doc.sheetPath);
      doc.framesPath = doc.framesPath && moved(doc.framesPath);
      if (doc.bank) doc.bank = { ...doc.bank, bank: moved(doc.bank.bank), index: doc.bank.index && moved(doc.bank.index) };
      doc.contents = new Map([...doc.contents].map(([path, value]) => [moved(path), value]));
      if (moved(doc.path) === doc.path) continue;
      changed = true;
      const wasActive = this.editors.active === doc.id;
      doc.moveTo(moved(doc.path));
      if (wasActive) this.editors.active = doc.id;
    }
    if (changed) this.editors.schedulePersist();
  }

  applyDelete(path: string): void {
    for (const doc of this.docs.filter(item => item.path === path || item.path.startsWith(path + '/'))) this.drop(doc.id);
  }

  /** Reports a failure without a dialog; used by the editor for quick actions. */
  status(message: string): void { this.ui.status = message; }
  fail(label: string, error: unknown): void { this.ui.fail(label, errorMessage(error)); }
}

/** Output files next to a new sheet: NAME.PMG, name_index.json, and the include. */
export function defaultOutputs(sheetPath: string, planes: number): SpriteSheet['outputs'] {
  const stem = baseName(sheetPath).replace(/\.sprites\.json$|\.json$/i, '');
  return { bank: `${stem.toUpperCase()}.PMG`, index: `${stem}_index.json`, indexStyle: planes > 1 ? 'planes' : 'simple', dataName: dataNameFor(sheetPath) };
}
