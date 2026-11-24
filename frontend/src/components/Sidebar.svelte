<script lang="ts">
  import { wb } from '../lib/workbench.svelte';
  import Explorer from './sidebar/Explorer.svelte';
  import Search from './sidebar/Search.svelte';
  import AtrExplorer from './sidebar/AtrExplorer.svelte';
  import SourceControl from './sidebar/SourceControl.svelte';
  import RunDebug from './sidebar/RunDebug.svelte';
  import EmulatorSide from './sidebar/EmulatorSide.svelte';
  import Manual from './sidebar/Manual.svelte';
  import Settings from './sidebar/Settings.svelte';
  import SpritesSide from './sidebar/SpritesSide.svelte';
  import ClaudeCode from './sidebar/ClaudeCode.svelte';

  const titles = { explorer: 'Explorer', search: 'Search', atr: 'ATR Explorer', source: 'Source Control', run: 'Run and Debug', emulator: 'ex800 Emulator', sprites: 'Sprite Studio', claude: 'Claude Code', manual: 'exBASIC Manual', settings: 'Settings' } as const;
</script>

<h2 class="sidebar-title">{titles[wb.ui.activity]}</h2>
<div class="sidebar-body">
  {#if wb.ui.activity === 'explorer'}<Explorer />
  {:else if wb.ui.activity === 'search'}<Search />
  {:else if wb.ui.activity === 'atr'}<AtrExplorer />
  {:else if wb.ui.activity === 'source'}<SourceControl />
  {:else if wb.ui.activity === 'run'}<RunDebug />
  {:else if wb.ui.activity === 'emulator'}<EmulatorSide />
  {:else if wb.ui.activity === 'sprites'}<SpritesSide />
  {:else if wb.ui.activity === 'claude'}<ClaudeCode />
  {:else if wb.ui.activity === 'manual'}<Manual />
  {:else}<Settings />{/if}
</div>
