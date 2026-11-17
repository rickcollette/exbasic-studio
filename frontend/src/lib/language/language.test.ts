import { describe, expect, it } from 'vitest';
import { EditorState } from '@codemirror/state';
import { CompletionContext } from '@codemirror/autocomplete';
import { ensureSyntaxTree, foldable } from '@codemirror/language';
import { exbasic, exbasicCompletions, exbasicLanguage, callAt, signatureParameters, matchingBlock, isStatementPosition, type LanguageHost } from './index';
import { classify, expand } from './tokens';
import { scanSymbols, outline, findDefinition, wordAt, includeAt, splitStatements } from './symbols';
import { catalog, syntaxFiles } from './catalog.generated';

function stateOf(doc: string) {
  return EditorState.create({ doc, extensions: [exbasic()] });
}

/** Lists "NodeName:text" for every leaf token. */
function tokens(doc: string): string[] {
  const state = stateOf(doc);
  const tree = ensureSyntaxTree(state, doc.length, 5000)!;
  const out: string[] = [];
  tree.iterate({
    enter(node) {
      if (node.node.firstChild || node.name === 'Program') return;
      const text = doc.slice(node.from, node.to);
      if (text.trim()) out.push(`${node.name}:${text}`);
    },
  });
  return out;
}

function treeText(doc: string): string {
  return ensureSyntaxTree(stateOf(doc), doc.length, 5000)!.toString();
}

describe('generated catalog', () => {
  it('is derived from the active syntax files', () => {
    expect(syntaxFiles).toContain('basic.syn');
    expect(syntaxFiles).toContain('exbasic.syn');
    expect(catalog.length).toBeGreaterThan(100);
  });

  it('carries abbreviations, signatures and manual links', () => {
    const print = catalog.find(entry => entry.name === 'PRINT' && entry.kind === 'statement')!;
    expect(print.abbr).toMatch(/^PR/);
    expect(print.chapter).toBe('10-console-io.md');
    expect(print.anchor).toBe('print');
    const mid = catalog.find(entry => entry.name === 'MID$' && entry.kind === 'function')!;
    expect(mid.signature).toBe('MID$(s, p [, n])');
    expect(catalog.some(entry => entry.name === 'INCLUDE' && entry.kind === 'directive')).toBe(true);
  });

  it('has no duplicate name/kind pairs', () => {
    const keys = catalog.map(entry => entry.name + '|' + entry.kind);
    expect(new Set(keys).size).toBe(keys.length);
  });
});

describe('word classification', () => {
  it('is case-insensitive and position-aware', () => {
    expect(classify('print', false, true)).toEqual({ cls: 'statement', name: 'PRINT' });
    expect(classify('Peek', false, false)).toEqual({ cls: 'function', name: 'PEEK' });
    expect(classify('to', false, false)).toEqual({ cls: 'word', name: 'TO' });
    expect(classify('score', false, true).cls).toBe('identifier');
    // A statement keyword in an expression position is not a statement.
    expect(classify('poke', false, false).cls).not.toBe('statement');
  });

  it('expands dot abbreviations like the compiler', () => {
    expect(classify('pri', true, true)).toEqual({ cls: 'statement', name: 'PRINT' });
    expect(classify('n', true, true)).toEqual({ cls: 'statement', name: 'NEXT' });
    expect(classify('zzz', true, true).cls).toBe('identifier');
  });

  it('rejects abbreviations shorter than the minimum', () => {
    const statements = new Map(catalog.filter(entry => entry.kind === 'statement').map(entry => [entry.name, entry]));
    const print = statements.get('PRINT')!;
    const minimum = print.abbr.length - 1;
    expect(expand(statements, 'PRINT'.slice(0, minimum))).not.toBeNull();
    expect(expand(statements, 'PRINT')).toBeNull();
  });
});

describe('grammar', () => {
  it('tokenizes statements, builtins, numbers, strings and comments', () => {
    const got = tokens('PRINT "A ""quoted"" :", PEEK($D40A)+%101 \' note\nscore = 1.5E3\n');
    expect(got).toContain('Statement:PRINT');
    expect(got).toContain('String:"A ""quoted"" :"');
    expect(got).toContain('Builtin:PEEK');
    expect(got).toContain('Number:$D40A');
    expect(got).toContain('Number:%101');
    expect(got).toContain("Comment:' note");
    expect(got).toContain('Identifier:score');
    expect(got).toContain('Number:1.5E3');
  });

  it('treats abbreviations and typed names as single tokens', () => {
    const got = tokens('PRI. name$, total%\n');
    expect(got).toContain('Statement:PRI.');
    expect(got).toContain('Identifier:name$');
    expect(got).toContain('Identifier:total%');
    expect(tokens('a$ = LEFT$(b$, 2)\n')).toContain('Builtin:LEFT$');
  });

  it('recognizes remarks', () => {
    expect(tokens('. whole line remark : PRINT 1\n')).toEqual(['Remark:. whole line remark : PRINT 1']);
    expect(tokens('x = .5\n')).toContain('Number:.5');
  });

  it('nests multi-line blocks', () => {
    const text = treeText('FUNC Twice(n)\n  FOR i = 1 TO n\n    IF i > 2\n      PRINT i\n    ENDIF\n  NEXT i\nENDFUNC\n');
    expect(text).toContain('FuncBlock(FuncOpen');
    expect(text).toContain('ForBlock(ForOpen');
    expect(text).toContain('IfBlock(IfOpen');
    expect(text).not.toContain('⚠');
  });

  it('keeps one-line forms from opening blocks', () => {
    const single = treeText('IF a = 1 THEN PRINT a\nFOR i = 1 TO 3 : PRINT i : NEXT i\nPRINT 2\n');
    expect(single).not.toContain('IfBlock');
    expect(single).not.toContain('ForBlock');
    expect(single).not.toContain('⚠');
    expect(treeText('IF a = 1 THEN\n  PRINT a\nELSE\n  PRINT 0\nENDIF\n')).toContain('IfBlock');
  });

  it('survives unbalanced blocks', () => {
    for (const doc of ['ENDIF\nPRINT 1\n', 'FOR i = 1 TO 3\nPRINT i\n', 'WEND\nNEXT\nLOOP\n', '']) {
      expect(() => treeText(doc)).not.toThrow();
    }
    // Tokens after a stray closer are still classified.
    expect(tokens('ENDIF\nPRINT 1\n')).toContain('Statement:PRINT');
  });

  it('folds a block from its first line to the line before its closer', () => {
    const doc = 'WHILE x < 3\n  x = x + 1\n  PRINT x\nWEND\nPRINT "done"\n';
    const state = stateOf(doc);
    ensureSyntaxTree(state, doc.length, 5000);
    const range = foldable(state, 0, state.doc.line(1).to)!;
    expect(range).not.toBeNull();
    expect(doc.slice(range.from, range.to)).toBe('\n  x = x + 1\n  PRINT x');
  });

  it('matches block keywords in both directions', () => {
    const doc = 'REPEAT\n  x = x + 1\nUNTIL x > 3\n';
    const state = stateOf(doc);
    ensureSyntaxTree(state, doc.length, 5000);
    const fromOpen = matchingBlock(state, 2)!;
    expect(doc.slice(fromOpen.close.from, fromOpen.close.to)).toBe('UNTIL');
    const fromClose = matchingBlock(state, doc.indexOf('UNTIL') + 1)!;
    expect(doc.slice(fromClose.open.from, fromClose.open.to)).toBe('REPEAT');
    expect(matchingBlock(state, doc.indexOf('x ='))).toBeNull();
  });

  it('exposes line comment metadata', () => {
    const state = stateOf('PRINT 1\n');
    expect(state.languageDataAt<{ line: string }>('commentTokens', 0)[0].line).toBe("'");
    expect(exbasicLanguage.name).toBe('exbasic');
  });
});

describe('symbol scanner', () => {
  const source = [
    'INCLUDE "lib/util.bas"',
    "DIM Scores(10), Flags(4) BYTE, Name$ ' trailing",
    'DATA Primes() BYTE = 2, 3, 5',
    'FUNC DivMod(A, B%) Quot, Remain',
    '  LOCAL Temp',
    '  Quot = A / 2 : Remain = A - Quot',
    'ENDFUNC',
    'Total = 0 : FOR I = 1 TO 3 : Total = Total + I : NEXT I',
    'PRINT "X = 1 : FUNC Fake(z)"',
    'DLI SET Sky = $10 INTO $D01A',
  ].join('\n');

  it('finds functions, parameters, arrays, data, variables and includes', () => {
    const symbols = scanSymbols(source);
    const names = (kind: string) => symbols.filter(symbol => symbol.kind === kind).map(symbol => symbol.name);
    expect(names('function')).toEqual(['DivMod']);
    expect(names('parameter')).toEqual(['A', 'B%']);
    expect(names('array')).toEqual(['Scores', 'Flags']);
    expect(names('data')).toEqual(['Primes']);
    expect(names('dli')).toEqual(['Sky']);
    expect(names('include')).toEqual(['lib/util.bas']);
    expect(names('variable')).toEqual(['Name$', 'Temp', 'Quot', 'Remain', 'Total', 'I']);
    expect(symbols.some(symbol => symbol.name === 'Fake' || symbol.name === 'X')).toBe(false);
  });

  it('reports exact positions', () => {
    const fn = scanSymbols(source).find(symbol => symbol.name === 'DivMod')!;
    expect([fn.line, fn.column]).toEqual([4, 6]);
    expect(fn.detail).toBe('FUNC DivMod(A, B%) Quot, Remain');
    const flags = scanSymbols(source).find(symbol => symbol.name === 'Flags')!;
    expect(source.split('\n')[1].slice(flags.column - 1, flags.column + 4)).toBe('Flags');
    const total = scanSymbols(source).find(symbol => symbol.name === 'Total')!;
    expect([total.line, total.column]).toEqual([8, 1]);
    const remain = scanSymbols(source).find(symbol => symbol.name === 'Remain')!;
    expect(source.split('\n')[5].slice(remain.column - 1, remain.column + 5)).toBe('Remain');
  });

  it('builds an outline and resolves definitions', () => {
    expect(outline(source).map(symbol => symbol.name)).toEqual(['lib/util.bas', 'Scores', 'Flags', 'Primes', 'DivMod', 'Sky']);
    expect(findDefinition(source, 'divmod')!.line).toBe(4);
    expect(findDefinition(source, 'TOTAL')!.line).toBe(8);
    expect(findDefinition(source, 'Missing')).toBeUndefined();
  });

  it('extracts words and INCLUDE strings under the cursor', () => {
    expect(wordAt('PRINT name$ + 1', 8)).toEqual({ text: 'name$', from: 6, to: 11 });
    expect(wordAt('PRINT 1', 6)).toBeNull();
    expect(includeAt('INCLUDE "lib/a.bas"', 12)).toEqual({ text: 'lib/a.bas', from: 9, to: 18 });
    expect(includeAt('PRINT "lib/a.bas"', 10)).toBeNull();
    expect(splitStatements('A=1:PRINT ":" \' x:y').map(part => part.text)).toEqual(['A=1', 'PRINT ":" ']);
  });
});

describe('signature help', () => {
  it('locates the enclosing call and argument index', () => {
    expect(callAt('x = MID$(a$, 2, ', 16)).toEqual({ name: 'MID$', argument: 2, openParen: 8 });
    expect(callAt('x = Outer(Inner(1, 2), ', 23)).toEqual({ name: 'Outer', argument: 1, openParen: 9 });
    expect(callAt('x = Outer(Inner(1, ', 19)).toEqual({ name: 'Inner', argument: 1, openParen: 15 });
    expect(callAt('PRINT "f(1, "', 12)).toBeNull();
    expect(callAt('PRINT 1 + 2', 11)).toBeNull();
    expect(callAt('PRINT ", (" ; F(', 16)).toEqual({ name: 'F', argument: 0, openParen: 15 });
  });

  it('splits signatures with optional parameters', () => {
    expect(signatureParameters('MID$(s, p [, n])')).toEqual(['s', 'p', '[ n]']);
    expect(signatureParameters('TIME')).toEqual([]);
    expect(signatureParameters('KEY()')).toEqual([]);
  });
});

describe('completion', () => {
  const host: LanguageHost = {
    workspaceSymbols: () => [{ path: 'lib/util.bas', symbol: { name: 'Clamp', kind: 'function', line: 3, column: 6, detail: 'FUNC Clamp(v, lo, hi)' } }],
    includeCandidates: () => ['lib/util.bas', 'src/gfx.bas'],
    currentPath: () => 'src/main.bas',
    openManual: () => {},
  };
  const complete = (doc: string, explicit = false) => {
    const state = EditorState.create({ doc, extensions: [exbasic(host)] });
    ensureSyntaxTree(state, doc.length, 5000);
    return exbasicCompletions(host)(new CompletionContext(state, doc.length, explicit));
  };
  const labels = (doc: string) => complete(doc)?.options.map(option => option.label) ?? [];

  it('offers statements at statement position and functions in expressions', () => {
    const atStart = labels('PR');
    expect(atStart).toContain('PRINT');
    const inExpression = labels('x = PE');
    expect(inExpression).toContain('PEEK');
    expect(inExpression).not.toContain('PRINT');
    expect(labels('IF a THEN PR')).toContain('PRINT');
    expect(labels('a = 1 : PO')).toContain('POKE');
  });

  it('offers local and workspace symbols', () => {
    const got = labels('FUNC Local1(a)\nENDFUNC\nscore = 1\nx = ');
    expect(got).toEqual([]); // nothing typed and not explicit
    const explicit = complete('FUNC Local1(a)\nENDFUNC\nscore = 1\nx = ', true)!.options.map(option => option.label);
    expect(explicit).toContain('Local1');
    expect(explicit).toContain('score');
    expect(explicit).toContain('Clamp');
  });

  it('completes INCLUDE paths and stays quiet in strings and comments', () => {
    const include = complete('INCLUDE "li')!;
    expect(include.options.map(option => option.label)).toEqual(['lib/util.bas', 'src/gfx.bas']);
    expect(include.from).toBe(9);
    expect(complete('PRINT "PR')).toBeNull();
    expect(complete("' PR")).toBeNull();
  });

  it('detects statement positions', () => {
    expect(isStatementPosition('  PR', 2)).toBe(true);
    expect(isStatementPosition('a = 1 : PR', 8)).toBe(true);
    expect(isStatementPosition('IF x THEN PR', 10)).toBe(true);
    expect(isStatementPosition('a = PE', 4)).toBe(false);
  });
});
