// The workbench composes every store, owns cross-cutting workflows (open,
// build, run, go to definition) and defines the command set shared by the
// menus, the command palette and the keyboard.
import { PreferencesStore } from './stores/preferences.svelte';
import { EditorState } from '@codemirror/state';
import { native, errorMessage, type BuildArtifact, type BuildResult, type ProjectView, type RunResult, type SearchResult } from './native';
import { UIStore, type Activity } from './stores/ui.svelte';
import { WorkspaceStore, baseName, parentOf, normalizeRel } from './stores/workspace.svelte';
import { EditorsStore, ProblemsStore, EMULATOR_TAB } from './stores/editors.svelte';
import { OperationsStore } from './stores/operations.svelte';
import { AtrStore, EmulatorStore, GitStore, ManualStore, TerminalStore } from './stores/services.svelte';
import { DebugStore } from './stores/debug.svelte';
import { SpritesStore } from './stores/sprites.svelte';
import { isSpritePath } from './sprites/model';
import { editorExtensions, type EditorCallbacks } from './editor/setup';
import { scanSymbols, findDefinition, wordAt, includeAt, isExbasicSource, type DocSymbol, type LanguageHost } from './language';

export type CommandID =
  | 'palette' | 'quickOpen' | 'symbols' | 'file.open' | 'file.close' | 'file.new' | 'file.newFolder' | 'file.rename' | 'file.delete'
  | 'file.save' | 'file.saveAll' | 'editor.close' | 'editor.undo' | 'editor.redo' | 'editor.split' | 'editor.copyPath'
  | 'editor.selectAll' | 'editor.find' | 'editor.definition' | 'nav.back' | 'nav.forward'
  | 'view.explorer' | 'view.atr' | 'view.search' | 'view.source' | 'view.run' | 'view.panel' | 'view.problems' | 'view.output' | 'view.keyboard' | 'view.sidebar'
  | 'build' | 'run' | 'build.cancel' | 'debug.start' | 'debug.continue' | 'debug.pause' | 'debug.stepLine' | 'debug.stepInstruction'
  | 'debug.stepOver' | 'debug.stepOut' | 'debug.stop' | 'debug.toggleBreakpoint'
  | 'emulator.open' | 'emulator.start' | 'emulator.restart' | 'emulator.stop' | 'emulator.joystick'
  | 'terminal.new' | 'terminal.focus' | 'terminal.kill' | 'manual' | 'settings' | 'trust' | 'support.bundle'
  | 'view.sprites' | 'sprites.new' | 'sprites.merge' | 'sprites.open' | 'sprites.openText' | 'sprites.generate';

export interface Command { id: CommandID; label: string; shortcut?: string; icon?: string }

export const commands: Command[] = [
  { id: 'file.open', label: 'File: Open Folder…', shortcut: 'Ctrl+K Ctrl+O', icon: 'folder-opened' },
  { id: 'file.close', label: 'File: Close Folder', icon: 'close-all' },
  { id: 'file.new', label: 'File: New File…', shortcut: 'Ctrl+N', icon: 'new-file' },
  { id: 'file.newFolder', label: 'File: New Folder…', icon: 'new-folder' },
  { id: 'file.rename', label: 'File: Rename Selected…', shortcut: 'F2', icon: 'edit' },
  { id: 'file.delete', label: 'File: Delete Selected', icon: 'trash' },
  { id: 'file.save', label: 'File: Save', shortcut: 'Ctrl+S', icon: 'save' },
  { id: 'file.saveAll', label: 'File: Save All', shortcut: 'Ctrl+K S', icon: 'save-all' },
  { id: 'editor.close', label: 'File: Close Editor', shortcut: 'Ctrl+W', icon: 'close' },
  { id: 'editor.undo', label: 'Edit: Undo', shortcut: 'Ctrl+Z', icon: 'discard' },
  { id: 'editor.redo', label: 'Edit: Redo', shortcut: 'Ctrl+Y', icon: 'redo' },
  { id: 'editor.find', label: 'Edit: Find in File', shortcut: 'Ctrl+F', icon: 'search' },
  { id: 'editor.selectAll', label: 'Selection: Select All', shortcut: 'Ctrl+A', icon: 'selection' },
  { id: 'editor.copyPath', label: 'File: Copy Path', icon: 'copy' },
  { id: 'editor.split', label: 'View: Toggle Editor Split', shortcut: 'Ctrl+\\', icon: 'split-horizontal' },
  { id: 'editor.definition', label: 'Go: Go to Definition', shortcut: 'F12', icon: 'references' },
  { id: 'quickOpen', label: 'Go: Go to File…', shortcut: 'Ctrl+P', icon: 'go-to-file' },
  { id: 'symbols', label: 'Go: Go to Symbol in Workspace…', shortcut: 'Ctrl+T', icon: 'symbol-class' },
  { id: 'nav.back', label: 'Go: Back', shortcut: 'Alt+Left', icon: 'arrow-left' },
  { id: 'nav.forward', label: 'Go: Forward', shortcut: 'Alt+Right', icon: 'arrow-right' },
  { id: 'view.explorer', label: 'View: Show Explorer', shortcut: 'Ctrl+Shift+E', icon: 'files' },
  { id: 'view.search', label: 'View: Show Search', shortcut: 'Ctrl+Shift+F', icon: 'search' },
  { id: 'view.atr', label: 'View: Show ATR Explorer', icon: 'atari-folder' },
  { id: 'view.source', label: 'View: Show Source Control', shortcut: 'Ctrl+Shift+G', icon: 'source-control' },
  { id: 'view.run', label: 'View: Show Run and Debug', shortcut: 'Ctrl+Shift+D', icon: 'debug-alt' },
  { id: 'view.sidebar', label: 'View: Toggle Side Bar', shortcut: 'Ctrl+B', icon: 'layout-sidebar-left' },
  { id: 'view.panel', label: 'View: Toggle Panel', shortcut: 'Ctrl+J', icon: 'layout-panel' },
  { id: 'view.problems', label: 'View: Show Problems', shortcut: 'Ctrl+Shift+M', icon: 'error' },
  { id: 'view.output', label: 'View: Show Output', icon: 'output' },
  { id: 'view.keyboard', label: 'View: Show Atari Keyboard Map', icon: 'keyboard' },
  { id: 'build', label: 'Tasks: Build exBASIC Project', shortcut: 'F6', icon: 'tools' },
  { id: 'run', label: 'Run: Run Without Debugging', shortcut: 'Ctrl+F5', icon: 'play' },
  { id: 'build.cancel', label: 'Tasks: Cancel Build', icon: 'debug-stop' },
  { id: 'debug.start', label: 'Debug: Start Debugging', shortcut: 'F5', icon: 'debug-start' },
  { id: 'debug.continue', label: 'Debug: Continue', shortcut: 'F5', icon: 'debug-continue' },
  { id: 'debug.pause', label: 'Debug: Pause', shortcut: 'F8', icon: 'debug-pause' },
  { id: 'debug.stepLine', label: 'Debug: Step to Next Source Line', shortcut: 'F10', icon: 'debug-step-over' },
  { id: 'debug.stepInstruction', label: 'Debug: Step One 6502 Instruction', shortcut: 'F11', icon: 'debug-step-into' },
  { id: 'debug.stepOver', label: 'Debug: Step Over JSR', shortcut: 'Shift+F10', icon: 'debug-step-over' },
  { id: 'debug.stepOut', label: 'Debug: Step Out', shortcut: 'Shift+F11', icon: 'debug-step-out' },
  { id: 'debug.stop', label: 'Debug: Stop', shortcut: 'Shift+F5', icon: 'debug-stop' },
  { id: 'debug.toggleBreakpoint', label: 'Debug: Toggle Breakpoint', shortcut: 'F9', icon: 'debug-breakpoint' },
  { id: 'emulator.open', label: 'ex800: Open Emulator', icon: 'device-desktop' },
  { id: 'emulator.start', label: 'ex800: Start', icon: 'play' },
  { id: 'emulator.restart', label: 'ex800: Restart', icon: 'debug-restart' },
  { id: 'emulator.stop', label: 'ex800: Stop', icon: 'debug-stop' },
  { id: 'emulator.joystick', label: 'ex800: Toggle Keyboard Joystick', icon: 'game' },
  { id: 'terminal.new', label: 'Terminal: New Terminal', shortcut: 'Ctrl+Shift+`', icon: 'terminal' },
  { id: 'terminal.focus', label: 'Terminal: Focus Terminal', shortcut: 'Ctrl+`', icon: 'terminal' },
  { id: 'terminal.kill', label: 'Terminal: Kill Active Terminal', icon: 'trash' },
  { id: 'view.sprites', label: 'View: Show Sprite Studio', icon: 'paintcan' },
  { id: 'sprites.new', label: 'Sprites: New Sprite Sheet…', icon: 'new-file' },
  { id: 'sprites.merge', label: 'Sprites: Merge Sprite Sheets…', icon: 'merge' },
  { id: 'sprites.open', label: 'Sprites: Open in Sprite Studio', icon: 'paintcan' },
  { id: 'sprites.openText', label: 'Sprites: Open Sheet as JSON Text', icon: 'json' },
  { id: 'sprites.generate', label: 'Sprites: Generate PMG Bank and Outputs', icon: 'run-all' },
  { id: 'manual', label: 'Help: Open exBASIC Manual', shortcut: 'F1', icon: 'book' },
  { id: 'settings', label: 'Preferences: Open Settings', shortcut: 'Ctrl+,', icon: 'gear' },
  { id: 'trust', label: 'Workspace: Manage Folder Trust', icon: 'shield' },
  { id: 'support.bundle', label: 'Help: Export Support Bundle…', icon: 'archive' },
  { id: 'palette', label: 'View: Command Palette', shortcut: 'Ctrl+Shift+P', icon: 'symbol-event' },
];

export const menus: Record<string, CommandID[]> = {
  File: ['file.new', 'file.newFolder', 'file.open', 'file.close', 'file.save', 'file.saveAll', 'editor.close'],
  Edit: ['editor.undo', 'editor.redo', 'editor.find', 'editor.selectAll', 'palette'],
  View: ['view.explorer', 'view.search', 'view.atr', 'view.source', 'view.run', 'view.sprites', 'view.sidebar', 'view.panel', 'view.problems', 'view.output', 'view.keyboard', 'editor.split'],
  Go: ['nav.back', 'nav.forward', 'quickOpen', 'symbols', 'editor.definition'],
  Run: ['debug.start', 'run', 'build', 'build.cancel', 'debug.stop', 'debug.toggleBreakpoint', 'emulator.open', 'emulator.start', 'emulator.restart', 'emulator.stop', 'emulator.joystick'],
  Sprites: ['sprites.new', 'sprites.open', 'sprites.openText', 'sprites.generate', 'sprites.merge'],
  Terminal: ['terminal.new', 'terminal.focus', 'terminal.kill'],
  Help: ['manual', 'settings', 'trust', 'support.bundle'],
};

export type PaletteMode = 'commands' | 'files' | 'symbols';
export interface PaletteItem { key: string; label: string; detail?: string; icon: string; shortcut?: string; disabled?: boolean; run(): void }

const SYMBOL_INDEX_LIMIT = 300;
const SYMBOL_FILE_LIMIT = 512 * 1024;

export class Workbench {
  ui = new UIStore();
  prefs = new PreferencesStore();
  workspace = new WorkspaceStore(this.ui);
  problems = new ProblemsStore();
  editors = new EditorsStore(this.ui, this.workspace, this.problems);
  operations = new OperationsStore(this.ui);
  git = new GitStore(this.ui, this.workspace, this.operations);
  emulator = new EmulatorStore(this.ui);
  terminal = new TerminalStore(this.ui, this.workspace);
  atr = new AtrStore(this.ui, this.workspace);
  manual = new ManualStore(this.ui);
  debug = new DebugStore(this.ui, this.workspace, this.editors, this.emulator);
  sprites = new SpritesStore(this.ui, this.workspace, this.editors);

  search = $state<{ query: string; results: SearchResult[]; ran: boolean }>({ query: '', results: [], ran: false });
  palette = $state<{ open: boolean; mode: PaletteMode; query: string; index: number }>({ open: false, mode: 'commands', query: '', index: 0 });
  /** Inline "new file/folder" inputs in the explorer. */
  creating = $state<{ kind: 'file' | 'folder'; value: string } | null>(null);
  lastBuild = $state<BuildResult | null>(null);
  ready = $state(false);

  /** Symbols of every indexed .bas file in the workspace. */
  private symbolIndex = new Map<string, DocSymbol[]>();
  private pendingChord = '';
  private chordTimer: ReturnType<typeof setTimeout> | undefined;
  private paletteFocus: Element | null = null;
  private unsubscribeDrops: (() => void) | undefined;
  /** Set by the editor pane: runs a named editor action on the active view. */
  editorAction: (action: 'undo' | 'redo' | 'selectAll' | 'find' | 'focus' | 'definition') => boolean = () => false;
  /** Set by the terminal panel: focuses the active terminal. */
  focusTerminal: () => void = () => {};

  readonly languageHost: LanguageHost = {
    workspaceSymbols: () => {
      const current = this.editors.active;
      const out: { path: string; symbol: DocSymbol }[] = [];
      for (const [path, symbols] of this.symbolIndex) {
        if (path === current) continue;
        for (const symbol of symbols) out.push({ path, symbol });
      }
      return out;
    },
    includeCandidates: fromPath => {
      const directory = parentOf(fromPath);
      const includeRoots = this.workspace.project?.config.includePaths ?? [];
      const names = new Set<string>();
      for (const file of this.workspace.project?.files ?? []) {
        if (!isExbasicSource(file.path) || file.path === fromPath) continue;
        // Offer the spelling the compiler resolves: relative to the including
        // file, or relative to an include path.
        if (directory === '' || file.path.startsWith(directory + '/')) names.add(directory ? file.path.slice(directory.length + 1) : file.path);
        for (const root of includeRoots) if (file.path.startsWith(root + '/')) names.add(file.path.slice(root.length + 1));
      }
      return [...names].sort();
    },
    currentPath: () => this.editors.active,
    openManual: (chapter, anchor) => void this.manual.open(chapter, anchor),
  };

  private readonly editorCallbacks: EditorCallbacks = {
    onUpdate: (path, state, docChanged) => {
      this.editors.update(path, state, docChanged);
      if (path === this.editors.active) {
        const head = state.selection.main.head;
        const line = state.doc.lineAt(head);
        this.editors.cursor = { line: line.number, column: head - line.from + 1 };
      }
      if (docChanged) {
        if (isExbasicSource(path)) this.symbolIndex.set(path, scanSymbols(state.doc.toString()));
        this.debug.documentChanged(path, state);
      }
    },
    onToggleBreakpoint: (path, line) => this.debug.toggleBreakpoint(path, line),
    onGoToDefinition: (path, state, pos) => void this.goToDefinition(path, state, pos),
    onSave: path => void this.editors.save(path),
  };

  constructor() {
    this.editors.stateFactory = (path, text) => EditorState.create({ doc: text, extensions: editorExtensions(path, this.languageHost, this.editorCallbacks) });
    this.editors.extraState = () => ({ emulatorProfile: this.emulator.profileId, breakpoints: this.debug.persistedBreakpoints, watches: this.debug.watches.map(watch => watch.expression), keyboardJoystick: this.emulator.joystickMode });
    this.emulator.onJoystickModeChanged = () => this.editors.schedulePersist();
    this.editors.virtual = this.sprites.host;
    this.workspace.onProjectChanged = view => this.projectChanged(view);
    this.workspace.onFilesChanged = batch => {
      void this.editors.handleExternal(batch);
      void this.sprites.handleExternal(batch);
      for (const change of batch.changes) {
        if (change.directory || !isExbasicSource(change.path)) continue;
        if (change.kind === 'remove') this.symbolIndex.delete(change.path);
        else if (!this.editors.tab(change.path)) void this.indexFile(change.path);
      }
      if (batch.overflow) void this.buildSymbolIndex();
    };
    this.emulator.onWorkspaceChanged = () => void this.workspace.refresh();
  }

  /** Creates a read-only state showing the same document (split preview). */
  splitState(path: string, text: string): EditorState {
    return EditorState.create({ doc: text, extensions: editorExtensions(path, this.languageHost, { ...this.editorCallbacks, onUpdate: () => {} }) });
  }

  async start(): Promise<void> {
    this.operations.connect();
    this.terminal.connect();
    this.debug.connect();
    this.unsubscribeDrops = native().on('files-dropped', event => void this.filesDropped(event.files, event.targetId));
    try {
      await this.workspace.init();
      this.prefs.load(this.workspace.storedPreferences);
      this.editors.autoSave = () => ({ mode: this.prefs.values.autoSave, delay: this.prefs.values.autoSaveDelay });
      await Promise.all([this.emulator.load(), this.manual.load()]);
    } catch (error) {
      this.ui.status = 'Unable to connect to the backend';
      this.ui.appendOutput(errorMessage(error) + '\n');
      this.ui.showPanel('output');
    }
    this.ready = true;
    // Startup: reopen the folder that was open last, if the user wants that.
    const last = this.workspace.recents[0];
    if (this.prefs.values.startup === 'reopen' && last && !this.workspace.project) await this.workspace.open(last.path);
  }

  dispose(): void {
    this.operations.disconnect(); this.terminal.disconnect(); this.debug.disconnect(); this.workspace.dispose();
    this.unsubscribeDrops?.();
    this.prefs.dispose();
    clearTimeout(this.chordTimer);
  }

  // ---- Workspace lifecycle --------------------------------------------------

  private async projectChanged(view: ProjectView | null): Promise<void> {
    await this.debug.end(true);
    this.editors.reset();
    this.sprites.reset();
    this.terminal.reset();
    this.git.reset();
    this.symbolIndex.clear();
    this.search = { query: '', results: [], ran: false };
    this.lastBuild = null;
    this.atr.view = null;
    if (this.emulator.running) { this.emulator.running = false; this.emulator.image = ''; }
    if (!view) return;
    const saved = this.workspace.saved;
    this.debug.restore(saved?.breakpoints ?? [], saved?.watches ?? []);
    if (saved?.emulatorProfile) this.emulator.select(saved.emulatorProfile);
    this.emulator.joystickMode = saved?.keyboardJoystick ?? false;
    await this.editors.restore(saved, view.config.entry);
    void this.buildSymbolIndex();
    void this.git.autoRefresh();
    await this.editors.offerRecovery();
  }

  async openFolder(path: string): Promise<void> {
    if (!path.trim() || !(await this.editors.confirmDiscardAll())) return;
    await this.workspace.open(path);
  }

  async chooseFolder(): Promise<void> {
    const selected = await native().dialogs.open({ title: 'Open Folder', directory: this.workspace.root || this.workspace.env?.workingDir, chooseDirectories: true, chooseFiles: false });
    if (selected[0]) await this.openFolder(selected[0]);
  }

  async createProject(parent: string, name: string): Promise<void> {
    if (!(await this.editors.confirmDiscardAll())) return;
    await this.workspace.create(parent, name);
  }

  async closeFolder(): Promise<void> {
    if (!this.workspace.project || !(await this.editors.confirmDiscardAll())) return;
    await this.workspace.close();
  }

  /** Opens a file in the editor that suits it: sprite files go to the Sprite Studio. */
  async openFile(path: string): Promise<boolean> {
    if (isSpritePath(path)) return this.sprites.open(path);
    return this.editors.open(path);
  }

  /** The file behind the active editor or the explorer selection. */
  get spriteCandidate(): string {
    const active = this.editors.active;
    if (this.sprites.doc(active)) return '';
    if (this.editors.activeTab && /\.json$/i.test(active)) return active;
    const selected = this.workspace.selected;
    return selected && this.workspace.hasFile(selected) && (/\.json$/i.test(selected) || isSpritePath(selected)) ? selected : '';
  }

  /** Opens the active sprite sheet's JSON in the text editor. */
  async openSpriteText(): Promise<void> {
    const doc = this.sprites.active;
    if (!doc) return;
    const path = doc.sheetPath ?? doc.framesPath ?? doc.bank?.index;
    if (!path) { this.ui.status = 'This sprite has no JSON file yet: save it as a sheet first'; return; }
    if (doc.dirty && !(await this.sprites.save(doc))) return;
    await this.editors.open(path);
  }

  // ---- Symbols and navigation -----------------------------------------------

  private async indexFile(path: string): Promise<void> {
    try {
      this.symbolIndex.set(path, scanSymbols(await native().studio.ReadFile(path)));
    } catch {
      this.symbolIndex.delete(path);
    }
  }

  private async buildSymbolIndex(): Promise<void> {
    const root = this.workspace.root;
    const files = (this.workspace.project?.files ?? []).filter(file => isExbasicSource(file.path) && file.size <= SYMBOL_FILE_LIMIT).slice(0, SYMBOL_INDEX_LIMIT);
    for (const file of files) {
      if (this.workspace.root !== root) return; // the workspace changed meanwhile
      const open = this.editors.stateFor(file.path);
      if (open) this.symbolIndex.set(file.path, scanSymbols(open.doc.toString()));
      else await this.indexFile(file.path);
    }
  }

  /** Every function, array and DATA definition known in the workspace. */
  get allSymbols(): { path: string; symbol: DocSymbol }[] {
    const out: { path: string; symbol: DocSymbol }[] = [];
    for (const [path, symbols] of this.symbolIndex) {
      for (const symbol of symbols) if (symbol.kind !== 'parameter' && symbol.kind !== 'include') out.push({ path, symbol });
    }
    return out.sort((a, b) => a.symbol.name.localeCompare(b.symbol.name) || a.path.localeCompare(b.path));
  }

  /** Resolves an INCLUDE name the way the compiler does. */
  resolveInclude(fromPath: string, name: string): string | null {
    const clean = normalizeRel(name);
    const candidates = [parentOf(fromPath), ...(this.workspace.project?.config.includePaths ?? [])].map(directory => (directory ? `${directory}/${clean}` : clean));
    return candidates.find(candidate => this.workspace.hasFile(candidate)) ?? null;
  }

  async goToDefinition(path: string, state: EditorState, pos: number): Promise<void> {
    const line = state.doc.lineAt(pos);
    const column = pos - line.from;
    const include = includeAt(line.text, column);
    if (include) {
      const target = this.resolveInclude(path, include.text);
      if (target) await this.editors.open(target);
      else this.ui.status = `INCLUDE file "${include.text}" was not found`;
      return;
    }
    const word = wordAt(line.text, column);
    if (!word) return;
    const local = findDefinition(state.doc.toString(), word.text);
    if (local) { await this.editors.open(path, { line: local.line, column: local.column }); return; }
    const upper = word.text.toUpperCase();
    const remote = this.allSymbols.find(item => item.path !== path && item.symbol.name.toUpperCase() === upper);
    if (remote) { await this.editors.open(remote.path, { line: remote.symbol.line, column: remote.symbol.column }); return; }
    this.ui.status = `No definition found for ${word.text}`;
  }

  // ---- Search ---------------------------------------------------------------

  async runSearch(): Promise<void> {
    const query = this.search.query.trim();
    if (!this.workspace.project || query.length < 2) return;
    const results = await this.ui.task('Searching', () => native().studio.SearchProject(query), { quiet: true });
    this.search = { query: this.search.query, results: results ?? [], ran: true };
  }

  // ---- Build and run --------------------------------------------------------

  /** Saves dirty sources, or asks before building what is on disk. */
  private async prepareSources(action: string): Promise<boolean> {
    const dirty = this.editors.dirtyTabs;
    if (!dirty.length) return true;
    const answer = await this.ui.choose({
      title: `Save before ${action}?`,
      message: `${dirty.length} file${dirty.length === 1 ? ' has' : 's have'} unsaved changes.`,
      detail: `${dirty.map(tab => tab.path).join('\n')}\n\nThe compiler reads files from disk, so unsaved changes are not included unless you save.`,
      buttons: [{ value: 'cancel', label: 'Cancel' }, { value: 'skip', label: `${action === 'building' ? 'Build' : 'Run'} Without Saving` }, { value: 'save', label: 'Save All', kind: 'primary' }],
      cancelValue: 'cancel',
    });
    if (answer === 'cancel') return false;
    if (answer === 'save') return this.editors.saveAll();
    return true;
  }

  private reportBuild(result: BuildResult): void {
    this.lastBuild = result;
    this.problems.setBuild(result.diagnostics ?? [], this.editors.dirtyTabs.map(tab => tab.path));
    const outcome = result.cancelled ? 'Build cancelled' : result.success ? 'Build succeeded' : 'Build failed';
    const artifacts = (result.artifacts ?? []).filter((item: BuildArtifact) => item.kind === 'executable' || item.kind === 'disk-image').map((item: BuildArtifact) => `${item.kind === 'disk-image' ? 'ATR image' : 'Artifact'}: ${item.path}`);
    this.ui.appendOutput(`\n${outcome} in ${result.durationMs} ms\n${artifacts.length ? artifacts.join('\n') + '\n' : ''}`);
    if (result.success && !result.debugAvailable && result.debugNote) this.ui.appendOutput(result.debugNote + '\n');
    this.ui.status = outcome;
    if (!result.success && !result.cancelled) this.ui.showPanel('problems');
  }

  /** Builds (and optionally runs) the project; returns the build result. */
  async build(run: boolean): Promise<BuildResult | null> {
    if (!this.workspace.project || this.operations.isRunning('build')) return null;
    const verb = run ? 'run this project' : 'build this project';
    if (!(await this.workspace.requestTrust(`${verb} with its configured compiler and tools`))) return null;
    if (!(await this.prepareSources(run ? 'running' : 'building'))) return null;
    this.ui.clearOutput();
    this.ui.showPanel('output');
    const label = run ? 'Running' : 'Building';
    this.ui.busy = [...this.ui.busy, label];
    this.ui.status = label + '…';
    try {
      const completed = await this.operations.run(label, () => (run ? native().studio.StartRun({ profileId: this.emulator.profileId }) : native().studio.StartBuild({})));
      const result = (run ? (completed.result as RunResult | undefined)?.build : (completed.result as BuildResult | undefined)) ?? null;
      if (result) {
        const argv = result.steps?.[0]?.argv ?? [];
        if (argv.length) this.ui.appendOutput(`\n> ${argv.map((arg: string) => (/\s/.test(arg) ? JSON.stringify(arg) : arg)).join(' ')}\n`);
        this.reportBuild(result);
      }
      if (completed.error && !completed.cancelled) {
        this.ui.fail(label, completed.error);
        this.ui.showPanel('output');
      }
      const runResult = run ? (completed.result as RunResult | undefined) : undefined;
      if (runResult?.started) {
        this.editors.openEmulator();
        this.emulator.started(runResult.emulator?.hostDeviceName ?? '');
      }
      await this.workspace.refresh();
      return result;
    } catch (error) {
      this.ui.fail(label, error);
      this.ui.showPanel('output');
      return null;
    } finally {
      this.ui.busy = this.ui.busy.filter(item => item !== label);
    }
  }

  // ---- Explorer actions -----------------------------------------------------

  beginCreate(kind: 'file' | 'folder'): void {
    if (!this.workspace.project) return;
    this.ui.showActivity('explorer');
    const selected = this.workspace.selected;
    const directory = selected && this.workspace.hasDirectory(selected) ? selected : parentOf(selected) || (this.workspace.hasDirectory('src') ? 'src' : '');
    this.creating = { kind, value: `${directory ? directory + '/' : ''}${kind === 'file' ? 'new.bas' : 'new-folder'}` };
  }

  async commitCreate(): Promise<void> {
    const request = this.creating;
    if (!request) return;
    const path = normalizeRel(request.value);
    if (!path) { this.creating = null; return; }
    if (request.kind === 'folder') {
      if (await this.workspace.createFolder(path)) this.creating = null;
      return;
    }
    const content = /\.bas$/i.test(path) ? "' New exBASIC source\n" : '';
    if (await this.workspace.createFile(path, content)) {
      this.creating = null;
      await this.editors.open(path);
    }
  }

  async renameSelected(): Promise<void> {
    const old = this.workspace.selected;
    if (!old) return;
    const next = await this.ui.prompt({ title: 'Rename', message: `Rename or move ${old}`, label: 'New path', value: old, confirmLabel: 'Rename' });
    if (next === null) return;
    await this.movePath(old, next);
  }

  async movePath(oldPath: string, newPath: string): Promise<void> {
    const target = normalizeRel(newPath);
    if (!target || target === oldPath || target.startsWith(oldPath + '/')) return;
    if (await this.workspace.rename(oldPath, target)) {
      this.editors.applyRename(oldPath, target);
      this.sprites.applyRename(oldPath, target);
      this.debug.pathRenamed(oldPath, target);
      void this.buildSymbolIndex();
      void this.git.autoRefresh();
    }
  }

  async deleteSelected(): Promise<void> {
    const target = this.workspace.selected;
    if (!target) return;
    const isFolder = this.workspace.hasDirectory(target);
    const confirmed = await this.ui.confirm({
      title: `Delete ${isFolder ? 'folder' : 'file'}?`, message: `Delete ${target}${isFolder ? ' and everything in it' : ''}?`,
      detail: 'This cannot be undone.', confirmLabel: 'Delete', danger: true,
    });
    if (!confirmed) return;
    if (await this.workspace.remove(target)) {
      this.editors.applyDelete(target);
      this.sprites.applyDelete(target);
      void this.git.autoRefresh();
    }
  }

  async copyActivePath(): Promise<void> {
    const tab = this.editors.activeTab;
    if (!tab || !this.workspace.project) return;
    await native().clipboard.setText(`${this.workspace.root}/${tab.path}`.replaceAll('\\', '/'));
    this.ui.status = 'Copied path';
  }

  /** Routes files dropped from the desktop to the element they landed on. */
  private async filesDropped(files: string[], targetId: string): Promise<void> {
    if (!files.length) return;
    const decode = (prefix: string) => { const encoded = targetId.slice(prefix.length); return encoded === 'root' ? '' : decodeURIComponent(encoded); };
    if (targetId.startsWith('atr-drop-') && this.atr.view) await this.atr.importHost(files, decode('atr-drop-'));
    else if (targetId.startsWith('project-drop-') && this.workspace.project) await this.workspace.importPaths(decode('project-drop-'), files);
  }

  async exportSupportBundle(): Promise<void> {
    const path = await native().dialogs.save({ title: 'Export Support Bundle', filename: 'exbasic-studio-support.zip', filters: [{ name: 'Zip archives', pattern: '*.zip' }] });
    if (!path) return;
    const ok = await this.ui.task('Exporting support bundle', async () => { await native().studio.ExportSupportBundle(path); return true; });
    if (ok) this.ui.appendOutput(`Support bundle written to ${path}. It contains diagnostic logs with file paths redacted and no project sources.\n`);
  }

  // ---- Commands -------------------------------------------------------------

  commandEnabled(id: CommandID): boolean {
    const project = !!this.workspace.project;
    const textTab = !!this.editors.activeTab;
    const sprite = this.sprites.active;
    const building = this.operations.isRunning('build');
    switch (id) {
      case 'file.close': case 'file.new': case 'file.newFolder': case 'quickOpen': case 'symbols': case 'trust': return project;
      case 'file.rename': case 'file.delete': return project && !!this.workspace.selected;
      case 'file.save': return textTab || !!sprite;
      case 'editor.undo': return textTab || !!sprite?.history.undo;
      case 'editor.redo': return textTab || !!sprite?.history.redo;
      case 'editor.split': case 'editor.copyPath': case 'editor.selectAll': case 'editor.find': case 'editor.definition': return textTab;
      case 'file.saveAll': return this.editors.dirtyTabs.length > 0 || this.sprites.host.dirty().length > 0;
      case 'editor.close': return this.editors.active !== '';
      case 'nav.back': return this.editors.historyState.back;
      case 'nav.forward': return this.editors.historyState.forward;
      case 'build': case 'run': return project && !building && !this.debug.active;
      case 'build.cancel': return building;
      case 'debug.start': return project && !building && !this.debug.active;
      case 'debug.continue': case 'debug.stepLine': case 'debug.stepInstruction': case 'debug.stepOver': case 'debug.stepOut': return this.debug.stopped;
      case 'debug.pause': return this.debug.active && !this.debug.stopped;
      case 'debug.stop': return this.debug.active || this.emulator.running;
      case 'debug.toggleBreakpoint': return textTab && /\.bas$/i.test(this.editors.active);
      case 'emulator.start': return !this.emulator.running;
      case 'emulator.restart': case 'emulator.stop': return this.emulator.running;
      case 'terminal.new': case 'terminal.focus': return project;
      case 'terminal.kill': return !!this.terminal.activeId;
      case 'sprites.new': case 'sprites.merge': return project;
      case 'sprites.open': return project && !!this.spriteCandidate;
      case 'sprites.openText': return !!sprite && !!(sprite.sheetPath ?? sprite.framesPath ?? sprite.bank?.index);
      case 'sprites.generate': return !!sprite;
      default: return true;
    }
  }

  async execute(id: CommandID): Promise<void> {
    this.closePalette(false);
    if (!this.commandEnabled(id)) return;
    switch (id) {
      case 'palette': this.showPalette('commands'); return;
      case 'quickOpen': this.showPalette('files'); return;
      case 'symbols': this.showPalette('symbols'); return;
      case 'file.open': await this.chooseFolder(); return;
      case 'file.close': await this.closeFolder(); return;
      case 'file.new': this.beginCreate('file'); return;
      case 'file.newFolder': this.beginCreate('folder'); return;
      case 'file.rename': await this.renameSelected(); return;
      case 'file.delete': await this.deleteSelected(); return;
      case 'file.save':
        if (this.sprites.active) await this.sprites.save(this.sprites.active); else await this.editors.save(this.editors.active);
        return;
      case 'file.saveAll': await this.ui.task('Saving all', async () => { if (!(await this.editors.saveAll())) throw new Error('some files could not be saved'); }); return;
      case 'editor.close':
        if (this.editors.active === EMULATOR_TAB) { await this.emulator.stop(); this.editors.closeEmulator(); }
        else if (this.sprites.active) await this.sprites.close(this.sprites.active);
        else await this.editors.close(this.editors.active);
        return;
      case 'editor.undo': if (this.sprites.active) this.sprites.active.undo(); else this.editorAction('undo'); return;
      case 'editor.redo': if (this.sprites.active) this.sprites.active.redo(); else this.editorAction('redo'); return;
      case 'editor.selectAll': this.editorAction('selectAll'); return;
      case 'editor.find': this.editorAction('find'); return;
      case 'editor.definition': this.editorAction('definition'); return;
      case 'editor.split': this.editors.splitOpen = !this.editors.splitOpen; return;
      case 'editor.copyPath': await this.copyActivePath(); return;
      case 'nav.back': await this.editors.navigate(-1); return;
      case 'nav.forward': await this.editors.navigate(1); return;
      case 'view.explorer': case 'view.atr': case 'view.search': case 'view.source': case 'view.run': {
        const activity = id.slice(5) as Activity;
        this.ui.showActivity(activity);
        if (activity === 'source') void this.git.autoRefresh();
        return;
      }
      case 'view.sidebar': this.ui.primaryOpen = !this.ui.primaryOpen; this.ui.persistLayout(); return;
      case 'view.panel': this.ui.panelOpen = !this.ui.panelOpen; this.ui.persistLayout(); return;
      case 'view.problems': this.ui.showPanel('problems'); return;
      case 'view.output': this.ui.showPanel('output'); return;
      case 'view.keyboard': this.ui.showPanel('keyboard'); return;
      case 'build': await this.build(false); return;
      case 'run': await this.build(true); return;
      case 'build.cancel': await this.operations.cancel('build'); return;
      case 'debug.start': await this.debug.start(() => this.build(false)); return;
      case 'debug.continue': await this.debug.command('continue'); return;
      case 'debug.pause': await this.debug.command('pause'); return;
      case 'debug.stepLine': await this.debug.command('stepLine'); return;
      case 'debug.stepInstruction': await this.debug.command('stepInstruction'); return;
      case 'debug.stepOver': await this.debug.command('stepOver'); return;
      case 'debug.stepOut': await this.debug.command('stepOut'); return;
      case 'debug.stop':
        if (this.debug.active) await this.debug.end(false); else await this.emulator.stop();
        return;
      case 'debug.toggleBreakpoint': this.debug.toggleBreakpoint(this.editors.active, this.editors.cursor.line); return;
      case 'emulator.open': this.editors.openEmulator(); return;
      case 'emulator.start': this.editors.openEmulator(); await this.emulator.start(); return;
      case 'emulator.restart': await this.emulator.stop(); await this.emulator.start(); return;
      case 'emulator.stop': await this.emulator.stop(); return;
      case 'emulator.joystick': this.emulator.setJoystickMode(!this.emulator.joystickMode); return;
      case 'terminal.new': this.ui.showPanel('terminal'); await this.terminal.open(); this.focusTerminal(); return;
      case 'terminal.focus':
        this.ui.showPanel('terminal');
        if (!this.terminal.sessions.length) await this.terminal.open();
        this.focusTerminal();
        return;
      case 'terminal.kill': await this.terminal.close(this.terminal.activeId); return;
      case 'manual': this.ui.showActivity('manual'); return;
      case 'settings': this.ui.showActivity('settings'); return;
      case 'trust':
        if (this.workspace.trust.trusted) {
          if (await this.ui.confirm({ title: 'Return to restricted mode?', message: 'Stop trusting this folder?', detail: 'Building, running, the terminal and Git hooks will be disabled, and open terminals will be closed.', confirmLabel: 'Restrict' })) {
            await this.workspace.revokeTrust();
            this.terminal.reset();
          }
        } else if (await this.workspace.requestTrust('build, run, open a terminal and use Git fully')) this.editors.relintAll();
        return;
      case 'support.bundle': await this.exportSupportBundle(); return;
      case 'view.sprites': this.ui.showActivity('sprites'); return;
      case 'sprites.new': await this.sprites.newSheet(); return;
      case 'sprites.merge': this.ui.showActivity('sprites'); this.sprites.merging = { paths: [] }; return;
      case 'sprites.open': await this.sprites.open(this.spriteCandidate); return;
      case 'sprites.openText': await this.openSpriteText(); return;
      case 'sprites.generate': if (this.sprites.active) await this.sprites.generate(this.sprites.active); return;
      default: id satisfies never;
    }
  }

  // ---- Command palette ------------------------------------------------------

  showPalette(mode: PaletteMode): void {
    if (!this.palette.open && typeof document !== 'undefined') this.paletteFocus = document.activeElement;
    this.palette = { open: true, mode, query: '', index: 0 };
  }

  closePalette(restoreFocus = true): void {
    if (!this.palette.open) return;
    this.palette = { ...this.palette, open: false };
    const target = this.paletteFocus;
    this.paletteFocus = null;
    if (restoreFocus && target instanceof HTMLElement && target.isConnected) queueMicrotask(() => target.focus());
  }

  get paletteItems(): PaletteItem[] {
    const query = this.palette.query.trim().toLowerCase();
    const matches = (text: string) => query.split(/\s+/).every(part => text.toLowerCase().includes(part));
    if (this.palette.mode === 'commands') {
      return commands.filter(command => command.id !== 'palette' && matches(command.label)).map(command => ({
        key: command.id, label: command.label, icon: command.icon ?? 'symbol-event', shortcut: command.shortcut,
        disabled: !this.commandEnabled(command.id), run: () => void this.execute(command.id),
      }));
    }
    if (this.palette.mode === 'files') {
      return (this.workspace.project?.files ?? []).filter(file => matches(file.path)).slice(0, 50).map(file => ({
        key: file.path, label: baseName(file.path), detail: parentOf(file.path), icon: 'file',
        run: () => { this.closePalette(false); void this.openFile(file.path); },
      }));
    }
    return this.allSymbols.filter(item => matches(item.symbol.name)).slice(0, 80).map(item => ({
      key: `${item.path}:${item.symbol.line}:${item.symbol.name}`, label: item.symbol.name, detail: `${item.symbol.detail} — ${item.path}:${item.symbol.line}`,
      icon: item.symbol.kind === 'function' ? 'symbol-method' : 'symbol-variable',
      run: () => { this.closePalette(false); void this.editors.open(item.path, { line: item.symbol.line, column: item.symbol.column }); },
    }));
  }

  /** Moves the palette selection, skipping disabled entries. */
  movePalette(delta: number): void {
    const items = this.paletteItems;
    if (!items.length) return;
    let index = this.palette.index;
    for (let step = 0; step < items.length; step++) {
      index = (index + delta + items.length) % items.length;
      if (!items[index].disabled) break;
    }
    this.palette = { ...this.palette, index };
  }

  runPaletteSelection(): void {
    const items = this.paletteItems;
    const item = items[Math.min(this.palette.index, items.length - 1)];
    if (item && !item.disabled) item.run();
  }

  // ---- Keyboard -------------------------------------------------------------

  /** Global shortcuts. Returns true when the event was handled. */
  handleKey(event: KeyboardEvent): boolean {
    const mod = event.metaKey || event.ctrlKey;
    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
    const run = (id: CommandID) => { event.preventDefault(); void this.execute(id); return true; };

    if (this.ui.dialog) return false; // the dialog owns the keyboard
    if (this.pendingChord) {
      const chord = this.pendingChord;
      this.pendingChord = '';
      clearTimeout(this.chordTimer);
      if (chord === 'k' && mod && key === 'o') return run('file.open');
      if (chord === 'k' && !mod && key === 's') return run('file.saveAll');
    }
    if (mod && !event.shiftKey && !event.altKey && key === 'k') {
      event.preventDefault();
      this.pendingChord = 'k';
      clearTimeout(this.chordTimer);
      this.chordTimer = setTimeout(() => { this.pendingChord = ''; }, 1500);
      return true;
    }
    if (event.altKey && !mod && event.key === 'ArrowLeft') return run('nav.back');
    if (event.altKey && !mod && event.key === 'ArrowRight') return run('nav.forward');
    if (mod && event.shiftKey && !event.altKey) {
      switch (key) {
        case 'p': return run('palette');
        case 'e': return run('view.explorer');
        case 'f': return run('view.search');
        case 'g': return run('view.source');
        case 'd': return run('view.run');
        case 'm': return run('view.problems');
        case '`': case '~': return run('terminal.new');
      }
    }
    if (mod && !event.shiftKey && !event.altKey) {
      switch (key) {
        case 'p': return run('quickOpen');
        case 't': return run('symbols');
        case 'n': return run('file.new');
        case 'w': return run('editor.close');
        case 's': return run('file.save');
        case 'b': return run('view.sidebar');
        case 'j': return run('view.panel');
        case ',': return run('settings');
        case '\\': return run('editor.split');
        case '`': return run('terminal.focus');
        case 'F5': return run('run');
      }
    }
    if (!mod && !event.altKey) {
      switch (event.key) {
        case 'F1': return run('manual');
        case 'F2': return this.ui.activity === 'explorer' && !!this.workspace.selected ? run('file.rename') : false;
        case 'F5': return run(event.shiftKey ? 'debug.stop' : this.debug.stopped ? 'debug.continue' : 'debug.start');
        case 'F6': return event.shiftKey ? false : run('build');
        case 'F8': return run('debug.pause');
        case 'F9': return run('debug.toggleBreakpoint');
        case 'F10': return this.debug.active ? run(event.shiftKey ? 'debug.stepOver' : 'debug.stepLine') : false;
        case 'F11': return this.debug.active ? run(event.shiftKey ? 'debug.stepOut' : 'debug.stepInstruction') : false;
      }
    }
    return false;
  }
}

/** The application-wide workbench instance. */
export let wb: Workbench;

export function createWorkbench(): Workbench {
  wb = new Workbench();
  return wb;
}
