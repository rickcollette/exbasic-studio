<script lang="ts">
  // The Atari800 screen with its toolbar and profile editor. Keyboard input
  // goes to the emulated machine while the screen has focus.
  import { wb } from '../../lib/workbench.svelte';
  import { native } from '../../lib/native';

  const emulator = wb.emulator;
  let stage = $state<HTMLElement>();

  // Turning the keyboard joystick on is followed by play: give the keys back
  // to the screen instead of leaving them on the toolbar button.
  function toggleJoystick() {
    emulator.setJoystickMode(!emulator.joystickMode);
    stage?.focus();
  }
  let args = $state('');
  $effect(() => { args = (emulator.draft?.arguments ?? []).join(', '); });

  // Keys the workbench keeps for itself even while the screen has focus.
  const reserved = new Set(['F5', 'F6', 'F8', 'F9', 'F10', 'F11']);

  function key(event: KeyboardEvent, down: boolean) {
    if (!emulator.running || event.ctrlKey && event.shiftKey || (wb.debug.active && reserved.has(event.key))) return;
    if (event.key === 'Tab' && !down) return;
    emulator.markHeld(event.code, down);
    if (emulator.joystickKey(event.code, down)) {
      event.preventDefault();
      event.stopPropagation();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    void emulator.key(event.key, down);
  }

  async function choose(title: string, pattern: string, name: string, apply: (path: string) => void) {
    const selected = await native().dialogs.open({ title, directory: wb.workspace.root || undefined, filters: [{ name, pattern }] });
    if (selected[0] && emulator.draft) apply(selected[0]);
  }

  async function chooseHostDir() {
    const selected = await native().dialogs.open({ title: 'Select the H: Folder', directory: emulator.draft?.hostDir || wb.workspace.root || undefined, chooseDirectories: true, chooseFiles: false });
    if (selected[0] && emulator.draft) emulator.draft.hostDir = selected[0];
  }

  // The folder shown beside the device name while the machine runs.
  const hostFolder = $derived(emulator.profiles.find(profile => profile.id === emulator.profileId)?.hostDir || '');

  // Arrow glyphs for the joystick indicator, indexed by STICK() value.
  const stickGlyphs: Record<number, string> = { 15: '·', 14: '↑', 13: '↓', 11: '←', 7: '→', 10: '↖', 6: '↗', 9: '↙', 5: '↘' };
  const joystickHelp = 'Keyboard joystick (joystick 0): Q W E / A D / Z X C, the arrow keys or the numeric keypad move; Space, Numpad 0 or Ctrl fire. Other keys still go to the Atari keyboard.';

  function capture() {
    if (!emulator.image) return;
    const link = document.createElement('a');
    link.href = emulator.image;
    link.download = `atari800-frame-${emulator.frame}.png`;
    link.click();
  }

  async function removeProfile() {
    if (await wb.ui.confirm({ title: 'Delete profile?', message: `Delete the emulator profile ${emulator.draft?.name}?`, confirmLabel: 'Delete', danger: true })) await emulator.deleteProfile();
  }

  const cartridgeSlots = [{ key: 'cartridge', label: 'Primary slot' }, { key: 'cartridge2', label: 'Secondary slot' }] as const;

  // Atari800 cartridge type numbers for images without a .car header.
  const cartridgeTypes = [
    { id: 0, name: 'Automatic' },
    { id: 1, name: 'Standard 8K' },
    { id: 2, name: 'Standard 16K' },
    { id: 11, name: 'SpartaDOS X 64K' },
    { id: 43, name: 'SpartaDOS X 128K' },
    { id: 3, name: 'OSS two-chip 16K (034M)' },
    { id: 15, name: 'OSS one-chip 16K' },
    { id: 12, name: 'XEGS 32K' },
    { id: 13, name: 'XEGS 64K' },
    { id: 14, name: 'XEGS 128K' },
    { id: 8, name: 'Williams 64K' },
    { id: 41, name: 'Atarimax 1 Mbit flash' },
    { id: 42, name: 'Atarimax 8 Mbit flash' },
    { id: 26, name: 'MegaCart 16K' },
    { id: 32, name: 'MegaCart 1024K' },
  ];

  function saveArgs() { if (emulator.draft) emulator.draft.arguments = args.split(',').map(item => item.trim()).filter(Boolean); }
</script>

<div class="emulator-view">
  <div class="emulator-toolbar" role="toolbar" aria-label="Emulator controls">
    <span class="state" class:running={emulator.running} role="status"><i aria-hidden="true" class="codicon codicon-circle-filled"></i> {emulator.running ? (wb.debug.stopped ? 'Paused by debugger' : `Running · frame ${emulator.frame}`) : 'Stopped'}</span>
    {#if emulator.hostDeviceName}<span class="state running" title={hostFolder ? `${hostFolder} is mapped read/write` : 'The project H folder is mapped read/write'}><i aria-hidden="true" class="codicon codicon-folder-opened"></i> {emulator.hostDeviceName} {hostFolder ? hostFolder.split(/[\\/]/).filter(Boolean).pop() : 'project H'}</span>{/if}
    <select aria-label="Emulator profile" value={emulator.profileId} onchange={event => { emulator.select((event.currentTarget as HTMLSelectElement).value); wb.editors.schedulePersist(); }}>
      {#each emulator.profiles as profile (profile.id)}<option value={profile.id}>{profile.name}</option>{/each}
    </select>
    <span class="tab-spacer"></span>
    <button class="icon-button" title="Start" aria-label="Start Atari800" disabled={!wb.commandEnabled('emulator.start')} onclick={() => wb.execute('emulator.start')}><i aria-hidden="true" class="codicon codicon-play"></i></button>
    <button class="icon-button" title="Restart" aria-label="Restart Atari800" disabled={!wb.commandEnabled('emulator.restart')} onclick={() => wb.execute('emulator.restart')}><i aria-hidden="true" class="codicon codicon-debug-restart"></i></button>
    <button class="icon-button" title="Stop" aria-label="Stop Atari800" disabled={!wb.commandEnabled('emulator.stop')} onclick={() => wb.execute('debug.stop')}><i aria-hidden="true" class="codicon codicon-debug-stop"></i></button>
    {#if emulator.joystickMode}<span class="state joystick-state" class:running={emulator.trigger} role="status" aria-label={`Joystick STICK ${emulator.stick}, fire ${emulator.trigger ? 'pressed' : 'released'}`} title="STICK(0) and STRIG(0) as the program reads them">{stickGlyphs[emulator.stick] ?? '·'} {emulator.stick} · {emulator.trigger ? 'FIRE' : 'fire'}</span>{/if}
    <button class="icon-button" title={joystickHelp} aria-label="Keyboard joystick" aria-pressed={emulator.joystickMode} onclick={toggleJoystick}><i aria-hidden="true" class="codicon codicon-game"></i></button>
    <button class="icon-button" title="Save the current frame as PNG" aria-label="Capture screen" disabled={!emulator.image} onclick={capture}><i aria-hidden="true" class="codicon codicon-device-camera"></i></button>
    <button class="icon-button" title="Configure profile" aria-label="Configure emulator profile" aria-pressed={emulator.configOpen} onclick={() => (emulator.configOpen = !emulator.configOpen)}><i aria-hidden="true" class="codicon codicon-settings-gear"></i></button>
  </div>
  <div class="emulator-workspace">
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div class="emulator-stage" bind:this={stage} tabindex="0" role="application" aria-label={emulator.joystickMode ? 'Atari800 screen. Keyboard joystick on: Q W E A D Z X C, arrows or keypad move, Space, Numpad 0 or Ctrl fire; press Tab to leave.' : 'Atari800 screen. Keyboard input is sent to the emulated Atari; press Tab to leave.'}
      onkeydown={event => key(event, true)} onkeyup={event => key(event, false)} onblur={() => emulator.releaseKeys()}>
      {#if emulator.image}
        <img src={emulator.image} style={`width:min(100%, ${336 * (emulator.draft?.scale ?? 2)}px)`} alt={`Atari800 screen, frame ${emulator.frame}`} draggable="false" />
      {:else if emulator.running}
        <div class="emulator-empty" role="status"><i aria-hidden="true" class="codicon codicon-loading codicon-modifier-spin"></i> Starting Atari800…</div>
      {:else}
        <div class="emulator-empty">
          <i aria-hidden="true" class="codicon codicon-device-desktop"></i><strong>Atari800 is stopped</strong>
          <span>Choose a profile and press Start. A project is not required.</span>
          <button class="primary" onclick={() => wb.execute('emulator.start')}><i aria-hidden="true" class="codicon codicon-play"></i> Start Emulator</button>
        </div>
      {/if}
    </div>
    {#if emulator.configOpen && emulator.draft}
      {@const draft = emulator.draft}
      <aside class="emulator-config" aria-label="Emulator profile">
        <header><strong>Emulator Profile</strong><button class="icon-button" title="New profile" aria-label="New profile" onclick={() => emulator.newProfile()}><i aria-hidden="true" class="codicon codicon-add"></i></button></header>
        <label for="emu-name">Name</label><input id="emu-name" bind:value={draft.name} autocomplete="off" />
        <h4>System</h4>
        <label for="emu-machine">Machine</label>
        <select id="emu-machine" bind:value={draft.machine}><option value="atari">Atari 400/800</option><option value="xl">Atari XL</option><option value="xe">Atari XE</option><option value="5200">Atari 5200</option></select>
        <label for="emu-video">Video</label><select id="emu-video" bind:value={draft.video}><option value="ntsc">NTSC</option><option value="pal">PAL</option></select>
        <label for="emu-memory">Memory</label>
        <select id="emu-memory" bind:value={draft.memory} disabled={draft.machine === '5200' || draft.ultimate1MB}><option value="64">64K</option><option value="128">128K</option><option value="256">256K Rambo</option><option value="320-rambo">320K Rambo</option><option value="320-compy">320K Compy Shop</option><option value="1088">1088K</option></select>
        <label for="emu-speed">CPU speed</label>
        <select id="emu-speed" bind:value={draft.cpuSpeed}><option value={100}>1×</option><option value={150}>1.5×</option><option value={200}>2×</option><option value={300}>3×</option><option value={500}>5×</option></select>
        <label class="check"><input type="checkbox" bind:checked={draft.ultimate1MB} /> Ultimate 1MB memory banking</label>
        <label class="check"><input type="checkbox" bind:checked={draft.sioPatch} /> Fast SIO patch</label>
        <h4>Cartridges</h4>
        {#each cartridgeSlots as slot (slot.key)}
          {@const cartridge = draft[slot.key]}
          <fieldset>
            <legend>{slot.label}</legend>
            <label for={`emu-${slot.key}-kind`}>Cartridge</label>
            <select id={`emu-${slot.key}-kind`} bind:value={cartridge.kind} onchange={() => { if (cartridge.kind !== 'sdx' && cartridge.kind !== 'image') { cartridge.image = ''; cartridge.type = 0; } }}>
              <option value="">Empty</option>
              <option value="basic">Atari BASIC</option>
              <option value="eos">EOS (included)</option>
              <option value="sdx">SpartaDOS X image…</option>
              <option value="image">Other cartridge image…</option>
            </select>
            {#if cartridge.kind === 'sdx' || cartridge.kind === 'image'}
              <div class="input-row">
                <input aria-label={`${slot.label} cartridge image`} bind:value={cartridge.image} placeholder="CAR, ROM or BIN image" autocomplete="off" />
                <button class="icon-button" title={`Browse ${slot.label.toLowerCase()} cartridge image`} aria-label={`Browse ${slot.label.toLowerCase()} cartridge image`} onclick={() => choose(`Select ${slot.label} Cartridge Image`, '*.car;*.rom;*.bin', 'Atari cartridge images', path => { cartridge.image = path; })}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
              </div>
              <label for={`emu-${slot.key}-type`}>Type of a raw image</label>
              <select id={`emu-${slot.key}-type`} bind:value={cartridge.type}>
                {#each cartridgeTypes as type}<option value={type.id}>{type.name}</option>{/each}
              </select>
            {/if}
          </fieldset>
        {/each}
        <p class="hint">The secondary slot is the pass-through connector on a SpartaDOS X cartridge. Atari BASIC switches on the BASIC built into XL and XE machines.</p>
        <h4>H: host folder</h4>
        <label for="emu-host-dir">Folder the Atari sees as H: (D8: under EOS)</label>
        <div class="input-row">
          <input id="emu-host-dir" bind:value={draft.hostDir} placeholder="The H folder of the project" autocomplete="off" />
          <button class="icon-button" title="Browse for the H: folder" aria-label="Browse for the H: folder" onclick={chooseHostDir}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
          <button class="icon-button" title="Use the H folder of the project" aria-label="Use the H folder of the project" disabled={!draft.hostDir} onclick={() => { draft.hostDir = ''; }}><i aria-hidden="true" class="codicon codicon-discard"></i></button>
        </div>
        <p class="hint">Leave empty to use the H folder of the open project. A path without a drive is taken from the project folder. The Atari can read and write this folder.</p>
        <h4>Floppy drives</h4>
        {#each draft.drives as drive, index}
          <fieldset>
            <legend>D{index + 1}:</legend>
            <label class="check"><input type="checkbox" bind:checked={drive.enabled} /> Enabled</label>
            <label class="check"><input type="checkbox" bind:checked={drive.readOnly} /> Read only</label>
            <div class="input-row">
              <input aria-label={`D${index + 1} disk image`} bind:value={drive.image} placeholder="ATR, XFD or ATX image" autocomplete="off" />
              <button class="icon-button" title={`Browse D${index + 1} image`} aria-label={`Browse D${index + 1} image`} onclick={() => choose(`Select D${index + 1}: Disk Image`, '*.atr;*.xfd;*.atx', 'Atari disk images', path => { drive.image = path; drive.enabled = true; })}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
            </div>
          </fieldset>
        {/each}
        <h4>R: serial device</h4>
        <label for="emu-r-mode">Interface</label>
        <select id="emu-r-mode" bind:value={draft.rDevice.mode} onchange={() => { if (draft.rDevice.mode === 'rverter') draft.rDevice.modem = true; }}><option value="disabled">Disabled</option><option value="850">Atari 850</option><option value="rverter">R-Verter modem</option></select>
        {#if draft.rDevice.mode !== 'disabled'}
          <label for="emu-r-port">R1: TCP port</label><input id="emu-r-port" type="number" min="1" max="65532" bind:value={draft.rDevice.listenPort} />
          <label for="emu-r-baud">Line speed</label>
          <select id="emu-r-baud" bind:value={draft.rDevice.connectBaud}>{#each [300, 1200, 2400, 4800, 9600, 19200] as baud}<option value={baud}>{baud} baud</option>{/each}</select>
          <label class="check"><input type="checkbox" bind:checked={draft.rDevice.modem} disabled={draft.rDevice.mode === 'rverter'} /> Hayes-compatible TCP modem</label>
          <label for="emu-r-serial">Host serial port (Linux; blank uses TCP)</label><input id="emu-r-serial" bind:value={draft.rDevice.serialPort} placeholder="/dev/ttyUSB0" autocomplete="off" />
        {/if}
        <h4>IDE Plus 2.0</h4>
        <label class="check"><input type="checkbox" bind:checked={draft.ide.enabled} /> Enable the IDE interface</label>
        {#if draft.ide.enabled}
          <div class="input-row">
            <input aria-label="IDE disk image" bind:value={draft.ide.image} placeholder="Raw hard disk image" autocomplete="off" />
            <button class="icon-button" title="Browse IDE image" aria-label="Browse IDE image" onclick={() => choose('Select IDE Plus 2.0 Disk Image', '*.img;*.raw;*.ide', 'Raw IDE disk images', path => { draft.ide.image = path; draft.ide.enabled = true; })}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
          </div>
          <label class="check"><input type="checkbox" bind:checked={draft.ide.compactFlash} /> CompactFlash identify mode</label>
        {/if}
        <h4>Startup</h4>
        <label for="emu-scale">Display scale</label><select id="emu-scale" bind:value={draft.scale}>{#each [1, 2, 3, 4] as scale}<option value={scale}>{scale}×</option>{/each}</select>
        <label for="emu-media">Startup media</label>
        <div class="input-row">
          <input id="emu-media" bind:value={draft.media} placeholder="XEX, CAR, ROM or D1: ATR" autocomplete="off" />
          <button class="icon-button" title="Browse media" aria-label="Browse startup media" onclick={() => choose('Select Atari800 Media', '*.atr;*.xex;*.car;*.rom', 'Atari media', path => { draft.media = path; })}><i aria-hidden="true" class="codicon codicon-folder-opened"></i></button>
        </div>
        <label for="emu-args">Advanced Atari800 arguments (comma-separated)</label><input id="emu-args" bind:value={args} onchange={saveArgs} autocomplete="off" />
        <footer>
          <button disabled={!draft.id || emulator.profiles.length <= 1} onclick={removeProfile}><i aria-hidden="true" class="codicon codicon-trash"></i> Delete</button>
          <button class="primary" onclick={() => { saveArgs(); void emulator.saveProfile(); }}><i aria-hidden="true" class="codicon codicon-save"></i> Save Profile</button>
        </footer>
      </aside>
    {/if}
  </div>
</div>