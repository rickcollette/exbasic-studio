// The Keyboard panel map: layouts fit their grid and keys show the Atari key
// the helper produces.
import { describe, expect, it } from 'vitest';
import { LAYOUT_HEIGHT, LAYOUT_WIDTH, atariKey, layouts, type KeyCap } from './keyboardMap';

function key(layout: 'pc' | 'mac', code: string): KeyCap {
  const found = layouts[layout].keys.find(cap => cap.code === code);
  if (!found) throw new Error(`${code} is not on the ${layout} layout`);
  return found;
}

describe('keyboard layouts', () => {
  for (const [id, layout] of Object.entries(layouts)) {
    it(`${id} keys fit the grid without overlapping`, () => {
      const cells = new Set<string>();
      for (const cap of layout.keys) {
        expect(cap.x + cap.w).toBeLessThanOrEqual(LAYOUT_WIDTH);
        expect(cap.y + cap.h).toBeLessThanOrEqual(LAYOUT_HEIGHT);
        for (let x = cap.x * 4; x < (cap.x + cap.w) * 4; x++) {
          for (let y = cap.y; y < cap.y + cap.h; y++) {
            const cell = `${x},${y}`;
            expect(cells.has(cell), `${cap.code || cap.label} overlaps at ${cell}`).toBe(false);
            cells.add(cell);
          }
        }
      }
    });

    it(`${id} main rows are 15 units wide`, () => {
      for (let y = 1; y <= 5; y++) {
        const right = Math.max(...layout.keys.filter(cap => cap.y === y && cap.x < 15).map(cap => cap.x + cap.w));
        expect(right, `row ${y}`).toBe(15);
      }
    });
  }
});

describe('atariKey', () => {
  it.each([
    ['F5', 'START', 'console'], ['F6', 'SELECT', 'console'], ['F7', 'OPTION', 'console'],
    ['Home', 'CLEAR', 'atari'], ['End', 'HELP', 'atari'], ['Pause', 'BREAK', 'atari'], ['F8', 'BREAK', 'atari'],
    ['Backspace', 'DELETE BACK S', 'atari'], ['Enter', 'RETURN', 'atari'], ['CapsLock', 'CAPS', 'atari'],
    ['AltLeft', 'ATARI ◪', 'atari'], ['ShiftLeft', 'SHIFT', 'modifier'], ['ControlLeft', 'CONTROL', 'modifier'],
  ])('PC %s is %s', (code, label, kind) => {
    expect(atariKey(key('pc', code))).toMatchObject({ label, kind });
  });

  it('maps the Mac command and option keys to the Atari key', () => {
    expect(atariKey(key('mac', 'MetaLeft')).label).toBe('ATARI ◪');
    expect(atariKey(key('mac', 'AltRight')).label).toBe('ATARI ◪');
    expect(key('mac', 'Backspace').label).toBe('delete');
  });

  it('shows characters as themselves and marks keys the Atari lacks', () => {
    expect(atariKey(key('pc', 'Digit2'))).toEqual({ kind: 'char', label: '2 @' });
    expect(atariKey(key('pc', 'Backquote')).kind).toBe('none');
    expect(atariKey(key('pc', 'PageUp')).kind).toBe('none');
  });

  it('shows the joystick only while it is on', () => {
    expect(atariKey(key('pc', 'KeyQ'), true)).toMatchObject({ kind: 'joystick', label: '↖' });
    expect(atariKey(key('pc', 'Numpad3'), true)).toMatchObject({ kind: 'joystick', label: '↘' });
    expect(atariKey(key('pc', 'Space'), true)).toMatchObject({ kind: 'joystick', label: 'FIRE' });
    expect(atariKey(key('pc', 'ControlRight'), true).label).toBe('FIRE');
    expect(atariKey(key('pc', 'KeyS'), true).kind).toBe('char');
    expect(atariKey(key('pc', 'KeyQ'), false).kind).toBe('char');
  });
});
