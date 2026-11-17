<script lang="ts">
  import { onMount } from 'svelte';
  import Workbench from './components/Workbench.svelte';
  import { createWorkbench } from './lib/workbench.svelte';

  const workbench = createWorkbench();
  onMount(() => {
    void workbench.start();
    return () => workbench.dispose();
  });
</script>

<Workbench />