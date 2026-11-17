// Workbench chrome state: which panels are open, their sizes, the status
// line, the Output channel and modal dialogs. Layout is persisted through
// the backend so it survives restarts.
import { native, errorMessage, type LayoutState } from '../native';

export type Activity = 'explorer' | 'search' | 'atr' | 'source' | 'run' | 'emulator' | 'sprites' | 'manual' | 'settings';
export type PanelTab = 'problems' | 'output' | 'terminal' | 'debug' | 'keyboard';

export const activities: { id: Activity; icon: string; label: string }[] = [
  { id: 'explorer', icon: 'files', label: 'Explorer' },
  { id: 'search', icon: 'search', label: 'Search' },
  { id: 'atr', icon: 'atari-folder', label: 'ATR Explorer' },
  { id: 'source', icon: 'source-control', label: 'Source Control' },
  { id: 'run', icon: 'debug-alt', label: 'Run and Debug' },
  { id: 'emulator', icon: 'device-desktop', label: 'Atari800 Emulator' },
  { id: 'sprites', icon: 'paintcan', label: 'Sprite Studio' },
  { id: 'manual', icon: 'book', label: 'exBASIC Manual' },
];

const activityIds = new Set<string>([...activities.map(item => item.id), 'settings']);
/** Panel height that shows the whole keyboard map with legible keys. */
const KEYBOARD_PANEL_HEIGHT = 360;
const panelTabs = new Set<string>(['problems', 'output', 'terminal', 'debug', 'keyboard']);
const MAX_OUTPUT = 400_000;

export interface DialogButton { value: string; label: string; kind?: 'primary' | 'danger' | 'default' }

export interface DialogRequest {
  title: string;
  message: string;
  detail?: string;
  /** When set, the dialog shows a text field with this initial value. */
  input?: { value: string; label: string; placeholder?: string };
  buttons: DialogButton[];
  /** Value returned by Escape and the backdrop. */
  cancelValue: string;
  resolve(result: { value: string; text: string }): void;
}

export class UIStore {
  activity = $state<Activity>('explorer');
  primaryOpen = $state(true);
  panelOpen = $state(true);
  panelTab = $state<PanelTab>('terminal');
  panelMaximized = $state(false);
  primaryWidth = $state(280);
  panelHeight = $state(240);

  status = $state('Ready');
  /** Labels of tasks in flight; the status bar spins while non-empty. */
  busy = $state<string[]>([]);
  output = $state('Welcome to exBASIC Studio. Open a folder or create a project to begin.\n');

  dialog = $state<DialogRequest | null>(null);
  private queue: DialogRequest[] = [];
  private saveTimer: ReturnType<typeof setTimeout> | undefined;
  private restoreFocus: Element | null = null;

  applyLayout(layout: LayoutState): void {
    if (activityIds.has(layout.activity)) this.activity = layout.activity as Activity;
    if (panelTabs.has(layout.panelTab)) this.panelTab = layout.panelTab as PanelTab;
    this.primaryOpen = layout.primaryOpen;
    this.panelOpen = layout.panelOpen;
    this.primaryWidth = layout.primaryWidth;
    this.panelHeight = layout.panelHeight;
  }

  /** Persists the layout shortly after the last change. */
  persistLayout(): void {
    clearTimeout(this.saveTimer);
    const layout: LayoutState = {
      activity: this.activity, panelTab: this.panelTab, primaryOpen: this.primaryOpen,
      panelOpen: this.panelOpen, primaryWidth: Math.round(this.primaryWidth), panelHeight: Math.round(this.panelHeight),
    };
    this.saveTimer = setTimeout(() => { void native().studio.SaveLayout(layout).catch(() => {}); }, 400);
  }

  showActivity(id: Activity): void {
    this.activity = id;
    this.primaryOpen = true;
    this.persistLayout();
  }

  toggleActivity(id: Activity): void {
    if (this.activity === id && this.primaryOpen) this.primaryOpen = false;
    else { this.activity = id; this.primaryOpen = true; }
    this.persistLayout();
  }

  showPanel(tab: PanelTab): void {
    this.panelTab = tab;
    this.panelOpen = true;
    // The keyboard map is unreadable in a short panel: make room for it once,
    // without taking more than about half of the window.
    if (tab === 'keyboard' && this.panelHeight < KEYBOARD_PANEL_HEIGHT) {
      const room = typeof window === 'undefined' ? KEYBOARD_PANEL_HEIGHT : Math.floor(window.innerHeight * 0.5);
      this.panelHeight = Math.max(this.panelHeight, Math.min(KEYBOARD_PANEL_HEIGHT, room));
    }
    this.persistLayout();
  }

  setPrimaryWidth(width: number): void {
    this.primaryWidth = Math.min(900, Math.max(160, width));
    this.persistLayout();
  }

  setPanelHeight(height: number): void {
    this.panelHeight = Math.min(1200, Math.max(80, height));
    this.persistLayout();
  }

  appendOutput(text: string): void {
    const next = this.output + text;
    this.output = next.length > MAX_OUTPUT ? next.slice(next.length - MAX_OUTPUT) : next;
  }

  clearOutput(): void { this.output = ''; }

  /** Reports a failure in the status bar and the Output channel. */
  fail(label: string, error: unknown): void {
    const message = errorMessage(error);
    this.status = `${label} failed`;
    this.appendOutput(`\n${label} failed: ${message}\n`);
    void native().studio.LogFrontendError(`${label}: ${message}`).catch(() => {});
  }

  /** Runs a task with busy tracking and uniform error reporting. */
  async task<T>(label: string, work: () => Promise<T>, options: { quiet?: boolean; reveal?: boolean } = {}): Promise<T | undefined> {
    this.busy = [...this.busy, label];
    if (!options.quiet) this.status = label + '…';
    try {
      const result = await work();
      if (!options.quiet) this.status = label + ' complete';
      return result;
    } catch (error) {
      this.fail(label, error);
      if (options.reveal !== false) this.showPanel('output');
      return undefined;
    } finally {
      const index = this.busy.indexOf(label);
      if (index >= 0) this.busy = [...this.busy.slice(0, index), ...this.busy.slice(index + 1)];
    }
  }

  // ---- Dialogs -------------------------------------------------------------

  private open(request: Omit<DialogRequest, 'resolve'>): Promise<{ value: string; text: string }> {
    return new Promise(resolve => {
      const entry: DialogRequest = { ...request, resolve };
      if (this.dialog) this.queue.push(entry);
      else {
        this.restoreFocus = typeof document !== 'undefined' ? document.activeElement : null;
        this.dialog = entry;
      }
    });
  }

  /** Called by the dialog component when the user answers. */
  closeDialog(value: string, text = ''): void {
    const active = this.dialog;
    if (!active) return;
    this.dialog = this.queue.shift() ?? null;
    active.resolve({ value, text });
    if (!this.dialog && this.restoreFocus instanceof HTMLElement && this.restoreFocus.isConnected) {
      // Return focus to whatever opened the dialog.
      const target = this.restoreFocus;
      queueMicrotask(() => target.focus());
    }
  }

  async confirm(options: { title: string; message: string; detail?: string; confirmLabel?: string; danger?: boolean }): Promise<boolean> {
    const result = await this.open({
      title: options.title, message: options.message, detail: options.detail, cancelValue: 'cancel',
      buttons: [
        { value: 'cancel', label: 'Cancel' },
        { value: 'confirm', label: options.confirmLabel ?? 'OK', kind: options.danger ? 'danger' : 'primary' },
      ],
    });
    return result.value === 'confirm';
  }

  async prompt(options: { title: string; message: string; label: string; value?: string; placeholder?: string; confirmLabel?: string }): Promise<string | null> {
    const result = await this.open({
      title: options.title, message: options.message, cancelValue: 'cancel',
      input: { value: options.value ?? '', label: options.label, placeholder: options.placeholder },
      buttons: [{ value: 'cancel', label: 'Cancel' }, { value: 'confirm', label: options.confirmLabel ?? 'OK', kind: 'primary' }],
    });
    return result.value === 'confirm' ? result.text : null;
  }

  async choose(options: { title: string; message: string; detail?: string; buttons: DialogButton[]; cancelValue: string }): Promise<string> {
    return (await this.open(options)).value;
  }
}
