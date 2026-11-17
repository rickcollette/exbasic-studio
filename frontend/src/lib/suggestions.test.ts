// Suggestions in the editor: block templates, Tab to accept, problems from
// included files and the one-click fix for a FUNC without parentheses.
import { describe, expect, it } from 'vitest';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { CompletionContext, startCompletion, currentCompletions, completionStatus, acceptCompletion } from '@codemirror/autocomplete';
import { exbasicCompletions, isExbasicSource } from './language';
import { editorExtensions, toEditorDiagnostics, applyQuickFix, setDiagnostics } from './editor/setup';
import { ProblemsStore, withIncludeMarkers } from './stores/editors.svelte';
import type { Diagnostic } from './native';

const host = { currentPath: () => 'new.bas', includeCandidates: () => [], workspaceSymbols: () => [], openManual: () => {} };
const callbacks = { onUpdate: () => {}, onSave: () => {}, onGoToDefinition: () => {}, onToggleBreakpoint: () => {} } as never;
const diagnostic = (over: Partial<Diagnostic>): Diagnostic => ({ path: 'new.bas', line: 1, column: 1, endLine: 0, endColumn: 0, severity: 'error', code: '', message: '', ...over } as Diagnostic);

describe('source files', () => {
  it('treats include files as exBASIC', () => {
    expect(isExbasicSource('lib/basfile.inc')).toBe(true);
    expect(isExbasicSource('src/MAIN.BAS')).toBe(true);
    expect(isExbasicSource('notes.md')).toBe(false);
  });
});

describe('completion', () => {
  it('offers block templates at the start of a statement', () => {
    const state = EditorState.create({ doc: 'FU' });
    const result = exbasicCompletions(host as never)(new CompletionContext(state, 2, false));
    const snippet = result?.options.find(option => option.type === 'snippet' && option.label === 'FUNC');
    expect(snippet?.detail).toContain('ENDFUNC');
  });

  it('does not offer block templates inside an expression', () => {
    const state = EditorState.create({ doc: 'X = FO' });
    const result = exbasicCompletions(host as never)(new CompletionContext(state, 6, false));
    expect(result?.options.some(option => option.type === 'snippet')).toBe(false);
  });

  it('accepts the highlighted suggestion with Tab', async () => {
    const parent = document.body.appendChild(document.createElement('div'));
    const view = new EditorView({ parent, state: EditorState.create({ doc: 'WHI', selection: { anchor: 3 }, extensions: editorExtensions('new.bas', host as never, callbacks) }) });
    startCompletion(view);
    for (let i = 0; i < 50 && completionStatus(view.state) !== 'active'; i++) await new Promise(resolve => setTimeout(resolve, 20));
    expect(currentCompletions(view.state).length).toBeGreaterThan(0);
    // CodeMirror ignores accept keys briefly after the list opens.
    await new Promise(resolve => setTimeout(resolve, 150));
    expect(acceptCompletion(view)).toBe(true);
    expect(view.state.doc.toString()).toMatch(/^WHILE/);
    view.destroy();
  });
});

describe('problems from included files', () => {
  const reported = [diagnostic({ path: 'basfile.inc', line: 1, message: "expected '(' after FUNC name", code: 'function' })];

  it('marks the INCLUDE line and keeps the problem on its own file', () => {
    const all = withIncludeMarkers('new.bas', 'PRINT 1\nINCLUDE "basfile.inc"\n', reported);
    const problems = new ProblemsStore();
    problems.setLint('new.bas', all);
    const inProgram = problems.forPath('new.bas');
    expect(inProgram).toHaveLength(1);
    expect(inProgram[0].line).toBe(2);
    expect(inProgram[0].message).toBe("basfile.inc, line 1: expected '(' after FUNC name");
    const inInclude = problems.forPath('basfile.inc');
    expect(inInclude).toHaveLength(1);
    expect(inInclude[0].line).toBe(1);
  });

  it('marks a word when the compiler points at the gap between words', () => {
    const state = EditorState.create({ doc: ['FUNC a()', '  PRINT 1', '', 'FUNC b()', 'ENDFUNC', ''].join(String.fromCharCode(10)) });
    const [converted] = toEditorDiagnostics(state, [diagnostic({ path: 'x.bas', line: 4, column: 5, endLine: 4, endColumn: 6, message: "FUNC can't be inside another FUNC" })]);
    expect(state.doc.sliceString(converted.from, converted.to)).toBe('FUNC');
    expect(converted.actions?.[0].name).toBe('Add ENDFUNC above');
  });

  it('applies the fix on the cursor line with the keyboard command', () => {
    const parent = document.body.appendChild(document.createElement('div'));
    const source = ['FUNC a()', '  PRINT 1', '', 'FUNC b()', 'ENDFUNC', ''].join(String.fromCharCode(10));
    const view = new EditorView({ parent, state: EditorState.create({ doc: source, extensions: editorExtensions('x.bas', host as never, callbacks) }) });
    const found = [diagnostic({ path: 'x.bas', line: 4, column: 5, endLine: 4, endColumn: 6, message: "FUNC can't be inside another FUNC" })];
    view.dispatch(setDiagnostics(view.state, toEditorDiagnostics(view.state, found)));
    expect(applyQuickFix(view)).toBe(false); // the cursor is on line 1
    view.dispatch({ selection: { anchor: view.state.doc.line(4).from + 2 } });
    expect(applyQuickFix(view)).toBe(true);
    expect(view.state.doc.line(3).text).toBe('ENDFUNC');
    expect(view.state.doc.line(5).text).toBe('FUNC b()');
    view.destroy();
  });

  it('offers Add () for a FUNC without parentheses', () => {
    const parent = document.body.appendChild(document.createElement('div'));
    const view = new EditorView({ parent, state: EditorState.create({ doc: 'func oneplusone\n  return 2\n' }) });
    const [converted] = toEditorDiagnostics(view.state, reported);
    expect(converted.actions?.[0].name).toBe('Add ()');
    converted.actions![0].apply(view, converted.from, converted.to);
    expect(view.state.doc.line(1).text).toBe('func oneplusone()');
    view.destroy();
  });
});
