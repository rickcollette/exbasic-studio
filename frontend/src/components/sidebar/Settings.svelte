<script lang="ts">
  // Project settings (written to exbasic.project.json only on Save), folder
  // trust, application preferences and privacy controls.
  import { wb } from '../../lib/workbench.svelte';
  import { native, type ProjectConfig } from '../../lib/native';
  import { themeChoices, syntaxSchemes, autoSaveChoices, startupChoices, fontSizes, autoSaveDelays, type ThemeChoice, type SyntaxScheme, type AutoSave, type Startup } from '../../lib/stores/preferences.svelte';

  const prefs = wb.prefs;
  const pick = (event: Event) => (event.currentTarget as HTMLSelectElement).value;

  let draft = $state<ProjectConfig | null>(null);
  let loadedFor = '';

  // Reload the draft when a different project (or a saved config) arrives.
  $effect(() => {
    const project = wb.workspace.project;
    const key = project ? project.root + JSON.stringify(project.config) : '';
    if (key === loadedFor) return;
    loadedFor = key;
    draft = project ? structuredClone($state.snapshot(project.config)) as ProjectConfig : null;
  });

  const changed = $derived(!!draft && !!wb.workspace.project && JSON.stringify(draft) !== JSON.stringify(wb.workspace.project.config));
  const toolsChanged = $derived(!!draft && !!wb.workspace.project && (draft.compiler !== wb.workspace.project.config.compiler || draft.emulator !== wb.workspace.project.config.emulator || draft.atr.tool !== wb.workspace.project.config.atr.tool));

  async function save() {
    if (!draft) return;
    if (toolsChanged && wb.workspace.trust.trusted) {
      const ok = await wb.ui.confirm({ title: 'Change tool paths?', message: 'Changing a compiler, emulator or ATR tool path returns this folder to restricted mode.', detail: 'You will be asked to trust the folder again before those programs can run.', confirmLabel: 'Save Settings' });
      if (!ok) return;
    }
    await wb.workspace.saveConfig($state.snapshot(draft) as ProjectConfig);
  }

  async function clearRecovery() {
    if (!(await wb.ui.confirm({ title: 'Delete recovery data?', message: 'Delete all crash-recovery copies of unsaved files?', detail: 'Unsaved changes in open editors are not affected, but they can no longer be restored after a crash until you edit them again.', confirmLabel: 'Delete', danger: true }))) return;
    await wb.ui.task('Deleting recovery data', () => native().studio.ClearRecoveryData());
  }

  const list = (value: string) => value.split(',').map(item => item.trim()).filter(Boolean);
</script>

<div class="settings">
  <h3>Appearance</h3>
  <label for="pref-theme">Theme</label>
  <select id="pref-theme" value={prefs.values.theme} onchange={event => prefs.set('theme', pick(event) as ThemeChoice)}>
    {#each themeChoices as choice}<option value={choice.id}>{choice.label}</option>{/each}
  </select>
  <label for="pref-syntax">Syntax colours</label>
  <select id="pref-syntax" value={prefs.values.syntax} disabled={prefs.theme === 'high-contrast'} onchange={event => prefs.set('syntax', pick(event) as SyntaxScheme)}>
    {#each syntaxSchemes as scheme}<option value={scheme.id}>{scheme.label}</option>{/each}
  </select>
  <pre class="syntax-sample" aria-label="Syntax colour sample"><span style="color:var(--syntax-comment);font-style:italic">' Count to ten</span>
<span style="color:var(--syntax-control);font-weight:600">FOR</span> <span style="color:var(--syntax-variable)">I</span> <span style="color:var(--syntax-operator)">=</span> <span style="color:var(--syntax-number)">1</span> <span style="color:var(--syntax-control);font-weight:600">TO</span> <span style="color:var(--syntax-number)">10</span>
  <span style="color:var(--syntax-keyword);font-weight:600">PRINT</span> <span style="color:var(--syntax-string)">"N="</span><span style="color:var(--syntax-operator)">;</span> <span style="color:var(--syntax-function)">PEEK</span><span style="color:var(--syntax-operator)">(</span><span style="color:var(--syntax-variable)">I</span><span style="color:var(--syntax-operator)">)</span>
<span style="color:var(--syntax-control);font-weight:600">NEXT</span> <span style="color:var(--syntax-variable)">I</span></pre>
  <label for="pref-font">Editor font size</label>
  <select id="pref-font" value={prefs.values.fontSize} onchange={event => prefs.set('fontSize', Number(pick(event)))}>
    {#each fontSizes as size}<option value={size}>{size} px</option>{/each}
  </select>

  <h3>Editor</h3>
  <label for="pref-autosave">Save automatically</label>
  <select id="pref-autosave" value={prefs.values.autoSave} onchange={event => prefs.set('autoSave', pick(event) as AutoSave)}>
    {#each autoSaveChoices as choice}<option value={choice.id}>{choice.label}</option>{/each}
  </select>
  {#if prefs.values.autoSave === 'delay'}
    <label for="pref-autosave-delay">Pause before saving</label>
    <select id="pref-autosave-delay" value={prefs.values.autoSaveDelay} onchange={event => prefs.set('autoSaveDelay', Number(pick(event)))}>
      {#each autoSaveDelays as delay}<option value={delay}>{delay / 1000} s</option>{/each}
    </select>
  {/if}

  <h3>Startup</h3>
  <label for="pref-startup">When exBASIC Studio starts</label>
  <select id="pref-startup" value={prefs.values.startup} onchange={event => prefs.set('startup', pick(event) as Startup)}>
    {#each startupChoices as choice}<option value={choice.id}>{choice.label}</option>{/each}
  </select>
  <h3>Privacy</h3>
  <p class="hint">exBASIC Studio sends no telemetry and uploads nothing. Diagnostic logs and crash-recovery copies stay on this computer.</p>
  <button onclick={clearRecovery}><i aria-hidden="true" class="codicon codicon-trash"></i> Delete Crash-Recovery Data</button>
  <button onclick={() => wb.execute('support.bundle')}><i aria-hidden="true" class="codicon codicon-archive"></i> Export Support Bundle…</button>
  {#if wb.workspace.recents.length}<button onclick={() => wb.workspace.forgetRecent('')}><i aria-hidden="true" class="codicon codicon-clear-all"></i> Clear Recent Projects</button>{/if}

  {#if draft && wb.workspace.project}
    <h3>Folder Trust</h3>
    <p class="hint">{wb.workspace.trust.trusted ? 'This folder is trusted: its configured tools, the terminal and Git hooks may run.' : 'Restricted mode: nothing from this folder is executed.'}</p>
    <button onclick={() => wb.execute('trust')}><i aria-hidden="true" class="codicon codicon-shield"></i> {wb.workspace.trust.trusted ? 'Return to Restricted Mode' : 'Trust Folder…'}</button>

    <h3>Project</h3>
    {#if !wb.workspace.project.hasConfig}<p class="hint">This folder has no exbasic.project.json yet. Saving creates it.</p>{/if}
    <label for="entry-setting">Entry source</label><input id="entry-setting" bind:value={draft.entry} autocomplete="off" spellcheck="false" />
    <label for="output-setting">Output file</label><input id="output-setting" bind:value={draft.output} autocomplete="off" spellcheck="false" />
    <label for="target-setting">Target</label>
    <select id="target-setting" bind:value={draft.target}><option value="atari">Atari executable (.XEX)</option><option value="atari-cart">8 KB cartridge (.ROM)</option></select>
    <label for="native-setting">Machine code</label>
    <select id="native-setting" bind:value={draft.native}><option value="auto">Automatic (smallest)</option><option value="hot">Hot paths only</option><option value="all">All code</option><option value="none">Bytecode only</option></select>
    <label class="check"><input type="checkbox" bind:checked={draft.debug} /> Debug symbols (needed for source-level debugging)</label>
    <label for="include-setting">Include paths (comma-separated)</label>
    <input id="include-setting" value={(draft.includePaths ?? []).join(', ')} oninput={event => (draft!.includePaths = list((event.currentTarget as HTMLInputElement).value))} autocomplete="off" spellcheck="false" />
    <label for="assembly-setting">Extra assembly files (comma-separated)</label>
    <input id="assembly-setting" value={(draft.assembly ?? []).join(', ')} oninput={event => (draft!.assembly = list((event.currentTarget as HTMLInputElement).value))} autocomplete="off" spellcheck="false" />

    <h3>Tools</h3>
    <p class="hint">Leave the defaults to use the tools bundled with exBASIC Studio. Changing a path revokes folder trust.</p>
    <label for="compiler-setting">exBASIC compiler</label><input id="compiler-setting" bind:value={draft.compiler} autocomplete="off" spellcheck="false" />
    <label for="emulator-setting">Atari800 helper</label><input id="emulator-setting" bind:value={draft.emulator} autocomplete="off" spellcheck="false" />

    <h3>ATR Disk Image</h3>
    <label class="check"><input type="checkbox" bind:checked={draft.atr.enabled} /> Package builds into an ATR image</label>
    {#if draft.atr.enabled}
      <label for="atr-filesystem-setting">Filesystem</label>
      <select id="atr-filesystem-setting" bind:value={draft.atr.filesystem} onchange={() => { if (draft?.atr.filesystem === 'eos') draft.atr.boot = false; }}>
        <option value="sparta">SpartaDOS / BW-DOS (SDFS)</option><option value="eos">ExtendedOS (EOSFS)</option>
      </select>
      <label for="atr-output-setting">ATR output file</label><input id="atr-output-setting" bind:value={draft.atr.output} autocomplete="off" spellcheck="false" />
      <label for="atr-tool-setting">mkatr-go executable</label><input id="atr-tool-setting" bind:value={draft.atr.tool} autocomplete="off" spellcheck="false" />
      <label class="check"><input type="checkbox" bind:checked={draft.atr.boot} disabled={draft.atr.filesystem === 'eos'} /> Boot the compiled program</label>
      {#if draft.atr.filesystem === 'eos'}<p class="hint">EOSFS images are mounted under ExtendedOS; the SDFS boot loader cannot start them.</p>{/if}
      <label class="check"><input type="checkbox" bind:checked={draft.atr.exactSize} /> Use the exact sector count</label>
      <label for="atr-size-setting">Minimum image size in bytes (0 = automatic)</label><input id="atr-size-setting" type="number" min="0" max="16776576" bind:value={draft.atr.minimumSize} />
      <label for="atr-page-setting">Boot loader page (4–239)</label><input id="atr-page-setting" type="number" min="4" max="239" bind:value={draft.atr.bootPage} />
      <label for="atr-files-setting">Additional project files (comma-separated)</label>
      <input id="atr-files-setting" value={(draft.atr.files ?? []).join(', ')} oninput={event => (draft!.atr.files = list((event.currentTarget as HTMLInputElement).value))} autocomplete="off" spellcheck="false" />
    {/if}
    <button class="primary wide" disabled={!changed} onclick={save}>Save Settings</button>
  {:else}
    <div class="empty-state">Open a folder to configure its project settings.</div>
  {/if}
  <p class="hint version">exBASIC Studio {wb.workspace.env?.version ?? ''} · {wb.workspace.env?.platform ?? ''}</p>
</div>
