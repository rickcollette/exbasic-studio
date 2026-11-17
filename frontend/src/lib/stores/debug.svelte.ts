// Source-level debugger state: breakpoints, the current stop, watches and
// the memory viewer. Execution control is delegated to the backend, which
// drives the emulator helper.
import type { EditorState } from '@codemirror/state';
import { native, errorMessage, type BuildResult, type DebugEvent, type DebugStop, type DebugSymbol, type WatchValue } from '../native';
import { breakpointLines } from '../editor/setup';
import type { UIStore } from './ui.svelte';
import type { WorkspaceStore } from './workspace.svelte';
import type { EditorsStore } from './editors.svelte';
import type { EmulatorStore } from './services.svelte';

export interface Breakpoint {
  path: string;
  line: number;
  enabled: boolean;
  /** Line the breakpoint was bound to in the running session, if any. */
  resolvedLine: number;
  verified: boolean;
}

export interface Watch { expression: string; value: string; type: string; error: string }

export type DebugCommand = 'continue' | 'pause' | 'stepInstruction' | 'stepLine' | 'stepOver' | 'stepOut';

const MEMORY_BYTES = 256;

export class DebugStore {
  breakpoints = $state<Breakpoint[]>([]);
  watches = $state<Watch[]>([]);
  active = $state(false);
  stopped = $state(false);
  stop = $state<DebugStop | null>(null);
  symbols = $state<DebugSymbol[]>([]);
  memory = $state<{ address: number; bytes: number[] } | null>(null);
  starting = $state(false);

  private unsubscribe: (() => void) | undefined;
  private lastSeq = 0;

  constructor(private ui: UIStore, private workspace: WorkspaceStore, private editors: EditorsStore, private emulator: EmulatorStore) {}

  connect(): void {
    this.unsubscribe?.();
    this.unsubscribe = native().on('debugger', event => void this.receive(event));
  }

  disconnect(): void { this.unsubscribe?.(); this.unsubscribe = undefined; }

  /** The line where execution is stopped, for the editor decoration. */
  get currentLine(): { path: string; line: number } | null {
    return this.stopped && this.stop?.path ? { path: this.stop.path, line: this.stop.line } : null;
  }

  get persistedBreakpoints() {
    return this.breakpoints.map(({ path, line, enabled }) => ({ path, line, enabled }));
  }

  breakpointsFor(path: string): Breakpoint[] { return this.breakpoints.filter(item => item.path === path); }

  restore(breakpoints: { path: string; line: number; enabled: boolean }[], watches: string[]): void {
    this.breakpoints = breakpoints.map(item => ({ ...item, resolvedLine: 0, verified: false }));
    this.watches = watches.map(expression => ({ expression, value: '', type: '', error: '' }));
  }

  // ---- Breakpoints ----------------------------------------------------------

  private async syncBreakpoints(): Promise<void> {
    this.editors.schedulePersist();
    if (!this.active) {
      this.breakpoints = this.breakpoints.map(item => ({ ...item, resolvedLine: 0, verified: false }));
      return;
    }
    try {
      const resolved = await native().studio.DebugSetBreakpoints(this.persistedBreakpoints);
      this.applyResolved(resolved ?? []);
    } catch (error) {
      this.ui.status = `Breakpoints: ${errorMessage(error)}`;
    }
  }

  private applyResolved(resolved: { path: string; line: number; resolvedLine: number; verified: boolean }[]): void {
    this.breakpoints = this.breakpoints.map(item => {
      const match = resolved.find(entry => entry.path === item.path && entry.line === item.line);
      return match ? { ...item, resolvedLine: match.resolvedLine, verified: match.verified } : item;
    });
  }

  toggleBreakpoint(path: string, line: number): void {
    if (!path || line < 1) return;
    const exists = this.breakpoints.some(item => item.path === path && item.line === line);
    this.breakpoints = exists
      ? this.breakpoints.filter(item => !(item.path === path && item.line === line))
      : [...this.breakpoints, { path, line, enabled: true, resolvedLine: 0, verified: false }];
    void this.syncBreakpoints();
  }

  setEnabled(path: string, line: number, enabled: boolean): void {
    this.breakpoints = this.breakpoints.map(item => (item.path === path && item.line === line ? { ...item, enabled } : item));
    void this.syncBreakpoints();
  }

  removeAll(): void { this.breakpoints = []; void this.syncBreakpoints(); }

  /** Breakpoints follow their line as the text above them changes. */
  documentChanged(path: string, state: EditorState): void {
    const mine = this.breakpointsFor(path);
    if (!mine.length) return;
    let lines: number[];
    try { lines = breakpointLines(state); } catch { return; }
    if (lines.length !== mine.length) return;
    const sorted = [...mine].sort((a, b) => a.line - b.line);
    if (sorted.every((item, index) => item.line === lines[index])) return;
    const moved = new Map(sorted.map((item, index) => [item, lines[index]]));
    this.breakpoints = this.breakpoints.map(item => (moved.has(item) ? { ...item, line: moved.get(item)! } : item));
    this.editors.schedulePersist();
  }

  pathRenamed(oldPath: string, newPath: string): void {
    this.breakpoints = this.breakpoints.map(item => (item.path === oldPath || item.path.startsWith(oldPath + '/') ? { ...item, path: newPath + item.path.slice(oldPath.length) } : item));
  }

  // ---- Session --------------------------------------------------------------

  /** Builds and starts debugging; `build` returns the fresh build result. */
  async start(build: () => Promise<BuildResult | null>): Promise<void> {
    if (this.active || this.starting || !this.workspace.project) return;
    this.starting = true;
    try {
      const result = await build();
      if (!result?.success) return;
      if (!result.debugAvailable) {
        this.ui.appendOutput(`\nSource-level debugging is unavailable for this build. ${result.debugNote}\n`);
        this.ui.showPanel('output');
        this.ui.status = 'Debugging unavailable';
        return;
      }
      const session = await native().studio.DebugStart({ profileId: this.emulator.profileId, breakpoints: this.persistedBreakpoints });
      this.active = true;
      this.stopped = false;
      this.stop = null;
      this.symbols = session.symbols ?? [];
      this.applyResolved(session.breakpoints ?? []);
      this.editors.emulatorOpen = true;
      this.emulator.keepRunning = true;
      this.emulator.started(session.emulator?.hostDeviceName ?? '');
      this.ui.showPanel('debug');
      this.ui.status = 'Debugging';
    } catch (error) {
      this.ui.fail('Starting the debugger', error);
      this.ui.showPanel('output');
    } finally {
      this.starting = false;
    }
  }

  async command(command: DebugCommand): Promise<void> {
    if (!this.active) return;
    try {
      await native().studio.DebugCommand(command);
    } catch (error) {
      this.ui.status = `Debugger: ${errorMessage(error)}`;
    }
  }

  async runToLine(path: string, line: number): Promise<void> {
    if (!this.stopped) return;
    try { await native().studio.DebugRunToLine(path, line); } catch (error) { this.ui.status = `Debugger: ${errorMessage(error)}`; }
  }

  /** Ends the session. `silent` skips the backend call (workspace closing). */
  async end(silent: boolean): Promise<void> {
    if (!this.active) return;
    if (!silent) await this.emulator.stop();
    this.ended('');
  }

  private ended(message: string): void {
    this.active = false; this.stopped = false; this.stop = null; this.symbols = []; this.memory = null;
    this.emulator.keepRunning = false;
    this.breakpoints = this.breakpoints.map(item => ({ ...item, resolvedLine: 0, verified: false }));
    if (message) {
      // A helper failure ends debugging as a visible stop, not a crash.
      this.emulator.running = false;
      this.ui.appendOutput(`\n${message}\nStart debugging again to continue.\n`);
      this.ui.showPanel('output');
    }
    this.ui.status = message ? 'Debugger stopped' : 'Debugging ended';
  }

  private async receive(event: DebugEvent): Promise<void> {
    if (event.seq <= this.lastSeq && event.type !== 'started') return;
    this.lastSeq = event.seq;
    switch (event.type) {
      case 'started':
        this.lastSeq = event.seq;
        return;
      case 'running':
        this.stopped = false; this.stop = null;
        this.ui.status = 'Running';
        return;
      case 'exited':
        this.ended(event.message ?? '');
        return;
      case 'stopped': {
        if (!event.stop) return;
        this.active = true;
        this.stopped = true;
        this.stop = event.stop;
        const reason = event.stop.reason === 'breakpoint' ? 'Breakpoint' : event.stop.reason === 'pause' ? 'Paused' : 'Stopped';
        this.ui.status = event.stop.path ? `${reason} at ${event.stop.path}:${event.stop.line}` : `${reason} at $${event.stop.registers.pc.toString(16).toUpperCase().padStart(4, '0')}`;
        // Bring the stopped line into view.
        if (event.stop.path) await this.editors.open(event.stop.path, { line: event.stop.line });
        await this.refreshWatches();
        if (this.memory) await this.readMemory(this.memory.address);
        return;
      }
    }
  }

  // ---- Watches and memory ---------------------------------------------------

  addWatch(expression: string): void {
    const clean = expression.trim();
    if (!clean || this.watches.some(watch => watch.expression === clean)) return;
    this.watches = [...this.watches, { expression: clean, value: '', type: '', error: '' }];
    this.editors.schedulePersist();
    void this.refreshWatches();
  }

  removeWatch(expression: string): void {
    this.watches = this.watches.filter(watch => watch.expression !== expression);
    this.editors.schedulePersist();
  }

  async refreshWatches(): Promise<void> {
    if (!this.active || !this.watches.length) return;
    try {
      const values = (await native().studio.DebugEvaluate(this.watches.map(watch => watch.expression))) ?? [];
      this.watches = this.watches.map(watch => {
        const value: WatchValue | undefined = values.find(item => item.expression === watch.expression);
        return value ? { expression: watch.expression, value: value.value, type: value.type, error: value.error ?? '' } : watch;
      });
    } catch (error) {
      this.ui.status = `Watches: ${errorMessage(error)}`;
    }
  }

  /** Reads a page of memory for the viewer (read-only). */
  async readMemory(address: number): Promise<void> {
    if (!this.active) return;
    const start = Math.max(0, Math.min(0x10000 - MEMORY_BYTES, address & 0xFFFF));
    try {
      const block = await native().studio.DebugReadMemory(start, MEMORY_BYTES);
      const bytes: number[] = [];
      for (let i = 0; i + 1 < block.data.length; i += 2) bytes.push(parseInt(block.data.slice(i, i + 2), 16));
      this.memory = { address: block.address, bytes };
    } catch (error) {
      this.ui.status = `Memory: ${errorMessage(error)}`;
    }
  }
}
