<script lang="ts">
  // Hosts the CodeMirror view for the active tab and, when split, a second
  // view of the same document. Tab states live in the editors store; this
  // component swaps them in and pushes diagnostics, breakpoints and the
  // debugger's current line into the view.
  import { onMount, untrack } from 'svelte';
  import { EditorView } from '@codemirror/view';
  import { undo, redo, selectAll } from '@codemirror/commands';
  import { openSearchPanel } from '@codemirror/search';
  import { wb } from '../../lib/workbench.svelte';
  import { setDiagnostics, toEditorDiagnostics, setBreakpoints, setDebugLine, mirrorChanges, mirrored } from '../../lib/editor/setup';
  import { Transaction, type StateEffect } from '@codemirror/state';

  let host = $state<HTMLDivElement>();
  let splitHost = $state<HTMLDivElement>();
  let view: EditorView | undefined;
  let splitView: EditorView | undefined;
  let shownPath = '';
  let shownEpoch = -1;

  const active = $derived(wb.editors.activeTab);

  onMount(() => {
    view = new EditorView({
      parent: host!,
      // Changes typed in the main view are mirrored into the split view.
      dispatchTransactions(transactions, target) {
        target.update(transactions);
        if (splitView) for (const transaction of transactions) mirrorChanges(transaction, splitView);
      },
    });
    wb.editorAction = action => {
      if (!view || !wb.editors.activeTab) return false;
      switch (action) {
        case 'undo': undo(view); break;
        case 'redo': redo(view); break;
        case 'selectAll': selectAll(view); break;
        case 'find': openSearchPanel(view); return true;
        case 'definition': void wb.goToDefinition(wb.editors.active, view.state, view.state.selection.main.head); return true;
        case 'focus': break;
      }
      view.focus();
      return true;
    };
    return () => { view?.destroy(); splitView?.destroy(); wb.editorAction = () => false; };
  });

  // Show the active tab's state. The store bumps `epoch` when it replaces a
  // state from outside (reload from disk, restored recovery copy).
  $effect(() => {
    const tab = active;
    const epoch = tab?.epoch ?? -1;
    if (!view || !tab) { shownPath = ''; return; }
    if (tab.path === shownPath && epoch === shownEpoch) return;
    const state = untrack(() => wb.editors.stateFor(tab.path));
    if (!state) return;
    shownPath = tab.path;
    shownEpoch = epoch;
    view.setState(state);
    if (splitView) splitView.setState(wb.splitState(tab.path, state.doc.toString()));
    untrack(() => { applyMarkers(); view!.focus(); });
  });

  function applyMarkers() {
    const tab = wb.editors.activeTab;
    if (!view || !tab) return;
    const diagnostics = toEditorDiagnostics(view.state, wb.problems.forPath(tab.path));
    const marks = wb.debug.breakpointsFor(tab.path).map(bp => ({ line: bp.line, enabled: bp.enabled, verified: !wb.debug.active || bp.verified }));
    const current = wb.debug.currentLine;
    const effects: StateEffect<unknown>[] = [setDebugLine.of(current && current.path === tab.path ? current.line : 0)];
    if (/\.bas$/i.test(tab.path)) effects.push(setBreakpoints.of(marks));
    view.dispatch(setDiagnostics(view.state, diagnostics), { effects });
  }

  // Re-apply markers when problems, breakpoints or the stop location change.
  $effect(() => {
    void wb.problems.lint; void wb.problems.build; void wb.debug.breakpoints; void wb.debug.currentLine; void wb.debug.active;
    untrack(applyMarkers);
  });

  // Reveal requests: go to a line/column and centre it.
  $effect(() => {
    const request = wb.editors.reveal;
    if (!request || !view || request.path !== shownPath || request.path !== active?.path) return;
    const doc = view.state.doc;
    const line = doc.line(Math.max(1, Math.min(request.line, doc.lines)));
    const pos = Math.min(line.to, line.from + Math.max(0, request.column - 1));
    view.dispatch({ selection: { anchor: pos }, effects: EditorView.scrollIntoView(pos, { y: 'center' }) });
    view.focus();
    untrack(() => { wb.editors.reveal = null; });
  });

  // The split pane edits the same document; its changes are mirrored back.
  $effect(() => {
    const open = wb.editors.splitOpen && !!active;
    if (open && splitHost && !splitView && view) {
      splitView = new EditorView({
        parent: splitHost,
        state: wb.splitState(active!.path, view.state.doc.toString()),
        dispatchTransactions(transactions, target) {
          target.update(transactions);
          for (const transaction of transactions) {
            if (!transaction.changes.empty && !transaction.annotation(mirrored)) {
              view!.dispatch({ changes: transaction.changes, annotations: [mirrored.of(true), Transaction.userEvent.of('input.split')] });
            }
          }
        },
      });
    } else if (!open && splitView) {
      splitView.destroy();
      splitView = undefined;
    }
  });
</script>

<div class="editor-views" class:split={wb.editors.splitOpen && !!active}>
  <div class="editor-container" bind:this={host}></div>
  {#if wb.editors.splitOpen && active}
    <div class="editor-container split-pane">
      <div class="split-label"><i aria-hidden="true" class="codicon codicon-split-horizontal"></i> {active.path} (split)</div>
      <div class="split-host" bind:this={splitHost}></div>
    </div>
  {/if}
</div>
