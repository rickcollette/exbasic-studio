// Feature stores for Git, the emulator, terminal sessions, the ATR explorer
// and the manual.
import { native, errorMessage, type ATRView, type EmulatorProfile, type GitBranch, type GitConflict, type GitRemote, type GitStatus, type ManualHit, type TerminalEvent, type TerminalInfo } from '../native';
import { KeyboardJoystick, NEUTRAL, isJoystickKey } from '../joystick';
import type { UIStore } from './ui.svelte';
import type { WorkspaceStore } from './workspace.svelte';
import type { OperationsStore } from './operations.svelte';

// ---- Git -------------------------------------------------------------------

export class GitStore {
  status = $state<GitStatus | null>(null);
  branches = $state<GitBranch[]>([]);
  remotes = $state<GitRemote[]>([]);
  diff = $state<{ path: string; staged: boolean; text: string } | null>(null);
  conflict = $state<GitConflict | null>(null);
  message = $state('');
  /**
   * In restricted mode Git is not run until the user asks for it: a
   * repository's own configuration can make even "git status" run programs.
   */
  consent = $state(false);

  constructor(private ui: UIStore, private workspace: WorkspaceStore, private operations: OperationsStore) {}

  get staged() { return (this.status?.changes ?? []).filter(change => change.staged && !change.conflicted); }
  get unstaged() { return (this.status?.changes ?? []).filter(change => !change.conflicted && (change.untracked || change.worktree !== '.')); }
  get conflicts() { return (this.status?.changes ?? []).filter(change => change.conflicted); }
  get networkBusy() { return this.operations.isRunning('git'); }

  reset(): void { this.status = null; this.branches = []; this.remotes = []; this.diff = null; this.conflict = null; this.message = ''; this.consent = false; }

  /** True when Git must not be run without the user asking. */
  get blocked(): boolean { return this.workspace.restricted && !this.consent; }

  /** Refreshes status unless Git is waiting for consent in restricted mode. */
  async autoRefresh(): Promise<void> {
    if (this.blocked) return;
    await this.refresh();
  }

  /** The user explicitly asked to see the repository of a restricted folder. */
  async allow(): Promise<void> {
    this.consent = true;
    await this.refresh();
  }

  async refresh(): Promise<void> {
    if (!this.workspace.project) { this.reset(); return; }
    try {
      this.status = await native().studio.GitStatus();
      if (this.status.available) {
        const [branches, remotes] = await Promise.all([native().studio.GitBranches().catch(() => null), native().studio.GitRemotes().catch(() => null)]);
        this.branches = branches ?? [];
        this.remotes = remotes ?? [];
      } else { this.branches = []; this.remotes = []; }
    } catch (error) {
      this.ui.status = `Git: ${errorMessage(error)}`;
    }
  }

  /** Runs a local Git command, reports its output and refreshes status. */
  private async action(label: string, work: () => Promise<string>): Promise<boolean> {
    const output = await this.ui.task(label, work);
    if (output === undefined) { await this.refresh(); return false; }
    if (output.trim()) this.ui.appendOutput(output.endsWith('\n') ? output : output + '\n');
    await this.refresh();
    return true;
  }

  /** Runs an action that needs a trusted workspace, prompting if necessary. */
  private async trusted(reason: string, label: string, work: () => Promise<string>): Promise<boolean> {
    if (!(await this.workspace.requestTrust(reason))) return false;
    return this.action(label, work);
  }

  init() { return this.action('Initializing repository', () => native().studio.GitInit()); }
  stage(path: string) { return this.action('Staging', () => native().studio.GitStage(path)); }
  stageAll() { return this.action('Staging all changes', () => native().studio.GitStageAll()); }
  unstage(path: string) { return this.action('Unstaging', () => native().studio.GitUnstage(path)); }
  unstageAll() { return this.action('Unstaging all changes', () => native().studio.GitUnstageAll()); }

  async commit(): Promise<boolean> {
    const message = this.message.trim();
    if (!message) return false;
    const ok = await this.trusted('commit (commit hooks may run)', 'Committing', () => native().studio.GitCommit(message));
    if (ok) this.message = '';
    return ok;
  }

  async showDiff(path: string, staged: boolean): Promise<void> {
    const text = await this.ui.task('Loading diff', () => native().studio.GitDiff(path, staged), { quiet: true });
    if (text !== undefined) this.diff = { path, staged, text };
  }

  createBranch(name: string, checkout: boolean) {
    return this.trusted('create and check out branches', 'Creating branch', () => native().studio.GitCreateBranch(name, checkout));
  }
  switchBranch(name: string) {
    return this.trusted('switch branches', 'Switching branch', () => native().studio.GitSwitchBranch(name));
  }
  deleteBranch(name: string, force: boolean) { return this.action('Deleting branch', () => native().studio.GitDeleteBranch(name, force)); }
  addRemote(name: string, url: string) { return this.action('Adding remote', () => native().studio.GitAddRemote(name, url)); }
  setRemoteURL(name: string, url: string) { return this.action('Updating remote', () => native().studio.GitSetRemoteURL(name, url)); }
  renameRemote(oldName: string, newName: string) { return this.action('Renaming remote', () => native().studio.GitRenameRemote(oldName, newName)); }
  removeRemote(name: string) { return this.action('Removing remote', () => native().studio.GitRemoveRemote(name)); }

  /** Explicit fetch, pull or push with streamed progress. */
  async network(action: 'fetch' | 'pull' | 'push', remote = ''): Promise<void> {
    if (!(await this.workspace.requestTrust(`${action} (network access; Git hooks may run)`))) return;
    this.ui.showPanel('output');
    this.ui.appendOutput(`\n> git ${action}${remote ? ' ' + remote : ''}\n`);
    this.ui.busy = [...this.ui.busy, `git ${action}`];
    try {
      const completed = await this.operations.run(`git ${action}`, () => native().studio.StartGitNetwork(action, remote));
      if (completed.cancelled) { this.ui.status = `git ${action} cancelled`; this.ui.appendOutput(`\ngit ${action} cancelled.\n`); }
      else if (completed.error) { this.ui.status = `git ${action} failed`; this.ui.appendOutput(`\n${completed.error}\n`); }
      else { this.ui.status = `git ${action} complete`; this.ui.appendOutput(`\ngit ${action} finished.\n`); }
    } catch (error) {
      this.ui.fail(`git ${action}`, error);
    } finally {
      this.ui.busy = this.ui.busy.filter(label => label !== `git ${action}`);
      await this.refresh();
    }
  }

  cancelNetwork() { return this.operations.cancel('git'); }

  async openConflict(path: string): Promise<void> {
    const conflict = await this.ui.task('Loading conflict', () => native().studio.GitConflict(path), { quiet: true });
    if (conflict) this.conflict = conflict;
  }

  async resolve(path: string, choice: 'current' | 'incoming' | 'both'): Promise<boolean> {
    const ok = await this.action('Resolving conflict', () => native().studio.GitResolveConflict(path, choice));
    if (ok && this.conflict?.path === path) this.conflict = null;
    return ok;
  }

  async stageResolved(path: string): Promise<boolean> {
    const ok = await this.action('Marking resolved', () => native().studio.GitStageResolved(path));
    if (ok && this.conflict?.path === path) this.conflict = null;
    return ok;
  }

  mergeContinue() { return this.trusted('conclude the merge (commit hooks may run)', 'Completing merge', () => native().studio.GitMergeContinue()); }

  async mergeAbort(): Promise<boolean> {
    const ok = await this.action('Aborting merge', () => native().studio.GitMergeAbort());
    if (ok) this.conflict = null;
    return ok;
  }
}

// ---- Emulator --------------------------------------------------------------

function blankProfile(): EmulatorProfile {
  return {
    id: '', name: 'New ex800 Profile', machine: 'xl', memory: '64', video: 'ntsc', cpuSpeed: 100, basic: false, sioPatch: true,
    eos: false, ultimate1MB: false, scale: 2, media: '', hostDir: '',
    cartridge: { kind: '', image: '', type: 0 }, cartridge2: { kind: '', image: '', type: 0 },
    drives: Array.from({ length: 4 }, () => ({ image: '', enabled: false, readOnly: false })),
    rDevice: { mode: 'disabled', listenPort: 9000, connectBaud: 2400, serialPort: '', modem: false },
    ide: { enabled: false, image: '', compactFlash: false }, arguments: [],
  };
}

export class EmulatorStore {
  profiles = $state<EmulatorProfile[]>([]);
  profileId = $state('');
  draft = $state<EmulatorProfile | null>(null);
  running = $state(false);
  image = $state('');
  frame = $state(0);
  hostDeviceName = $state('');
  configOpen = $state(false);
  /** True while the emulator screen is the visible editor tab. */
  visible = $state(false);
  /** A debug session keeps the machine running while its screen is hidden. */
  keepRunning = $state(false);
  /** Keyboard joystick: Q W E A D Z X C, the arrows and the numeric keypad
   * drive joystick 0, and Space, Numpad 0 and Ctrl are fire. */
  joystickMode = $state(false);
  /** The last STICK() and STRIG() sent, for the toolbar indicator. */
  stick = $state(NEUTRAL);
  trigger = $state(false);

  /** KeyboardEvent.code of the keys held on the emulator screen, for the
   * Keyboard panel. */
  held = $state<string[]>([]);

  private pad = new KeyboardJoystick();
  private joystickSending = false;
  private joystickDirty = false;

  private timer: ReturnType<typeof setTimeout> | undefined;
  private polling = false;
  /** Called when the emulated machine changed files in the workspace. */
  onWorkspaceChanged: (() => void) | undefined;
  /** Called when the keyboard joystick is switched, so the project remembers it. */
  onJoystickModeChanged: (() => void) | undefined;

  constructor(private ui: UIStore) {}

  async load(): Promise<void> {
    this.profiles = (await native().studio.EmulatorProfiles()) ?? [];
    if (!this.profiles.length) return;
    this.select(this.profiles.some(profile => profile.id === this.profileId) ? this.profileId : this.profiles[0].id);
    const state = await native().studio.EmulatorStatus();
    this.running = state.running;
    if (state.profileId && this.profiles.some(profile => profile.id === state.profileId)) this.select(state.profileId);
  }

  select(id: string): void {
    const profile = this.profiles.find(item => item.id === id);
    if (!profile) return;
    this.profileId = id;
    const draft = structuredClone($state.snapshot(profile)) as EmulatorProfile;
    const blank = blankProfile();
    draft.arguments ??= [];
    draft.drives = [...(draft.drives ?? []), ...(blank.drives ?? [])].slice(0, 4);
    draft.rDevice ??= blank.rDevice;
    draft.ide ??= blank.ide;
    draft.hostDir ??= '';
    draft.memory ||= '64';
    draft.cpuSpeed ||= 100;
    draft.scale ||= 2;
    this.draft = draft;
  }

  newProfile(): void { this.draft = blankProfile(); this.profileId = ''; this.configOpen = true; }

  async saveProfile(): Promise<EmulatorProfile | null> {
    if (!this.draft) return null;
    const saved = await this.ui.task('Saving emulator profile', () => native().studio.EmulatorSaveProfile($state.snapshot(this.draft!) as EmulatorProfile), { quiet: true });
    if (!saved) return null;
    this.profiles = (await native().studio.EmulatorProfiles()) ?? [];
    this.select(saved.id);
    return saved;
  }

  async deleteProfile(): Promise<void> {
    const id = this.draft?.id;
    if (!id) return;
    await this.ui.task('Deleting emulator profile', () => native().studio.EmulatorDeleteProfile(id));
    this.profileId = '';
    await this.load();
  }

  /** Marks the machine as running after a Run operation started it. */
  started(hostDeviceName = ''): void {
    this.running = true; this.image = ''; this.frame = 0; this.hostDeviceName = hostDeviceName;
    void this.poll();
  }

  async start(): Promise<void> {
    await this.ui.task('Starting ex800', async () => {
      if (!this.draft) await this.load();
      const profile = await this.saveProfile();
      if (!profile) throw new Error('No ex800 profile is available');
      const state = await native().studio.EmulatorStart(profile.id, profile.media);
      this.running = state.running;
      this.started(state.hostDeviceName ?? '');
    });
  }

  async stop(): Promise<void> {
    clearTimeout(this.timer);
    try {
      await native().studio.EmulatorStop();
      this.ui.status = 'ex800 stopped; project disk synchronized';
    } catch (error) {
      this.ui.status = `ex800 stopped: ${errorMessage(error)}`;
    } finally {
      this.running = false; this.image = ''; this.hostDeviceName = '';
      this.onWorkspaceChanged?.();
    }
  }

  setVisible(visible: boolean): void {
    this.visible = visible;
    if (visible) void this.poll();
    else {
      if (!this.keepRunning) clearTimeout(this.timer);
      void this.releaseKeys();
    }
  }

  /** Pulls frames while the screen is visible. A helper failure is a stop. */
  private async poll(): Promise<void> {
    if (!(this.visible || this.keepRunning) || !this.running || this.polling) return;
    this.polling = true;
    try {
      const frame = await native().studio.EmulatorFrame();
      if (frame.image) this.image = frame.image;
      this.frame = frame.number ?? this.frame;
      this.hostDeviceName = frame.hostDevice ? frame.hostDeviceName ?? '' : '';
      if (frame.workspaceChanged) this.onWorkspaceChanged?.();
      if (frame.error) this.ui.status = frame.error;
    } catch (error) {
      this.running = false;
      this.image = '';
      this.ui.status = 'ex800 stopped unexpectedly';
      this.ui.appendOutput(`\nex800 stopped: ${errorMessage(error)}\nUse Start to run it again.\n`);
    } finally {
      this.polling = false;
      if ((this.visible || this.keepRunning) && this.running) this.timer = setTimeout(() => void this.poll(), 66);
    }
  }

  async key(key: string, down: boolean): Promise<void> {
    if (!this.running) return;
    try { await native().studio.EmulatorKey(key, down); } catch { /* the next frame reports a stopped helper */ }
  }

  /** Sends a key to the keyboard joystick when joystick mode is on.
   * Returns false for a key that belongs to the Atari keyboard. */
  joystickKey(code: string, down: boolean): boolean {
    if (!this.joystickMode || !isJoystickKey(code)) return false;
    if (this.pad.press(code, down)) this.sendJoystick();
    return true;
  }

  setJoystickMode(on: boolean): void {
    if (this.joystickMode === on) return;
    this.joystickMode = on;
    this.onJoystickModeChanged?.();
    // Keys held when the mode changes would otherwise stay pressed.
    if (this.pad.clear()) this.sendJoystick();
    void this.releaseKeys();
  }

  /** Sends the joystick state, one request at a time and always the newest,
   * so a quick press and release cannot arrive out of order. */
  private sendJoystick(): void {
    this.stick = this.pad.stick;
    this.trigger = this.pad.trigger;
    this.joystickDirty = true;
    if (this.joystickSending) return;
    this.joystickSending = true;
    void (async () => {
      try {
        while (this.joystickDirty && this.running) {
          this.joystickDirty = false;
          try { await native().studio.EmulatorJoystick(this.stick, this.trigger); } catch { break; /* the next frame reports a stopped helper */ }
        }
      } finally {
        this.joystickSending = false;
        this.joystickDirty = false;
      }
    })();
  }

  /** Records a key held on the emulator screen. */
  markHeld(code: string, down: boolean): void {
    if (!code) return;
    if (down && !this.held.includes(code)) this.held = [...this.held, code];
    else if (!down) this.held = this.held.filter(held => held !== code);
  }

  async releaseKeys(): Promise<void> {
    // The helper also centres the joystick and releases fire.
    this.held = [];
    this.pad.clear();
    this.stick = NEUTRAL;
    this.trigger = false;
    if (!this.running) return;
    try { await native().studio.EmulatorReleaseKeys(); } catch { /* ignore */ }
  }
}

// ---- Terminal --------------------------------------------------------------

export interface TerminalSession { id: string; title: string; exited: boolean; exitCode: number }

export class TerminalStore {
  sessions = $state<TerminalSession[]>([]);
  activeId = $state('');

  private listeners = new Map<string, (data: string) => void>();
  /** Output received before the terminal view attached. */
  private backlog = new Map<string, string>();
  private lastSeq = new Map<string, number>();
  private unsubscribe: (() => void) | undefined;
  private counter = 0;

  constructor(private ui: UIStore, private workspace: WorkspaceStore) {}

  connect(): void {
    this.unsubscribe?.();
    this.unsubscribe = native().on('terminal', event => this.receive(event));
  }

  disconnect(): void { this.unsubscribe?.(); this.unsubscribe = undefined; }

  private receive(event: TerminalEvent): void {
    if ((this.lastSeq.get(event.id) ?? 0) >= event.seq) return;
    this.lastSeq.set(event.id, event.seq);
    if (event.type === 'output' && event.data) this.write(event.id, event.data);
    if (event.type === 'exit') {
      this.write(event.id, `\r\n\x1b[2m[Process exited with code ${event.exitCode}]\x1b[0m\r\n`);
      this.sessions = this.sessions.map(session => (session.id === event.id ? { ...session, exited: true, exitCode: event.exitCode } : session));
    }
  }

  private write(id: string, data: string): void {
    const listener = this.listeners.get(id);
    if (listener) listener(data);
    else this.backlog.set(id, ((this.backlog.get(id) ?? '') + data).slice(-200_000));
  }

  /** Attaches a terminal view; buffered output is replayed first. */
  attach(id: string, listener: (data: string) => void): () => void {
    const pending = this.backlog.get(id);
    if (pending) { listener(pending); this.backlog.delete(id); }
    this.listeners.set(id, listener);
    return () => { if (this.listeners.get(id) === listener) this.listeners.delete(id); };
  }

  async open(cols = 80, rows = 24): Promise<string | null> {
    if (!this.workspace.project) return null;
    if (!(await this.workspace.requestTrust('open a terminal in this folder'))) return null;
    const info = await this.ui.task<TerminalInfo>('Opening terminal', () => native().studio.TerminalOpen(cols, rows), { quiet: true });
    if (!info) return null;
    this.counter++;
    const shell = info.shell.replaceAll('\\', '/').split('/').pop()?.replace(/\.exe$/i, '') ?? 'shell';
    this.sessions = [...this.sessions, { id: info.id, title: `${shell} ${this.counter}`, exited: false, exitCode: 0 }];
    this.activeId = info.id;
    return info.id;
  }

  input(id: string, data: string): void { void native().studio.TerminalWrite(id, data).catch(() => {}); }
  resize(id: string, cols: number, rows: number): void { void native().studio.TerminalResize(id, cols, rows).catch(() => {}); }

  /** Shuts a session down explicitly, ending everything it started. */
  async close(id: string): Promise<void> {
    await native().studio.TerminalClose(id).catch(() => {});
    this.listeners.delete(id); this.backlog.delete(id); this.lastSeq.delete(id);
    const index = this.sessions.findIndex(session => session.id === id);
    this.sessions = this.sessions.filter(session => session.id !== id);
    if (this.activeId === id) this.activeId = this.sessions[Math.min(index, this.sessions.length - 1)]?.id ?? '';
  }

  /** Forgets sessions the backend already closed (workspace switched). */
  reset(): void {
    this.sessions = []; this.activeId = '';
    this.listeners.clear(); this.backlog.clear(); this.lastSeq.clear();
  }
}

// ---- ATR explorer ----------------------------------------------------------

export function atrParent(path: string): string { const index = path.lastIndexOf('/'); return index < 0 ? '' : path.slice(0, index); }

export function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export class AtrStore {
  view = $state<ATRView | null>(null);
  selected = $state('');
  collapsed = $state(new Set<string>());

  constructor(private ui: UIStore, private workspace: WorkspaceStore) {}

  get visibleEntries() {
    return (this.view?.entries ?? []).filter(entry => {
      for (let parent = atrParent(entry.path); parent; parent = atrParent(parent)) if (this.collapsed.has(parent)) return false;
      return true;
    });
  }

  /** Directory that new items go into, based on the selection. */
  get destination(): string {
    const entry = this.view?.entries?.find(item => item.path === this.selected);
    return entry?.directory ? entry.path : atrParent(this.selected);
  }

  toggle(path: string): void {
    const next = new Set(this.collapsed);
    if (next.has(path)) next.delete(path); else next.add(path);
    this.collapsed = next;
  }

  private async apply(label: string, work: () => Promise<ATRView>): Promise<boolean> {
    const view = await this.ui.task(label, work);
    if (view) this.view = view;
    return !!view;
  }

  async open(path: string): Promise<void> {
    if (await this.apply('Opening ATR image', () => native().studio.ATROpen(path))) { this.selected = ''; this.collapsed = new Set(); }
  }

  async openConfigured(): Promise<void> {
    if (await this.apply('Opening project ATR', () => native().studio.ATRConfigured())) { this.selected = ''; this.collapsed = new Set(); }
  }

  refresh() { return this.view ? this.apply('Refreshing ATR image', () => native().studio.ATROpen(this.view!.path)) : Promise.resolve(false); }
  importHost(paths: string[], destination = this.destination) {
    return this.view && paths.length ? this.apply('Importing into ATR', () => native().studio.ATRImportFiles(this.view!.path, destination, paths)) : Promise.resolve(false);
  }
  importProject(paths: string[], destination: string) {
    return this.view ? this.apply('Copying project item into ATR', () => native().studio.ATRImportProject(this.view!.path, destination, paths)) : Promise.resolve(false);
  }

  async mkdir(name: string): Promise<void> {
    const clean = name.trim();
    if (!this.view || !clean) return;
    const path = this.destination ? `${this.destination}/${clean}` : clean;
    if (await this.apply('Creating ATR folder', () => native().studio.ATRMkdir(this.view!.path, path))) this.selected = path;
  }

  async rename(oldPath: string, newPath: string): Promise<void> {
    if (!this.view || !newPath || newPath === oldPath || newPath.startsWith(oldPath + '/')) return;
    if (await this.apply('Renaming ATR entry', () => native().studio.ATRRename(this.view!.path, oldPath, newPath))) this.selected = newPath;
  }

  async remove(path: string): Promise<void> {
    if (!this.view) return;
    const confirmed = await this.ui.confirm({ title: 'Delete from ATR image?', message: `Delete ${path} from ${this.view.path.replaceAll('\\', '/').split('/').pop()}?`, detail: 'This cannot be undone.', confirmLabel: 'Delete', danger: true });
    if (confirmed && (await this.apply('Deleting ATR entry', () => native().studio.ATRRemove(this.view!.path, path)))) this.selected = '';
  }

  /** Extracts with an overwrite prompt when the destination exists. */
  private async extract(run: (overwrite: boolean) => Promise<unknown>, name: string): Promise<boolean> {
    try {
      await run(false);
      return true;
    } catch (error) {
      if (!errorMessage(error).includes('exists:')) { this.ui.fail('Extracting ATR entry', error); return false; }
    }
    const replace = await this.ui.confirm({ title: 'Replace existing file?', message: `${name} already exists in the destination.`, detail: 'Replacing it overwrites its current contents.', confirmLabel: 'Replace', danger: true });
    if (!replace) return false;
    return (await this.ui.task('Extracting ATR entry', () => run(true))) !== undefined;
  }

  async extractToDirectory(path: string, directory: string): Promise<void> {
    if (!this.view) return;
    const image = this.view.path;
    if (await this.extract(overwrite => native().studio.ATRExtractToDirectory(image, path, directory, overwrite), path.split('/').pop() ?? path)) {
      this.ui.status = `Extracted ${path}`;
    }
  }

  async extractToProject(path: string, destination: string): Promise<void> {
    if (!this.view || !this.workspace.project) return;
    const image = this.view.path;
    if (await this.extract(overwrite => native().studio.ATRExtractToProject(image, path, destination || '.', overwrite), path.split('/').pop() ?? path)) {
      await this.workspace.refresh();
    }
  }
}

// ---- Manual ----------------------------------------------------------------

export interface ManualLocation { chapter: string; anchor: string }

export class ManualStore {
  chapters = $state<string[]>([]);
  hits = $state<ManualHit[]>([]);
  query = $state('');
  /** The chapter on screen and its Markdown source. */
  current = $state<ManualLocation | null>(null);
  markdown = $state('');
  historyState = $state({ back: false, forward: false });

  private history: ManualLocation[] = [];
  private index = -1;

  constructor(private ui: UIStore) {}

  async load(): Promise<void> { this.chapters = (await native().studio.ManualFiles()) ?? []; }

  private async show(location: ManualLocation): Promise<boolean> {
    const text = await this.ui.task('Opening manual chapter', () => native().studio.ManualChapter(location.chapter), { quiet: true });
    if (text === undefined) return false;
    this.markdown = text;
    this.current = location;
    return true;
  }

  private sync(): void { this.historyState = { back: this.index > 0, forward: this.index < this.history.length - 1 }; }

  /** Opens a chapter (and heading) and records it in the navigation history. */
  async open(chapter: string, anchor = ''): Promise<void> {
    if (!(await this.show({ chapter, anchor }))) return;
    const top = this.history[this.index];
    if (!top || top.chapter !== chapter || top.anchor !== anchor) {
      this.history = [...this.history.slice(0, this.index + 1), { chapter, anchor }].slice(-100);
      this.index = this.history.length - 1;
    }
    this.sync();
    this.ui.showActivity('manual');
  }

  async go(delta: number): Promise<void> {
    const next = this.index + delta;
    if (next < 0 || next >= this.history.length) return;
    if (await this.show(this.history[next])) this.index = next;
    this.sync();
  }

  async search(): Promise<void> {
    const query = this.query.trim();
    if (query.length < 2) { this.hits = []; return; }
    const hits = await this.ui.task('Searching manual', () => native().studio.SearchManual(query), { quiet: true });
    this.hits = hits ?? [];
  }
}
