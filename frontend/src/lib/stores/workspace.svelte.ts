// The open workspace: project view, trust, recent projects and file
// operations. External changes arrive as watcher batches and are applied
// incrementally so the explorer never needs a full rescan.
import { native, type AppSnapshot, type Environment, type ProjectConfig, type ProjectFile, type ProjectView, type RecentProject, type TrustInfo, type WorkspaceBatch } from '../native';
import type { UIStore } from './ui.svelte';

export interface TreeEntry { path: string; name: string; depth: number; directory: boolean; size: number }

export function baseName(path: string): string { return path.split('/').pop() ?? path; }
export function parentOf(path: string): string { const slash = path.lastIndexOf('/'); return slash < 0 ? '' : path.slice(0, slash); }
export function normalizeRel(path: string): string { return path.trim().replaceAll('\\', '/').replace(/^\.\//, '').replace(/\/+$/, ''); }

export function fileIcon(path: string): string {
  const lower = path.toLowerCase();
  if (lower.endsWith('.bas')) return 'symbol-method';
  if (lower.endsWith('.asm') || lower.endsWith('.s')) return 'symbol-namespace';
  if (lower.endsWith('.sprites.json')) return 'paintcan';
  if (lower.endsWith('.pmg')) return 'symbol-color';
  if (/\.(png|jpe?g|gif|webp|bmp|svg|ico|avif)$/.test(lower)) return 'file-media';
  if (lower.endsWith('.json')) return 'json';
  if (/\.(md|markdown|mdown|mkd)$/.test(lower)) return 'markdown';
  if (/\.(mmd|mermaid)$/.test(lower)) return 'type-hierarchy-sub';
  if (/\.(js|mjs|cjs|jsx|ts|tsx)$/.test(lower)) return 'symbol-key';
  if (/\.(html?|css|scss|less|xml|ya?ml)$/.test(lower)) return 'code';
  if (/\.(txt|log|ini|cfg|conf|toml|env)$/.test(lower)) return 'file-text';
  if (lower.endsWith('.atr')) return 'atari-folder';
  return 'file';
}

export class WorkspaceStore {
  env = $state<Environment | null>(null);
  project = $state<ProjectView | null>(null);
  trust = $state<TrustInfo>({ trusted: false, tools: [] });
  recents = $state<RecentProject[]>([]);
  collapsed = $state(new Set<string>());
  selected = $state('');
  /** Saved per-workspace state loaded when the project opened. */
  saved = $state<AppSnapshot['workspace'] | null>(null);

  /** Called after a project is opened/closed so other stores can react. */
  onProjectChanged: ((view: ProjectView | null) => Promise<void> | void) | undefined;
  /** Called with external changes to files (not directories). */
  onFilesChanged: ((batch: WorkspaceBatch) => void) | undefined;

  private unsubscribe: (() => void) | undefined;

  /** Application preferences as stored; read once at startup. */
  storedPreferences: Record<string, string | undefined> = {};

  constructor(private ui: UIStore) {}

  get root(): string { return this.project?.root ?? ''; }
  get restricted(): boolean { return !!this.project && !this.trust.trusted; }

  async init(): Promise<void> {
    this.unsubscribe?.();
    this.unsubscribe = native().on('workspace-changed', batch => this.applyBatch(batch));
    this.env = await native().studio.GetEnvironment();
    const snapshot = await native().studio.AppState();
    this.recents = snapshot.recentProjects ?? [];
    this.ui.applyLayout(snapshot.layout);
    this.storedPreferences = snapshot.preferences ?? {};
    if (snapshot.notice) this.ui.appendOutput(snapshot.notice + '\n');
    if (this.env?.toolsNote) this.ui.appendOutput(this.env.toolsNote + '\n');
  }

  dispose(): void { this.unsubscribe?.(); this.unsubscribe = undefined; }

  private async adopt(view: ProjectView): Promise<void> {
    this.project = view;
    // Keep large dependency and dot trees cheap to render while still showing
    // every top-level entry. Revealing the active file expands only its path.
    const collapsed = new Set(view.directories ?? []);
    for (let current = parentOf(view.config.entry); current; current = parentOf(current)) collapsed.delete(current);
    this.collapsed = collapsed;
    this.selected = '';
    const snapshot = await native().studio.AppState();
    this.recents = snapshot.recentProjects ?? [];
    this.saved = snapshot.workspace;
    this.trust = await native().studio.WorkspaceTrust();
    this.ui.appendOutput(`Opened ${view.root}${this.trust.trusted ? '' : ' (restricted mode)'}\n`);
    await this.onProjectChanged?.(view);
  }

  /** True while a folder is being opened or created; only one at a time. */
  opening = $state(false);

  async open(path: string): Promise<boolean> {
    if (this.opening) return false;
    this.opening = true;
    try {
      const view = await this.ui.task('Opening folder', () => native().studio.OpenProject(path));
      if (!view) return false;
      await this.adopt(view);
      return true;
    } finally { this.opening = false; }
  }

  async create(parent: string, name: string): Promise<boolean> {
    if (this.opening) return false;
    this.opening = true;
    try {
      const view = await this.ui.task('Creating project', () => native().studio.CreateProject(parent, name));
      if (!view) return false;
      await this.adopt(view);
      return true;
    } finally { this.opening = false; }
  }

  async close(): Promise<void> {
    await this.ui.task('Closing folder', () => native().studio.CloseProject(), { quiet: true });
    this.project = null;
    this.saved = null;
    this.trust = { trusted: false, tools: [] };
    await this.onProjectChanged?.(null);
  }

  async refresh(): Promise<void> {
    if (!this.project) return;
    const view = await this.ui.task('Refreshing explorer', () => native().studio.GetProject(), { quiet: true });
    if (view) this.project = view;
  }

  async forgetRecent(path: string): Promise<void> {
    const snapshot = await this.ui.task('Updating recent projects', () => native().studio.RemoveRecentProject(path), { quiet: true });
    if (snapshot) this.recents = snapshot.recentProjects ?? [];
  }

  // ---- Trust ---------------------------------------------------------------

  /** Asks the user to trust the workspace; resolves true when trusted. */
  async requestTrust(reason: string): Promise<boolean> {
    if (!this.project) return false;
    if (this.trust.trusted) return true;
    const tools = (this.trust.tools ?? []).filter(Boolean).join(', ');
    const answer = await this.ui.choose({
      title: 'Do you trust this folder?',
      message: `${this.project.root}`,
      detail: `Trusting it lets exBASIC Studio ${reason}. That runs programs chosen by this folder's project settings (${tools || 'the bundled tools'}), Git hooks, and anything you start in the terminal. Only trust folders whose contents you know. You can keep browsing and editing without trusting.`,
      buttons: [{ value: 'cancel', label: 'Stay in Restricted Mode' }, { value: 'trust', label: 'Trust Folder', kind: 'primary' }],
      cancelValue: 'cancel',
    });
    if (answer !== 'trust') return false;
    const trust = await this.ui.task('Trusting folder', () => native().studio.TrustWorkspace(), { quiet: true });
    if (trust) this.trust = trust;
    return this.trust.trusted;
  }

  async revokeTrust(): Promise<void> {
    const trust = await this.ui.task('Returning to restricted mode', () => native().studio.RevokeWorkspaceTrust(), { quiet: true });
    if (trust) this.trust = trust;
  }

  async reloadTrust(): Promise<void> {
    if (this.project) this.trust = await native().studio.WorkspaceTrust();
  }

  // ---- Settings ------------------------------------------------------------

  async saveConfig(config: ProjectConfig): Promise<boolean> {
    const view = await this.ui.task('Saving settings', () => native().studio.UpdateConfig(config));
    if (!view) return false;
    this.project = view;
    const wasTrusted = this.trust.trusted;
    await this.reloadTrust();
    if (wasTrusted && !this.trust.trusted) {
      this.ui.appendOutput('A tool path changed, so this folder is back in restricted mode until you trust it again.\n');
    }
    return true;
  }

  // ---- Explorer tree -------------------------------------------------------

  get tree(): TreeEntry[] {
    const project = this.project;
    if (!project) return [];
    const directories = new Set<string>();
    const addDirectory = (path: string) => {
      for (let current = path; current; current = parentOf(current)) directories.add(current);
    };
    for (const directory of project.directories ?? []) addDirectory(directory);
    for (const file of project.files ?? []) if (file.path.includes('/')) addDirectory(parentOf(file.path));
    const childDirectories = new Map<string, string[]>();
    const childFiles = new Map<string, ProjectFile[]>();
    for (const directory of directories) {
      const parent = parentOf(directory);
      childDirectories.set(parent, [...(childDirectories.get(parent) ?? []), directory]);
    }
    for (const file of project.files ?? []) {
      const parent = parentOf(file.path);
      childFiles.set(parent, [...(childFiles.get(parent) ?? []), file]);
    }
    const entries: TreeEntry[] = [];
    const byName = (a: string, b: string) => baseName(a).localeCompare(baseName(b), undefined, { sensitivity: 'base' });
    const walk = (parent: string, depth: number) => {
      for (const directory of (childDirectories.get(parent) ?? []).sort(byName)) {
        entries.push({ path: directory, name: baseName(directory), depth, directory: true, size: 0 });
        if (!this.collapsed.has(directory)) walk(directory, depth + 1);
      }
      for (const file of (childFiles.get(parent) ?? []).sort((a, b) => byName(a.path, b.path))) {
        entries.push({ path: file.path, name: baseName(file.path), depth, directory: false, size: file.size });
      }
    };
    walk('', 0);
    return entries;
  }

  toggleFolder(path: string): void {
    const next = new Set(this.collapsed);
    if (next.has(path)) next.delete(path); else next.add(path);
    this.collapsed = next;
  }

  collapseAll(): void {
    this.collapsed = new Set(this.project?.directories ?? []);
  }

  /** Makes a path visible without disturbing the expansion state of unrelated folders. */
  reveal(path: string): void {
    const next = new Set(this.collapsed);
    for (let current = parentOf(path); current; current = parentOf(current)) next.delete(current);
    this.collapsed = next;
    this.selected = path;
  }

  hasFile(path: string): boolean {
    const lower = path.toLowerCase();
    return !!this.project?.files?.some(file => file.path.toLowerCase() === lower);
  }

  hasDirectory(path: string): boolean {
    const lower = path.toLowerCase();
    return !!this.project?.directories?.some(directory => directory.toLowerCase() === lower);
  }

  // ---- External changes ----------------------------------------------------

  /** Applies a watcher batch to the file list without rescanning the disk. */
  applyBatch(batch: WorkspaceBatch): void {
    const project = this.project;
    if (!project || batch.root !== project.root) return;
    if (batch.overflow) { void this.refresh(); this.onFilesChanged?.(batch); return; }
    let files = project.files ?? [];
    let directories = project.directories ?? [];
    let changed = false;
    for (const change of batch.changes) {
      if (change.kind === 'remove') {
        const prefix = change.path + '/';
        const keptFiles = files.filter(file => file.path !== change.path && !file.path.startsWith(prefix));
        const keptDirectories = directories.filter(directory => directory !== change.path && !directory.startsWith(prefix));
        changed ||= keptFiles.length !== files.length || keptDirectories.length !== directories.length;
        files = keptFiles; directories = keptDirectories;
      } else if (change.directory) {
        if (!directories.includes(change.path)) { directories = [...directories, change.path].sort(); changed = true; }
      } else {
        const index = files.findIndex(file => file.path === change.path);
        if (index < 0) { files = [...files, { path: change.path, size: change.size }].sort((a, b) => a.path.localeCompare(b.path)); changed = true; }
        else if (files[index].size !== change.size) { files = files.map((file, i) => (i === index ? { ...file, size: change.size } : file)); changed = true; }
      }
    }
    if (changed) this.project = { ...project, files, directories };
    this.onFilesChanged?.(batch);
  }

  // ---- File operations -----------------------------------------------------

  async createFile(path: string, content: string): Promise<boolean> {
    const rel = normalizeRel(path);
    if (!rel) return false;
    if (this.hasFile(rel) || this.hasDirectory(rel)) { this.ui.fail('Creating file', `${rel} already exists`); return false; }
    const ok = await this.ui.task('Creating file', async () => { await native().studio.SaveFile(rel, content); return true; });
    if (ok) await this.refresh();
    return !!ok;
  }

  async createFolder(path: string): Promise<boolean> {
    const rel = normalizeRel(path);
    if (!rel) return false;
    if (this.hasFile(rel) || this.hasDirectory(rel)) { this.ui.fail('Creating folder', `${rel} already exists`); return false; }
    const view = await this.ui.task('Creating folder', () => native().studio.CreateFolder(rel));
    if (view) this.project = view;
    return !!view;
  }

  async rename(oldPath: string, newPath: string): Promise<boolean> {
    const target = normalizeRel(newPath);
    if (!target || target === oldPath) return false;
    const view = await this.ui.task('Renaming', () => native().studio.RenamePath(oldPath, target));
    if (!view) return false;
    this.project = view;
    this.selected = target;
    return true;
  }

  async remove(path: string): Promise<boolean> {
    const view = await this.ui.task('Deleting', () => native().studio.DeletePath(path));
    if (!view) return false;
    this.project = view;
    if (this.selected === path || this.selected.startsWith(path + '/')) this.selected = '';
    return true;
  }

  async importPaths(destination: string, paths: string[]): Promise<void> {
    if (!this.project || !paths.length) return;
    const view = await this.ui.task('Importing into project', () => native().studio.ImportProjectPaths(destination || '.', paths));
    if (view) this.project = view;
  }

  async download(path: string, destination: string): Promise<boolean> {
    const ok = await this.ui.task('Downloading file', async () => {
      await native().studio.ExportProjectFile(path, destination);
      return true;
    });
    return !!ok;
  }
}
