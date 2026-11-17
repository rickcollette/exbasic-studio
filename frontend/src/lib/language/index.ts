// CodeMirror language support for exBASIC: highlighting, folding, block
// matching, completion, signature help and hover documentation.
import { LRLanguage, LanguageSupport, foldNodeProp, syntaxTree } from '@codemirror/language';
import { styleTags, tags as t } from '@lezer/highlight';
import type { SyntaxNode } from '@lezer/common';
import { autocompletion, snippetCompletion, type Completion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete';
import { Decoration, type DecorationSet, EditorView, ViewPlugin, type ViewUpdate, hoverTooltip, showTooltip, type Tooltip } from '@codemirror/view';
import { type EditorState, type Extension, StateField } from '@codemirror/state';
import { parser } from './parser.generated.js';
import { catalog, type CatalogEntry } from './catalog.generated';
import { classify, lookupEntry } from './tokens';
import { scanSymbols, includeAt, wordAt, type DocSymbol } from './symbols';

const blockNames = ['FuncBlock', 'IfBlock', 'ForBlock', 'WhileBlock', 'RepeatBlock', 'DoBlock'];
const openNames = new Set(['FuncOpen', 'IfOpen', 'ForOpen', 'WhileOpen', 'RepeatOpen', 'DoOpen']);
const closeNames = new Set(['FuncClose', 'IfClose', 'ForClose', 'WhileClose', 'RepeatClose', 'DoClose']);

export const exbasicLanguage = LRLanguage.define({
  name: 'exbasic',
  parser: parser.configure({
    props: [
      styleTags({
        'FuncOpen FuncClose IfOpen IfClose ForOpen ForClose WhileOpen WhileClose RepeatOpen RepeatClose DoOpen DoClose': t.controlKeyword,
        Statement: t.keyword,
        Keyword: t.operatorKeyword,
        Builtin: t.standard(t.function(t.variableName)),
        Identifier: t.variableName,
        Number: t.number,
        String: t.string,
        'Comment Remark': t.lineComment,
        Operator: t.operator,
        Separator: t.separator,
      }),
      foldNodeProp.add(Object.fromEntries(blockNames.map(name => [name, (node: SyntaxNode, state: EditorState) => {
        const first = node.firstChild, last = node.lastChild;
        if (!first || !last || first === last || !closeNames.has(last.name)) return null;
        const from = state.doc.lineAt(first.to).to;
        const to = state.doc.lineAt(last.from).from - 1;
        return to > from ? { from, to } : null;
      }]))),
    ],
  }),
  languageData: { commentTokens: { line: "'" }, closeBrackets: { brackets: ['(', '[', '"'] } },
});

// ---- Hooks supplied by the workbench ---------------------------------------

/** Everything the language features need from the surrounding application. */
export interface LanguageHost {
  /** Symbols defined in other workspace files, with the file they are in. */
  workspaceSymbols(): { path: string; symbol: DocSymbol }[];
  /** Workspace-relative paths that an INCLUDE in `fromPath` could name. */
  includeCandidates(fromPath: string): string[];
  /** Path of the document being edited. */
  currentPath(): string;
  /** Opens the manual at a chapter and heading. */
  openManual(chapter: string, anchor: string): void;
}

const noHost: LanguageHost = { workspaceSymbols: () => [], includeCandidates: () => [], currentPath: () => '', openManual: () => {} };

// ---- Context helpers -------------------------------------------------------

function inCommentOrString(state: EditorState, pos: number): 'comment' | 'string' | null {
  const node = syntaxTree(state).resolveInner(pos, -1);
  if (node.name === 'Comment' || node.name === 'Remark') return 'comment';
  if (node.name === 'String' && pos > node.from && (pos < node.to || !/"$/.test(state.sliceDoc(node.from, node.to)) || node.to - node.from < 2)) return 'string';
  return null;
}

/** True when a statement may start at this position of the line. */
export function isStatementPosition(lineText: string, column: number): boolean {
  const before = lineText.slice(0, column).replace(/[A-Za-z0-9_.$%]*$/, '');
  const trimmed = before.trimEnd();
  if (trimmed === '') return true;
  if (trimmed.endsWith(':')) return true;
  return /\b(THEN|ELSE)$/i.test(trimmed);
}

function kindLabel(entry: CatalogEntry): string {
  return entry.kind === 'statement' ? 'keyword' : entry.kind === 'function' ? 'function' : entry.kind === 'directive' ? 'keyword' : 'constant';
}

function symbolCompletion(symbol: DocSymbol, path?: string): Completion {
  const type = symbol.kind === 'function' ? 'function' : symbol.kind === 'include' ? 'text' : 'variable';
  return {
    label: symbol.name, type,
    detail: path ? path : symbol.kind,
    info: symbol.detail,
    apply: symbol.kind === 'function' ? symbol.name + '(' : symbol.name,
    boost: symbol.kind === 'function' ? 2 : 1,
  };
}

// ---- Completion ------------------------------------------------------------

/** exBASIC source files: programs and the files they INCLUDE. */
export function isExbasicSource(path: string): boolean { return /\.(bas|inc)$/i.test(path); }

/**
 * Block templates offered at the start of a statement. Tab moves between
 * the placeholders after one is inserted.
 */
const blockSnippets: Completion[] = ([
  ['FUNC', 'FUNC ${name}(${})\n  ${}\nENDFUNC', 'FUNC name() … ENDFUNC', 'Define a function'],
  ['IF', 'IF ${condition}\n  ${}\nENDIF', 'IF … ENDIF', 'Conditional block'],
  ['IF ELSE', 'IF ${condition}\n  ${}\nELSE\n  ${}\nENDIF', 'IF … ELSE … ENDIF', 'Conditional block with an alternative'],
  ['FOR', 'FOR ${I} = ${1} TO ${10}\n  ${}\nNEXT ${I}', 'FOR … NEXT', 'Counted loop'],
  ['WHILE', 'WHILE ${condition}\n  ${}\nWEND', 'WHILE … WEND', 'Loop while a condition holds'],
  ['REPEAT', 'REPEAT\n  ${}\nUNTIL ${condition}', 'REPEAT … UNTIL', 'Loop until a condition holds'],
  ['DO', 'DO\n  ${}\nLOOP', 'DO … LOOP', 'Endless loop'],
] as const).map(([label, template, detail, info]) => snippetCompletion(template, { label, detail, info, type: 'snippet', boost: 3 }));

export function exbasicCompletions(host: LanguageHost) {
  return (context: CompletionContext): CompletionResult | null => {
    const line = context.state.doc.lineAt(context.pos);
    const column = context.pos - line.from;

    // INCLUDE "..." completes workspace file paths.
    const include = includeAt(line.text, column);
    if (include) {
      const options = host.includeCandidates(host.currentPath()).map(path => ({ label: path, type: 'text' }));
      return { from: line.from + include.from, to: line.from + include.to, options, validFor: /^[^"]*$/ };
    }
    if (inCommentOrString(context.state, context.pos)) return null;

    const word = context.matchBefore(/[A-Za-z_][A-Za-z0-9_]*[$%]?/);
    if (!word && !context.explicit) return null;
    const from = word ? word.from : context.pos;
    const statementPosition = isStatementPosition(line.text, from - line.from);

    const options: Completion[] = [];
    for (const entry of catalog) {
      if (!/^[A-Z]/.test(entry.name)) continue;
      const isStatement = entry.kind === 'statement' || entry.kind === 'directive';
      if (isStatement !== statementPosition && !(statementPosition && entry.kind === 'function')) continue;
      options.push({
        label: entry.name,
        type: kindLabel(entry),
        detail: entry.abbr ? `${entry.signature}   ${entry.abbr}` : entry.signature,
        info: entry.summary || undefined,
        apply: entry.kind === 'function' && entry.signature.includes('(') ? entry.name + '(' : entry.name,
      });
    }
    if (statementPosition) options.push(...blockSnippets);
    const local = scanSymbols(context.state.doc.toString());
    const seen = new Set<string>();
    for (const symbol of local) {
      if (symbol.kind === 'include') continue;
      const key = symbol.name.toUpperCase();
      if (seen.has(key)) continue;
      seen.add(key);
      options.push(symbolCompletion(symbol));
    }
    for (const { path, symbol } of host.workspaceSymbols()) {
      if (symbol.kind !== 'function' || seen.has(symbol.name.toUpperCase())) continue;
      seen.add(symbol.name.toUpperCase());
      options.push(symbolCompletion(symbol, path));
    }
    return { from, options, validFor: /^[A-Za-z_][A-Za-z0-9_]*[$%]?$/ };
  };
}

// ---- Signature help --------------------------------------------------------

export interface CallContext { name: string; argument: number; openParen: number }

/** Finds the innermost unclosed call on the line before the cursor. */
export function callAt(lineText: string, column: number): CallContext | null {
  let depth = 0, argument = 0, inString = false;
  // Track strings from the start of the line so quotes pair correctly.
  const stringAt: boolean[] = [];
  for (let i = 0; i < column; i++) {
    if (lineText[i] === '"') inString = !inString;
    stringAt[i] = inString || lineText[i] === '"';
  }
  if (inString) return null;
  for (let i = column - 1; i >= 0; i--) {
    if (stringAt[i]) continue;
    const ch = lineText[i];
    if (ch === "'") return null;
    if (ch === ')' || ch === ']') depth++;
    else if (ch === '[') { if (depth === 0) return null; depth--; }
    else if (ch === ',' && depth === 0) argument++;
    else if (ch === '(') {
      if (depth > 0) { depth--; continue; }
      const match = /([A-Za-z_][A-Za-z0-9_]*[$%]?)\s*$/.exec(lineText.slice(0, i));
      return match ? { name: match[1], argument, openParen: i } : null;
    }
  }
  return null;
}

function signatureFor(name: string, state: EditorState, host: LanguageHost): string | null {
  const upper = name.toUpperCase();
  const entry = catalog.find(item => item.name === upper && item.signature.includes('('));
  if (entry) return entry.signature;
  const local = scanSymbols(state.doc.toString()).find(symbol => symbol.kind === 'function' && symbol.name.toUpperCase() === upper);
  if (local) return local.detail;
  const remote = host.workspaceSymbols().find(item => item.symbol.kind === 'function' && item.symbol.name.toUpperCase() === upper);
  return remote ? remote.symbol.detail : null;
}

/** Splits "MID$(s, p [, n])" into its parameter texts. */
export function signatureParameters(signature: string): string[] {
  const open = signature.indexOf('('), close = signature.lastIndexOf(')');
  if (open < 0 || close < open) return [];
  const inner = signature.slice(open + 1, close).trim();
  if (!inner) return [];
  return inner.replace(/\[\s*,/g, ',[').split(',').map(part => part.trim());
}

function signatureTooltip(state: EditorState, host: LanguageHost): Tooltip | null {
  const selection = state.selection.main;
  if (!selection.empty) return null;
  const line = state.doc.lineAt(selection.head);
  const call = callAt(line.text, selection.head - line.from);
  if (!call) return null;
  const signature = signatureFor(call.name, state, host);
  if (!signature) return null;
  return {
    pos: line.from + call.openParen,
    above: true,
    strictSide: true,
    arrow: false,
    create() {
      const dom = document.createElement('div');
      dom.className = 'cm-signature-help';
      dom.setAttribute('role', 'status');
      const open = signature.indexOf('(');
      const params = signatureParameters(signature);
      dom.append(signature.slice(0, open + 1));
      params.forEach((param, index) => {
        const span = document.createElement(index === Math.min(call.argument, params.length - 1) ? 'strong' : 'span');
        span.textContent = param;
        dom.append(span);
        if (index < params.length - 1) dom.append(', ');
      });
      dom.append(signature.slice(signature.lastIndexOf(')')));
      return { dom };
    },
  };
}

function signatureHelp(host: LanguageHost): Extension {
  return StateField.define<Tooltip | null>({
    create: state => signatureTooltip(state, host),
    update(value, transaction) {
      return transaction.docChanged || transaction.selection ? signatureTooltip(transaction.state, host) : value;
    },
    provide: field => showTooltip.from(field),
  });
}

// ---- Hover -----------------------------------------------------------------

function hoverDocs(host: LanguageHost): Extension {
  return hoverTooltip((view, pos) => {
    if (inCommentOrString(view.state, pos)) return null;
    const line = view.state.doc.lineAt(pos);
    const word = wordAt(line.text, pos - line.from);
    if (!word) return null;
    const abbreviated = line.text[word.to] === '.';
    const result = classify(word.text, abbreviated, isStatementPosition(line.text, word.from));
    const entry = result.cls === 'identifier' ? undefined : lookupEntry(result.name, result.cls);
    let title = '', body = '', chapter = '', anchor = '';
    if (entry) {
      title = entry.signature;
      body = entry.summary + (entry.abbr ? ` (abbreviation ${entry.abbr})` : '');
      chapter = entry.chapter; anchor = entry.anchor;
    } else {
      const upper = word.text.toUpperCase();
      const local = scanSymbols(view.state.doc.toString()).find(symbol => symbol.kind !== 'include' && symbol.name.toUpperCase() === upper);
      const remote = local ? undefined : host.workspaceSymbols().find(item => item.symbol.name.toUpperCase() === upper && item.symbol.kind === 'function');
      if (!local && !remote) return null;
      title = (local ?? remote!.symbol).detail;
      body = local ? `Defined on line ${local.line}` : `Defined in ${remote!.path}`;
    }
    return {
      pos: line.from + word.from, end: line.from + word.to, above: false,
      create() {
        const dom = document.createElement('div');
        dom.className = 'cm-hover-doc';
        const heading = document.createElement('code');
        heading.textContent = title;
        const text = document.createElement('p');
        text.textContent = body;
        dom.append(heading, text);
        if (chapter) {
          const link = document.createElement('button');
          link.type = 'button';
          link.textContent = 'Open in manual';
          link.onclick = () => host.openManual(chapter, anchor);
          dom.append(link);
        }
        return { dom };
      },
    };
  }, { hoverTime: 350 });
}

// ---- Block matching --------------------------------------------------------

const matchMark = Decoration.mark({ class: 'cm-block-match' });

/** Returns the open and close keyword ranges of the block at a position. */
export function matchingBlock(state: EditorState, pos: number): { open: { from: number; to: number }; close: { from: number; to: number } } | null {
  for (const side of [-1, 1] as const) {
    const node = syntaxTree(state).resolveInner(pos, side);
    if (!openNames.has(node.name) && !closeNames.has(node.name)) continue;
    const block = node.parent;
    if (!block || !blockNames.includes(block.name)) continue;
    const first = block.firstChild, last = block.lastChild;
    if (!first || !last || !openNames.has(first.name) || !closeNames.has(last.name)) continue;
    return { open: { from: first.from, to: first.to }, close: { from: last.from, to: last.to } };
  }
  return null;
}

const blockMatching = ViewPlugin.fromClass(class {
  decorations: DecorationSet;
  constructor(view: EditorView) { this.decorations = this.compute(view); }
  update(update: ViewUpdate) {
    if (update.docChanged || update.selectionSet) this.decorations = this.compute(update.view);
  }
  compute(view: EditorView): DecorationSet {
    const match = matchingBlock(view.state, view.state.selection.main.head);
    if (!match) return Decoration.none;
    return Decoration.set([matchMark.range(match.open.from, match.open.to), matchMark.range(match.close.from, match.close.to)]);
  }
}, { decorations: plugin => plugin.decorations });

// ---- Bundle ----------------------------------------------------------------

export function exbasic(host: LanguageHost = noHost): LanguageSupport {
  return new LanguageSupport(exbasicLanguage, [
    autocompletion({ override: [exbasicCompletions(host)], icons: true, activateOnTyping: true, selectOnOpen: true }),
    signatureHelp(host),
    hoverDocs(host),
    blockMatching,
  ]);
}

export { scanSymbols, outline, findDefinition, wordAt, includeAt, type DocSymbol } from './symbols';
export { catalog } from './catalog.generated';
