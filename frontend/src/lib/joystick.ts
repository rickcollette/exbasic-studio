// Keyboard joystick: PC keys held down become joystick 0 of the emulated
// Atari. Keys are identified by KeyboardEvent.code, so the numeric keypad
// works with Num Lock on or off and the letters follow their position, not
// the keyboard layout.
//
//   Q W E      ↖ ↑ ↗        Numpad 7 8 9
//   A   D      ←   →        Numpad 4   6
//   Z X C      ↙ ↓ ↘        Numpad 1 2 3
//
// The arrow keys give the four directions; two held together give a
// diagonal. Fire is Space, Numpad 0 or either Ctrl key.

/** STICK() bits; a bit is 0 while its direction is pressed. */
export const UP = 1, DOWN = 2, LEFT = 4, RIGHT = 8;
/** STICK() of a centred joystick. */
export const NEUTRAL = 15;

const directions: Record<string, number> = {
  KeyQ: UP | LEFT, KeyW: UP, KeyE: UP | RIGHT,
  KeyA: LEFT, KeyD: RIGHT,
  KeyZ: DOWN | LEFT, KeyX: DOWN, KeyC: DOWN | RIGHT,
  ArrowUp: UP, ArrowDown: DOWN, ArrowLeft: LEFT, ArrowRight: RIGHT,
  Numpad7: UP | LEFT, Numpad8: UP, Numpad9: UP | RIGHT,
  Numpad4: LEFT, Numpad6: RIGHT,
  Numpad1: DOWN | LEFT, Numpad2: DOWN, Numpad3: DOWN | RIGHT,
};

const fire = new Set(['Space', 'Numpad0', 'ControlLeft', 'ControlRight']);

/** True when the key with this KeyboardEvent.code belongs to the joystick. */
export function isJoystickKey(code: string): boolean {
  return code in directions || fire.has(code);
}

const arrows: Record<number, string> = {
  [UP]: '↑', [DOWN]: '↓', [LEFT]: '←', [RIGHT]: '→',
  [UP | LEFT]: '↖', [UP | RIGHT]: '↗', [DOWN | LEFT]: '↙', [DOWN | RIGHT]: '↘',
};

/** What a key does on the keyboard joystick: an arrow, "FIRE", or null. */
export function joystickRole(code: string): string | null {
  if (fire.has(code)) return 'FIRE';
  return code in directions ? arrows[directions[code]] : null;
}

/** The STICK() value for a set of held keys. Opposite directions held
 * together cancel out, as a real stick cannot point both ways. */
export function stickValue(held: Iterable<string>): number {
  let pressed = 0;
  for (const code of held) pressed |= directions[code] ?? 0;
  if ((pressed & (UP | DOWN)) === (UP | DOWN)) pressed &= ~(UP | DOWN);
  if ((pressed & (LEFT | RIGHT)) === (LEFT | RIGHT)) pressed &= ~(LEFT | RIGHT);
  return NEUTRAL & ~pressed;
}

/** True while any fire key is held. STRIG() then returns 0. */
export function triggerPressed(held: Iterable<string>): boolean {
  for (const code of held) if (fire.has(code)) return true;
  return false;
}

/** The joystick keys currently held. */
export class KeyboardJoystick {
  private held = new Set<string>();

  /** Records a key press or release. Returns true when STICK() or STRIG()
   * changed, so auto-repeat and keys outside the joystick send nothing. */
  press(code: string, down: boolean): boolean {
    if (!isJoystickKey(code)) return false;
    const before = this.stick * 2 + (this.trigger ? 1 : 0);
    if (down) this.held.add(code);
    else this.held.delete(code);
    return before !== this.stick * 2 + (this.trigger ? 1 : 0);
  }

  /** Releases every key. Returns true when the joystick was not centred. */
  clear(): boolean {
    const moved = this.stick !== NEUTRAL || this.trigger;
    this.held.clear();
    return moved;
  }

  get stick(): number { return stickValue(this.held); }
  get trigger(): boolean { return triggerPressed(this.held); }
}
