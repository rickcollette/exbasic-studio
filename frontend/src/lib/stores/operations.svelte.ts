// Tracks long-running backend operations (builds, runs, Git network
// commands). Each operation delivers ordered events ending in exactly one
// completion; this store orders them, streams their output and lets callers
// await the result.
import { native, type OperationEvent, type OperationInfo } from '../native';
import type { UIStore } from './ui.svelte';

interface Tracked {
  info: OperationInfo;
  label: string;
  lastSeq: number;
  /** Events that arrived out of order, keyed by sequence number. */
  held: Map<number, OperationEvent>;
  resolve?: (event: OperationEvent) => void;
  completion?: OperationEvent;
}

export class OperationsStore {
  /** Operations that have started and not yet completed. */
  running = $state<{ id: string; kind: string; label: string }[]>([]);

  private tracked = new Map<string, Tracked>();
  /** Events for operations whose start call has not returned yet. */
  private early = new Map<string, OperationEvent[]>();
  private unsubscribe: (() => void) | undefined;

  constructor(private ui: UIStore) {}

  connect(): void {
    this.unsubscribe?.();
    this.unsubscribe = native().on('operation', event => this.receive(event));
  }

  disconnect(): void {
    this.unsubscribe?.();
    this.unsubscribe = undefined;
  }

  isRunning(kind: string): boolean {
    return this.running.some(item => item.kind === kind);
  }

  private receive(event: OperationEvent): void {
    const tracked = this.tracked.get(event.id);
    if (!tracked) {
      // The backend can emit before the call that started the operation
      // has returned its ID to us; keep those events until it does.
      const list = this.early.get(event.id) ?? [];
      list.push(event);
      this.early.set(event.id, list);
      if (this.early.size > 32) this.early.delete(this.early.keys().next().value!);
      return;
    }
    if (event.seq <= tracked.lastSeq) return; // duplicate
    tracked.held.set(event.seq, event);
    // Apply strictly in sequence so output is never interleaved wrongly.
    for (let next = tracked.held.get(tracked.lastSeq + 1); next; next = tracked.held.get(tracked.lastSeq + 1)) {
      tracked.held.delete(next.seq);
      tracked.lastSeq = next.seq;
      this.apply(tracked, next);
    }
  }

  private apply(tracked: Tracked, event: OperationEvent): void {
    if (event.type === 'output' && event.text) this.ui.appendOutput(event.text);
    if (event.type !== 'completed') return;
    tracked.completion = event;
    this.running = this.running.filter(item => item.id !== event.id);
    this.tracked.delete(event.id);
    tracked.resolve?.(event);
  }

  /**
   * Starts an operation and resolves with its completion event. `start` is
   * the backend call returning the operation's ID.
   */
  async run(label: string, start: () => Promise<OperationInfo>): Promise<OperationEvent> {
    const info = await start();
    const tracked: Tracked = { info, label, lastSeq: 0, held: new Map() };
    this.tracked.set(info.id, tracked);
    this.running = [...this.running, { id: info.id, kind: info.kind, label }];
    const done = new Promise<OperationEvent>(resolve => { tracked.resolve = resolve; });
    const early = this.early.get(info.id);
    if (early) {
      this.early.delete(info.id);
      for (const event of early.sort((a, b) => a.seq - b.seq)) this.receive(event);
    }
    return tracked.completion ?? done;
  }

  async cancel(kind: string): Promise<void> {
    for (const item of this.running.filter(entry => entry.kind === kind)) {
      await native().studio.CancelOperation(item.id);
    }
  }
}
