// Where the Atari keys are on a PC or Mac keyboard in the emulator. The map
// mirrors the Atari800 helper (third_party/atari800/studio-helper/main.c):
// printable characters reach the Atari as the same character, so Shift+2
// types @ on a US keyboard and @ on the Atari; named keys become the Atari
// keys listed here. Layouts are US ANSI; keys are KeyboardEvent.code values.
import { joystickRole } from './joystick';

export type KeyKind =
  | 'char'      // types the character printed on the key
  | 'atari'     // an Atari key with a different name or no PC equivalent
  | 'console'   // START, SELECT, OPTION
  | 'modifier'  // SHIFT, CONTROL
  | 'joystick'  // keyboard joystick direction or fire
  | 'none';     // nothing on the Atari

export interface KeyCap {
  /** KeyboardEvent.code, empty for keys the browser never reports (fn). */
  code: string;
  /** Legend on the PC or Mac key. */
  label: string;
  /** Shifted legend, for character keys. */
  shift?: string;
  /** Column and row in key units (row 0 is the function key row). */
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface AtariKey {
  kind: KeyKind;
  /** Legend of the Atari key, shown under the PC legend. */
  label: string;
  /** Longer explanation for the tooltip. */
  detail?: string;
}

export type KeyboardLayout = 'pc' | 'mac';

type Spec = [code: string, label: string, w?: number, shift?: string, h?: number] | number;

/** Lays a row out left to right from x; a number is a gap of that width. */
function row(y: number, x: number, specs: Spec[]): KeyCap[] {
  const keys: KeyCap[] = [];
  for (const spec of specs) {
    if (typeof spec === 'number') { x += spec; continue; }
    const [code, label, w = 1, shift, h = 1] = spec;
    keys.push({ code, label, shift, x, y, w, h });
    x += w;
  }
  return keys;
}

const digits: Spec[] = [
  ['Digit1', '1', 1, '!'], ['Digit2', '2', 1, '@'], ['Digit3', '3', 1, '#'], ['Digit4', '4', 1, '$'], ['Digit5', '5', 1, '%'],
  ['Digit6', '6', 1, '^'], ['Digit7', '7', 1, '&'], ['Digit8', '8', 1, '*'], ['Digit9', '9', 1, '('], ['Digit0', '0', 1, ')'],
  ['Minus', '-', 1, '_'], ['Equal', '=', 1, '+'],
];
const letters = (text: string): Spec[] => [...text].map(letter => [`Key${letter}`, letter]);
const top: Spec[] = [...letters('QWERTYUIOP'), ['BracketLeft', '[', 1, '{'], ['BracketRight', ']', 1, '}']];
const home: Spec[] = [...letters('ASDFGHJKL'), ['Semicolon', ';', 1, ':'], ['Quote', "'", 1, '"']];
const bottom: Spec[] = [...letters('ZXCVBNM'), ['Comma', ',', 1, '<'], ['Period', '.', 1, '>'], ['Slash', '/', 1, '?']];
const fkeys = (from: number, to: number): Spec[] => Array.from({ length: to - from + 1 }, (_, i) => [`F${from + i}`, `F${from + i}`] as Spec);

// Block positions: main 15 units, navigation 3, keypad 4, half-unit gaps.
const NAV = 15.5, PAD = 19;

function pcLayout(): KeyCap[] {
  return [
    ...row(0, 0, [['Escape', 'Esc'], 1, ...fkeys(1, 4), 0.5, ...fkeys(5, 8), 0.5, ...fkeys(9, 12)]),
    ...row(0, NAV, [['PrintScreen', 'PrtSc'], ['ScrollLock', 'ScrLk'], ['Pause', 'Pause']]),
    ...row(1, 0, [['Backquote', '`', 1, '~'], ...digits, ['Backspace', 'Backspace', 2]]),
    ...row(1, NAV, [['Insert', 'Ins'], ['Home', 'Home'], ['PageUp', 'PgUp']]),
    ...row(1, PAD, [['NumLock', 'Num'], ['NumpadDivide', '/'], ['NumpadMultiply', '*'], ['NumpadSubtract', '-']]),
    ...row(2, 0, [['Tab', 'Tab', 1.5], ...top, ['Backslash', '\\', 1.5, '|']]),
    ...row(2, NAV, [['Delete', 'Del'], ['End', 'End'], ['PageDown', 'PgDn']]),
    ...row(2, PAD, [['Numpad7', '7'], ['Numpad8', '8'], ['Numpad9', '9'], ['NumpadAdd', '+', 1, undefined, 2]]),
    ...row(3, 0, [['CapsLock', 'Caps Lock', 1.75], ...home, ['Enter', 'Enter', 2.25]]),
    ...row(3, PAD, [['Numpad4', '4'], ['Numpad5', '5'], ['Numpad6', '6']]),
    ...row(4, 0, [['ShiftLeft', 'Shift', 2.25], ...bottom, ['ShiftRight', 'Shift', 2.75]]),
    ...row(4, NAV + 1, [['ArrowUp', '↑']]),
    ...row(4, PAD, [['Numpad1', '1'], ['Numpad2', '2'], ['Numpad3', '3'], ['NumpadEnter', 'Enter', 1, undefined, 2]]),
    ...row(5, 0, [['ControlLeft', 'Ctrl', 1.25], ['MetaLeft', 'Win', 1.25], ['AltLeft', 'Alt', 1.25], ['Space', '', 6.25],
      ['AltRight', 'Alt', 1.25], ['MetaRight', 'Win', 1.25], ['ContextMenu', 'Menu', 1.25], ['ControlRight', 'Ctrl', 1.25]]),
    ...row(5, NAV, [['ArrowLeft', '←'], ['ArrowDown', '↓'], ['ArrowRight', '→']]),
    ...row(5, PAD, [['Numpad0', '0', 2], ['NumpadDecimal', '.']]),
  ];
}

function macLayout(): KeyCap[] {
  return [
    ...row(0, 0, [['Escape', 'esc'], 1, ...fkeys(1, 4), 0.5, ...fkeys(5, 8), 0.5, ...fkeys(9, 12)]),
    ...row(0, NAV, fkeys(13, 15)),
    ...row(1, 0, [['Backquote', '`', 1, '~'], ...digits, ['Backspace', 'delete', 2]]),
    ...row(1, NAV, [['', 'fn'], ['Home', 'home'], ['PageUp', 'pg up']]),
    ...row(1, PAD, [['NumLock', 'clear'], ['NumpadEqual', '='], ['NumpadDivide', '/'], ['NumpadMultiply', '*']]),
    ...row(2, 0, [['Tab', 'tab', 1.5], ...top, ['Backslash', '\\', 1.5, '|']]),
    ...row(2, NAV, [['Delete', 'del ⌦'], ['End', 'end'], ['PageDown', 'pg dn']]),
    ...row(2, PAD, [['Numpad7', '7'], ['Numpad8', '8'], ['Numpad9', '9'], ['NumpadSubtract', '-']]),
    ...row(3, 0, [['CapsLock', 'caps lock', 1.75], ...home, ['Enter', 'return', 2.25]]),
    ...row(3, PAD, [['Numpad4', '4'], ['Numpad5', '5'], ['Numpad6', '6'], ['NumpadAdd', '+']]),
    ...row(4, 0, [['ShiftLeft', 'shift', 2.25], ...bottom, ['ShiftRight', 'shift', 2.75]]),
    ...row(4, NAV + 1, [['ArrowUp', '↑']]),
    ...row(4, PAD, [['Numpad1', '1'], ['Numpad2', '2'], ['Numpad3', '3'], ['NumpadEnter', 'enter', 1, undefined, 2]]),
    ...row(5, 0, [['ControlLeft', 'control', 1.75], ['AltLeft', 'option ⌥', 1.25], ['MetaLeft', 'command ⌘', 1.5], ['Space', '', 6],
      ['MetaRight', 'command ⌘', 1.5], ['AltRight', 'option ⌥', 1.25], ['ControlRight', 'control', 1.75]]),
    ...row(5, NAV, [['ArrowLeft', '←'], ['ArrowDown', '↓'], ['ArrowRight', '→']]),
    ...row(5, PAD, [['Numpad0', '0', 2], ['NumpadDecimal', '.']]),
  ];
}

export const layouts: Record<KeyboardLayout, { label: string; keys: KeyCap[] }> = {
  pc: { label: 'PC', keys: pcLayout() },
  mac: { label: 'Mac', keys: macLayout() },
};

/** Width and height of a layout in key units. */
export const LAYOUT_WIDTH = PAD + 4;
export const LAYOUT_HEIGHT = 6;

const named: Record<string, AtariKey> = {
  Escape: { kind: 'atari', label: 'ESC' },
  F1: { kind: 'atari', label: 'F1', detail: 'F1 of the 1200XL' },
  F2: { kind: 'atari', label: 'F2', detail: 'F2 of the 1200XL' },
  F3: { kind: 'atari', label: 'F3', detail: 'F3 of the 1200XL' },
  F4: { kind: 'atari', label: 'F4', detail: 'F4 of the 1200XL' },
  F5: { kind: 'console', label: 'START', detail: 'START. While debugging, F5 continues instead.' },
  F6: { kind: 'console', label: 'SELECT', detail: 'SELECT. While debugging, Studio keeps F6.' },
  F7: { kind: 'console', label: 'OPTION', detail: 'OPTION' },
  F8: { kind: 'atari', label: 'BREAK', detail: 'BREAK. While debugging, F8 pauses instead.' },
  F9: { kind: 'none', label: '', detail: 'Kept by Studio for the debugger' },
  F10: { kind: 'none', label: '', detail: 'Kept by Studio for the debugger' },
  F11: { kind: 'none', label: '', detail: 'Kept by Studio for the debugger' },
  Pause: { kind: 'atari', label: 'BREAK' },
  Backspace: { kind: 'atari', label: 'DELETE BACK S', detail: 'DELETE BACK S: Shift deletes the line' },
  Tab: { kind: 'atari', label: 'TAB', detail: 'TAB: Shift clears a tab stop, Ctrl sets one' },
  CapsLock: { kind: 'atari', label: 'CAPS', detail: 'CAPS (LOWR): toggles upper and lower case' },
  Enter: { kind: 'atari', label: 'RETURN' },
  NumpadEnter: { kind: 'atari', label: 'RETURN' },
  Insert: { kind: 'atari', label: 'INSERT', detail: 'Inserts a character (Ctrl+INSERT on the Atari)' },
  Delete: { kind: 'atari', label: 'DEL CHAR', detail: 'Deletes the character under the cursor (Ctrl+DELETE on the Atari)' },
  Home: { kind: 'atari', label: 'CLEAR', detail: 'CLEAR: clears the screen (Shift+< on the Atari)' },
  End: { kind: 'atari', label: 'HELP', detail: 'HELP of the XL and XE' },
  ArrowUp: { kind: 'atari', label: '↑', detail: 'Cursor up (Ctrl+- on the Atari)' },
  ArrowDown: { kind: 'atari', label: '↓', detail: 'Cursor down (Ctrl+= on the Atari)' },
  ArrowLeft: { kind: 'atari', label: '←', detail: 'Cursor left (Ctrl++ on the Atari)' },
  ArrowRight: { kind: 'atari', label: '→', detail: 'Cursor right (Ctrl+* on the Atari)' },
  ShiftLeft: { kind: 'modifier', label: 'SHIFT' },
  ShiftRight: { kind: 'modifier', label: 'SHIFT' },
  ControlLeft: { kind: 'modifier', label: 'CONTROL' },
  ControlRight: { kind: 'modifier', label: 'CONTROL' },
  AltLeft: { kind: 'atari', label: 'ATARI ◪', detail: 'The Atari logo key (inverse video)' },
  AltRight: { kind: 'atari', label: 'ATARI ◪', detail: 'The Atari logo key (inverse video)' },
  MetaLeft: { kind: 'atari', label: 'ATARI ◪', detail: 'The Atari logo key (inverse video)' },
  MetaRight: { kind: 'atari', label: 'ATARI ◪', detail: 'The Atari logo key (inverse video)' },
  Space: { kind: 'char', label: 'SPACE' },
  Backquote: { kind: 'none', label: '', detail: 'The Atari has no ` or ~' },
  NumLock: { kind: 'none', label: '' },
};

/** What a key does on the emulated Atari. With the keyboard joystick on,
 * its keys show their direction or FIRE instead. */
export function atariKey(cap: KeyCap, joystick = false): AtariKey {
  if (joystick) {
    const role = joystickRole(cap.code);
    if (role) return { kind: 'joystick', label: role, detail: role === 'FIRE' ? 'Fire: STRIG(0) = 0' : `Joystick 0 ${role}` };
  }
  const known = named[cap.code];
  if (known) return known;
  if (cap.code === 'BracketLeft' || cap.code === 'BracketRight') {
    return { kind: 'char', label: cap.label, detail: `${cap.label}; the Atari has no ${cap.shift}` };
  }
  // Characters, the digits and the keypad operators type themselves.
  if (/^(Key|Digit|Numpad)/.test(cap.code) || ['Minus', 'Equal', 'Backslash', 'Semicolon', 'Quote', 'Comma', 'Period', 'Slash'].includes(cap.code)) {
    return { kind: 'char', label: cap.shift ? `${cap.label} ${cap.shift}` : cap.label };
  }
  return { kind: 'none', label: '' };
}
