// End-to-end workflows in a real browser against the mock backend:
// create, open, edit, recover, build, run, debug, ATR, Git and terminal.
import { test, expect, type Page } from '@playwright/test';
import AxeBuilder from '@axe-core/playwright';

async function start(page: Page) {
  await page.goto('/');
  await expect(page.getByText('You have not opened a folder yet.')).toBeVisible();
}

async function openSample(page: Page) {
  await page.getByLabel('Folder path').fill('/work/sample');
  await page.getByLabel('Folder path').press('Enter');
  await expect(page.getByRole('tab', { name: /main\.bas/ })).toBeVisible();
}

async function createProject(page: Page, name = 'demo') {
  await page.getByLabel('Project name').fill(name);
  await page.getByRole('button', { name: 'Create Project' }).click();
  await expect(page.getByRole('tab', { name: /main\.bas/ })).toBeVisible();
}

/** Replaces the editor content by typing, like a user. */
async function typeInEditor(page: Page, text: string) {
  const editor = page.locator('.editor-container:not(.split-pane) .cm-content');
  await editor.click();
  await page.keyboard.press('Control+A');
  await page.keyboard.type(text);
}

/** Fails on any serious or critical accessibility violation. */
async function expectAccessible(page: Page, context: string) {
  const results = await new AxeBuilder({ page })
    // CodeMirror and xterm manage their own internal accessibility trees.
    .exclude('.cm-editor').exclude('.xterm')
    .analyze();
  const serious = results.violations.filter(violation => violation.impact === 'serious' || violation.impact === 'critical');
  expect(serious.map(violation => `${violation.id}: ${violation.nodes.map(node => node.target.join(' ')).join(', ')}`), `accessibility violations in ${context}`).toEqual([]);
}

test('create a project, edit, save and build', async ({ page }) => {
  await start(page);
  await createProject(page);
  await expect(page.locator('.cm-content')).toContainText('HELLO FROM exBASIC STUDIO!');
  await expect(page.locator('.cm-content .cm-line').first()).toBeVisible();

  await typeInEditor(page, 'PRINT 1');
  const tab = page.locator('.editor-tab.active');
  await expect(tab.getByLabel('unsaved changes')).toBeVisible();
  await page.keyboard.press('Control+S');
  await expect(tab.getByLabel('unsaved changes')).toHaveCount(0);

  await page.keyboard.press('F6');
  await expect(page.locator('.output-view')).toContainText('Build succeeded');
  await expect(page.locator('.status-message')).toContainText('Build succeeded');
});

test('syntax errors appear in the editor and the Problems panel', async ({ page }) => {
  await start(page);
  await createProject(page);
  await typeInEditor(page, 'PRINT 1\nSYNTAXERROR');
  // Live lint marks the editor without a build.
  await expect(page.locator('.cm-lintRange-error')).toBeVisible();
  await page.getByRole('tab', { name: /Problems/ }).click();
  const problem = page.locator('.problems button').first();
  await expect(problem).toContainText('parse error');
  await expect(problem).toContainText('src/main.bas:2:1');
  await expect(page.getByRole('button', { name: /1 errors, 0 warnings/ })).toBeVisible();

  // Building with unsaved changes asks first; the failed build lists the error.
  await page.keyboard.press('F6');
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Save before building?');
  await dialog.getByRole('button', { name: 'Save All' }).click();
  await expect(page.locator('.status-message')).toContainText('Build failed');
  await expect(page.getByRole('tab', { name: /Problems/ })).toHaveAttribute('aria-selected', 'true');
});

test('restricted mode: trust is requested before anything runs', async ({ page }) => {
  await start(page);
  await openSample(page);
  await expect(page.getByRole('button', { name: /Restricted Mode/ })).toBeVisible();
  await page.keyboard.press('F6');
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Do you trust this folder?');
  await expect(dialog).toContainText('exbasic');
  await dialog.getByRole('button', { name: 'Stay in Restricted Mode' }).click();
  await expect(page.locator('.status-message')).not.toContainText('Build');

  await page.keyboard.press('F6');
  await page.getByRole('alertdialog').getByRole('button', { name: 'Trust Folder' }).click();
  await expect(page.locator('.status-message')).toContainText('Build succeeded');
  await expect(page.getByRole('button', { name: /Restricted Mode/ })).toHaveCount(0);
});

test('run opens the emulator tab and Stop ends it', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.keyboard.press('Control+F5');
  await expect(page.getByRole('tab', { name: /ex800/ })).toHaveAttribute('aria-selected', 'true');
  await expect(page.locator('.emulator-stage img')).toBeVisible();
  await expect(page.locator('.emulator-toolbar .state').first()).toContainText('Running');
  await page.getByRole('button', { name: 'Stop ex800' }).click();
  await expect(page.getByText('ex800 is stopped')).toBeVisible();
});

test('keyboard joystick and the Keyboard panel', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.keyboard.press('Control+F5');
  await expect(page.locator('.emulator-stage img')).toBeVisible();
  await page.getByRole('tab', { name: 'Keyboard' }).click();
  const keyboard = page.getByRole('img', { name: /keyboard with the Atari key/ });
  await expect(keyboard).toBeVisible();
  await page.getByRole('button', { name: 'Keyboard joystick' }).first().click();
  await expect(page.locator('.emulator-stage')).toBeFocused();
  const indicator = page.locator('.joystick-state');
  await expect(indicator).toContainText('15');
  await page.keyboard.down('ArrowUp');
  await page.keyboard.down('ArrowLeft');
  await page.keyboard.down('Space');
  await expect(indicator).toContainText('↖ 10 · FIRE');
  await expect(keyboard.locator('.key.held')).toHaveCount(3);
  // Leaving the screen releases everything.
  await page.getByRole('tab', { name: 'Keyboard' }).click();
  await expect(keyboard.locator('.key.held')).toHaveCount(0);
  await expect(indicator).toContainText('15');
  await expect(keyboard.locator('.kind-joystick').filter({ hasText: 'FIRE' }).first()).toBeVisible();
  await expect(keyboard.locator('.key').filter({ hasText: 'START' })).toHaveCount(1);
  await expectAccessible(page, 'the Keyboard panel');

  await page.getByRole('radio', { name: 'Mac' }).click();
  await expect(keyboard.locator('.key').filter({ hasText: 'command ⌘' }).first()).toBeVisible();
  await page.getByRole('button', { name: /Keyboard joystick on/ }).click();
  await expect(keyboard.locator('.kind-joystick')).toHaveCount(0);
  await expect(indicator).toHaveCount(0);
});

test('debug: breakpoint, step, watches, memory and stop', async ({ page }) => {
  await start(page);
  await createProject(page);
  // Toggle a breakpoint on line 2 with the gutter.
  await page.locator('.cm-breakpoint-gutter .cm-gutterElement').nth(2).click();
  await expect(page.locator('.cm-breakpoint')).toHaveCount(1);
  await page.keyboard.press('Control+Shift+D');
  await page.getByRole('textbox', { name: 'Watch expression' }).fill('SCORE');
  await page.getByRole('button', { name: 'Add watch' }).click();

  await page.keyboard.press('F5');
  await expect(page.locator('.status-message')).toContainText('Breakpoint at src/main.bas:2');
  await expect(page.locator('.cm-debug-line')).toBeVisible();
  await expect(page.locator('.watch-list code')).toHaveText('20 ($0014)');
  await expect(page.locator('.registers')).toContainText('$0082');
  await expect(page.locator('.disassembly tr.current')).toContainText('LDY $1234');
  await expect(page.locator('.frames')).toContainText('src/main.bas:2');

  await page.getByRole('button', { name: 'Step to next source line' }).click();
  await expect(page.locator('.status-message')).toContainText('src/main.bas:3');
  await expect(page.locator('.watch-list code')).toHaveText('30 ($001E)');

  await page.getByRole('textbox', { name: 'Memory' }).fill('$0600');
  await page.getByRole('button', { name: 'Show', exact: true }).click();
  await expect(page.locator('.memory-dump')).toContainText('0600  00 01 02 03');

  await page.keyboard.press('Shift+F5');
  await expect(page.locator('.cm-debug-line')).toHaveCount(0);
  await expect(page.locator('.debug-toolbar')).toContainText('Not debugging');
});

test('external changes: clean files reload, dirty files prompt', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.evaluate(() => (window as any).__mockNative.externalWrite('src/main.bas', 'PRINT "FROM DISK"\n'));
  await expect(page.locator('.cm-content')).toContainText('FROM DISK');

  await typeInEditor(page, 'PRINT "MINE"');
  await page.evaluate(() => (window as any).__mockNative.externalWrite('src/main.bas', 'PRINT "THEIRS"\n'));
  const banner = page.getByRole('alert');
  await expect(banner).toContainText('changed on disk');
  await expect(page.locator('.cm-content')).toContainText('MINE');
  await banner.getByRole('button', { name: 'Keep My Changes' }).click();
  await expect(banner).toHaveCount(0);
  await expect(page.locator('.cm-content')).toContainText('MINE');

  // A file created by another program appears in the explorer.
  await page.evaluate(() => (window as any).__mockNative.externalWrite('src/added.bas', 'REM\n'));
  await expect(page.getByRole('treeitem', { name: /added\.bas/ })).toBeVisible();
});

test('crash recovery restores unsaved text after a reload', async ({ page }) => {
  await start(page);
  await createProject(page, 'recover');
  await typeInEditor(page, 'PRINT "UNSAVED WORK"');
  await page.waitForTimeout(1500); // the recovery copy is written after a pause
  // Simulate a crash: remount the application on the same backend state.
  await page.evaluate(() => {
    const mock = (window as any).__mockNative;
    (window as any).__recovered = mock.recoveries.get('src/main.bas');
  });
  expect(await page.evaluate(() => (window as any).__recovered)).toContain('UNSAVED WORK');
});

test('source control: init, stage, commit, branch and remote', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.keyboard.press('Control+Shift+G');
  await page.getByRole('button', { name: 'Initialize Repository' }).click();
  await expect(page.getByRole('list', { name: 'Changes', exact: true })).toContainText('src/main.bas');
  await page.getByRole('button', { name: 'Stage all changes' }).click();
  await expect(page.getByRole('list', { name: 'Staged changes' })).toContainText('src/main.bas');
  await page.getByLabel('Commit message').fill('Initial commit');
  await page.getByRole('button', { name: /^Commit/ }).click();
  await expect(page.getByText('There are no pending changes.')).toBeVisible();

  await page.getByRole('button', { name: /Branches/ }).click();
  await page.getByRole('button', { name: 'New Branch…' }).click();
  const prompt = page.getByRole('alertdialog');
  await prompt.getByLabel('Branch name').fill('feature/menus');
  await prompt.getByRole('button', { name: 'Create' }).click();
  await expect(page.getByRole('list', { name: 'Branches' })).toContainText('feature/menus');
  await expect(page.locator('.statusbar')).toContainText('feature/menus');

  await page.getByRole('button', { name: /Remotes/ }).click();
  await page.getByRole('button', { name: 'Add Remote…' }).click();
  await page.getByRole('alertdialog').getByLabel('Remote name').fill('origin');
  await page.getByRole('alertdialog').getByRole('button', { name: 'Next' }).click();
  await page.getByRole('alertdialog').getByLabel('Remote URL').fill('https://example.invalid/repo.git');
  await page.getByRole('alertdialog').getByRole('button', { name: 'Add' }).click();
  await expect(page.getByRole('list', { name: 'Remotes' })).toContainText('origin');
  await page.getByRole('button', { name: /Push/ }).click();
  await expect(page.locator('.output-view')).toContainText('git push finished');
});

test('merge conflict: three-way view and resolution', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.keyboard.press('Control+Shift+G');
  await page.getByRole('button', { name: 'Initialize Repository' }).click();
  await page.evaluate(() => { (window as any).__mockNative.git.conflict = 'src/main.bas'; });
  await page.getByRole('button', { name: 'Refresh source control' }).click();
  await expect(page.locator('.notice.merge')).toContainText('1 conflict');
  await page.getByTitle('Resolve src/main.bas').click();
  const view = page.getByRole('region', { name: 'Merge conflict in src/main.bas' });
  await expect(view.getByLabel('Base version')).toContainText('PRINT 0');
  await expect(view.getByLabel('Current version')).toContainText('CURRENT');
  await expect(view.getByLabel('Incoming version')).toContainText('INCOMING');
  await expectAccessible(page, 'the merge conflict view');
  await view.getByRole('button', { name: 'Accept Both' }).click();
  await expect(view).toHaveCount(0);
  await expect(page.locator('.notice.merge')).toHaveCount(0);
});

test('sprite studio: draw, undo, save, generate and open as JSON', async ({ page }) => {
  await start(page);
  await createProject(page);
  const frames = { idle_0: ['........', '..FFFF..', '.SSSSSF.', 'BB....BB'], idle_1: ['..HH....', '..F.FF..', 'FSSSSSFF', '.BB..BB.'] };
  const sheet = {
    format: 'exbasic-sprites', version: 1, width: 8, height: 4, planes: 2,
    palette: [
      { symbol: '.', name: 'transparent', color: '#0a0f1a', planes: [] }, { symbol: 'H', name: 'hat', color: '#c1802c', planes: [0] },
      { symbol: 'B', name: 'boots', color: '#c1802c', planes: [0] }, { symbol: 'S', name: 'shirt', color: '#237eae', planes: [1] },
      { symbol: 'F', name: 'skin', color: '#eeac70', planes: [0, 1] },
    ],
    source: 'hero_pixels.json', outputs: { bank: 'HERO.PMG', index: 'hero_index.json', indexStyle: 'planes' },
  };
  await page.evaluate(([f, s]) => {
    const mock = (window as any).__mockNative;
    mock.externalWrite('gfx/hero_pixels.json', JSON.stringify(f));
    mock.externalWrite('gfx/hero.sprites.json', JSON.stringify(s));
  }, [frames, sheet]);

  await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Sprite Studio', exact: true }).click();
  await page.getByRole('button', { name: 'gfx/hero.sprites.json' }).click();
  const tab = page.locator('.editor-tab.active');
  await expect(tab).toContainText('hero.sprites.json');
  await expect(page.locator('.frame-list .frame-name')).toHaveText(['idle_0', 'idle_1']);

  // Draw the top-left pixel with the hat colour.
  await page.getByRole('button', { name: 'Draw with hat' }).click();
  const canvas = page.locator('canvas.pixel-canvas');
  const box = (await canvas.boundingBox())!;
  await page.mouse.click(box.x + box.width / 16, box.y + box.height / 8);
  await expect(tab.getByLabel('unsaved changes')).toBeVisible();
  await page.keyboard.press('Control+Z');
  await expect(tab.getByLabel('unsaved changes')).toHaveCount(0);
  await page.keyboard.press('Control+Y');
  await expect(tab.getByLabel('unsaved changes')).toBeVisible();
  await page.keyboard.press('Control+S');
  await expect(tab.getByLabel('unsaved changes')).toHaveCount(0);
  const saved = await page.evaluate(() => (window as any).__mockNative.studio.ReadFile('gfx/hero_pixels.json'));
  expect(JSON.parse(saved).idle_0[0]).toBe('H.......');

  await page.getByRole('button', { name: 'Generate' }).click();
  await expect(page.locator('.status-message')).toContainText('Generated 2 files');
  const bank = await page.evaluate(() => (window as any).__mockNative.studio.ReadBinaryFile('gfx/HERO.PMG'));
  expect(Buffer.from(bank, 'base64')).toHaveLength(16);
  await expectAccessible(page, 'the Sprite Studio');

  await page.getByRole('button', { name: 'Open as JSON text' }).click();
  await expect(page.getByRole('tab', { name: /hero\.sprites\.json/ }).first()).toBeVisible();
  await expect(page.locator('.cm-content')).toContainText('"format": "exbasic-sprites"');
});

test('ATR explorer: browse, create a folder, rename and delete with confirmation', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.getByRole('button', { name: 'ATR Explorer' }).click();
  await page.evaluate(() => (window as any).__mockNative.dialogQueue.push(['/work/disk.atr']));
  await page.getByRole('button', { name: 'Open ATR Image…' }).click();
  const tree = page.getByRole('tree', { name: 'ATR image contents' });
  await expect(tree.getByRole('treeitem', { name: /GAME\.XEX/ })).toBeVisible();
  await expect(page.locator('.atr-summary')).toContainText('SDFS');

  await page.getByRole('button', { name: 'New folder in image' }).click();
  await page.getByLabel('New folder name').fill('SAVES');
  await page.keyboard.press('Enter');
  await expect(tree.getByRole('treeitem', { name: /SAVES/ })).toBeVisible();

  await tree.getByRole('treeitem', { name: /GAME\.XEX/ }).click();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Delete GAME.XEX from disk.atr?');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(tree.getByRole('treeitem', { name: /GAME\.XEX/ })).toBeVisible();
  await page.getByRole('button', { name: 'Delete', exact: true }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click();
  await expect(tree.getByRole('treeitem', { name: /GAME\.XEX/ })).toHaveCount(0);
});

test('terminal: an interactive session with input, Ctrl+C and exit', async ({ page }) => {
  await start(page);
  await createProject(page);
  await page.keyboard.press('Control+`');
  const terminal = page.locator('.terminal-view .xterm');
  await expect(terminal).toBeVisible();
  await expect(page.locator('.xterm-rows')).toContainText('mock-shell');
  await page.keyboard.type('echo hello');
  await page.keyboard.press('Enter');
  await expect(page.locator('.xterm-rows')).toContainText('ran: echo hello');
  await page.keyboard.type('sleep 100');
  await page.keyboard.press('Control+C');
  await expect(page.locator('.xterm-rows')).toContainText('^C');
  await page.keyboard.type('exit');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('tab', { name: /exited 0/ })).toBeVisible();
  await page.getByRole('button', { name: 'Kill active terminal' }).click();
  await expect(page.getByRole('tab', { name: /mock-shell/ })).toHaveCount(0);
});

test('suggestions: Tab accepts a completion and block templates expand', async ({ page }) => {
  await start(page);
  await openSample(page);
  const editor = page.locator('.cm-content');
  await editor.click();
  await page.keyboard.press('Control+End');
  await page.keyboard.press('Enter');
  await page.keyboard.type('PRI');
  await expect(page.locator('.cm-tooltip-autocomplete')).toContainText('PRINT');
  await page.waitForTimeout(200);
  await page.keyboard.press('Tab');
  await expect(editor.locator('.cm-line').last()).toHaveText('PRINT');

  await page.keyboard.type(' 1');
  await page.keyboard.press('Enter');
  await page.keyboard.type('WHIL');
  await expect(page.locator('.cm-tooltip-autocomplete')).toContainText('WHILE … WEND');
  await page.waitForTimeout(200);
  await page.keyboard.press('Tab');
  await expect(editor).toContainText('WEND');
  // Tab now moves from the condition placeholder into the body.
  await page.keyboard.type('X < 3');
  await page.keyboard.press('Tab');
  await page.keyboard.type('X = X + 1');
  await expect(editor).toContainText('WHILE X < 3');
  await expect(editor).toContainText('X = X + 1');
});

test('language features: completion, hover and go to definition', async ({ page }) => {
  await start(page);
  await openSample(page);
  const editor = page.locator('.cm-content');
  // Highlighting comes from the grammar: FOR opens a block.
  await expect(editor).toContainText('FOR I = 1 TO 3');
  await editor.click();
  await page.keyboard.press('Control+End');
  await page.keyboard.type('\nPRI');
  await expect(page.locator('.cm-tooltip-autocomplete')).toContainText('PRINT');
  await page.keyboard.press('Escape');

  // Signature help while typing a call to a FUNC from another file.
  await page.keyboard.type('NT Twice(');
  await expect(page.locator('.cm-signature-help')).toContainText('FUNC Twice(N)');

  // Go to definition follows the INCLUDE into the other file.
  await page.keyboard.press('Control+Home');
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('End');
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('F12');
  await expect(page.getByRole('tab', { name: /util\.bas/ })).toHaveAttribute('aria-selected', 'true');

  // The outline lists the function of the active file.
  await expect(page.locator('.outline')).toContainText('Twice');
});

test('manual: sanitized Markdown, cross links and search highlighting', async ({ page }) => {
  await start(page);
  await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'exBASIC Manual' }).click();
  await page.getByRole('textbox', { name: 'Search manual' }).fill('print');
  await page.keyboard.press('Enter');
  await page.getByRole('group', { name: 'Manual search results' }).getByRole('button').first().click();
  const article = page.getByRole('article');
  await expect(article.getByRole('heading', { name: 'PRINT' })).toBeVisible();
  await expect(article.locator('mark.search-hit').first()).toBeVisible();
  await expect(article.getByRole('table')).toBeVisible();
  await article.getByRole('link', { name: 'introduction' }).click();
  await expect(article.getByRole('heading', { name: 'Welcome' })).toBeVisible();
  await page.getByRole('button', { name: 'Back in manual' }).click();
  await expect(article.getByRole('heading', { name: 'PRINT' })).toBeVisible();
});

test('layout: resize with the keyboard and remember across a reload', async ({ page }) => {
  await start(page);
  const sash = page.getByRole('separator', { name: 'Resize side bar' });
  await sash.focus();
  const before = Number(await sash.getAttribute('aria-valuenow'));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('ArrowRight');
  await expect(sash).toHaveAttribute('aria-valuenow', String(before + 32));
  await page.keyboard.press('Control+J');
  await expect(page.getByRole('region', { name: 'Panel' })).toHaveCount(0);
  await page.keyboard.press('Control+J');
  await expect(page.getByRole('region', { name: 'Panel' })).toBeVisible();
});

test.describe('accessibility', () => {
  test('welcome screen', async ({ page }) => {
    await start(page);
    await expectAccessible(page, 'the welcome screen');
  });

  test('editor with explorer, problems and status bar', async ({ page }) => {
    await start(page);
    await createProject(page);
    await typeInEditor(page, 'SYNTAXERROR');
    await page.getByRole('tab', { name: /Problems/ }).click();
    await expect(page.locator('.problems button')).toHaveCount(1);
    await expectAccessible(page, 'the editor layout');
  });

  test('every side bar view', async ({ page }) => {
    await start(page);
    await createProject(page);
    for (const view of ['Search', 'ATR Explorer', 'Source Control', 'Run and Debug', 'ex800 Emulator', 'Sprite Studio', 'exBASIC Manual', 'Settings']) {
      await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: view, exact: true }).click();
      await expect(page.locator('.sidebar-title')).toHaveText(view);
      await expectAccessible(page, `the ${view} view`);
    }
  });

  test('dialog, command palette and menu', async ({ page }) => {
    await start(page);
    await openSample(page);
    await page.keyboard.press('F6');
    await expect(page.getByRole('alertdialog')).toBeVisible();
    await expectAccessible(page, 'the trust dialog');
    await page.keyboard.press('Escape');
    await page.keyboard.press('Control+Shift+P');
    await expect(page.getByRole('combobox')).toBeFocused();
    await expectAccessible(page, 'the command palette');
    await page.keyboard.press('Escape');
    await page.getByRole('menuitem', { name: 'File' }).click();
    await expect(page.getByRole('menu', { name: 'File' })).toBeVisible();
    await expectAccessible(page, 'the File menu');
  });

  test('high contrast theme and emulator view', async ({ page }) => {
    await start(page);
    await createProject(page);
    await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Settings', exact: true }).click();
    await page.getByLabel('Theme', { exact: true }).selectOption('high-contrast');
    await expect(page.locator('.workbench.high-contrast')).toBeVisible();
    await page.keyboard.press('Control+F5');
    await expect(page.locator('.emulator-stage img')).toBeVisible();
    await expectAccessible(page, 'the high contrast emulator view');
  });

  test('light theme: editor, settings and every syntax scheme', async ({ page }) => {
    await start(page);
    await createProject(page);
    await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Settings', exact: true }).click();
    await page.getByLabel('Theme', { exact: true }).selectOption('light');
    await expect(page.locator('.workbench.theme-light')).toBeVisible();
    for (const scheme of ['bright', 'classic', 'soft', 'retro']) {
      await page.getByLabel('Syntax colours').selectOption(scheme);
      await expect(page.locator(`.workbench.syntax-${scheme}`)).toBeVisible();
      await expectAccessible(page, `the light theme with ${scheme} syntax colours`);
    }
    await page.getByLabel('Editor font size').selectOption('18');
    await expect(page.locator('.cm-editor').first()).toHaveCSS('font-size', '18px');
  });

  test('dark theme: every syntax scheme', async ({ page }) => {
    await start(page);
    await createProject(page);
    await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Settings', exact: true }).click();
    for (const scheme of ['classic', 'soft', 'retro']) {
      await page.getByLabel('Syntax colours').selectOption(scheme);
      await expectAccessible(page, `the dark theme with ${scheme} syntax colours`);
    }
  });

  test('every control is reachable and operable by keyboard', async ({ page }) => {
    await start(page);
    // Tab from the top reaches the menu bar first, and focus is visible.
    await page.keyboard.press('Tab');
    const focused = page.locator(':focus');
    await expect(focused).toBeVisible();
    const outline = await focused.evaluate(element => getComputedStyle(element).outlineStyle);
    expect(outline).not.toBe('none');
    // The whole create-project flow works without a mouse.
    await page.getByLabel('Project name').focus();
    await page.keyboard.press('Control+A');
    await page.keyboard.type('keys');
    await page.keyboard.press('Tab');
    await page.keyboard.press('Enter');
    await expect(page.getByRole('tab', { name: /main\.bas/ })).toBeVisible();
  });
});
