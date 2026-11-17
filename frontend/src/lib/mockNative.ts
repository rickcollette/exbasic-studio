// A deterministic in-memory backend. It implements the same adapter as the
// Wails backend so the whole UI can run in a plain browser: component tests
// and Playwright end-to-end tests install it instead of the native bindings.
//
// Behaviour is scripted rather than emulated: a source containing the text
// "SYNTAXERROR" fails to compile on that line, breakpoints stop on the line
// requested, and the terminal echoes what is typed.
import type {
  AppSnapshot, ATRView, BuildResult, DebugStop, Diagnostic, EmulatorProfile, GitStatus, LayoutState, Native, NativeEvents,
  OpenDialogOptions, ProjectConfig, ProjectView, StudioAPI, WorkspaceState,
} from './native';

type Handler = (data: any) => void;

/** Deep copy that also accepts reactive state proxies. */
function clone<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }

export interface MockNative extends Native {
  /** Workspace files by root, then relative path. */
  disk: Map<string, Map<string, string>>;
  /** Every backend call, in order: "Method(arg, ...)". */
  calls: string[];
  /** Paths returned by the next open dialogs, consumed in order. */
  dialogQueue: string[][];
  /** Text copied to the clipboard. */
  clipboard: { setText(text: string): Promise<void>; text: string };
  /** Emits a backend event to the UI. */
  emit<K extends keyof NativeEvents>(name: K, data: NativeEvents[K]): void;
  /** Changes a file as if another program wrote it, and notifies the UI. */
  externalWrite(path: string, content: string | null): void;
  /** Roots the user has trusted. */
  trusted: Set<string>;
  /** Makes the next call of a method fail with this message. */
  failNext(method: string, message: string): void;
  /** Stored crash-recovery copies for the open root. */
  recoveries: Map<string, string>;
  git: { initialized: boolean; staged: Set<string>; committed: Map<string, string>; branch: string; branches: string[]; remotes: Map<string, string>; conflict: string | null };
  /** Whether the Claude Code CLI is "installed". */
  claude: { available: boolean };
}

const PIXEL = 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';
const BINARY_MARKER = '\u0000bin:';

function defaultConfig(name: string): ProjectConfig {
  return {
    schemaVersion: 1, name, entry: 'src/main.bas', target: 'atari', output: `build/${name}.xex`, compiler: 'exbasic', emulator: 'atari800-studio',
    native: 'auto', debug: true, includePaths: ['src', 'lib'], assembly: [],
    atr: { enabled: false, tool: 'mkatr-go', filesystem: 'sparta', output: `build/${name}.atr`, boot: true, exactSize: false, minimumSize: 0, bootPage: 7, files: [] },
  };
}

function defaultProfiles(): EmulatorProfile[] {
  const blank = () => Array.from({ length: 4 }, () => ({ image: '', enabled: false, readOnly: false }));
  const base = { cartridge: { kind: '', image: '', type: 0 }, cartridge2: { kind: '', image: '', type: 0 }, machine: 'xl', memory: '64', video: 'ntsc', cpuSpeed: 100, basic: false, sioPatch: true, ultimate1MB: false, scale: 2, media: '', hostDir: '', rDevice: { mode: 'disabled', listenPort: 9000, connectBaud: 2400, serialPort: '', modem: false }, ide: { enabled: false, image: '', compactFlash: false }, arguments: [] };
  return [
    { ...base, id: 'atari-800xl-ntsc', name: 'Atari 800XL (NTSC)', eos: false, drives: blank() },
    { ...base, id: 'eos-800xl-ntsc', name: 'ExtendedOS / EOS', eos: true, cartridge: { kind: 'eos', image: '', type: 0 }, drives: blank() },
  ];
}

export function createMockNative(): MockNative {
  const handlers = new Map<string, Set<Handler>>();
  const disk = new Map<string, Map<string, string>>();
  const configs = new Map<string, ProjectConfig>();
  const hasConfig = new Set<string>();
  const trusted = new Set<string>();
  const trustFingerprint = new Map<string, string>();
  const calls: string[] = [];
  const failures = new Map<string, string>();
  const recoveryByRoot = new Map<string, Map<string, string>>();
  const workspaceStates = new Map<string, WorkspaceState>();
  let root = '';
  let recents: { path: string; name: string; openedAt: string }[] = [];
  let layout: LayoutState = { primaryOpen: true, panelOpen: true, activity: 'explorer', panelTab: 'terminal', primaryWidth: 280, panelHeight: 240 };
  const preferences: Record<string, string> = {};
  let profiles = defaultProfiles();
  let operationCounter = 0;
  let terminalCounter = 0;
  const claude = { available: true };
  const terminals = new Map<string, { seq: number; line: string }>();
  const pendingBuilds = new Map<string, () => void>();
  const emulator = { running: false, profileId: '', artifact: '', frame: 0 };
  const debug = { active: false, stopped: false, seq: 0, breakpoints: [] as { path: string; line: number; enabled: boolean }[], line: 0, path: '', stop: null as DebugStop | null, pendingStop: null as { path: string; line: number; reason: string } | null };
  const atrImages = new Map<string, ATRView>();
  const git: MockNative['git'] = { initialized: false, staged: new Set(), committed: new Map(), branch: 'main', branches: [], remotes: new Map(), conflict: null };

  const files = () => disk.get(root) ?? new Map<string, string>();
  const config = () => configs.get(root)!;
  const fingerprint = (cfg: ProjectConfig) => [cfg.compiler, cfg.emulator, cfg.atr.tool].join('\u0000');
  const isTrusted = () => trusted.has(root) && trustFingerprint.get(root) === fingerprint(config());
  const emit = <K extends keyof NativeEvents>(name: K, data: NativeEvents[K]) => { for (const handler of handlers.get(name) ?? []) handler(data); };
  const later = (work: () => void) => { setTimeout(work, 0); };
  const requireProject = () => { if (!root) throw new Error('open or create a project first'); };
  const requireTrust = (action: string) => { requireProject(); if (!isTrusted()) throw new Error(`restricted mode: trust this workspace to ${action}`); };
  const checkRel = (rel: string) => { if (!rel || rel.startsWith('/') || rel.split(/[\\/]/).includes('..') || /^[A-Za-z]:/.test(rel)) throw new Error('path outside project'); };

  function view(): ProjectView {
    const list = [...files().keys()].filter(path => !path.endsWith('/.keep') && !path.split('/').some(part => part.startsWith('.') || part === 'build')).sort();
    const directories = new Set<string>();
    for (const path of files().keys()) {
      const parts = path.split('/');
      for (let i = 1; i < parts.length; i++) {
        const directory = parts.slice(0, i).join('/');
        if (!directory.split('/').some(part => part.startsWith('.') || part === 'build')) directories.add(directory);
      }
    }
    return { root, config: clone(config()), files: list.map(path => ({ path, size: files().get(path)!.length })), directories: [...directories].sort(), hasConfig: hasConfig.has(root) };
  }

  function snapshot(): AppSnapshot {
    return {
      schemaVersion: 1, recentProjects: recents.map(item => ({ ...item })) as any, layout: { ...layout }, preferences: { ...preferences },
      workspace: clone(workspaceStates.get(root) ?? { tabs: [], activeEditor: '', emulatorProfile: '', breakpoints: [], watches: [] }),
    };
  }

  function adopt(path: string, name: string) {
    root = path;
    recents = [{ path, name, openedAt: new Date(0).toISOString() }, ...recents.filter(item => item.path !== path)].slice(0, 20);
    for (const id of terminals.keys()) terminals.delete(id);
    emulator.running = false; debug.active = false; debug.stopped = false;
    git.initialized = files().has('.git/HEAD');
  }

  function diagnosticsFor(source: Map<string, string>): Diagnostic[] {
    const out: Diagnostic[] = [];
    for (const [path, content] of source) {
      if (!/\.bas$/i.test(path)) continue;
      content.split('\n').forEach((line, index) => {
        const column = line.indexOf('SYNTAXERROR');
        if (column >= 0) out.push({ path, line: index + 1, column: column + 1, endLine: index + 1, endColumn: column + 12, severity: 'error', code: 'syntax', message: 'parse error, expected: statement' });
      });
    }
    return out;
  }

  function buildResult(): BuildResult {
    const diagnostics = diagnosticsFor(files());
    const output = `${root}/${config().output}`;
    const success = diagnostics.length === 0;
    if (success) files().set(config().output, 'XEX');
    return {
      success, cancelled: false, steps: [{ name: 'compile', argv: ['exbasic', '-t:atari', '-o', output, `${root}/${config().entry}`] }],
      output: success ? 'BAS compile\nLINK\n' : diagnostics.map(item => `${item.path}:${item.line}:${item.column - 1}: ${item.message}\n`).join(''),
      artifacts: success ? [{ kind: 'executable', path: output }] : [], diagnostics, metadataPath: success ? output + '.exbmeta.json' : '',
      debugAvailable: success && config().debug, debugNote: success && !config().debug ? 'This build has no debug information. Enable "Debug symbols" in the project settings to debug at source level.' : '',
      durationMs: 12,
    };
  }

  function startOperation(kind: string, run: (finish: (result: unknown, error?: string) => void, output: (text: string) => void, cancelled: () => boolean) => void) {
    const id = `op-${++operationCounter}`;
    let seq = 0;
    let cancelledFlag = false;
    let done = false;
    const send = (event: Partial<NativeEvents['operation']>) => emit('operation', { id, kind, seq: ++seq, type: 'output', ...event } as NativeEvents['operation']);
    const finish = (result: unknown, error?: string) => {
      if (done) return;
      done = true;
      pendingBuilds.delete(id);
      send({ type: 'completed', result, error, cancelled: cancelledFlag });
    };
    pendingBuilds.set(id, () => { cancelledFlag = true; });
    send({ type: 'started' });
    later(() => run(finish, text => send({ type: 'output', stream: 'stdout', text }), () => cancelledFlag));
    return { id, kind };
  }

  function debugStop(path: string, line: number, reason: string): DebugStop {
    return {
      reason, path, line, location: 0x3000 + line, bytecode: true,
      registers: { state: 'stopped', reason, pc: 0x0082, a: 1, x: 2, y: 3, s: 0xF7, p: 0x30, location: 0x3000 + line, bytecode: true },
      frames: [{ return: 0x2103, call: 0x2100, target: 0x2200, path, line }],
      disassembly: [{ address: 0x0082, bytes: 'AC 34 12', text: 'LDY $1234' }, { address: 0x0085, bytes: 'E6 83', text: 'INC $83' }],
    };
  }

  function emitDebug(type: 'started' | 'running' | 'stopped' | 'exited', stop?: DebugStop, message?: string) {
    emit('debugger', { seq: ++debug.seq, type, stop: stop ?? null, message });
  }

  /** Scripted execution: stop at the next enabled breakpoint after `from`. */
  function runToNextBreakpoint(from: number) {
    const lines = (files().get(config().entry) ?? '').split('\n').length;
    const candidates = debug.breakpoints.filter(bp => bp.enabled && bp.path === config().entry).map(bp => bp.line).sort((a, b) => a - b);
    const next = candidates.find(line => line > from) ?? candidates[0];
    debug.stopped = false;
    emitDebug('running');
    if (next === undefined || next > lines) return;
    debug.pendingStop = { path: config().entry, line: next, reason: 'breakpoint' };
  }

  const studio: Record<string, (...args: any[]) => unknown> = {
    GetEnvironment: () => ({ version: '1.0.0-test', platform: 'test/mock', workingDir: '/home/tester', compilerHint: '/tools/exbasic/exbasic', emulatorHint: '/tools/atari800-studio', toolsAvailable: true, toolsNote: '' }),
    AppState: () => snapshot(),
    SaveLayout: (value: LayoutState) => { layout = { ...value }; },
    SetPreference: (key: string, value: string) => { if (value) preferences[key] = value; else delete preferences[key]; },
    SaveWorkspaceState: (value: WorkspaceState) => { if (root) workspaceStates.set(root, clone(value)); },
    RemoveRecentProject: (path: string) => { recents = path ? recents.filter(item => item.path !== path) : []; return snapshot(); },
    LogFrontendError: () => {},
    ExportSupportBundle: (path: string) => { if (!path) throw new Error('choose where to save the support bundle'); },

    CreateProject: (parent: string, name: string) => {
      if (!/^[a-zA-Z][a-zA-Z0-9_-]{0,63}$/.test(name)) throw new Error('project name must begin with a letter and contain only letters, numbers, _ or -');
      const path = `${parent.replace(/[\\/]+$/, '')}/${name}`;
      if (disk.has(path)) throw new Error('create project: the folder already exists');
      disk.set(path, new Map([['src/main.bas', `' ${name} - exBASIC project\nGRAPHICS 0\nPRINT "HELLO FROM exBASIC STUDIO!"\n`], ['lib/.keep', '']]));
      configs.set(path, defaultConfig(name));
      hasConfig.add(path);
      trusted.add(path);
      trustFingerprint.set(path, fingerprint(configs.get(path)!));
      adopt(path, name);
      return view();
    },
    OpenProject: (path: string) => {
      if (!disk.has(path)) throw new Error(`CreateFile ${path}: The system cannot find the path specified.`);
      if (!configs.has(path)) {
        const name = path.split(/[\\/]/).filter(Boolean).pop() ?? 'workspace';
        const cfg = defaultConfig(name);
        const basic = [...disk.get(path)!.keys()].filter(file => /\.bas$/i.test(file)).sort();
        cfg.entry = basic.includes('src/main.bas') ? 'src/main.bas' : basic[0] ?? cfg.entry;
        configs.set(path, cfg);
      }
      adopt(path, configs.get(path)!.name);
      return view();
    },
    CloseProject: () => { root = ''; },
    GetProject: () => { requireProject(); return view(); },
    WorkspaceTrust: () => { requireProject(); return { trusted: isTrusted(), tools: [config().compiler, config().emulator, config().atr.tool] }; },
    TrustWorkspace: () => { requireProject(); trusted.add(root); trustFingerprint.set(root, fingerprint(config())); return studio.WorkspaceTrust(); },
    RevokeWorkspaceTrust: () => { requireProject(); trusted.delete(root); terminals.clear(); return studio.WorkspaceTrust(); },
    ReadFile: (rel: string) => {
      requireProject(); checkRel(rel);
      const content = files().get(rel);
      if (content === undefined) throw new Error(`open ${rel}: The system cannot find the file specified.`);
      if (content.includes('\u0000')) throw new Error('file is not UTF-8 text and cannot be edited');
      return content;
    },
    SaveFile: (rel: string, content: string) => {
      requireProject(); checkRel(rel);
      if (rel === 'exbasic.project.json') throw new Error('use project settings to update the configuration');
      files().set(rel, content);
      recoveryByRoot.get(root)?.delete(rel);
    },
    // Binary files are kept as a NUL marker plus base64, so ReadFile refuses
    // them as text the way the backend does.
    ReadBinaryFile: (rel: string) => {
      requireProject(); checkRel(rel);
      const content = files().get(rel);
      if (content === undefined) throw new Error(`open ${rel}: The system cannot find the file specified.`);
      if (content.startsWith(BINARY_MARKER)) return content.slice(BINARY_MARKER.length);
      return btoa(String.fromCharCode(...new TextEncoder().encode(content)));
    },
    SaveBinaryFile: (rel: string, data: string) => {
      requireProject(); checkRel(rel);
      if (rel === 'exbasic.project.json') throw new Error('use project settings to update the configuration');
      atob(data);
      files().set(rel, BINARY_MARKER + data);
    },
    CreateFolder: (rel: string) => { requireProject(); checkRel(rel); files().set(`${rel}/.keep`, ''); return view(); },
    ImportProjectPaths: (destination: string, paths: string[]) => {
      requireProject();
      for (const path of paths) {
        const name = path.split(/[\\/]/).pop()!;
        const target = destination && destination !== '.' ? `${destination}/${name}` : name;
        if (files().has(target)) throw new Error(`"${name}" already exists in the destination`);
        files().set(target, `imported from ${path}\n`);
      }
      return view();
    },
    RenamePath: (oldRel: string, newRel: string) => {
      requireProject(); checkRel(oldRel); checkRel(newRel);
      if (files().has(newRel)) throw new Error('the destination already exists');
      for (const [path, content] of [...files()]) {
        if (path === oldRel || path.startsWith(oldRel + '/')) { files().delete(path); files().set(newRel + path.slice(oldRel.length), content); }
      }
      const cfg = config();
      if (cfg.entry === oldRel || cfg.entry.startsWith(oldRel + '/')) cfg.entry = newRel + cfg.entry.slice(oldRel.length);
      return view();
    },
    DeletePath: (rel: string) => {
      requireProject(); checkRel(rel);
      for (const path of [...files().keys()]) if (path === rel || path.startsWith(rel + '/')) files().delete(path);
      return view();
    },
    SearchProject: (query: string) => {
      requireProject();
      const needle = query.trim().toLowerCase();
      const out: { path: string; line: number; text: string }[] = [];
      for (const [path, content] of [...files()].sort()) {
        content.split('\n').forEach((line, index) => { if (needle.length >= 2 && line.toLowerCase().includes(needle) && out.length < 200) out.push({ path, line: index + 1, text: line.trim() }); });
      }
      return out;
    },
    UpdateConfig: (cfg: ProjectConfig) => {
      requireProject();
      if (!cfg.name) throw new Error('missing name');
      if (cfg.atr.enabled && cfg.atr.filesystem === 'eos' && cfg.atr.boot) throw new Error('EOSFS images cannot use the SDFS bootloader; disable ATR boot and mount the image under EOS');
      configs.set(root, clone({ ...cfg, schemaVersion: 1 }));
      hasConfig.add(root);
      return view();
    },

    Lint: (rel: string, content: string, version: number, overlays: Record<string, string> = {}) => {
      requireProject();
      if (!isTrusted()) return { path: rel, version, available: false, diagnostics: [] };
      return { path: rel, version, available: true, diagnostics: diagnosticsFor(new Map([...Object.entries(overlays), [rel, content]])) };
    },
    StartBuild: () => {
      requireTrust('build it');
      return startOperation('build', (finish, output, cancelled) => {
        output('BAS compile\n');
        later(() => { if (cancelled()) finish({ ...buildResult(), success: false, cancelled: true, artifacts: [], diagnostics: [] }); else { const result = buildResult(); output(result.output); finish(result); } });
      });
    },
    StartRun: (request: { profileId: string }) => {
      requireTrust('run it');
      return startOperation('build', (finish, output) => {
        const build = buildResult();
        output(build.output);
        if (build.success) { emulator.running = true; emulator.profileId = request.profileId; emulator.artifact = build.artifacts![0].path; emulator.frame = 0; }
        finish({ build, started: build.success, emulator: { running: emulator.running, profileId: emulator.profileId, artifact: emulator.artifact, hostDeviceName: build.success ? 'H1:' : '' } });
      });
    },
    CancelOperation: (id: string) => { const cancel = pendingBuilds.get(id); cancel?.(); return !!cancel; },
    RunningOperations: () => [...pendingBuilds.keys()].map(id => ({ id, kind: 'build' })),

    TerminalOpen: (cols: number, rows: number) => {
      requireTrust('open a terminal in it');
      const id = `term-${++terminalCounter}`;
      terminals.set(id, { seq: 0, line: '' });
      later(() => emit('terminal', { id, seq: ++terminals.get(id)!.seq, type: 'output', data: `mock-shell ${root}> `, exitCode: 0 }));
      return { id, shell: '/bin/mock-shell', dir: root, cols, rows };
    },
    ClaudeStatus: () => ({ available: claude.available, path: claude.available ? '/usr/local/bin/claude' : '', install: 'curl -fsSL https://claude.ai/install.sh | bash' }),
    ClaudeOpen: (cols: number, rows: number) => {
      requireTrust('start Claude Code in it');
      if (!claude.available) throw new Error('Claude Code is not installed');
      const id = `term-${++terminalCounter}`;
      terminals.set(id, { seq: 0, line: '' });
      later(() => emit('terminal', { id, seq: ++terminals.get(id)!.seq, type: 'output', data: 'Welcome to Claude Code (mock)\r\n> ', exitCode: 0 }));
      return { id, shell: '/usr/local/bin/claude', dir: root, cols, rows };
    },
    TerminalWrite: (id: string, data: string) => {
      const session = terminals.get(id);
      if (!session) throw new Error('the terminal session has ended');
      const send = (text: string) => emit('terminal', { id, seq: ++session.seq, type: 'output', data: text, exitCode: 0 });
      for (const ch of data) {
        if (ch === '\u0003') { session.line = ''; send('^C\r\nmock-shell> '); }
        else if (ch === '\r' || ch === '\n') {
          const command = session.line.trim();
          session.line = '';
          if (command === 'exit') { send('\r\n'); terminals.delete(id); emit('terminal', { id, seq: ++session.seq, type: 'exit', exitCode: 0 }); return; }
          send(`\r\n${command ? `ran: ${command}\r\n` : ''}mock-shell> `);
        } else { session.line += ch; send(ch); }
      }
    },
    TerminalResize: () => {},
    TerminalClose: (id: string) => { const session = terminals.get(id); if (session) { terminals.delete(id); emit('terminal', { id, seq: ++session.seq, type: 'exit', exitCode: 1 }); } },
    TerminalList: () => [...terminals.keys()].map(id => ({ id, shell: '/bin/mock-shell', dir: root, cols: 80, rows: 24 })),

    GitStatus: (): GitStatus => {
      requireProject();
      if (!git.initialized) return { available: false, branch: '', detached: false, unborn: false, upstream: '', ahead: 0, behind: 0, merging: false, changes: [], message: 'No Git repository (or Git is unavailable).' };
      const changes: GitStatus['changes'] = [];
      for (const [path, content] of [...files()].sort()) {
        if (path.startsWith('.git/') || path.startsWith('build/') || path.endsWith('/.keep')) continue;
        const committed = git.committed.get(path);
        if (git.conflict === path) { changes.push({ path, index: 'U', worktree: 'U', staged: false, untracked: false, conflicted: true }); continue; }
        if (committed === content) continue;
        const staged = git.staged.has(path);
        if (committed === undefined) changes.push(staged ? { path, index: 'A', worktree: '.', staged: true, untracked: false, conflicted: false } : { path, index: '?', worktree: '?', staged: false, untracked: true, conflicted: false });
        else changes.push(staged ? { path, index: 'M', worktree: '.', staged: true, untracked: false, conflicted: false } : { path, index: '.', worktree: 'M', staged: false, untracked: false, conflicted: false });
      }
      return { available: true, branch: git.branch, detached: false, unborn: git.committed.size === 0, upstream: git.remotes.size && git.committed.size ? `origin/${git.branch}` : '', ahead: 0, behind: 0, merging: git.conflict !== null, changes };
    },
    GitInit: () => { requireProject(); git.initialized = true; files().set('.git/HEAD', 'ref: refs/heads/main\n'); return 'Initialized empty Git repository\n'; },
    GitDiff: (path: string) => `--- a/${path}\n+++ b/${path}\n@@ -1 +1 @@\n-${(git.committed.get(path) ?? '').split('\n')[0]}\n+${(files().get(path) ?? '').split('\n')[0]}\n`,
    GitStage: (path: string) => { git.staged.add(path); return ''; },
    GitStageAll: () => { for (const path of files().keys()) if (!path.startsWith('.git/')) git.staged.add(path); return ''; },
    GitUnstage: (path: string) => { git.staged.delete(path); return ''; },
    GitUnstageAll: () => { git.staged.clear(); return ''; },
    GitCommit: (message: string) => {
      requireTrust('commit (commit hooks may run)');
      if (!message.trim()) throw new Error('commit message is empty');
      for (const path of git.staged) { const content = files().get(path); if (content !== undefined) git.committed.set(path, content); }
      git.staged.clear();
      if (!git.branches.includes(git.branch)) git.branches.push(git.branch);
      return `[${git.branch}] ${message}\n`;
    },
    GitBranches: () => git.branches.map(name => ({ name, current: name === git.branch, upstream: '' })),
    GitCreateBranch: (name: string, checkout: boolean) => {
      requireTrust('create branches');
      if (!name || /\s|^-/.test(name)) throw new Error(`invalid branch name "${name}"`);
      git.branches.push(name);
      if (checkout) git.branch = name;
      return '';
    },
    GitSwitchBranch: (name: string) => { requireTrust('switch branches'); git.branch = name; return ''; },
    GitDeleteBranch: (name: string) => { git.branches = git.branches.filter(item => item !== name); return ''; },
    GitRemotes: () => [...git.remotes].map(([name, url]) => ({ name, fetchUrl: url, pushUrl: url })),
    GitAddRemote: (name: string, url: string) => { if (git.remotes.has(name)) throw new Error(`git remote: remote ${name} already exists.`); git.remotes.set(name, url); return ''; },
    GitSetRemoteURL: (name: string, url: string) => { git.remotes.set(name, url); return ''; },
    GitRenameRemote: (oldName: string, newName: string) => { git.remotes.set(newName, git.remotes.get(oldName) ?? ''); git.remotes.delete(oldName); return ''; },
    GitRemoveRemote: (name: string) => { git.remotes.delete(name); return ''; },
    StartGitNetwork: (action: string, remote: string) => {
      requireTrust(`${action} (network access and hooks)`);
      return startOperation('git', (finish, output) => {
        output(`${action} ${remote || 'origin'}: done\n`);
        const url = git.remotes.get(remote || 'origin') ?? '';
        finish(studio.GitStatus(), url.includes('rejected') ? 'git ' + action + ': Authentication failed' : undefined);
      });
    },
    GitConflict: (path: string) => ({ path, base: 'PRINT 0\n', current: 'PRINT "CURRENT"\n', incoming: 'PRINT "INCOMING"\n', hasBase: true, hasCurrent: true, hasIncoming: true, working: files().get(path) ?? '', binary: false }),
    GitResolveConflict: (path: string, choice: string) => {
      files().set(path, choice === 'current' ? 'PRINT "CURRENT"\n' : choice === 'incoming' ? 'PRINT "INCOMING"\n' : 'PRINT "CURRENT"\nPRINT "INCOMING"\n');
      git.conflict = null; git.staged.add(path);
      return '';
    },
    GitStageResolved: (path: string) => {
      if ((files().get(path) ?? '').includes('<<<<<<< ')) throw new Error(`${path} still contains conflict markers`);
      git.conflict = null; git.staged.add(path);
      return '';
    },
    GitMergeContinue: () => { requireTrust('conclude the merge'); return studio.GitCommit('Merge'); },
    GitMergeAbort: () => { if (git.conflict) files().set(git.conflict, git.committed.get(git.conflict) ?? ''); git.conflict = null; return ''; },

    ManualFiles: () => ['01-introduction.md', '10-console-io.md'],
    ManualChapter: (file: string) => {
      if (file === '10-console-io.md') return '# Console I/O\n\n## PRINT\n\n`PRINT` writes values. See the [introduction](01-introduction.md#welcome).\n\n| Form | Meaning |\n|---|---|\n| `PRINT x` | print x |\n';
      if (file === '01-introduction.md') return '# Introduction\n\n## Welcome\n\nexBASIC is a compiler. Read about [PRINT](10-console-io.md#print).\n';
      throw new Error('invalid chapter name');
    },
    SearchManual: (query: string) => (query.trim().toLowerCase().includes('print') ? [{ file: '10-console-io.md', title: 'Console I/O', snippet: '`PRINT` writes values.' }] : []),

    SaveRecovery: (rel: string, content: string) => { requireProject(); if (!recoveryByRoot.has(root)) recoveryByRoot.set(root, new Map()); recoveryByRoot.get(root)!.set(rel, content); },
    DiscardRecovery: (rel: string) => { recoveryByRoot.get(root)?.delete(rel); },
    Recoveries: () => [...(recoveryByRoot.get(root) ?? [])].filter(([path, content]) => files().get(path) !== content).map(([path, content]) => ({ path, content, savedAt: new Date(0).toISOString() })),
    ClearRecoveryData: () => { recoveryByRoot.clear(); },

    EmulatorProfiles: () => clone(profiles),
    EmulatorSaveProfile: (profile: EmulatorProfile) => {
      const saved = { ...clone(profile), id: profile.id || `profile-${profiles.length + 1}` };
      profiles = [...profiles.filter(item => item.id !== saved.id), saved];
      return saved;
    },
    EmulatorDeleteProfile: (id: string) => { profiles = profiles.filter(item => item.id !== id); },
    EmulatorStart: (profileId: string, artifact: string) => { emulator.running = true; emulator.profileId = profileId; emulator.artifact = artifact; emulator.frame = 0; return { running: true, profileId, artifact, hostDeviceName: '' }; },
    EmulatorRestart: () => { emulator.frame = 0; return { running: true, profileId: emulator.profileId, artifact: emulator.artifact }; },
    EmulatorStatus: () => ({ running: emulator.running, profileId: emulator.profileId, artifact: emulator.artifact }),
    EmulatorFrame: () => {
      if (!emulator.running) throw new Error('Atari800 is not running');
      if (!debug.stopped) emulator.frame += 4;
      if (debug.active && debug.pendingStop) {
        const { path, line, reason } = debug.pendingStop;
        debug.pendingStop = null;
        debug.stopped = true; debug.path = path; debug.line = line;
        debug.stop = debugStop(path, line, reason);
        emitDebug('stopped', debug.stop);
      }
      return { running: true, image: PIXEL, number: emulator.frame, hostDevice: false };
    },
    EmulatorKey: () => {},
    EmulatorJoystick: () => {},
    EmulatorReleaseKeys: () => {},
    EmulatorStop: () => {
      emulator.running = false;
      if (debug.active) { debug.active = false; debug.stopped = false; emitDebug('exited'); }
    },

    DebugStart: (request: { profileId: string; breakpoints: { path: string; line: number; enabled: boolean }[] }) => {
      requireTrust('debug it');
      if (!files().has(config().output)) throw new Error('build the project before debugging');
      debug.active = true; debug.stopped = false; debug.breakpoints = clone(request.breakpoints ?? []); debug.line = 0;
      emulator.running = true; emulator.profileId = request.profileId; emulator.frame = 0;
      emitDebug('started');
      later(() => runToNextBreakpoint(0));
      return {
        breakpoints: debug.breakpoints.map(bp => ({ ...bp, resolvedLine: bp.line, verified: true, addresses: [0x3000 + bp.line] })),
        symbols: [{ name: 'SCORE', type: 'word', address: 0x2400, size: 2 }, { name: 'NAME$', type: 'string', address: 0x2402, size: 2 }],
        emulator: { running: true, profileId: request.profileId, artifact: `${root}/${config().output}`, hostDeviceName: 'H1:' },
      };
    },
    DebugCommand: (command: string) => {
      if (!debug.active) throw new Error('no debug session is active');
      if (command === 'pause') { if (!debug.stopped) debug.pendingStop = { path: config().entry, line: Math.max(1, debug.line), reason: 'pause' }; return; }
      if (!debug.stopped) throw new Error('the machine is not stopped');
      if (command === 'continue') { runToNextBreakpoint(debug.line); return; }
      if (!['stepLine', 'stepInstruction', 'stepOver', 'stepOut'].includes(command)) throw new Error(`unknown debugger command "${command}"`);
      debug.stopped = false;
      emitDebug('running');
      debug.pendingStop = { path: debug.path, line: command === 'stepLine' ? debug.line + 1 : debug.line, reason: 'step' };
    },
    DebugRunToLine: (path: string, line: number) => { debug.stopped = false; emitDebug('running'); debug.pendingStop = { path, line, reason: 'step' }; },
    DebugSetBreakpoints: (breakpoints: { path: string; line: number; enabled: boolean }[]) => {
      debug.breakpoints = clone(breakpoints);
      return breakpoints.map(bp => ({ ...bp, resolvedLine: debug.active ? bp.line : 0, verified: debug.active, addresses: debug.active ? [0x3000 + bp.line] : [] }));
    },
    DebugReadMemory: (address: number, length: number) => {
      if (!debug.active) throw new Error('no debug session is active');
      if (length < 1 || length > 1024 || address < 0 || address + length > 0x10000) throw new Error(`invalid memory range ${address}+${length}`);
      return { address, data: Array.from({ length }, (_, i) => ((address + i) & 0xFF).toString(16).padStart(2, '0')).join('') };
    },
    DebugEvaluate: (expressions: string[]) => {
      if (!debug.active) throw new Error('no debug session is active');
      return expressions.map(expression => {
        const upper = expression.trim().toUpperCase();
        if (upper === 'SCORE') return { expression, address: 0x2400, type: 'word', value: `${debug.line * 10} ($${(debug.line * 10).toString(16).toUpperCase().padStart(4, '0')})` };
        if (/^\$[0-9A-F]+$/.test(upper)) return { expression, address: parseInt(upper.slice(1), 16), type: 'byte', value: '0 ($00)' };
        return { expression, address: 0, type: '', value: '', error: `unknown symbol "${expression}"` };
      });
    },
    DebugSymbols: () => [{ name: 'SCORE', type: 'word', address: 0x2400, size: 2 }],
    DebugState: () => ({ active: debug.active, stopped: debug.stopped, stop: debug.stop }),
    DebugStop: () => studio.EmulatorStop(),

    ATROpen: (path: string) => { const image = atrImages.get(path); if (!image) throw new Error(`open ${path}: no such file`); return clone(image); },
    ATRConfigured: () => {
      requireProject();
      const path = `${root}/${config().atr.output}`;
      if (!atrImages.has(path)) throw new Error(`${config().atr.output} does not exist yet; build the project with ATR packaging enabled`);
      return clone(atrImages.get(path)!);
    },
    ATRImportFiles: (image: string, destination: string, paths: string[]) => {
      const atr = atrImages.get(image)!;
      for (const path of paths) { const name = path.split(/[\\/]/).pop()!.toUpperCase(); atr.entries!.push({ path: destination ? `${destination}/${name}` : name, name, size: 10, directory: false, attributes: 0, boot: false, modified: new Date(0).toISOString() as any }); }
      return clone(atr);
    },
    ATRImportProject: (image: string, destination: string, paths: string[]) => studio.ATRImportFiles(image, destination, paths),
    ATRMkdir: (image: string, path: string) => { const atr = atrImages.get(image)!; atr.entries!.push({ path, name: path.split('/').pop()!, size: 0, directory: true, attributes: 0, boot: false, modified: new Date(0).toISOString() as any }); return clone(atr); },
    ATRRemove: (image: string, path: string) => { const atr = atrImages.get(image)!; atr.entries = atr.entries!.filter(entry => entry.path !== path && !entry.path.startsWith(path + '/')); return clone(atr); },
    ATRRename: (image: string, oldPath: string, newPath: string) => {
      const atr = atrImages.get(image)!;
      atr.entries = atr.entries!.map(entry => (entry.path === oldPath || entry.path.startsWith(oldPath + '/') ? { ...entry, path: newPath + entry.path.slice(oldPath.length), name: (newPath + entry.path.slice(oldPath.length)).split('/').pop()! } : entry));
      return clone(atr);
    },
    ATRExtractToDirectory: () => {},
    ATRExtractToProject: (image: string, path: string, destination: string, overwrite: boolean) => {
      requireProject();
      const name = path.split('/').pop()!;
      const target = destination && destination !== '.' ? `${destination}/${name}` : name;
      if (files().has(target) && !overwrite) throw new Error(`exists: ${name} already exists`);
      files().set(target, `extracted from ${image}\n`);
      return view();
    },
  };

  // Wrap every method: record the call, apply scripted failures, and return
  // a promise like the real bindings do.
  const api = new Proxy({}, {
    get(_target, method: string) {
      return (...args: unknown[]) => {
        calls.push(`${method}(${args.map(arg => (typeof arg === 'string' ? (arg.length > 40 ? arg.slice(0, 40) + '…' : arg) : JSON.stringify(arg) ?? '')).join(', ')})`);
        const failure = failures.get(method);
        if (failure !== undefined) { failures.delete(method); return Promise.reject(new Error(failure)); }
        const implementation = studio[method];
        if (!implementation) return Promise.reject(new Error(`mock backend: ${method} is not implemented`));
        try { return Promise.resolve(implementation(...args)); } catch (error) { return Promise.reject(error); }
      };
    },
  }) as StudioAPI;

  const mock: MockNative = {
    studio: api,
    disk, calls, trusted, claude,
    dialogQueue: [],
    recoveries: new Map(),
    git,
    window: { minimise() {}, toggleMaximise() {}, close() {} },
    dialogs: {
      async open(_options: OpenDialogOptions) { return mock.dialogQueue.shift() ?? []; },
      async save() { return (mock.dialogQueue.shift() ?? [])[0] ?? ''; },
    },
    clipboard: { text: '', async setText(text: string) { mock.clipboard.text = text; } },
    on(name, handler) {
      if (!handlers.has(name)) handlers.set(name, new Set());
      handlers.get(name)!.add(handler as Handler);
      return () => { handlers.get(name)?.delete(handler as Handler); };
    },
    emit,
    externalWrite(path: string, content: string | null) {
      const existed = files().has(path);
      if (content === null) files().delete(path); else files().set(path, content);
      emit('workspace-changed', { root, overflow: false, changes: [{ path, kind: content === null ? 'remove' : existed ? 'write' : 'create', directory: false, size: content?.length ?? 0 }] });
    },
    failNext(method: string, message: string) { failures.set(method, message); },
  };
  Object.defineProperty(mock, 'recoveries', { get: () => recoveryByRoot.get(root) ?? new Map<string, string>() });

  // A ready-made workspace for end-to-end tests: an untrusted folder with
  // an include file and an ATR image next to it.
  disk.set('/work/sample', new Map([
    ['src/main.bas', "' sample\nINCLUDE \"util.bas\"\nSCORE = 0\nFOR I = 1 TO 3\n  SCORE = SCORE + Twice(I)\nNEXT I\nPRINT SCORE\n"],
    ['src/util.bas', 'FUNC Twice(N)\n  RETURN N * 2\nENDFUNC\n'],
    ['README.md', '# Sample\n'],
  ]));
  atrImages.set('/work/disk.atr', {
    path: '/work/disk.atr', filesystem: 'sparta', sectorSize: 128, sectors: 720, bytes: 92160, freeBytes: 80000,
    entries: [{ path: 'GAME.XEX', name: 'GAME.XEX', size: 2048, directory: false, attributes: 0, boot: true, modified: new Date(0).toISOString() as any }, { path: 'DATA', name: 'DATA', size: 0, directory: true, attributes: 0, boot: false, modified: new Date(0).toISOString() as any }],
  });
  return mock;
}
