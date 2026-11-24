// CodeMirror configuration shared by every editor tab.
import { EditorState, StateEffect, StateField, RangeSet, Prec, type Extension, Annotation, Transaction } from '@codemirror/state';
import { EditorView, keymap, lineNumbers, drawSelection, highlightActiveLine, highlightActiveLineGutter, gutter, GutterMarker, Decoration, type DecorationSet, rectangularSelection, crosshairCursor, dropCursor } from '@codemirror/view';
import { defaultKeymap, history, historyKeymap, indentWithTab, toggleComment } from '@codemirror/commands';
import { syntaxHighlighting, HighlightStyle, foldGutter, foldKeymap, indentOnInput, bracketMatching } from '@codemirror/language';
import { closeBrackets, closeBracketsKeymap, completionKeymap, acceptCompletion } from '@codemirror/autocomplete';
import { lintGutter, setDiagnostics, forEachDiagnostic, type Diagnostic as CMDiagnostic } from '@codemirror/lint';
import { searchKeymap, highlightSelectionMatches, search } from '@codemirror/search';
import { tags as t } from '@lezer/highlight';
import { exbasic, isExbasicSource, type LanguageHost } from '../language';
import type { Diagnostic } from '../native';
import { quickFixes } from './quickfix';
import { languageFor } from '../fileTypes';

export const highlightStyle = HighlightStyle.define([
  { tag: t.controlKeyword, color: 'var(--syntax-control)', fontWeight: '600' },
  { tag: t.keyword, color: 'var(--syntax-keyword)', fontWeight: '600' },
  { tag: t.operatorKeyword, color: 'var(--syntax-keyword)' },
  { tag: t.standard(t.function(t.variableName)), color: 'var(--syntax-function)' },
  { tag: t.variableName, color: 'var(--syntax-variable)' },
  { tag: t.number, color: 'var(--syntax-number)' },
  { tag: [t.string, t.regexp], color: 'var(--syntax-string)' },
  { tag: [t.lineComment, t.blockComment, t.docComment], color: 'var(--syntax-comment)', fontStyle: 'italic' },
  { tag: [t.bool, t.null], color: 'var(--syntax-keyword)' },
  { tag: [t.propertyName, t.typeName, t.className], color: 'var(--syntax-function)' },
  { tag: [t.heading, t.strong], color: 'var(--syntax-control)', fontWeight: '600' },
  { tag: t.emphasis, fontStyle: 'italic' },
  { tag: [t.link, t.url], color: 'var(--link)', textDecoration: 'underline' },
  { tag: t.invalid, color: 'var(--danger)', textDecoration: 'underline wavy' },
  { tag: t.operator, color: 'var(--syntax-operator)' },
  { tag: t.separator, color: 'var(--syntax-operator)' },
]);

export const editorTheme = EditorView.theme({
  '&': { height: '100%', fontSize: 'var(--editor-font-size, 13px)', backgroundColor: 'var(--editor-bg)', color: 'var(--editor-fg)' },
  '.cm-content': { fontFamily: 'var(--mono)', padding: '9px 0', caretColor: 'var(--editor-cursor)' },
  '.cm-gutters': { backgroundColor: 'var(--editor-bg)', color: 'var(--editor-gutter)', border: 'none' },
  // The selection is drawn behind the text lines, so the current-line tint
  // must be translucent or it hides the selection on that line.
  '.cm-activeLine': { backgroundColor: 'color-mix(in srgb, var(--editor-fg) 6%, transparent)' },
  // Other occurrences of the selected text are outlined; only the selection
  // itself is filled.
  '.cm-selectionMatch': { backgroundColor: 'transparent', outline: '1px solid var(--editor-gutter)', borderRadius: '2px' },
  '.cm-activeLineGutter': { backgroundColor: 'var(--editor-active-line)', color: 'var(--editor-fg)' },
  '.cm-cursor': { borderLeftColor: 'var(--editor-cursor)' },
  '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': { background: 'var(--editor-selection) !important' },
  '.cm-scroller': { overflow: 'auto', fontFamily: 'var(--mono)' },
  '.cm-block-match': { outline: '1px solid var(--focus)', borderRadius: '2px' },
  '.cm-breakpoint-gutter': { width: '16px', cursor: 'pointer' },
  '.cm-breakpoint-gutter .cm-gutterElement': { display: 'flex', alignItems: 'center', justifyContent: 'center' },
  '.cm-breakpoint': { width: '10px', height: '10px', borderRadius: '50%', background: 'var(--danger)' },
  '.cm-breakpoint.disabled': { background: 'transparent', border: '2px solid var(--danger)' },
  '.cm-breakpoint.unverified': { background: 'var(--muted)' },
  '.cm-debug-line': { backgroundColor: 'var(--debug-line)' },
  '.cm-tooltip': { backgroundColor: 'var(--popup-bg)', color: 'var(--fg)', border: '1px solid var(--border)' },
  '.cm-tooltip-autocomplete ul li[aria-selected]': { backgroundColor: 'var(--list-active)', color: 'var(--fg)' },
  '.cm-tooltip-autocomplete ul li': { padding: '2px 8px 2px 4px' },
  '.cm-completionDetail': { color: 'var(--muted)', fontStyle: 'normal' },
  '.cm-snippetField': { backgroundColor: 'var(--hover)', outline: '1px solid var(--border)' },
  // The fix button sits under the message, at the left, so it is directly
  // above or below the marked word and easy to reach without the tooltip closing.
  '.cm-diagnosticAction': { display: 'block', backgroundColor: 'var(--accent)', color: 'var(--accent-fg, #fff)', border: 'none', borderRadius: '2px', padding: '3px 10px', margin: '6px 0 2px 0', cursor: 'pointer' },
  '.cm-hover-doc, .cm-signature-help': { padding: '6px 9px', maxWidth: '460px', fontSize: '12px' },
  '.cm-hover-doc code': { fontFamily: 'var(--mono)', color: 'var(--syntax-function)' },
  '.cm-hover-doc p': { margin: '4px 0' },
  '.cm-hover-doc button': { background: 'none', border: 'none', color: 'var(--link)', padding: 0, cursor: 'pointer', textDecoration: 'underline' },
  '.cm-signature-help': { fontFamily: 'var(--mono)' },
  '.cm-signature-help strong': { color: 'var(--syntax-function)', textDecoration: 'underline' },
  '.cm-panels': { backgroundColor: 'var(--panel-bg)', color: 'var(--fg)' },
  '.cm-searchMatch': { backgroundColor: 'var(--search-match)' },
}, { dark: true });

// ---- Breakpoints -----------------------------------------------------------

export interface BreakpointMark { line: number; enabled: boolean; verified: boolean }

/** Replaces the breakpoint markers shown in the gutter. */
export const setBreakpoints = StateEffect.define<BreakpointMark[]>();

class BreakpointMarker extends GutterMarker {
  constructor(private mark: BreakpointMark) { super(); }
  eq(other: BreakpointMarker) { return other.mark.enabled === this.mark.enabled && other.mark.verified === this.mark.verified; }
  toDOM() {
    const dot = document.createElement('span');
    dot.className = 'cm-breakpoint' + (this.mark.enabled ? '' : ' disabled') + (this.mark.verified ? '' : ' unverified');
    dot.title = this.mark.enabled ? (this.mark.verified ? 'Breakpoint' : 'Breakpoint (not yet bound to code)') : 'Disabled breakpoint';
    return dot;
  }
}

/** Reserves the gutter width without looking like a breakpoint. */
const spacerMarker = new (class extends GutterMarker {
  toDOM() {
    const spacer = document.createElement('span');
    spacer.style.cssText = 'display:inline-block;width:10px;height:10px';
    return spacer;
  }
})();

const breakpointField = StateField.define<RangeSet<GutterMarker>>({
  create: () => RangeSet.empty,
  update(set, transaction) {
    set = set.map(transaction.changes);
    for (const effect of transaction.effects) {
      if (!effect.is(setBreakpoints)) continue;
      const doc = transaction.state.doc;
      const marks = effect.value.filter(mark => mark.line >= 1 && mark.line <= doc.lines).sort((a, b) => a.line - b.line);
      set = RangeSet.of(marks.map(mark => new BreakpointMarker(mark).range(doc.line(mark.line).from)));
    }
    return set;
  },
});

/** Lines (1-based) currently carrying a breakpoint marker, after edits. */
export function breakpointLines(state: EditorState): number[] {
  const lines: number[] = [];
  const cursor = state.field(breakpointField).iter();
  for (; cursor.value; cursor.next()) lines.push(state.doc.lineAt(cursor.from).number);
  return lines;
}

// ---- Debugger current line -------------------------------------------------

/** Highlights the line where execution is stopped (0 clears it). */
export const setDebugLine = StateEffect.define<number>();

const debugLineField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(value, transaction) {
    value = value.map(transaction.changes);
    for (const effect of transaction.effects) {
      if (!effect.is(setDebugLine)) continue;
      const doc = transaction.state.doc;
      value = effect.value >= 1 && effect.value <= doc.lines
        ? Decoration.set([Decoration.line({ class: 'cm-debug-line' }).range(doc.line(effect.value).from)])
        : Decoration.none;
    }
    return value;
  },
  provide: field => EditorView.decorations.from(field),
});

// ---- Diagnostics -----------------------------------------------------------

/** Converts backend diagnostics to CodeMirror ranges within a document. */
export function toEditorDiagnostics(state: EditorState, diagnostics: Diagnostic[]): CMDiagnostic[] {
  const doc = state.doc;
  return diagnostics.map((item): CMDiagnostic => {
    const line = doc.line(Math.max(1, Math.min(item.line || 1, doc.lines)));
    let from = Math.min(line.to, line.from + Math.max(0, (item.column || 1) - 1));
    let to = from;
    if (item.endLine > 0) {
      const endLine = doc.line(Math.max(1, Math.min(item.endLine, doc.lines)));
      to = Math.min(endLine.to, endLine.from + Math.max(0, item.endColumn - 1));
    }
    if (doc.sliceString(from, Math.max(to, from + 1)).trim() === '') {
      // The compiler pointed between words (it counts columns in its own
      // normalized copy of the line). Mark the word before that position, or
      // the first word of the line, so there is something to point at.
      const before = /([A-Za-z0-9_$%."]+)\s*$/.exec(doc.sliceString(line.from, from));
      const first = /^\s*([A-Za-z0-9_$%."]+)/.exec(line.text);
      if (before) { from = line.from + before.index; to = from + before[1].length; }
      else if (first) { from = line.from + first[0].length - first[1].length; to = from + first[1].length; }
    }
    if (to <= from) {
      // No range from the compiler: mark the word at the position.
      const rest = doc.sliceString(from, line.to);
      const word = /^[A-Za-z0-9_$%."]+/.exec(rest);
      to = Math.min(line.to, from + Math.max(1, word ? word[0].length : 1));
    }
    const severity: CMDiagnostic['severity'] = item.severity === 'warning' ? 'warning' : item.severity === 'info' ? 'info' : 'error';
    return { from, to: Math.max(to, Math.min(line.to, from + 1)), severity, message: item.code ? `${item.message} [${item.code}]` : item.message, source: 'exBASIC', actions: quickFixes(doc, item.message, line.number, item.column || 1).map(fix => ({ name: fix.name, apply: (view: EditorView) => { view.dispatch({ changes: { from: fix.from, to: fix.to, insert: fix.insert }, selection: { anchor: fix.cursor }, scrollIntoView: true, userEvent: 'input.fix' }); view.focus(); } })) };
  }).sort((a, b) => a.from - b.from);
}

export { setDiagnostics };

/**
 * Applies the first available fix for a problem on the cursor's line
 * (Ctrl+. / Cmd+.). Returns false when there is nothing to apply.
 */
export function applyQuickFix(view: EditorView): boolean {
  const cursorLine = view.state.doc.lineAt(view.state.selection.main.head);
  let done = false;
  forEachDiagnostic(view.state, (diagnostic, from, to) => {
    if (done || !diagnostic.actions?.length) return;
    if (view.state.doc.lineAt(from).number !== cursorLine.number) return;
    done = true;
    diagnostic.actions[0].apply(view, from, to);
  });
  return done;
}

// ---- Split view synchronization --------------------------------------------

/** Marks a transaction that mirrors a change made in the other split pane. */
export const mirrored = Annotation.define<boolean>();

/** Forwards the document changes of a transaction to another view. */
export function mirrorChanges(transaction: Transaction, target: EditorView): void {
  if (transaction.changes.empty || transaction.annotation(mirrored)) return;
  target.dispatch({ changes: transaction.changes, annotations: [mirrored.of(true), Transaction.addToHistory.of(false)] });
}

// ---- State factory ---------------------------------------------------------

export interface EditorCallbacks {
  /** Every transaction: the new state and whether the text changed. */
  onUpdate(path: string, state: EditorState, docChanged: boolean, transactions: readonly Transaction[]): void;
  onToggleBreakpoint(path: string, line: number): void;
  onGoToDefinition(path: string, state: EditorState, pos: number): void;
  onSave(path: string): void;
}

export function editorExtensions(path: string, host: LanguageHost, callbacks: EditorCallbacks, options: { readOnly?: boolean } = {}): Extension[] {
  const isBasic = isExbasicSource(path);
  // The breakpoint gutter sits to the left of the line numbers.
  const breakpointGutter: Extension[] = isBasic ? [
    breakpointField,
    gutter({
      class: 'cm-breakpoint-gutter',
      // Every line gets a gutter cell, so a breakpoint can be set anywhere.
      renderEmptyElements: true,
      markers: view => view.state.field(breakpointField),
      initialSpacer: () => spacerMarker,
      domEventHandlers: {
        mousedown(view, block) {
          callbacks.onToggleBreakpoint(path, view.state.doc.lineAt(block.from).number);
          return true;
        },
      },
    }),
  ] : [];
  const extensions: Extension[] = [
    ...breakpointGutter,
    lineNumbers(),
    highlightActiveLine(),
    highlightActiveLineGutter(),
    drawSelection(),
    dropCursor(),
    rectangularSelection(),
    crosshairCursor(),
    history(),
    indentOnInput(),
    bracketMatching(),
    closeBrackets(),
    highlightSelectionMatches(),
    search({ top: true }),
    foldGutter(),
    lintGutter(),
    syntaxHighlighting(highlightStyle),
    editorTheme,
    EditorView.lineWrapping,
    EditorState.tabSize.of(2),
    EditorView.contentAttributes.of({ 'aria-label': `Editor: ${path}`, spellcheck: 'false' }),
    debugLineField,
    // Tab accepts the highlighted suggestion; otherwise it indents.
    Prec.highest(keymap.of([{ key: 'Tab', run: acceptCompletion }])),
    keymap.of([
      { key: 'Mod-s', preventDefault: true, run: () => { callbacks.onSave(path); return true; } },
      { key: 'F12', run: view => { callbacks.onGoToDefinition(path, view.state, view.state.selection.main.head); return true; } },
      { key: 'Mod-/', run: toggleComment },
      { key: 'Mod-.', run: applyQuickFix },
      ...closeBracketsKeymap, ...defaultKeymap, ...searchKeymap, ...historyKeymap, ...foldKeymap, ...completionKeymap, indentWithTab,
    ]),
    EditorView.domEventHandlers({
      mousedown(event, view) {
        if (!(event.ctrlKey || event.metaKey) || event.button !== 0) return false;
        const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
        if (pos === null) return false;
        callbacks.onGoToDefinition(path, view.state, pos);
        return true;
      },
    }),
    EditorView.updateListener.of(update => callbacks.onUpdate(path, update.state, update.docChanged, update.transactions)),
  ];
  if (isBasic) {
    extensions.push(exbasic(host));
  } else {
    const language = languageFor(path);
    if (language) extensions.push(language);
  }
  if (options.readOnly) extensions.push(EditorState.readOnly.of(true));
  return extensions;
}
