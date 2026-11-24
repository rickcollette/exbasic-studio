// The single doorway between the UI and the native backend. Components and
// stores call `native()`; the real Wails implementation is installed at
// startup and tests install a deterministic mock, so no UI code imports the
// Wails runtime or the generated bindings directly.
import type * as StudioBindings from '../../bindings/exbasicstudio/internal/studio/studioservice';
import type * as studioModels from '../../bindings/exbasicstudio/internal/studio/models';
import type * as projectModels from '../../bindings/exbasicstudio/internal/project/models';
import type * as compilerModels from '../../bindings/exbasicstudio/internal/compiler/models';
import type * as gitModels from '../../bindings/exbasicstudio/internal/git/models';
import type * as emulatorModels from '../../bindings/exbasicstudio/internal/emulator/models';
import type * as atrModels from '../../bindings/exbasicstudio/internal/atrexplorer/models';
import type * as stateModels from '../../bindings/exbasicstudio/internal/appstate/models';
import type * as operationModels from '../../bindings/exbasicstudio/internal/operations/models';
import type * as terminalModels from '../../bindings/exbasicstudio/internal/terminal/models';
import type * as manualModels from '../../bindings/exbasicstudio/internal/manual/models';
import type * as debuggerModels from '../../bindings/exbasicstudio/internal/debugger/models';

type Promisified<T> = {
  [K in keyof T]: T[K] extends (...args: infer A) => PromiseLike<infer R> ? (...args: A) => Promise<R> : never;
};

/** Every backend method, returning plain promises. */
export type StudioAPI = Promisified<typeof StudioBindings>;

export type ProjectView = projectModels.View;
export type ProjectConfig = projectModels.Config;
export type ProjectFile = projectModels.File;
/** A file produced by a build. */
export interface BuildArtifact { kind: string; path: string }

/** Structured outcome of a build, delivered with the operation's completion. */
export interface BuildResult {
  success: boolean;
  cancelled: boolean;
  steps: { name: string; argv: string[] }[] | null;
  output: string;
  artifacts: BuildArtifact[] | null;
  diagnostics: Diagnostic[] | null;
  metadataPath: string;
  debugAvailable: boolean;
  debugNote: string;
  durationMs: number;
}
export type Diagnostic = compilerModels.Diagnostic;
/** Completion result of a run operation. */
export interface RunResult { build: BuildResult; started: boolean; emulator: EmulatorState }
export type LintResult = studioModels.LintResult;
export type Environment = studioModels.Environment;
export type ClaudeCodeInfo = studioModels.ClaudeCodeInfo;
export type SearchResult = studioModels.SearchResult;
export type TrustInfo = studioModels.TrustInfo;
export type GitStatus = gitModels.Status;
export type GitChange = gitModels.Change;
export type GitBranch = gitModels.Branch;
export type GitRemote = gitModels.Remote;
export type GitConflict = gitModels.Conflict;
export type EmulatorProfile = emulatorModels.Profile;
export type EmulatorFrame = emulatorModels.Frame;
export type EmulatorState = emulatorModels.State;
export type ATRView = atrModels.View;
export type ATREntry = atrModels.Entry;
export type AppSnapshot = stateModels.Snapshot;
export type WorkspaceState = stateModels.Workspace;
export type LayoutState = stateModels.Layout;
export type Recovery = stateModels.Recovery;
export type RecentProject = stateModels.RecentProject;
export type OperationInfo = operationModels.Info;
export type TerminalInfo = terminalModels.Info;
export type ManualHit = manualModels.Hit;
export type DebugStop = studioModels.DebugStop;
export type DebugSymbol = debuggerModels.Symbol;
export type WatchValue = debuggerModels.WatchValue;
export type StackFrame = debuggerModels.StackFrame;
export type Instruction = debuggerModels.Instruction;

/** Ordered debugger update. */
export interface DebugEvent {
  seq: number;
  type: 'started' | 'running' | 'stopped' | 'exited';
  stop?: DebugStop | null;
  message?: string;
}

/** Ordered update about a long-running backend operation. */
export interface OperationEvent {
  id: string;
  kind: string;
  seq: number;
  type: 'started' | 'output' | 'completed';
  stream?: string;
  text?: string;
  result?: unknown;
  error?: string;
  cancelled?: boolean;
}

export interface TerminalEvent {
  id: string;
  seq: number;
  type: 'output' | 'exit';
  data?: string;
  exitCode: number;
}

export interface WorkspaceChange {
  path: string;
  kind: 'create' | 'write' | 'remove';
  directory: boolean;
  size: number;
}

export interface WorkspaceBatch {
  root: string;
  changes: WorkspaceChange[];
  overflow: boolean;
}

export interface FilesDropped {
  files: string[];
  /** Element id of the drop target, when it declared one. */
  targetId: string;
}

export interface NativeEvents {
  operation: OperationEvent;
  terminal: TerminalEvent;
  'workspace-changed': WorkspaceBatch;
  debugger: DebugEvent;
  'files-dropped': FilesDropped;
}

export interface OpenDialogOptions {
  title: string;
  directory?: string;
  chooseDirectories?: boolean;
  chooseFiles?: boolean;
  multiple?: boolean;
  filters?: { name: string; pattern: string }[];
}

export interface Native {
  studio: StudioAPI;
  window: { minimise(): void; toggleMaximise(): void; close(): void };
  dialogs: {
    /** Returns the chosen paths; empty when the user cancels. */
    open(options: OpenDialogOptions): Promise<string[]>;
    /** Returns the chosen path, or "" when the user cancels. */
    save(options: { title: string; filename?: string; filters?: { name: string; pattern: string }[] }): Promise<string>;
  };
  clipboard: { setText(text: string): Promise<void> };
  /** Subscribes to a backend event; returns the unsubscribe function. */
  on<K extends keyof NativeEvents>(name: K, handler: (data: NativeEvents[K]) => void): () => void;
}

let current: Native | undefined;

export function setNative(native: Native): void {
  current = native;
}

export function native(): Native {
  if (!current) throw new Error('The native backend adapter has not been installed.');
  return current;
}

/** Turns anything thrown by a backend call into a displayable message. */
export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  if (error && typeof error === 'object' && 'message' in error) return String((error as { message: unknown }).message);
  return String(error);
}

/** True for the backend's "restricted mode" refusal. */
export function isRestricted(error: unknown): boolean {
  return errorMessage(error).includes('restricted mode');
}
