// Smoke test of the real application: the built executable, the real
// backend and the bundled tools, driven through the WebView2 debugging port.
// The Playwright suite in ../e2e runs against the mock backend and cannot see
// faults in the native layer; this one can.
//
//   npm run test:real            (Windows; needs a build in ../bin)
//
// It uses a private configuration directory and a temporary project, so it
// does not touch the developer's own recent projects or trust decisions.
import { chromium } from '@playwright/test';
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';

const scratch = process.argv[2] || fs.mkdtempSync(path.join(os.tmpdir(), 'exbasic-studio-smoke-'));
const exe = process.env.EXBASIC_STUDIO_EXE || path.resolve(import.meta.dirname, '../../bin/exbasic-studio.exe');
const port = process.env.EXBASIC_STUDIO_DEBUG_PORT || '9333';
const appdata = path.join(scratch, 'appdata');
const project = path.join(scratch, 'proj', 'TEST1');
const failures = [];
const check = (ok, what) => { console.log((ok ? 'ok   ' : 'FAIL ') + what); if (!ok) failures.push(what); };

fs.rmSync(appdata, { recursive: true, force: true });
fs.rmSync(project, { recursive: true, force: true });
fs.mkdirSync(project, { recursive: true });
fs.writeFileSync(path.join(project, 'new.bas'), ['PRINT "HI"', 'GET K', ''].join('\n'));

const child = spawn(exe, [], { cwd: scratch, env: { ...process.env, APPDATA: appdata, XDG_CONFIG_HOME: appdata, EXBASIC_STUDIO_DEBUG_PORT: port }, stdio: 'ignore' });
let browser;
for (let attempt = 0; attempt < 40 && !browser; attempt++) {
  try { browser = await chromium.connectOverCDP(`http://127.0.0.1:${port}`); } catch { await new Promise(resolve => setTimeout(resolve, 500)); }
}
if (!browser) { child.kill(); console.error('FAIL could not attach to the application window'); process.exit(1); }

const page = browser.contexts()[0].pages()[0];
const pageErrors = [];
page.on('pageerror', error => pageErrors.push(error.message));
const onDisk = () => fs.readdirSync(project).sort().join(',');
const outputText = async () => { await page.getByRole('tab', { name: 'Output' }).click(); return page.locator('[role=tabpanel]').last().innerText(); };

try {
  await page.waitForSelector('#folder-path', { timeout: 20000 });

  // A folder far too large to be a project fails quickly and changes nothing.
  await page.fill('#folder-path', os.homedir());
  const started = Date.now();
  await page.press('#folder-path', 'Enter');
  await page.waitForFunction(() => document.body.innerText.includes('too many for a project'), null, { timeout: 15000 });
  check(Date.now() - started < 10000, 'opening an oversized folder fails quickly');
  check(await page.locator('#folder-path').count() === 1, 'the welcome view is still shown after the failed open');

  // Open the project, create a file, edit and save it.
  await page.fill('#folder-path', project);
  await page.press('#folder-path', 'Enter');
  await page.waitForSelector('[role=tree]', { timeout: 10000 });
  await page.click('button[aria-label="New file"]');
  await page.locator('.inline-create input').fill('basfile.inc');
  await page.locator('.inline-create input').press('Enter');
  await page.waitForFunction(() => [...document.querySelectorAll('[role=treeitem] .name')].some(item => item.textContent === 'basfile.inc'), null, { timeout: 5000 });
  check(onDisk() === 'basfile.inc,new.bas', 'the new file is created inside the project folder');
  await page.locator('.cm-content').first().click();
  await page.keyboard.type('REM edited');
  await page.keyboard.press('Control+S');
  await page.waitForTimeout(700);
  check(fs.readFileSync(path.join(project, 'basfile.inc'), 'utf8') === 'REM edited', 'saving writes the file in the project folder');

  // Sprite Studio: a binary PMG bank is read, edited, saved and exported as PNG.
  fs.writeFileSync(path.join(project, 'ENEMY.PMG'), Buffer.from([0x18, 0x3c, 0x7e, 0xff, 0xdb, 0x7e, 0x24, 0x42]));
  fs.writeFileSync(path.join(project, 'enemy_index.json'), JSON.stringify({ bat: { offset: 0, length: 8, width: 8, height: 8 } }, null, 2) + '\n');
  await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Sprite Studio', exact: true }).click();
  await page.getByRole('button', { name: 'ENEMY.PMG' }).click({ timeout: 10000 });
  await page.waitForSelector('.frame-list .frame-name:has-text("bat")', { timeout: 10000 });
  check(true, 'a PMG bank opens in the Sprite Studio with its index');
  const pixels = await page.locator('canvas.pixel-canvas').boundingBox();
  await page.mouse.click(pixels.x + pixels.width / 16, pixels.y + pixels.height / 16);
  await page.keyboard.press('Control+S');
  await page.waitForTimeout(700);
  check(fs.readFileSync(path.join(project, 'ENEMY.PMG')).equals(Buffer.from([0x98, 0x3c, 0x7e, 0xff, 0xdb, 0x7e, 0x24, 0x42])), 'drawing a pixel and saving rewrites the PMG bank on disk');
  await page.getByRole('button', { name: 'PNG', exact: true }).click();
  await page.getByRole('button', { name: 'Export', exact: true }).click();
  await page.waitForTimeout(700);
  const png = path.join(project, 'ENEMY_sheet.png');
  check(fs.existsSync(png) && fs.readFileSync(png).subarray(1, 4).toString() === 'PNG', 'the frames export as a PNG sheet');
  await page.keyboard.press('Control+W');
  await page.getByRole('navigation', { name: 'Views' }).getByRole('button', { name: 'Explorer', exact: true }).click();

  // Run: the trust prompt appears, then the build runs and the emulator starts.
  await page.locator('[role=treeitem]', { hasText: 'new.bas' }).click();
  await page.keyboard.press('Control+F5');
  await page.waitForSelector('text=Do you trust this folder?', { timeout: 5000 });
  await page.getByRole('button', { name: 'Trust Folder', exact: true }).click();
  await page.waitForSelector('text=/Running · frame \\d+/', { timeout: 30000 });
  const output = await outputText();
  check(output.includes('Build succeeded'), 'the build succeeds');
  check(output.includes('ex800 started'), 'the emulator starts');
  check(fs.existsSync(path.join(project, 'build', 'TEST1.xex')), 'the executable is written to the project build folder');
  await page.waitForSelector('.emulator-toolbar >> text=/H1:/', { timeout: 10000 }).catch(() => {});
  check(await page.locator('text=/H1:/').count() > 0, 'the H1: host drive is mapped and shown');

  // Keyboard joystick and the Keyboard panel against the real helper.
  await page.getByRole('tab', { name: 'Keyboard' }).click();
  await page.getByRole('button', { name: 'Keyboard joystick' }).first().click();
  await page.keyboard.down('ArrowUp');
  await page.keyboard.down('ArrowLeft');
  await page.keyboard.down('Space');
  await page.waitForSelector('.joystick-state:has-text("↖ 10 · FIRE")', { timeout: 5000 });
  check(await page.locator('.keyboard .key.held').count() === 3, 'the keyboard joystick reports STICK 10 with fire and lights the keys');
  for (const key of ['Space', 'ArrowLeft', 'ArrowUp']) await page.keyboard.up(key);
  await page.waitForSelector('.joystick-state:has-text("15")', { timeout: 5000 });
  check(await page.locator('text=ex800 stopped').count() === 0, 'the helper accepts joystick requests');

  check(pageErrors.length === 0, `no script errors in the window${pageErrors.length ? ': ' + pageErrors.join('; ') : ''}`);
} catch (error) {
  check(false, `scenario aborted: ${error.message}`);
  await page.screenshot({ path: path.join(scratch, 'failure.png') }).catch(() => {});
  console.log(`screenshot and logs kept in ${scratch}`);
}

await browser.close().catch(() => {});
child.kill();
console.log(failures.length ? `FAIL (${failures.length})` : 'PASS');
process.exit(failures.length ? 1 : 0);
