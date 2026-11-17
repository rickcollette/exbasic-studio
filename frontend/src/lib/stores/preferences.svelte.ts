// Application preferences: appearance, editor behaviour and startup. They
// are stored per user in the application state (never in a project) as
// plain strings, so an unknown or damaged value falls back to its default.
import { native } from '../native';

export type ThemeChoice = 'system' | 'dark' | 'light' | 'high-contrast';
export type SyntaxScheme = 'bright' | 'classic' | 'retro' | 'soft';
export type AutoSave = 'off' | 'delay' | 'focus';
export type Startup = 'reopen' | 'welcome';

export interface Preferences {
  theme: ThemeChoice;
  syntax: SyntaxScheme;
  fontSize: number;
  autoSave: AutoSave;
  /** Milliseconds of inactivity before an automatic save. */
  autoSaveDelay: number;
  startup: Startup;
}

export const defaults: Preferences = { theme: 'dark', syntax: 'bright', fontSize: 13, autoSave: 'off', autoSaveDelay: 1000, startup: 'reopen' };

export const themeChoices: { id: ThemeChoice; label: string }[] = [
  { id: 'system', label: 'Follow the system' },
  { id: 'dark', label: 'Dark' },
  { id: 'light', label: 'Light' },
  { id: 'high-contrast', label: 'High contrast' },
];

export const syntaxSchemes: { id: SyntaxScheme; label: string }[] = [
  { id: 'bright', label: 'Bright' },
  { id: 'classic', label: 'Classic' },
  { id: 'soft', label: 'Soft' },
  { id: 'retro', label: 'Retro phosphor' },
];

export const autoSaveChoices: { id: AutoSave; label: string }[] = [
  { id: 'off', label: 'Off' },
  { id: 'delay', label: 'After a pause in typing' },
  { id: 'focus', label: 'When the editor or window loses focus' },
];

export const startupChoices: { id: Startup; label: string }[] = [
  { id: 'reopen', label: 'Reopen the last folder' },
  { id: 'welcome', label: 'Show the welcome page' },
];

export const fontSizes = [11, 12, 13, 14, 15, 16, 18, 20, 24];
export const autoSaveDelays = [500, 1000, 2000, 5000];

function oneOf<T extends string>(value: string | undefined, allowed: readonly { id: T }[], fallback: T): T {
  return allowed.some(item => item.id === value) ? (value as T) : fallback;
}

function numberOf(value: string | undefined, allowed: readonly number[], fallback: number): number {
  const parsed = Number(value);
  return allowed.includes(parsed) ? parsed : fallback;
}

/** Reads preferences from their stored form, replacing anything invalid. */
export function parsePreferences(stored: Record<string, string | undefined> | null | undefined): Preferences {
  const map = stored ?? {};
  return {
    theme: oneOf(map.theme, themeChoices, defaults.theme),
    syntax: oneOf(map.syntax, syntaxSchemes, defaults.syntax),
    fontSize: numberOf(map.fontSize, fontSizes, defaults.fontSize),
    autoSave: oneOf(map.autoSave, autoSaveChoices, defaults.autoSave),
    autoSaveDelay: numberOf(map.autoSaveDelay, autoSaveDelays, defaults.autoSaveDelay),
    startup: oneOf(map.startup, startupChoices, defaults.startup),
  };
}

export class PreferencesStore {
  values = $state<Preferences>({ ...defaults });
  /** What the operating system asks for; used by the "system" theme. */
  systemDark = $state(true);
  private media: MediaQueryList | undefined;
  private mediaListener = (event: MediaQueryListEvent) => { this.systemDark = event.matches; };

  /** The theme actually shown. */
  get theme(): 'dark' | 'light' | 'high-contrast' {
    if (this.values.theme === 'system') return this.systemDark ? 'dark' : 'light';
    return this.values.theme;
  }

  load(stored: Record<string, string | undefined> | null | undefined): void {
    this.values = parsePreferences(stored);
    if (!this.media && typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      this.media = window.matchMedia('(prefers-color-scheme: dark)');
      this.systemDark = this.media.matches;
      this.media.addEventListener?.('change', this.mediaListener);
    }
  }

  dispose(): void {
    this.media?.removeEventListener?.('change', this.mediaListener);
    this.media = undefined;
  }

  /** Changes one preference and stores it; a default value is stored as absent. */
  set<K extends keyof Preferences>(key: K, value: Preferences[K]): void {
    const next = parsePreferences({ ...this.stored(), [key]: String(value) });
    this.values = next;
    const kept = next[key];
    void native().studio.SetPreference(key, kept === defaults[key] ? '' : String(kept)).catch(() => {});
  }

  private stored(): Record<string, string> {
    return Object.fromEntries(Object.entries(this.values).map(([key, value]) => [key, String(value)]));
  }
}
