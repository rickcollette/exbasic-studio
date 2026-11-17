// Application preferences: parsing, persistence, startup and auto-save.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setNative } from './native';
import { createMockNative, type MockNative } from './mockNative';
import { createWorkbench, type Workbench } from './workbench.svelte';
import { parsePreferences, defaults } from './stores/preferences.svelte';

let mock: MockNative;
let wb: Workbench;

beforeEach(async () => {
  vi.useFakeTimers();
  mock = createMockNative();
  setNative(mock);
  wb = createWorkbench();
  await wb.start();
});

afterEach(() => { wb.dispose(); vi.useRealTimers(); });

function type(text: string) {
  const path = wb.editors.active;
  const state = wb.editors.stateFor(path)!;
  const next = state.update({ changes: { from: 0, to: state.doc.length, insert: text } }).state;
  wb.editors.update(path, next, true);
}

describe('preferences', () => {
  it('falls back to defaults for missing or invalid stored values', () => {
    expect(parsePreferences(undefined)).toEqual(defaults);
    expect(parsePreferences({ theme: 'neon', fontSize: '999', autoSave: 'always', syntax: 'retro' })).toEqual({ ...defaults, syntax: 'retro' });
  });

  it('applies and stores a change, and survives a restart', async () => {
    wb.prefs.set('theme', 'light');
    wb.prefs.set('syntax', 'classic');
    wb.prefs.set('fontSize', 16);
    expect(wb.prefs.theme).toBe('light');
    await vi.advanceTimersByTimeAsync(5);
    wb.dispose();
    wb = createWorkbench();
    await wb.start();
    expect(wb.prefs.values).toMatchObject({ theme: 'light', syntax: 'classic', fontSize: 16 });
  });

  it('follows the system when asked', () => {
    wb.prefs.set('theme', 'system');
    wb.prefs.systemDark = false;
    expect(wb.prefs.theme).toBe('light');
    wb.prefs.systemDark = true;
    expect(wb.prefs.theme).toBe('dark');
  });
});

describe('startup', () => {
  it('reopens the last folder by default', async () => {
    await wb.openFolder('/work/sample');
    wb.dispose();
    wb = createWorkbench();
    await wb.start();
    expect(wb.workspace.project?.root).toBe('/work/sample');
  });

  it('shows the welcome page when that is preferred', async () => {
    await wb.openFolder('/work/sample');
    wb.prefs.set('startup', 'welcome');
    await vi.advanceTimersByTimeAsync(5);
    wb.dispose();
    wb = createWorkbench();
    await wb.start();
    expect(wb.workspace.project).toBeNull();
  });
});

describe('auto-save', () => {
  beforeEach(async () => { await wb.openFolder('/work/sample'); });
  const onDisk = () => mock.disk.get('/work/sample')!.get('src/main.bas');

  it('is off by default', async () => {
    type('EDITED\n');
    await vi.advanceTimersByTimeAsync(6000);
    expect(onDisk()).not.toBe('EDITED\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);
  });

  it('saves after a pause in typing', async () => {
    wb.prefs.set('autoSave', 'delay');
    type('ONE\n');
    await vi.advanceTimersByTimeAsync(600);
    type('TWO\n'); // typing again restarts the wait
    await vi.advanceTimersByTimeAsync(600);
    expect(onDisk()).not.toBe('TWO\n');
    await vi.advanceTimersByTimeAsync(600);
    expect(onDisk()).toBe('TWO\n');
    expect(wb.editors.activeTab?.dirty).toBe(false);
  });

  it('saves when focus is lost', async () => {
    wb.prefs.set('autoSave', 'focus');
    type('FOCUS\n');
    await vi.advanceTimersByTimeAsync(3000);
    expect(onDisk()).not.toBe('FOCUS\n');
    wb.editors.focusLost();
    await vi.advanceTimersByTimeAsync(5);
    expect(onDisk()).toBe('FOCUS\n');
  });

  it('never overwrites a file that changed on disk', async () => {
    wb.prefs.set('autoSave', 'delay');
    type('MINE\n');
    mock.externalWrite('src/main.bas', 'THEIRS\n');
    await vi.advanceTimersByTimeAsync(3000);
    expect(onDisk()).toBe('THEIRS\n');
    expect(wb.editors.activeTab?.dirty).toBe(true);
  });
});
