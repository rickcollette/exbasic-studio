// Keyboard joystick: key to STICK()/STRIG() mapping and the emulator store.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { KeyboardJoystick, NEUTRAL, isJoystickKey, joystickRole, stickValue, triggerPressed } from './joystick';
import { setNative } from './native';
import { createMockNative, type MockNative } from './mockNative';
import { createWorkbench, type Workbench } from './workbench.svelte';

describe('stickValue', () => {
  it('is 15 when nothing is held', () => {
    expect(stickValue([])).toBe(NEUTRAL);
  });

  it.each([
    // Q W E / A D / Z X C
    ['KeyW', 14], ['KeyX', 13], ['KeyA', 11], ['KeyD', 7],
    ['KeyQ', 10], ['KeyE', 6], ['KeyZ', 9], ['KeyC', 5],
    // arrows
    ['ArrowUp', 14], ['ArrowDown', 13], ['ArrowLeft', 11], ['ArrowRight', 7],
    // numeric keypad
    ['Numpad8', 14], ['Numpad2', 13], ['Numpad4', 11], ['Numpad6', 7],
    ['Numpad7', 10], ['Numpad9', 6], ['Numpad1', 9], ['Numpad3', 5],
  ])('%s gives STICK %i', (code, value) => {
    expect(stickValue([code])).toBe(value);
  });

  it('combines two arrows into a diagonal', () => {
    expect(stickValue(['ArrowUp', 'ArrowLeft'])).toBe(10);
    expect(stickValue(['ArrowUp', 'ArrowRight'])).toBe(6);
    expect(stickValue(['ArrowDown', 'ArrowLeft'])).toBe(9);
    expect(stickValue(['ArrowDown', 'ArrowRight'])).toBe(5);
  });

  it('cancels opposite directions', () => {
    expect(stickValue(['ArrowUp', 'ArrowDown'])).toBe(NEUTRAL);
    expect(stickValue(['KeyA', 'KeyD'])).toBe(NEUTRAL);
    expect(stickValue(['KeyQ', 'KeyD'])).toBe(14); // up-left + right = up
  });

  it('ignores other keys', () => {
    expect(stickValue(['KeyS', 'Digit1', 'Enter'])).toBe(NEUTRAL);
    expect(isJoystickKey('KeyS')).toBe(false);
    expect(isJoystickKey('Numpad5')).toBe(false);
  });
});

describe('trigger', () => {
  it.each(['Space', 'Numpad0', 'ControlLeft', 'ControlRight'])('%s is fire', code => {
    expect(triggerPressed([code])).toBe(true);
    expect(joystickRole(code)).toBe('FIRE');
  });

  it('is released with no fire key held', () => {
    expect(triggerPressed(['KeyW', 'ShiftLeft'])).toBe(false);
  });
});

describe('KeyboardJoystick', () => {
  it('reports only changes, so auto-repeat sends nothing', () => {
    const pad = new KeyboardJoystick();
    expect(pad.press('KeyW', true)).toBe(true);
    expect(pad.press('KeyW', true)).toBe(false);
    expect(pad.press('ArrowUp', true)).toBe(false); // still up
    expect(pad.press('KeyW', false)).toBe(false);   // ArrowUp still held
    expect(pad.stick).toBe(14);
    expect(pad.press('ArrowUp', false)).toBe(true);
    expect(pad.stick).toBe(NEUTRAL);
    expect(pad.press('KeyS', true)).toBe(false);
  });

  it('tracks fire separately from direction', () => {
    const pad = new KeyboardJoystick();
    pad.press('KeyC', true);
    expect(pad.press('Space', true)).toBe(true);
    expect([pad.stick, pad.trigger]).toEqual([5, true]);
    expect(pad.clear()).toBe(true);
    expect([pad.stick, pad.trigger]).toEqual([NEUTRAL, false]);
    expect(pad.clear()).toBe(false);
  });
});

describe('emulator keyboard joystick', () => {
  let mock: MockNative;
  let wb: Workbench;

  beforeEach(async () => {
    mock = createMockNative();
    setNative(mock);
    wb = createWorkbench();
    await wb.start();
    wb.emulator.running = true;
  });

  afterEach(() => wb.dispose());

  const joystickCalls = () => mock.calls.filter(call => call.startsWith('EmulatorJoystick'));

  it('leaves keys to the Atari keyboard while off', () => {
    expect(wb.emulator.joystickKey('KeyW', true)).toBe(false);
    expect(joystickCalls()).toEqual([]);
  });

  it('sends STICK and STRIG changes while on', async () => {
    await wb.execute('emulator.joystick');
    expect(wb.emulator.joystickMode).toBe(true);
    expect(wb.emulator.joystickKey('KeyS', true)).toBe(false); // still a letter
    expect(wb.emulator.joystickKey('ArrowUp', true)).toBe(true);
    expect(wb.emulator.joystickKey('ArrowLeft', true)).toBe(true);
    expect(wb.emulator.joystickKey('ArrowLeft', true)).toBe(true); // auto-repeat
    expect(wb.emulator.joystickKey('Space', true)).toBe(true);
    await vi.waitFor(() => expect(joystickCalls().at(-1)).toBe('EmulatorJoystick(10, true)'));
    expect([wb.emulator.stick, wb.emulator.trigger]).toEqual([10, true]);
    // Requests are serialized and coalesced: the newest state always wins.
    expect(joystickCalls().length).toBeLessThanOrEqual(3);
  });

  it('centres the joystick when the mode is turned off', async () => {
    wb.emulator.setJoystickMode(true);
    wb.emulator.joystickKey('KeyD', true);
    await vi.waitFor(() => expect(joystickCalls().at(-1)).toBe('EmulatorJoystick(7, false)'));
    wb.emulator.setJoystickMode(false);
    await vi.waitFor(() => expect(joystickCalls().at(-1)).toBe('EmulatorJoystick(15, false)'));
    expect(wb.emulator.stick).toBe(NEUTRAL);
  });
});
