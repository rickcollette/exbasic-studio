<script lang="ts">
  import { wb } from '../../lib/workbench.svelte';
  const emulator = wb.emulator;
</script>

<div class="status-line" role="status"><i aria-hidden="true" class="codicon codicon-circle-filled" class:running={emulator.running}></i><strong>{emulator.running ? 'Running' : 'Stopped'}</strong><span>{wb.editors.emulatorOpen ? 'Tab open' : 'Tab closed'}</span></div>
<div class="sidebar-form">
  <label for="sidebar-emulator-profile">Profile</label>
  <select id="sidebar-emulator-profile" value={emulator.profileId} onchange={event => { emulator.select((event.currentTarget as HTMLSelectElement).value); wb.editors.schedulePersist(); }}>
    {#each emulator.profiles as profile (profile.id)}<option value={profile.id}>{profile.name}</option>{/each}
  </select>
</div>
<div class="run-actions">
  <button class="primary" onclick={() => wb.execute('emulator.open')}><i aria-hidden="true" class="codicon codicon-device-desktop"></i> {wb.editors.emulatorOpen ? 'Show ex800 Screen' : 'Open ex800 Screen'}</button>
  <button disabled={!wb.commandEnabled('emulator.start')} onclick={() => wb.execute('emulator.start')}><i aria-hidden="true" class="codicon codicon-play"></i> Start</button>
  <button disabled={!wb.commandEnabled('emulator.restart')} onclick={() => wb.execute('emulator.restart')}><i aria-hidden="true" class="codicon codicon-debug-restart"></i> Restart</button>
  <button disabled={!wb.commandEnabled('emulator.stop')} onclick={() => wb.execute('emulator.stop')}><i aria-hidden="true" class="codicon codicon-debug-stop"></i> Stop</button>
</div>
<p class="hint">ex800 runs inside an editor tab; a project is not required. Closing the tab stops the emulated machine.</p>
