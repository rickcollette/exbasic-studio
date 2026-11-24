// Workflow tests: the real stores and commands running against the
// deterministic mock backend.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync } from 'svelte';
import { setNative } from './native';
import { createMockNative, type MockNative } from './mockNative';
import { createWorkbench, type Workbench } from './workbench.svelte';
import { EMULATOR_TAB } from './stores/editors.svelte';

let mock: MockNative;
let wb: Workbench;

/**
 * Lets pending promises and short timers run. Fake timers schedule a
 * zero-delay timer created inside another timer one millisecond later, so
 * each round advances the clock by 1ms.
 */
async function settle(rounds = 8) {
  for (let i = 0; i < rounds; i++) { await vi.advanceTimersByTimeAsync(1); await Promise.resolve(); }
  flushSync();
}

/** Answers the dialog that is (or is about to be) open. */
async function answer(value: string, text = '') {
  await settle();
  expect(wb.ui.dialog, `expected a dialog to answer with "${value}"`).not.toBeNull();
  wb.ui.closeDialog(value, text);
  await settle();
}

/** Replaces the text of the active editor like typing would. */
function type(text: string) {
  const path = wb.editors.active;
  const state = wb.editors.stateFor(path)!;
  const transaction = state.update({ changes: { from: 0, to: state.doc.length, insert: text } });
  wb.editors.update(path, transaction.state, true);
}

beforeEach(async () => {
  vi.useFakeTimers();
  mock = createMockNative();
  setNative(mock);
  wb = createWorkbench();
  await wb.start();
  await settle();
});

async function createProject(name = 'demo') {
  const pending = wb.createProject('/home/tester', name);
  await settle();
  await pending;
  await settle();
}

describe('project lifecycle', () => {
  it('creates a trusted project and opens its entry file', async () => {
    await createProject();
    expect(wb.workspace.project?.config.name).toBe('demo');
    expect(wb.workspace.trust.trusted).toBe(true);
    expect(wb.editors.active).toBe('src/main.bas');
    expect(wb.editors.activeTab?.dirty).toBe(false);
    expect(wb.workspace.recents[0]).toMatchObject({ path: '/home/tester/demo', name: 'demo' });
    expect(wb.workspace.tree.map(entry => entry.path)).toEqual(['lib', 'src', 'src/main.bas']);
  });

  it('opens an existing folder in restricted mode and writes nothing', async () => {
    const before = JSON.stringify([...mock.disk.get('/work/sample')!]);
    await wb.openFolder('/work/sample');
    await settle();
    expect(wb.workspace.restricted).toBe(true);
    expect(wb.editors.active).toBe('src/main.bas');
    expect(JSON.stringify([...mock.disk.get('/work/sample')!])).toBe(before);
    // Lint does not run the compiler in restricted mode.
    await vi.advanceTimersByTimeAsync(1000);
    expect(wb.problems.all).toEqual([]);
  });

  it('reports a missing folder without changing the workbench', async () => {
    await wb.openFolder('/does/not/exist');
    await settle();
    expect(wb.workspace.project).toBeNull();
    expect(wb.ui.status).toBe('Opening folder failed');
    expect(wb.ui.output).toContain('cannot find the path');
  });

  it('opens images in the normal image viewer and formats JSON text', async () => {
    await createProject();
    await mock.studio.SaveBinaryFile('art/hero.png', 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');
    await wb.workspace.refresh();
    expect(await wb.openFile('art/hero.png')).toBe(true);
    expect(wb.editors.activeTab).toMatchObject({ path: 'art/hero.png', kind: 'image', dirty: false });
    expect(wb.editors.activeTab?.image).toMatch(/^data:image\/png;base64,/);
    expect(wb.sprites.active).toBeUndefined();

    await wb.workspace.createFile('data.json', '{"answer":42,"items":[1,2]}');
    await wb.openFile('data.json');
    wb.editors.formatJson('data.json');
    expect(wb.editors.text('data.json')).toBe('{\n  "answer": 42,\n  "items": [\n    1,\n    2\n  ]\n}\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);
    vi.clearAllTimers();
  });

  it('restores tabs, breakpoints and watches when a folder is reopened', async () => {
    await createProject();
    await wb.workspace.createFile('src/extra.bas', 'PRINT 2\n');
    await wb.editors.open('src/extra.bas');
    wb.debug.toggleBreakpoint('src/main.bas', 2);
    wb.debug.addWatch('SCORE');
    await vi.advanceTimersByTimeAsync(1000);
    await wb.closeFolder();
    await settle();
    expect(wb.editors.tabs).toEqual([]);
    await wb.openFolder('/home/tester/demo');
    await settle();
    expect(wb.editors.tabs.map(tab => tab.path)).toEqual(['src/main.bas', 'src/extra.bas']);
    expect(wb.editors.active).toBe('src/extra.bas');
    expect(wb.debug.persistedBreakpoints).toEqual([{ path: 'src/main.bas', line: 2, enabled: true }]);
    expect(wb.debug.watches.map(watch => watch.expression)).toEqual(['SCORE']);
  });

  it('remembers the keyboard joystick for each project', async () => {
    await createProject();
    expect(wb.emulator.joystickMode).toBe(false);
    wb.emulator.setJoystickMode(true);
    await vi.advanceTimersByTimeAsync(1000);
    await wb.closeFolder();
    await settle();
    await createProject('other');
    expect(wb.emulator.joystickMode).toBe(false);
    await wb.openFolder('/home/tester/demo');
    await settle();
    expect(wb.emulator.joystickMode).toBe(true);
  });
});

describe('editing', () => {
  beforeEach(() => createProject());

  it('tracks dirty state per tab and clears it on undo to the saved text', async () => {
    const saved = wb.editors.text('src/main.bas');
    type('PRINT 1\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);
    type(saved);
    expect(wb.editors.activeTab?.dirty).toBe(false);
    type('PRINT 2\n');
    await wb.execute('file.save');
    await settle();
    expect(wb.editors.activeTab?.dirty).toBe(false);
    expect(mock.disk.get('/home/tester/demo')!.get('src/main.bas')).toBe('PRINT 2\n');
  });

  it('saves all dirty tabs', async () => {
    await wb.workspace.createFile('src/b.bas', 'REM\n');
    await wb.editors.open('src/b.bas');
    type('PRINT "B"\n');
    wb.editors.activate('src/main.bas');
    type('PRINT "A"\n');
    expect(wb.editors.dirtyTabs).toHaveLength(2);
    expect(wb.commandEnabled('file.saveAll')).toBe(true);
    await wb.execute('file.saveAll');
    await settle();
    expect(wb.editors.dirtyTabs).toHaveLength(0);
    expect(mock.disk.get('/home/tester/demo')!.get('src/b.bas')).toBe('PRINT "B"\n');
  });

  it('asks before closing a dirty tab and honours each answer', async () => {
    type('PRINT 9\n');
    let closing = wb.editors.close('src/main.bas');
    await answer('cancel');
    expect(await closing).toBe(false);
    expect(wb.editors.tab('src/main.bas')).toBeDefined();

    closing = wb.editors.close('src/main.bas');
    await answer('save');
    expect(await closing).toBe(true);
    expect(mock.disk.get('/home/tester/demo')!.get('src/main.bas')).toBe('PRINT 9\n');
    expect(wb.editors.tabs).toEqual([]);

    await wb.editors.open('src/main.bas');
    type('DISCARDED\n');
    closing = wb.editors.close('src/main.bas');
    await answer('discard');
    expect(await closing).toBe(true);
    expect(mock.disk.get('/home/tester/demo')!.get('src/main.bas')).toBe('PRINT 9\n');
  });

  it('keeps the tab open and dirty when saving fails', async () => {
    type('PRINT 1\n');
    mock.failNext('SaveFile', 'disk full');
    expect(await wb.editors.save('src/main.bas')).toBe(false);
    expect(wb.editors.activeTab?.dirty).toBe(true);
    expect(wb.ui.output).toContain('disk full');
  });

  it('discards a lint result that arrives for outdated text', async () => {
    type('SYNTAXERROR\n');
    await vi.advanceTimersByTimeAsync(460); // the lint request is now in flight
    type('PRINT 1\n'); // newer text before the answer is applied
    await settle();
    await vi.advanceTimersByTimeAsync(1000);
    expect(wb.problems.forPath('src/main.bas')).toEqual([]);
    type('PRINT 1\nSYNTAXERROR\n');
    await vi.advanceTimersByTimeAsync(1000);
    expect(wb.problems.forPath('src/main.bas')).toMatchObject([{ line: 2, column: 1, code: 'syntax' }]);
    expect(wb.problems.errorCount).toBe(1);
  });

  it('renames an open file and keeps its tab, text and navigation', async () => {
    type('PRINT "X"\n');
    wb.workspace.selected = 'src/main.bas';
    const renaming = wb.renameSelected();
    await answer('confirm', 'src/game.bas');
    await renaming;
    expect(wb.editors.active).toBe('src/game.bas');
    expect(wb.editors.text('src/game.bas')).toBe('PRINT "X"\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);
    expect(wb.workspace.project?.config.entry).toBe('src/game.bas');
  });

  it('confirms before deleting and closes the affected tabs', async () => {
    wb.workspace.selected = 'src';
    let deleting = wb.deleteSelected();
    await answer('cancel');
    await deleting;
    expect(wb.workspace.hasFile('src/main.bas')).toBe(true);
    deleting = wb.deleteSelected();
    await answer('confirm');
    await deleting;
    expect(wb.workspace.hasFile('src/main.bas')).toBe(false);
    expect(wb.editors.tabs).toEqual([]);
  });

  it('creates files and folders from the inline input and refuses duplicates', async () => {
    wb.beginCreate('file');
    wb.creating!.value = 'src/level.bas';
    await wb.commitCreate();
    await settle();
    expect(wb.editors.active).toBe('src/level.bas');
    wb.beginCreate('file');
    wb.creating!.value = 'src/level.bas';
    await wb.commitCreate();
    expect(wb.ui.output).toContain('already exists');
    expect(wb.creating).not.toBeNull();
    wb.creating = null;
    wb.beginCreate('folder');
    wb.creating!.value = 'assets';
    await wb.commitCreate();
    expect(wb.workspace.hasDirectory('assets')).toBe(true);
  });

  it('navigates back and forward through visited files', async () => {
    await wb.workspace.createFile('src/b.bas', 'REM\n');
    await wb.editors.open('src/b.bas');
    expect(wb.commandEnabled('nav.back')).toBe(true);
    await wb.execute('nav.back');
    expect(wb.editors.active).toBe('src/main.bas');
    expect(wb.commandEnabled('nav.forward')).toBe(true);
    await wb.execute('nav.forward');
    expect(wb.editors.active).toBe('src/b.bas');
  });
});

describe('external changes and recovery', () => {
  beforeEach(() => createProject());

  it('reloads a clean tab changed on disk', async () => {
    mock.externalWrite('src/main.bas', 'PRINT "EXTERNAL"\n');
    await settle();
    expect(wb.editors.text('src/main.bas')).toBe('PRINT "EXTERNAL"\n');
    expect(wb.editors.activeTab?.dirty).toBe(false);
    expect(wb.editors.activeTab?.external).toBeNull();
  });

  it('prompts instead of overwriting unsaved edits', async () => {
    type('MINE\n');
    mock.externalWrite('src/main.bas', 'THEIRS\n');
    await settle();
    expect(wb.editors.text('src/main.bas')).toBe('MINE\n');
    expect(wb.editors.activeTab?.external).toBe('modified');
    await wb.editors.resolveExternal('src/main.bas', 'keep');
    expect(wb.editors.text('src/main.bas')).toBe('MINE\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);

    mock.externalWrite('src/main.bas', 'THEIRS 2\n');
    await settle();
    await wb.editors.resolveExternal('src/main.bas', 'reload');
    expect(wb.editors.text('src/main.bas')).toBe('THEIRS 2\n');
    expect(wb.editors.activeTab?.dirty).toBe(false);
  });

  it('keeps the buffer of a file deleted outside the editor', async () => {
    mock.externalWrite('src/main.bas', null);
    await settle();
    expect(wb.editors.activeTab?.external).toBe('deleted');
    expect(wb.editors.text('src/main.bas')).toContain('HELLO');
    expect(wb.workspace.hasFile('src/main.bas')).toBe(false);
    await wb.editors.save('src/main.bas');
    expect(mock.disk.get('/home/tester/demo')!.has('src/main.bas')).toBe(true);
  });

  it('updates the explorer incrementally from watcher events', async () => {
    const calls = mock.calls.length;
    mock.externalWrite('src/new.bas', 'REM\n');
    await settle();
    expect(wb.workspace.hasFile('src/new.bas')).toBe(true);
    expect(mock.calls.slice(calls).some(call => call.startsWith('GetProject'))).toBe(false);
    mock.emit('workspace-changed', { root: '/home/tester/demo', overflow: false, changes: [{ path: 'build/out.xex', kind: 'create', directory: false, size: 3 }, { path: 'assets', kind: 'create', directory: true, size: 0 }] });
    await settle();
    expect(wb.workspace.hasFile('build/out.xex')).toBe(true);
    expect(wb.workspace.hasDirectory('assets')).toBe(true);
    mock.emit('workspace-changed', { root: '/other/root', overflow: false, changes: [{ path: 'x.bas', kind: 'create', directory: false, size: 1 }] });
    await settle();
    expect(wb.workspace.hasFile('x.bas')).toBe(false);
  });

  it('stores recovery copies while dirty and removes them once saved', async () => {
    type('UNSAVED\n');
    await vi.advanceTimersByTimeAsync(1300);
    expect(mock.recoveries.get('src/main.bas')).toBe('UNSAVED\n');
    await wb.editors.save('src/main.bas');
    expect(mock.recoveries.has('src/main.bas')).toBe(false);
  });

  it('offers to restore unsaved work after a crash', async () => {
    type('LOST WORK\n');
    await vi.advanceTimersByTimeAsync(1300);
    // Simulate a crash: a fresh workbench on the same backend state.
    wb.dispose();
    wb = createWorkbench();
    // Starting reopens the last folder, which offers the recovery copy.
    const starting = wb.start();
    await answer('restore');
    await starting;
    expect(wb.editors.text('src/main.bas')).toBe('LOST WORK\n');
    expect(wb.editors.tab('src/main.bas')?.dirty).toBe(true);
    expect(mock.disk.get('/home/tester/demo')!.get('src/main.bas')).not.toBe('LOST WORK\n');
  });

  it('discards recovery copies when asked', async () => {
    type('LOST WORK\n');
    await vi.advanceTimersByTimeAsync(1300);
    wb.dispose();
    wb = createWorkbench();
    // Starting reopens the last folder, which offers the recovery copy.
    const starting = wb.start();
    await answer('discard');
    await starting;
    expect(wb.editors.tab('src/main.bas')?.dirty).toBe(false);
    expect(mock.recoveries.size).toBe(0);
  });
});

describe('trust', () => {
  it('asks before building an untrusted folder and stays restricted on refusal', async () => {
    await wb.openFolder('/work/sample');
    await settle();
    let building = wb.build(false);
    await answer('cancel');
    expect(await building).toBeNull();
    expect(mock.calls.some(call => call.startsWith('StartBuild'))).toBe(false);

    building = wb.build(false);
    await answer('trust');
    const result = await building;
    expect(result?.success).toBe(true);
    expect(wb.workspace.trust.trusted).toBe(true);
  });

  it('does not run Git in a restricted folder until asked', async () => {
    await wb.openFolder('/work/sample');
    await settle();
    wb.ui.showActivity('source');
    await wb.git.autoRefresh();
    expect(wb.git.blocked).toBe(true);
    expect(mock.calls.some(call => call.startsWith('Git'))).toBe(false);
    await wb.git.allow();
    expect(mock.calls.some(call => call.startsWith('GitStatus'))).toBe(true);
    expect(wb.git.status?.available).toBe(false);
  });

  it('returns to restricted mode when a tool path changes', async () => {
    await createProject();
    const config = JSON.parse(JSON.stringify(wb.workspace.project!.config));
    config.compiler = '/tmp/other-compiler';
    await wb.workspace.saveConfig(config);
    expect(wb.workspace.restricted).toBe(true);
    expect(wb.ui.output).toContain('restricted mode');
  });
});

describe('build, run and debug', () => {
  beforeEach(() => createProject());

  it('builds, streams output and reports the artifact', async () => {
    const building = wb.build(false);
    await settle();
    const result = await building;
    expect(result?.success).toBe(true);
    expect(wb.ui.output).toContain('BAS compile');
    expect(wb.ui.output).toContain('Build succeeded');
    expect(wb.ui.output).toContain('build/demo.xex');
    expect(wb.ui.status).toBe('Build succeeded');
    expect(wb.operations.running).toEqual([]);
  });

  it('shows structured diagnostics for a failed build', async () => {
    type('PRINT 1\nSYNTAXERROR here\n');
    const building = wb.build(false);
    await answer('save');
    const result = await building;
    expect(result?.success).toBe(false);
    expect(wb.problems.all).toMatchObject([{ path: 'src/main.bas', line: 2, column: 1, severity: 'error' }]);
    expect(wb.ui.panelTab).toBe('problems');
  });

  it('can build what is on disk without saving, or cancel', async () => {
    type('SYNTAXERROR\n');
    let building = wb.build(false);
    await answer('cancel');
    expect(await building).toBeNull();
    building = wb.build(false);
    await answer('skip');
    expect((await building)?.success).toBe(true); // the saved file is still valid
    expect(wb.editors.activeTab?.dirty).toBe(true);
  });

  it('cancels a running build and receives exactly one completion', async () => {
    const building = wb.build(false);
    // Two zero-length ticks: the build starts and prints, but its nested
    // completion timer has not fired yet.
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(0);
    expect(wb.operations.isRunning('build')).toBe(true);
    await wb.execute('build.cancel');
    await settle();
    const result = await building;
    expect(result?.cancelled).toBe(true);
    expect(wb.ui.status).toBe('Build cancelled');
    expect(wb.operations.running).toEqual([]);
  });

  it('runs the program and opens the emulator tab', async () => {
    const running = wb.build(true);
    await settle();
    await running;
    expect(wb.editors.active).toBe(EMULATOR_TAB);
    expect(wb.emulator.running).toBe(true);
    wb.emulator.setVisible(true);
    await vi.advanceTimersByTimeAsync(200);
    expect(wb.emulator.frame).toBeGreaterThan(0);
    expect(wb.emulator.image).toContain('data:image/png');
    await wb.execute('emulator.stop');
    expect(wb.emulator.running).toBe(false);
  });

  it('stops on a breakpoint, steps, evaluates watches and ends cleanly', async () => {
    wb.debug.toggleBreakpoint('src/main.bas', 2);
    wb.debug.addWatch('SCORE');
    wb.debug.addWatch('NOPE');
    const starting = wb.execute('debug.start');
    await settle();
    await starting;
    expect(wb.debug.active).toBe(true);
    await vi.advanceTimersByTimeAsync(300); // frames are polled while debugging
    await settle();
    expect(wb.debug.stopped).toBe(true);
    expect(wb.debug.currentLine).toEqual({ path: 'src/main.bas', line: 2 });
    expect(wb.debug.breakpoints[0]).toMatchObject({ verified: true, resolvedLine: 2 });
    expect(wb.debug.watches[0]).toMatchObject({ expression: 'SCORE', value: '20 ($0014)', type: 'word' });
    expect(wb.debug.watches[1].error).toContain('unknown symbol');
    expect(wb.ui.status).toBe('Breakpoint at src/main.bas:2');

    await wb.execute('debug.stepLine');
    await vi.advanceTimersByTimeAsync(300);
    await settle();
    expect(wb.debug.currentLine).toEqual({ path: 'src/main.bas', line: 3 });

    await wb.debug.readMemory(0x0600);
    expect(wb.debug.memory?.address).toBe(0x0600);
    expect(wb.debug.memory?.bytes).toHaveLength(256);

    await wb.execute('debug.stop');
    await settle();
    expect(wb.debug.active).toBe(false);
    expect(wb.debug.currentLine).toBeNull();
    expect(wb.emulator.running).toBe(false);
  });

  it('explains when a build has no debug information', async () => {
    const config = JSON.parse(JSON.stringify(wb.workspace.project!.config));
    config.debug = false;
    await wb.workspace.saveConfig(config);
    const starting = wb.execute('debug.start');
    await settle();
    await starting;
    expect(wb.debug.active).toBe(false);
    expect(wb.ui.output).toContain('Source-level debugging is unavailable');
  });

  it('treats a helper failure as a debugger stop and allows a restart', async () => {
    wb.debug.toggleBreakpoint('src/main.bas', 2);
    let starting = wb.execute('debug.start');
    await settle();
    await starting;
    mock.emit('debugger', { seq: 999, type: 'exited', message: 'The emulator stopped unexpectedly: helper crashed' });
    await settle();
    expect(wb.debug.active).toBe(false);
    expect(wb.ui.output).toContain('helper crashed');
    expect(wb.commandEnabled('debug.start')).toBe(true);
    starting = wb.execute('debug.start');
    await settle();
    await starting;
    expect(wb.debug.active).toBe(true);
  });
});

describe('terminal', () => {
  beforeEach(() => createProject());

  it('opens a session, streams output in order and reports exit', async () => {
    const id = (await wb.terminal.open(80, 24))!;
    let screen = '';
    wb.terminal.attach(id, data => { screen += data; });
    await settle();
    expect(screen).toContain('mock-shell');
    wb.terminal.input(id, 'echo hi\r');
    await settle();
    expect(screen).toContain('ran: echo hi');
    wb.terminal.input(id, 'exit\r');
    await settle();
    expect(wb.terminal.sessions[0]).toMatchObject({ exited: true, exitCode: 0 });
    expect(screen).toContain('Process exited with code 0');
  });

  it('replays output received before the view attached and closes explicitly', async () => {
    const id = (await wb.terminal.open())!;
    await settle();
    let screen = '';
    wb.terminal.attach(id, data => { screen += data; });
    expect(screen).toContain('mock-shell');
    await wb.terminal.close(id);
    expect(wb.terminal.sessions).toEqual([]);
    expect(mock.calls.some(call => call.startsWith('TerminalClose'))).toBe(true);
  });

  it('opens Claude Code as a named integrated terminal session', async () => {
    await wb.execute('claude.open');
    await settle();
    expect(wb.ui.activity).toBe('claude');
    expect(wb.ui.panelTab).toBe('terminal');
    expect(wb.terminal.sessions).toHaveLength(1);
    expect(wb.terminal.sessions[0]).toMatchObject({ kind: 'claude', title: 'Claude Code 1', exited: false });
    expect(wb.terminal.activeClaudeId).toBe(wb.terminal.sessions[0].id);
    expect(mock.calls.some(call => call.startsWith('TerminalOpenClaude'))).toBe(true);
  });

  it('is unavailable in restricted mode until the folder is trusted', async () => {
    await wb.openFolder('/work/sample');
    await settle();
    const opening = wb.terminal.open();
    await answer('cancel');
    expect(await opening).toBeNull();
    expect(mock.calls.some(call => call.startsWith('TerminalOpen'))).toBe(false);
  });
});

describe('git', () => {
  beforeEach(() => createProject());

  it('initializes, stages, commits and manages branches and remotes', async () => {
    await wb.git.refresh();
    expect(wb.git.status?.available).toBe(false);
    await wb.git.init();
    expect(wb.git.status?.unborn).toBe(true);
    expect(wb.git.unstaged.map(change => change.path)).toContain('src/main.bas');
    await wb.git.stage('src/main.bas');
    expect(wb.git.staged.map(change => change.path)).toEqual(['src/main.bas']);
    wb.git.message = 'first';
    expect(await wb.git.commit()).toBe(true);
    expect(wb.git.message).toBe('');
    expect(wb.git.status?.changes).toEqual([]);
    expect(await wb.git.createBranch('feature/x', true)).toBe(true);
    expect(wb.git.branches.find(branch => branch.current)?.name).toBe('feature/x');
    expect(await wb.git.createBranch('bad name', true)).toBe(false);
    await wb.git.addRemote('origin', 'https://example.invalid/repo.git');
    expect(wb.git.remotes).toMatchObject([{ name: 'origin' }]);
    const pushing = wb.git.network('push');
    await settle();
    await pushing;
    expect(wb.ui.output).toContain('git push finished');
  });

  it('surfaces a rejected network operation without prompting for credentials', async () => {
    await wb.git.init();
    await wb.git.addRemote('origin', 'https://rejected.invalid/repo.git');
    const fetching = wb.git.network('fetch');
    await settle();
    await fetching;
    expect(wb.ui.status).toBe('git fetch failed');
    expect(wb.ui.output).toContain('Authentication failed');
    expect(wb.ui.dialog).toBeNull();
  });

  it('resolves a merge conflict and completes the merge', async () => {
    await wb.git.init();
    await wb.git.stageAll();
    wb.git.message = 'base';
    await wb.git.commit();
    mock.git.conflict = 'src/main.bas';
    mock.disk.get('/home/tester/demo')!.set('src/main.bas', '<<<<<<< HEAD\nA\n=======\nB\n>>>>>>> feature\n');
    await wb.git.refresh();
    expect(wb.git.status?.merging).toBe(true);
    expect(wb.git.conflicts.map(change => change.path)).toEqual(['src/main.bas']);
    await wb.git.openConflict('src/main.bas');
    expect(wb.git.conflict).toMatchObject({ current: 'PRINT "CURRENT"\n', incoming: 'PRINT "INCOMING"\n' });
    expect(await wb.git.stageResolved('src/main.bas')).toBe(false); // markers remain
    expect(await wb.git.resolve('src/main.bas', 'both')).toBe(true);
    expect(wb.git.conflict).toBeNull();
    expect(wb.git.conflicts).toEqual([]);
    expect(await wb.git.mergeContinue()).toBe(true);
  });
});

describe('ATR explorer', () => {
  beforeEach(() => createProject());

  it('browses, imports, renames and deletes with confirmation', async () => {
    await wb.atr.open('/work/disk.atr');
    expect(wb.atr.view?.entries?.map(entry => entry.path)).toEqual(['GAME.XEX', 'DATA']);
    await wb.atr.importHost(['/tmp/level.dat'], 'DATA');
    expect(wb.atr.view?.entries?.some(entry => entry.path === 'DATA/LEVEL.DAT')).toBe(true);
    await wb.atr.mkdir('SAVES');
    expect(wb.atr.selected).toBe('SAVES');
    await wb.atr.rename('GAME.XEX', 'DATA/GAME.XEX');
    expect(wb.atr.view?.entries?.some(entry => entry.path === 'DATA/GAME.XEX')).toBe(true);
    const removing = wb.atr.remove('DATA');
    await answer('confirm');
    await removing;
    expect(wb.atr.view?.entries?.map(entry => entry.path)).toEqual(['SAVES']);
  });

  it('asks before overwriting a project file when extracting', async () => {
    await wb.atr.open('/work/disk.atr');
    await wb.workspace.createFile('GAME.XEX', 'local');
    let extracting = wb.atr.extractToProject('GAME.XEX', '');
    await answer('cancel');
    await extracting;
    expect(mock.disk.get('/home/tester/demo')!.get('GAME.XEX')).toBe('local');
    extracting = wb.atr.extractToProject('GAME.XEX', '');
    await answer('confirm');
    await extracting;
    expect(mock.disk.get('/home/tester/demo')!.get('GAME.XEX')).toContain('extracted');
  });

  it('reports a missing project image clearly', async () => {
    await wb.atr.openConfigured();
    expect(wb.atr.view).toBeNull();
    expect(wb.ui.output).toContain('does not exist yet');
  });
});

describe('commands and palette', () => {
  beforeEach(() => createProject());

  it('filters commands and skips disabled entries with the keyboard', async () => {
    wb.showPalette('commands');
    wb.palette = { ...wb.palette, query: 'debug' };
    const labels = wb.paletteItems.map(item => item.label);
    expect(labels).toContain('Debug: Start Debugging');
    const firstEnabled = wb.paletteItems.findIndex(item => !item.disabled);
    wb.palette = { ...wb.palette, index: firstEnabled };
    wb.movePalette(1);
    expect(wb.paletteItems[wb.palette.index].disabled).toBeFalsy();
    wb.closePalette();
    expect(wb.palette.open).toBe(false);
  });

  it('finds files and workspace symbols', async () => {
    await wb.workspace.createFile('lib/util.bas', 'FUNC Clamp(v)\nENDFUNC\n');
    await settle();
    await vi.advanceTimersByTimeAsync(10);
    mock.externalWrite('lib/util.bas', 'FUNC Clamp(v)\n  RETURN v\nENDFUNC\n');
    await settle();
    wb.showPalette('files');
    wb.palette = { ...wb.palette, query: 'util' };
    expect(wb.paletteItems.map(item => item.label)).toEqual(['util.bas']);
    wb.showPalette('symbols');
    wb.palette = { ...wb.palette, query: 'clamp' };
    expect(wb.paletteItems[0]).toMatchObject({ label: 'Clamp' });
    wb.runPaletteSelection();
    await settle();
    expect(wb.editors.active).toBe('lib/util.bas');
    expect(wb.editors.reveal ?? { line: 1 }).toMatchObject({ line: 1 });
  });

  it('goes to definitions across files and into INCLUDE targets', async () => {
    await wb.workspace.createFile('src/util.bas', 'FUNC Twice(N)\n  RETURN N * 2\nENDFUNC\n');
    mock.externalWrite('src/util.bas', 'FUNC Twice(N)\n  RETURN N * 2\nENDFUNC\n');
    await settle();
    type('INCLUDE "util.bas"\nX = Twice(2)\n');
    const state = wb.editors.stateFor('src/main.bas')!;
    await wb.goToDefinition('src/main.bas', state, state.doc.line(2).from + 6);
    expect(wb.editors.active).toBe('src/util.bas');
    expect(wb.editors.reveal).toMatchObject({ path: 'src/util.bas', line: 1 });
    wb.editors.activate('src/main.bas');
    await wb.goToDefinition('src/main.bas', state, state.doc.line(1).from + 12);
    expect(wb.editors.active).toBe('src/util.bas');
    expect(wb.languageHost.includeCandidates('src/main.bas')).toContain('util.bas');
  });

  it('maps keyboard shortcuts to commands', async () => {
    const press = (init: KeyboardEventInit) => wb.handleKey(new KeyboardEvent('keydown', { cancelable: true, ...init }));
    expect(press({ key: 'p', ctrlKey: true, shiftKey: true })).toBe(true);
    expect(wb.palette).toMatchObject({ open: true, mode: 'commands' });
    wb.closePalette();
    expect(press({ key: 'p', ctrlKey: true })).toBe(true);
    expect(wb.palette.mode).toBe('files');
    wb.closePalette();
    expect(press({ key: 'j', ctrlKey: true })).toBe(true);
    expect(wb.ui.panelOpen).toBe(false);
    expect(press({ key: 'F9' })).toBe(true);
    expect(wb.debug.breakpoints).toHaveLength(1);
    expect(press({ key: 'x' })).toBe(false);
    // Ctrl+K then S saves all.
    type('CHORD\n');
    expect(press({ key: 'k', ctrlKey: true })).toBe(true);
    expect(press({ key: 's' })).toBe(true);
    await settle();
    expect(wb.editors.dirtyTabs).toHaveLength(0);
  });

  it('persists layout changes', async () => {
    wb.ui.setPrimaryWidth(5000);
    expect(wb.ui.primaryWidth).toBe(900);
    wb.ui.showActivity('search');
    await vi.advanceTimersByTimeAsync(500);
    const saved = mock.calls.filter(call => call.startsWith('SaveLayout')).at(-1)!;
    expect(saved).toContain('"activity":"search"');
    expect(saved).toContain('"primaryWidth":900');
  });
});
