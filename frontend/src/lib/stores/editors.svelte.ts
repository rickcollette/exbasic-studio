// Open editors. Each tab owns a CodeMirror EditorState, so undo history,
// selection and scroll anchors are per tab. A tab is dirty exactly when its
// document differs from the last saved text, so undoing back to the saved
// content clears the marker.
import { EditorState, Text } from '@codemirror/state';
import { native, errorMessage, type Diagnostic, type WorkspaceBatch, type WorkspaceState } from '../native';
import { scanSymbols } from '../language/symbols';
import { isExbasicSource } from '../language';
import type { UIStore } from './ui.svelte';
import type { WorkspaceStore } from './workspace.svelte';
import { imageMime, isImagePath, isJsonPath } from '../fileTypes';

export interface Tab {
  path: string;
  kind: 'text' | 'image';
  /** Data URL used by image tabs. */
  image?: string;
  dirty: boolean;
  /** Increments on every document change; lint results carry it back. */
  version: number;
  /** Set when the file changed on disk while the tab had unsaved edits. */
  external: 'modified' | 'deleted' | null;
  /** Increments when the store replaces the tab's state from outside. */
  epoch: number;
}

export interface RevealRequest { path: string; line: number; column: number; token: number }

export const EMULATOR_TAB = '@atari800';

/**
 * Editors that are not text documents (the Sprite Studio). Their ids start
 * with "@" and the owning store handles loading, saving and closing; the
 * editors store only orders, persists and restores them.
 */
export interface VirtualTabHost {
  /** Open tab ids, in display order. */
  ids(): string[];
  open(id: string): Promise<boolean>;
  /** Ids with unsaved changes. */
  dirty(): string[];
  /** The file shown for an id, for prompts and the explorer selection. */
  path(id: string): string;
  save(id: string): Promise<boolean>;
}

export function isVirtualTab(id: string): boolean { return id.startsWith('@') && id !== EMULATOR_TAB; }

/** Diagnostics grouped by where they came from. */
/**
 * A problem inside an included file is shown in that file. The program being
 * checked also gets a marker on its INCLUDE line that names the file, so the
 * cause is visible without opening the include.
 */
export function withIncludeMarkers(path: string, text: string, diagnostics: Diagnostic[]): Diagnostic[] {
  const foreign = diagnostics.filter(item => item.path && item.path !== path);
  if (!foreign.length) return diagnostics;
  const includes = scanSymbols(text).filter(symbol => symbol.kind === 'include');
  const baseName = (value: string) => value.split(/[\\/]/).pop()!.toLowerCase();
  const markers: Diagnostic[] = [];
  for (const item of foreign) {
    const include = includes.find(symbol => baseName(symbol.name) === baseName(item.path)) ?? includes[0];
    if (!include) continue;
    markers.push({ ...item, path, line: include.line, column: include.column, endLine: 0, endColumn: 0, code: '', message: `${item.path}, line ${item.line}: ${item.message}` });
  }
  return [...diagnostics, ...markers];
}

export class ProblemsStore {
  lint = $state<Record<string, Diagnostic[]>>({});
  build = $state<Diagnostic[]>([]);

  setLint(path: string, diagnostics: Diagnostic[]): void {
    this.lint = { ...this.lint, [path]: diagnostics };
  }

  clearLint(path: string): void {
    if (!(path in this.lint)) return;
    const next = { ...this.lint };
    delete next[path];
    this.lint = next;
  }

  renamePath(oldPath: string, newPath: string): void {
    const next: Record<string, Diagnostic[]> = {};
    for (const [path, list] of Object.entries(this.lint)) {
      const moved = path === oldPath || path.startsWith(oldPath + '/') ? newPath + path.slice(oldPath.length) : path;
      next[moved] = list.map(item => ({ ...item, path: moved }));
    }
    this.lint = next;
  }

  reset(): void { this.lint = {}; this.build = []; }

  /**
   * Records the diagnostics of a build. The compiler just read every saved
   * file, so earlier lint results for unmodified files are superseded;
   * files with unsaved edits keep their live results.
   */
  setBuild(diagnostics: Diagnostic[], dirtyPaths: string[]): void {
    const dirty = new Set(dirtyPaths);
    this.lint = Object.fromEntries(Object.entries(this.lint).filter(([path]) => dirty.has(path)));
    this.build = diagnostics;
  }

  /**
   * Diagnostics for one file. Linting a program also reports problems in the
   * files it includes, so results are matched by their own path, whichever
   * file was being checked. Live lint results replace stale build ones.
   */
  forPath(path: string): Diagnostic[] {
    const seen = new Set<string>();
    const fromLint = Object.values(this.lint).flat().filter(item => {
      if (item.path !== path) return false;
      const key = `${item.line}:${item.column}:${item.message}`;
      if (seen.has(key)) return false;
      seen.add(key);
      return true;
    });
    if (path in this.lint || fromLint.length) return fromLint;
    return this.build.filter(item => item.path === path);
  }

  get all(): Diagnostic[] {
    const linted = new Set(Object.keys(this.lint));
    const list = [...Object.values(this.lint).flat(), ...this.build.filter(item => !linted.has(item.path))];
    const rank = (severity: string) => (severity === 'error' ? 0 : severity === 'warning' ? 1 : 2);
    return list.sort((a, b) => rank(a.severity) - rank(b.severity) || a.path.localeCompare(b.path) || a.line - b.line || a.column - b.column);
  }

  get errorCount(): number { return this.all.filter(item => item.severity === 'error').length; }
  get warningCount(): number { return this.all.filter(item => item.severity === 'warning').length; }
}

const LINT_DELAY = 450;
const RECOVERY_DELAY = 1200;
const PERSIST_DELAY = 600;

export class EditorsStore {
  tabs = $state<Tab[]>([]);
  /** Path of the active tab, EMULATOR_TAB, or "" for the welcome page. */
  active = $state('');
  emulatorOpen = $state(false);
  splitOpen = $state(false);
  reveal = $state<RevealRequest | null>(null);
  cursor = $state({ line: 1, column: 1 });

  /** Builds the EditorState for a file; supplied by the editor module. */
  stateFactory: (path: string, text: string) => EditorState = (_path, text) => EditorState.create({ doc: text });
  /** Extra per-workspace state (breakpoints, watches) merged when persisting. */
  extraState: () => Partial<WorkspaceState> = () => ({});
  /** Non-text editors; installed by the workbench. */
  virtual: VirtualTabHost = { ids: () => [], open: async () => false, dirty: () => [], path: id => id, save: async () => false };

  private states = new Map<string, EditorState>();
  private saved = new Map<string, Text>();
  private history: string[] = [];
  private historyIndex = -1;
  private navigating = false;
  private revealToken = 0;
  private lintTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private recoveryTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private persistTimer: ReturnType<typeof setTimeout> | undefined;
  historyState = $state({ back: false, forward: false });

  constructor(private ui: UIStore, private workspace: WorkspaceStore, private problems: ProblemsStore) {}

  get activeTab(): Tab | null { return this.tabs.find(tab => tab.path === this.active) ?? null; }
  get dirtyTabs(): Tab[] { return this.tabs.filter(tab => tab.dirty); }
  tab(path: string): Tab | undefined { return this.tabs.find(tab => tab.path === path); }
  stateFor(path: string): EditorState | undefined { return this.states.get(path); }
  text(path: string): string { return this.states.get(path)?.doc.toString() ?? ''; }

  private patch(path: string, change: Partial<Tab>): void {
    this.tabs = this.tabs.map(tab => (tab.path === path ? { ...tab, ...change } : tab));
  }

  // ---- State flow from the editor view --------------------------------------

  /** Called by the editor view after every transaction of the active tab. */
  update(path: string, state: EditorState, docChanged: boolean): void {
    const tab = this.tab(path);
    if (!tab) return;
    this.states.set(path, state);
    if (!docChanged) return;
    const dirty = !state.doc.eq(this.saved.get(path) ?? Text.empty);
    this.patch(path, { dirty, version: tab.version + 1 });
    this.scheduleLint(path);
    this.scheduleRecovery(path);
    this.scheduleAutoSave(path);
  }

  // ---- Automatic saving ------------------------------------------------------

  /** Supplies the current auto-save preference; set by the workbench. */
  autoSave: () => { mode: 'off' | 'delay' | 'focus'; delay: number } = () => ({ mode: 'off', delay: 1000 });
  private autoSaveTimers = new Map<string, ReturnType<typeof setTimeout>>();

  private scheduleAutoSave(path: string): void {
    clearTimeout(this.autoSaveTimers.get(path));
    const { mode, delay } = this.autoSave();
    if (mode !== 'delay') return;
    this.autoSaveTimers.set(path, setTimeout(() => void this.autoSaveNow(path), delay));
  }

  /** Saves one file automatically, unless it needs the user's attention. */
  private async autoSaveNow(path: string): Promise<void> {
    const tab = this.tab(path);
    // A file changed on disk by another program is never overwritten silently.
    if (!tab || !tab.dirty || tab.external) return;
    await this.save(path);
  }

  /** Called when the editor or the window loses focus. */
  focusLost(): void {
    if (this.autoSave().mode !== 'focus') return;
    for (const tab of this.dirtyTabs) void this.autoSaveNow(tab.path);
  }

  /** Replaces a tab's content from outside the editor (reload, recovery). */
  private replace(path: string, text: string, savedText: string | null): void {
    const tab = this.tab(path);
    if (!tab) return;
    const state = this.stateFactory(path, text);
    this.states.set(path, state);
    if (savedText !== null) this.saved.set(path, Text.of(savedText.split(/\r\n|\r|\n/)));
    const dirty = !state.doc.eq(this.saved.get(path) ?? Text.empty);
    this.patch(path, { dirty, version: tab.version + 1, epoch: tab.epoch + 1, external: null });
    this.scheduleLint(path);
  }

  // ---- Opening and closing --------------------------------------------------

  async open(path: string, position?: { line: number; column?: number }): Promise<boolean> {
    if (!this.workspace.project) return false;
    if (isImagePath(path)) return this.openImage(path);
    if (!this.tab(path)) {
      let content: string;
      try {
        content = await native().studio.ReadFile(path);
      } catch (error) {
        this.ui.fail(`Opening ${path}`, error);
        return false;
      }
      if (!this.tab(path)) {
        this.states.set(path, this.stateFactory(path, content));
        this.saved.set(path, this.states.get(path)!.doc);
        this.tabs = [...this.tabs, { path, kind: 'text', dirty: false, version: 0, external: null, epoch: 0 }];
        this.scheduleLint(path, 0);
      }
    }
    this.activate(path);
    if (position) this.reveal = { path, line: position.line, column: position.column ?? 1, token: ++this.revealToken };
    return true;
  }

  /** Opens a browser-supported image without treating it as sprite data. */
  async openImage(path: string): Promise<boolean> {
    if (!this.workspace.project) return false;
    if (!this.tab(path)) {
      try {
        const data = await native().studio.ReadBinaryFile(path);
        this.tabs = [...this.tabs, { path, kind: 'image', image: `data:${imageMime(path)};base64,${data}`, dirty: false, version: 0, external: null, epoch: 0 }];
      } catch (error) {
        this.ui.fail(`Opening ${path}`, error);
        return false;
      }
    }
    this.activate(path);
    return true;
  }

  /** Pretty-prints a JSON document and leaves it as an ordinary unsaved edit. */
  formatJson(path: string): void {
    const tab = this.tab(path);
    const state = this.states.get(path);
    if (!tab || tab.kind !== 'text' || !state || !isJsonPath(path)) return;
    try {
      const formatted = JSON.stringify(JSON.parse(state.doc.toString()), null, 2) + '\n';
      if (formatted === state.doc.toString()) return;
      const next = this.stateFactory(path, formatted);
      this.states.set(path, next);
      this.patch(path, { dirty: !next.doc.eq(this.saved.get(path) ?? Text.empty), version: tab.version + 1, epoch: tab.epoch + 1 });
      this.scheduleRecovery(path);
      this.scheduleAutoSave(path);
    } catch (error) {
      this.ui.fail(`Formatting ${path}`, error);
    }
  }

  activate(path: string): void {
    if (path !== this.active) this.focusLost();
    this.active = path;
    if (isVirtualTab(path)) this.workspace.selected = this.virtual.path(path);
    else if (path && path !== EMULATOR_TAB) {
      this.workspace.selected = path;
      this.record(path);
    }
    this.schedulePersist();
  }

  showWelcome(): void { this.active = ''; }

  openEmulator(): void { this.emulatorOpen = true; this.active = EMULATOR_TAB; }

  closeEmulator(): void {
    this.emulatorOpen = false;
    if (this.active === EMULATOR_TAB) this.active = this.tabs.at(-1)?.path ?? this.virtual.ids().at(-1) ?? '';
  }

  /** Closes a tab, asking what to do with unsaved changes. */
  async close(path: string): Promise<boolean> {
    const tab = this.tab(path);
    if (!tab) return true;
    if (tab.dirty) {
      const answer = await this.ui.choose({
        title: 'Save changes?',
        message: `${path} has unsaved changes.`,
        detail: 'Your changes will be lost if you do not save them.',
        buttons: [{ value: 'cancel', label: 'Cancel' }, { value: 'discard', label: "Don't Save", kind: 'danger' }, { value: 'save', label: 'Save', kind: 'primary' }],
        cancelValue: 'cancel',
      });
      if (answer === 'cancel') return false;
      if (answer === 'save' && !(await this.save(path))) return false;
    }
    this.drop(path);
    return true;
  }

  private drop(path: string): void {
    const index = this.tabs.findIndex(tab => tab.path === path);
    if (index < 0) return;
    this.tabs = this.tabs.filter(tab => tab.path !== path);
    this.states.delete(path);
    this.saved.delete(path);
    clearTimeout(this.lintTimers.get(path));
    clearTimeout(this.recoveryTimers.get(path));
    this.problems.clearLint(path);
    void native().studio.DiscardRecovery(path).catch(() => {});
    if (this.active === path) {
      const next = this.tabs[Math.min(index, this.tabs.length - 1)];
      this.active = next?.path ?? this.virtual.ids()[0] ?? (this.emulatorOpen ? EMULATOR_TAB : '');
      if (!next) this.splitOpen = false;
    }
    this.schedulePersist();
  }

  /**
   * Asks before discarding unsaved work when the workspace is about to be
   * replaced. Resolves false when the user cancels.
   */
  async confirmDiscardAll(): Promise<boolean> {
    const dirty = [...this.dirtyTabs.map(tab => tab.path), ...this.virtual.dirty().map(id => this.virtual.path(id))];
    if (!dirty.length) return true;
    const answer = await this.ui.choose({
      title: 'Save changes?',
      message: `${dirty.length} file${dirty.length === 1 ? ' has' : 's have'} unsaved changes.`,
      detail: dirty.join('\n'),
      buttons: [{ value: 'cancel', label: 'Cancel' }, { value: 'discard', label: "Don't Save", kind: 'danger' }, { value: 'save', label: 'Save All', kind: 'primary' }],
      cancelValue: 'cancel',
    });
    if (answer === 'cancel') return false;
    if (answer === 'save') return this.saveAll();
    return true;
  }

  /** Forgets every tab without prompting (after confirmDiscardAll). */
  reset(): void {
    for (const timer of [...this.lintTimers.values(), ...this.recoveryTimers.values()]) clearTimeout(timer);
    this.lintTimers.clear(); this.recoveryTimers.clear();
    this.tabs = []; this.states.clear(); this.saved.clear();
    this.active = ''; this.splitOpen = false;
    this.history = []; this.historyIndex = -1;
    this.historyState = { back: false, forward: false };
    this.problems.reset();
  }

  // ---- Saving ---------------------------------------------------------------

  async save(path: string): Promise<boolean> {
    const state = this.states.get(path);
    const tab = this.tab(path);
    if (tab?.kind === 'image') return true;
    if (!state || !tab) return false;
    const content = state.doc.toString();
    try {
      await native().studio.SaveFile(path, content);
    } catch (error) {
      this.ui.fail(`Saving ${path}`, error);
      this.ui.showPanel('output');
      return false;
    }
    this.saved.set(path, state.doc);
    clearTimeout(this.recoveryTimers.get(path));
    // The user may have typed while the save was in flight.
    const latest = this.states.get(path);
    this.patch(path, { dirty: latest ? !latest.doc.eq(state.doc) : false, external: null });
    this.ui.status = `Saved ${path}`;
    return true;
  }

  async saveAll(): Promise<boolean> {
    let ok = true;
    for (const tab of this.dirtyTabs) ok = (await this.save(tab.path)) && ok;
    for (const id of this.virtual.dirty()) ok = (await this.virtual.save(id)) && ok;
    return ok;
  }

  // ---- Lint -----------------------------------------------------------------

  private scheduleLint(path: string, delay = LINT_DELAY): void {
    clearTimeout(this.lintTimers.get(path));
    if (!isExbasicSource(path)) return;
    // Programs are what the compiler checks. A change to any source file,
    // including an INCLUDE file, re-checks every open program.
    for (const tab of this.tabs) {
      if (!tab.path.toLowerCase().endsWith('.bas')) continue;
      clearTimeout(this.lintTimers.get(tab.path));
      this.lintTimers.set(tab.path, setTimeout(() => void this.runLint(tab.path), delay));
    }
  }

  private async runLint(path: string): Promise<void> {
    const tab = this.tab(path);
    if (!tab || this.workspace.restricted) { this.problems.clearLint(path); return; }
    const version = tab.version;
    try {
      // Other open files with unsaved edits are checked as they appear on
      // screen, so fixing an INCLUDE file clears its error before saving.
      const overlays: Record<string, string> = {};
      for (const other of this.dirtyTabs) {
        if (other.path !== path && isExbasicSource(other.path)) overlays[other.path] = this.text(other.path);
      }
      const result = await native().studio.Lint(path, this.text(path), version, overlays);
      // Drop a result computed for text that has since changed.
      const current = this.tab(path);
      if (!current || current.version !== result.version) return;
      if (result.available) this.problems.setLint(path, withIncludeMarkers(path, this.text(path), result.diagnostics ?? []));
      else this.problems.clearLint(path);
    } catch (error) {
      this.ui.status = `Lint unavailable: ${errorMessage(error)}`;
    }
  }

  relintAll(): void { for (const tab of this.tabs) this.scheduleLint(tab.path, 0); }

  // ---- Crash recovery -------------------------------------------------------

  private scheduleRecovery(path: string): void {
    clearTimeout(this.recoveryTimers.get(path));
    this.recoveryTimers.set(path, setTimeout(() => {
      const tab = this.tab(path);
      if (!tab) return;
      const call = tab.dirty ? native().studio.SaveRecovery(path, this.text(path)) : native().studio.DiscardRecovery(path);
      void call.catch(() => {});
    }, RECOVERY_DELAY));
  }

  /** Offers to restore unsaved work left behind by a crash. */
  async offerRecovery(): Promise<void> {
    let recoveries;
    try { recoveries = await native().studio.Recoveries(); } catch { return; }
    if (!recoveries?.length) return;
    const answer = await this.ui.choose({
      title: 'Restore unsaved changes?',
      message: `exBASIC Studio closed with unsaved changes in ${recoveries.length} file${recoveries.length === 1 ? '' : 's'}.`,
      detail: recoveries.map(item => item.path).join('\n'),
      buttons: [{ value: 'later', label: 'Decide Later' }, { value: 'discard', label: 'Discard', kind: 'danger' }, { value: 'restore', label: 'Restore', kind: 'primary' }],
      cancelValue: 'later',
    });
    if (answer === 'later') return;
    for (const item of recoveries) {
      if (answer === 'discard') { await native().studio.DiscardRecovery(item.path).catch(() => {}); continue; }
      let onDisk = '';
      let exists = true;
      try { onDisk = await native().studio.ReadFile(item.path); } catch { exists = false; }
      if (!this.tab(item.path)) {
        this.states.set(item.path, this.stateFactory(item.path, item.content));
        this.saved.set(item.path, exists ? Text.of(onDisk.split(/\r\n|\r|\n/)) : Text.empty);
        this.tabs = [...this.tabs, { path: item.path, kind: 'text', dirty: true, version: 1, external: null, epoch: 0 }];
      } else {
        this.replace(item.path, item.content, null);
      }
      this.activate(item.path);
    }
  }

  // ---- External changes -----------------------------------------------------

  /** Reacts to files changed outside the editor. */
  async handleExternal(batch: WorkspaceBatch): Promise<void> {
    const changes = batch.overflow
      ? this.tabs.map(tab => ({ path: tab.path, kind: 'write' as const, directory: false, size: 0 }))
      : batch.changes.filter(change => !change.directory);
    for (const change of changes) {
      const tab = this.tab(change.path);
      if (!tab) continue;
      if (tab.kind === 'image') {
        if (change.kind === 'remove') { this.patch(change.path, { external: 'deleted' }); continue; }
        try {
          const data = await native().studio.ReadBinaryFile(change.path);
          this.patch(change.path, { image: `data:${imageMime(change.path)};base64,${data}`, external: null, version: tab.version + 1, epoch: tab.epoch + 1 });
        } catch { this.patch(change.path, { external: 'deleted' }); }
        continue;
      }
      let content: string | null = null;
      try { content = await native().studio.ReadFile(change.path); } catch { content = null; }
      const current = this.tab(change.path);
      if (!current) continue;
      if (content === null) {
        // Deleted (or unreadable). Keep the buffer so nothing is lost.
        if (change.kind === 'remove' || batch.overflow) this.patch(change.path, { external: 'deleted', dirty: true });
        continue;
      }
      if (content === this.text(change.path)) {
        // Disk now matches the buffer (typically our own save).
        this.saved.set(change.path, this.states.get(change.path)!.doc);
        this.patch(change.path, { dirty: false, external: null });
        continue;
      }
      if (content === this.saved.get(change.path)?.toString()) continue;
      if (!current.dirty) this.replace(change.path, content, content);
      else this.patch(change.path, { external: 'modified' });
    }
  }

  /** Resolves an external-change banner: take the disk version or keep ours. */
  async resolveExternal(path: string, choice: 'reload' | 'keep'): Promise<void> {
    const tab = this.tab(path);
    if (!tab) return;
    if (choice === 'keep') { this.patch(path, { external: null }); return; }
    try {
      if (tab.kind === 'image') {
        const data = await native().studio.ReadBinaryFile(path);
        this.patch(path, { image: `data:${imageMime(path)};base64,${data}`, external: null, version: tab.version + 1, epoch: tab.epoch + 1 });
        return;
      }
      const content = await native().studio.ReadFile(path);
      this.replace(path, content, content);
    } catch (error) {
      this.ui.fail(`Reloading ${path}`, error);
    }
  }

  // ---- Renames --------------------------------------------------------------

  applyRename(oldPath: string, newPath: string): void {
    const moved = (path: string) => (path === oldPath || path.startsWith(oldPath + '/') ? newPath + path.slice(oldPath.length) : path);
    for (const tab of this.tabs) {
      const target = moved(tab.path);
      if (target === tab.path) continue;
      const state = this.states.get(tab.path);
      const saved = this.saved.get(tab.path);
      if (state) this.states.set(target, state);
      if (saved) this.saved.set(target, saved);
      this.states.delete(tab.path); this.saved.delete(tab.path);
    }
    this.tabs = this.tabs.map(tab => {
      const path = moved(tab.path);
      const image = tab.kind === 'image' && tab.image && path !== tab.path ? tab.image.replace(/^data:[^;]+;/, `data:${imageMime(path)};`) : tab.image;
      return { ...tab, path, image };
    });
    this.active = moved(this.active);
    this.history = this.history.map(moved);
    this.problems.renamePath(oldPath, newPath);
    this.schedulePersist();
  }

  /** Drops tabs of deleted files without prompting. */
  applyDelete(path: string): void {
    for (const tab of this.tabs.filter(item => item.path === path || item.path.startsWith(path + '/'))) this.drop(tab.path);
  }

  // ---- Navigation history ---------------------------------------------------

  private record(path: string): void {
    if (this.navigating || this.history[this.historyIndex] === path) return;
    this.history = [...this.history.slice(0, this.historyIndex + 1), path].slice(-50);
    this.historyIndex = this.history.length - 1;
    this.syncHistory();
  }

  private syncHistory(): void {
    this.historyState = { back: this.historyIndex > 0, forward: this.historyIndex >= 0 && this.historyIndex < this.history.length - 1 };
  }

  async navigate(delta: number): Promise<void> {
    const next = this.historyIndex + delta;
    if (next < 0 || next >= this.history.length) return;
    this.navigating = true;
    try {
      if (await this.open(this.history[next])) this.historyIndex = next;
    } finally {
      this.navigating = false;
      this.syncHistory();
    }
  }

  // ---- Persistence ----------------------------------------------------------

  schedulePersist(): void {
    clearTimeout(this.persistTimer);
    if (!this.workspace.project) return;
    this.persistTimer = setTimeout(() => {
      const state: WorkspaceState = {
        tabs: [...this.tabs.map(tab => tab.path), ...this.virtual.ids()],
        activeEditor: this.active === EMULATOR_TAB ? '' : this.active,
        emulatorProfile: '', breakpoints: [], watches: [],
        ...this.extraState(),
      };
      void native().studio.SaveWorkspaceState(state).catch(() => {});
    }, PERSIST_DELAY);
  }

  /** Reopens the tabs remembered for this workspace. */
  async restore(saved: WorkspaceState | null, fallback: string): Promise<void> {
    const wanted = (saved?.tabs ?? []).filter(path => this.workspace.hasFile(isVirtualTab(path) ? this.virtual.path(path) : path));
    for (const path of wanted) {
      if (isVirtualTab(path)) await this.virtual.open(path); else await this.open(path);
    }
    const isOpen = (id: string) => !!this.tab(id) || this.virtual.ids().includes(id);
    const active = saved?.activeEditor && isOpen(saved.activeEditor) ? saved.activeEditor : wanted.filter(isOpen).at(-1);
    if (active) this.activate(active);
    else if (fallback && this.workspace.hasFile(fallback)) await this.open(fallback);
  }
}
