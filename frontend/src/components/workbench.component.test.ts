// Component tests: the real workbench rendered into jsdom against the mock
// backend, driven through the DOM the way a user would.
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { render, screen, fireEvent, waitFor, cleanup, within } from '@testing-library/svelte';
import { setNative } from '../lib/native';
import { createMockNative, type MockNative } from '../lib/mockNative';
import { wb } from '../lib/workbench.svelte';
import App from '../App.svelte';

let mock: MockNative;

beforeEach(async () => {
  mock = createMockNative();
  setNative(mock);
  render(App);
  await waitFor(() => expect(wb.ready).toBe(true));
});

afterEach(() => cleanup());

async function openSample() {
  await fireEvent.input(screen.getByLabelText('Folder path'), { target: { value: '/work/sample' } });
  await fireEvent.keyDown(screen.getByLabelText('Folder path'), { key: 'Enter' });
  await waitFor(() => expect(wb.workspace.project?.root).toBe('/work/sample'));
  await waitFor(() => expect(wb.editors.active).toBe('src/main.bas'));
}

describe('welcome and explorer', () => {
  it('opens a folder, shows restricted mode and lists files as a tree', async () => {
    expect(screen.getByText('You have not opened a folder yet.')).toBeInTheDocument();
    await openSample();
    const tree = screen.getByRole('tree', { name: 'Project files' });
    const items = within(tree).getAllByRole('treeitem');
    expect(items.map(item => item.getAttribute('data-path'))).toEqual(['src', 'src/main.bas', 'src/util.bas', 'README.md']);
    expect(items[0]).toHaveAttribute('aria-expanded', 'true');
    expect(items[1]).toHaveAttribute('aria-level', '2');
    expect(screen.getAllByText(/Restricted mode/i).length).toBeGreaterThan(0);
    expect(screen.getByRole('tab', { name: /main\.bas/ })).toHaveAttribute('aria-selected', 'true');
  });

  it('opens the folder picked with the folder icon and has no separate open button', async () => {
    expect(screen.queryByRole('button', { name: 'Open Folder' })).toBeNull();
    mock.dialogQueue.push(['/work/sample']);
    await fireEvent.click(screen.getByRole('button', { name: 'Choose a folder to open' }));
    await waitFor(() => expect(wb.workspace.project?.root).toBe('/work/sample'));
  });

  it('supports keyboard navigation in the file tree', async () => {
    await openSample();
    const tree = screen.getByRole('tree', { name: 'Project files' });
    const rows = () => within(tree).getAllByRole('treeitem');
    rows()[0].focus();
    await fireEvent.keyDown(rows()[0], { key: 'ArrowDown' });
    expect(document.activeElement).toBe(rows()[1]);
    await fireEvent.keyDown(rows()[1], { key: 'End' });
    expect(document.activeElement).toBe(rows().at(-1));
    await fireEvent.keyDown(document.activeElement!, { key: 'Home' });
    // Collapse and expand the folder with the arrow keys.
    await fireEvent.keyDown(rows()[0], { key: 'ArrowLeft' });
    await waitFor(() => expect(rows()).toHaveLength(2));
    expect(rows()[0]).toHaveAttribute('aria-expanded', 'false');
    await fireEvent.keyDown(rows()[0], { key: 'ArrowRight' });
    await waitFor(() => expect(rows()).toHaveLength(4));
    // Enter opens a file.
    rows()[2].focus();
    await fireEvent.keyDown(rows()[2], { key: 'Enter' });
    await waitFor(() => expect(wb.editors.active).toBe('src/util.bas'));
    // Only one row is in the tab order (roving tabindex).
    expect(rows().filter(row => row.tabIndex === 0)).toHaveLength(1);
  });

  it('lists recent projects and removes one', async () => {
    await openSample();
    await wb.closeFolder();
    const list = await screen.findByRole('list', { name: 'Recent projects' });
    expect(within(list).getByText('/work/sample')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Remove sample from recent projects' }));
    await waitFor(() => expect(wb.workspace.recents).toHaveLength(0));
  });
});

describe('dialogs', () => {
  it('traps focus, answers with the keyboard and restores focus', async () => {
    await openSample();
    const opener = screen.getByRole('button', { name: 'New file' });
    opener.focus();
    const answer = wb.ui.confirm({ title: 'Delete file?', message: 'Delete src/util.bas?', confirmLabel: 'Delete', danger: true });
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveAttribute('aria-modal', 'true');
    expect(within(dialog).getByText('Delete src/util.bas?')).toBeInTheDocument();
    // A destructive button never receives initial focus.
    expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Cancel' }));
    await fireEvent.keyDown(dialog, { key: 'Escape' });
    expect(await answer).toBe(false);
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    await waitFor(() => expect(document.activeElement).toBe(opener));
  });

  it('returns typed text from a prompt on Enter', async () => {
    const answer = wb.ui.prompt({ title: 'Rename', message: 'Rename the file', label: 'New path', value: 'src/a.bas' });
    const input = (await screen.findByLabelText('New path')) as HTMLInputElement;
    expect(document.activeElement).toBe(input);
    await fireEvent.input(input, { target: { value: 'src/b.bas' } });
    await fireEvent.keyDown(input, { key: 'Enter' });
    expect(await answer).toBe('src/b.bas');
  });

  it('queues a second dialog behind the first', async () => {
    const first = wb.ui.confirm({ title: 'First', message: 'one' });
    const second = wb.ui.confirm({ title: 'Second', message: 'two' });
    expect(await screen.findByText('First')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(await first).toBe(true);
    expect(await screen.findByText('Second')).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(await second).toBe(false);
  });
});

describe('command palette', () => {
  it('filters, navigates with arrows, runs with Enter and closes with Escape', async () => {
    await openSample();
    await fireEvent.keyDown(window, { key: 'P', ctrlKey: true, shiftKey: true });
    const input = await screen.findByRole('combobox');
    expect(document.activeElement).toBe(input);
    await fireEvent.input(input, { target: { value: 'show search' } });
    const options = screen.getAllByRole('option');
    expect(options).toHaveLength(1);
    expect(options[0]).toHaveAttribute('aria-selected', 'true');
    expect(input).toHaveAttribute('aria-activedescendant', options[0].id);
    await fireEvent.keyDown(input, { key: 'Enter' });
    await waitFor(() => expect(wb.ui.activity).toBe('search'));
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();

    await fireEvent.keyDown(window, { key: 'p', ctrlKey: true });
    const files = await screen.findByRole('combobox');
    await fireEvent.input(files, { target: { value: 'util' } });
    await fireEvent.keyDown(files, { key: 'ArrowDown' });
    await fireEvent.keyDown(files, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('combobox')).not.toBeInTheDocument());
    expect(wb.editors.active).toBe('src/main.bas');
  });

  it('reports when nothing matches', async () => {
    wb.showPalette('commands');
    const input = await screen.findByRole('combobox');
    await fireEvent.input(input, { target: { value: 'zzzz-no-such-command' } });
    expect(within(screen.getByRole('listbox')).getByRole('status')).toHaveTextContent('No matching commands.');
  });
});

describe('application menu', () => {
  it('opens with the keyboard, moves between menus and runs an item', async () => {
    await openSample();
    const file = screen.getByRole('menuitem', { name: 'File' });
    file.focus();
    await fireEvent.keyDown(file, { key: 'ArrowDown' });
    const menu = await screen.findByRole('menu', { name: 'File' });
    const items = within(menu).getAllByRole('menuitem');
    await waitFor(() => expect(document.activeElement).toBe(items.find(item => !(item as HTMLButtonElement).disabled)));
    await fireEvent.keyDown(menu, { key: 'ArrowRight' });
    expect(await screen.findByRole('menu', { name: 'Edit' })).toBeInTheDocument();
    await fireEvent.keyDown(screen.getByRole('menu', { name: 'Edit' }), { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('menu')).not.toBeInTheDocument());
    expect(document.activeElement).toBe(screen.getByRole('menuitem', { name: 'Edit' }));

    await fireEvent.click(screen.getByRole('menuitem', { name: 'View' }));
    const view = await screen.findByRole('menu', { name: 'View' });
    await fireEvent.click(within(view).getByRole('menuitem', { name: /Show Search/ }));
    expect(wb.ui.activity).toBe('search');
  });

  it('disables items that do not apply', async () => {
    await fireEvent.click(screen.getByRole('menuitem', { name: 'File' }));
    const menu = await screen.findByRole('menu', { name: 'File' });
    expect(within(menu).getByRole('menuitem', { name: 'Save Ctrl+S' })).toBeDisabled();
    expect(within(menu).getByRole('menuitem', { name: /Open Folder/ })).toBeEnabled();
  });
});

describe('panels', () => {
  it('lists problems and jumps to them', async () => {
    await openSample();
    wb.problems.build = [{ path: 'src/util.bas', line: 2, column: 3, endLine: 2, endColumn: 9, severity: 'error', code: 'syntax', message: 'parse error' }];
    wb.ui.showPanel('problems');
    const problem = await screen.findByText('parse error');
    expect(screen.getByText('src/util.bas:2:3')).toBeInTheDocument();
    await fireEvent.click(problem);
    await waitFor(() => expect(wb.editors.active).toBe('src/util.bas'));
    expect(screen.getByRole('tab', { name: /Problems/ })).toHaveTextContent('1');
  });

  it('switches panel tabs with the arrow keys', async () => {
    wb.ui.showPanel('problems');
    const tab = await screen.findByRole('tab', { name: /Problems/ });
    await fireEvent.keyDown(tab, { key: 'ArrowRight' });
    expect(wb.ui.panelTab).toBe('output');
    expect(screen.getByRole('tab', { name: 'Output' })).toHaveAttribute('aria-selected', 'true');
  });

  it('shows the external-change banner and resolves it', async () => {
    await openSample();
    const state = wb.editors.stateFor('src/main.bas')!;
    wb.editors.update('src/main.bas', state.update({ changes: { from: 0, insert: 'X' } }).state, true);
    mock.externalWrite('src/main.bas', 'CHANGED ON DISK\n');
    const banner = await screen.findByRole('alert');
    expect(banner).toHaveTextContent('changed on disk');
    await fireEvent.click(within(banner).getByRole('button', { name: 'Reload From Disk' }));
    await waitFor(() => expect(wb.editors.text('src/main.bas')).toBe('CHANGED ON DISK\n'));
    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument());
  });
});

describe('manual', () => {
  it('renders sanitized Markdown, follows cross links and keeps history', async () => {
    await wb.manual.open('10-console-io.md', 'print');
    const article = await screen.findByRole('article');
    expect(within(article).getByRole('heading', { name: 'PRINT' })).toHaveAttribute('id', 'print');
    expect(within(article).getByRole('table')).toBeInTheDocument();
    await fireEvent.click(within(article).getByRole('link', { name: 'introduction' }));
    await waitFor(() => expect(wb.manual.current).toEqual({ chapter: '01-introduction.md', anchor: 'welcome' }));
    expect(await screen.findByRole('heading', { name: 'Welcome' })).toBeInTheDocument();
    await fireEvent.click(screen.getByRole('button', { name: 'Back in manual' }));
    await waitFor(() => expect(wb.manual.current?.chapter).toBe('10-console-io.md'));
  });
});
