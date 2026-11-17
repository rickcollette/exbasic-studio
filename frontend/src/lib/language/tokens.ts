// External tokenizer for the exBASIC grammar: classifies words using the
// catalog generated from the compiler's syntax definitions.
import { ExternalTokenizer, type InputStream } from '@lezer/lr';
import { catalog, type CatalogEntry } from './catalog.generated';
import {
  FuncOpen, FuncClose, IfOpen, IfClose, ForOpen, ForClose, WhileOpen, WhileClose,
  RepeatOpen, RepeatClose, DoOpen, DoClose, Statement, Keyword, Builtin, Identifier, Remark,
} from './parser.terms.generated.js';

const statements = new Map<string, CatalogEntry>();
const functions = new Map<string, CatalogEntry>();
const wordTable = new Map<string, CatalogEntry>();
for (const entry of catalog) {
  const table = entry.kind === 'statement' || entry.kind === 'directive' ? statements
    : entry.kind === 'function' ? functions : wordTable;
  if (!table.has(entry.name)) table.set(entry.name, entry);
}

/**
 * Expands a dot abbreviation ("PR." -> "PRINT") within one keyword table.
 * An abbreviation must be at least as long as the entry's shortest one, and
 * the shortest keyword that it prefixes wins, as in the compiler.
 */
export function expand(table: Map<string, CatalogEntry>, typed: string): string | null {
  let best: string | null = null;
  for (const entry of table.values()) {
    if (!entry.abbr || !entry.name.startsWith(typed)) continue;
    if (typed.length < entry.abbr.length - 1 || typed.length >= entry.name.length) continue;
    if (best === null || entry.name.length < best.length) best = entry.name;
  }
  return best;
}

export type WordClass = 'statement' | 'function' | 'word' | 'identifier';

/** Classifies a word the way the tokenizer does; exported for completion and hover. */
export function classify(word: string, abbreviated: boolean, statementPosition: boolean): { cls: WordClass; name: string } {
  const upper = word.toUpperCase();
  const lookup = (table: Map<string, CatalogEntry>) => (abbreviated ? expand(table, upper) : table.has(upper) ? upper : null);
  if (statementPosition) {
    const statement = lookup(statements);
    if (statement) return { cls: 'statement', name: statement };
  }
  const fn = lookup(functions);
  if (fn) return { cls: 'function', name: fn };
  const keyword = lookup(wordTable);
  if (keyword) return { cls: 'word', name: keyword };
  if (!statementPosition) {
    // Statement keywords such as TO-less words never appear mid-expression,
    // but ELSE/ENDIF after THEN on one line do.
    const statement = lookup(statements);
    if (statement && (statement === 'ELSE' || statement === 'ENDIF' || statement === 'ELIF')) return { cls: 'statement', name: statement };
  }
  return { cls: 'identifier', name: upper };
}

export function lookupEntry(name: string, cls: WordClass): CatalogEntry | undefined {
  const upper = name.toUpperCase();
  return cls === 'statement' ? statements.get(upper) : cls === 'function' ? functions.get(upper) : wordTable.get(upper) ?? statements.get(upper) ?? functions.get(upper);
}

const NEWLINE = 10, RETURN = 13, SPACE = 32, TAB = 9, COLON = 58, DOT = 46, QUOTE = 34, APOSTROPHE = 39, DOLLAR = 36, PERCENT = 37, EOF = -1;

function isLetter(ch: number) { return (ch >= 65 && ch <= 90) || (ch >= 97 && ch <= 122) || ch === 95; }
function isWordChar(ch: number) { return isLetter(ch) || (ch >= 48 && ch <= 57); }

/** Reads the word that ends just before offset (relative to input.pos), upper-cased. */
function wordBefore(input: InputStream, offset: number): string {
  let text = '';
  for (let i = offset; i > offset - 16; i--) {
    const ch = input.peek(i);
    if (!isWordChar(ch)) break;
    text = String.fromCharCode(ch) + text;
  }
  return text.toUpperCase();
}

/**
 * Describes what precedes the token: whether it is the first thing on its
 * line and whether a statement may start here (line start, after ":" or
 * after THEN/ELSE).
 */
function context(input: InputStream): { first: boolean; statement: boolean } {
  let i = -1;
  while (input.peek(i) === SPACE || input.peek(i) === TAB) i--;
  const ch = input.peek(i);
  if (ch === EOF || ch === NEWLINE || ch === RETURN) return { first: true, statement: true };
  if (ch === COLON) return { first: false, statement: true };
  const previous = wordBefore(input, i);
  return { first: false, statement: previous === 'THEN' || previous === 'ELSE' };
}

/** Text from offset to the end of the line, without the trailing comment. */
function restOfLine(input: InputStream, offset: number): string {
  let text = '';
  let inString = false;
  for (let i = offset; i < offset + 512; i++) {
    const ch = input.peek(i);
    if (ch === EOF || ch === NEWLINE || ch === RETURN) break;
    if (ch === QUOTE) inString = !inString;
    if (ch === APOSTROPHE && !inString) break;
    text += inString && ch !== QUOTE ? '_' : String.fromCharCode(ch);
  }
  return text;
}

const openers: Record<string, number> = { FUNC: FuncOpen, IF: IfOpen, FOR: ForOpen, WHILE: WhileOpen, REPEAT: RepeatOpen, DO: DoOpen };
const closers: Record<string, number> = { ENDFUNC: FuncClose, ENDIF: IfClose, NEXT: ForClose, WEND: WhileClose, UNTIL: RepeatClose, LOOP: DoClose };

export const words = new ExternalTokenizer(input => {
  const start = input.next;
  const where = context(input);

  // A statement that starts with "." is a remark to the end of the line.
  if (start === DOT && where.statement) {
    const after = input.peek(1);
    if (!(after >= 48 && after <= 57)) {
      while (input.next !== EOF && input.next !== NEWLINE && input.next !== RETURN) input.advance();
      input.acceptToken(Remark);
      return;
    }
  }
  if (!isLetter(start)) return;

  let word = '';
  while (isWordChar(input.next)) {
    word += String.fromCharCode(input.next);
    input.advance();
  }
  let abbreviated = false;
  if (input.next === DOLLAR) {
    // String functions and variables carry the "$" in their name.
    if (functions.has(word.toUpperCase() + '$') || !classifyHasKeyword(word)) {
      word += '$';
      input.advance();
    }
  } else if (input.next === PERCENT && !classifyHasKeyword(word)) {
    // Floating point variables and functions end in "%".
    word += '%';
    input.advance();
  } else if (input.next === DOT) {
    // "PR." is an abbreviation only when it expands to a keyword here.
    const expanded = classify(word, true, where.statement);
    if (expanded.cls !== 'identifier') {
      abbreviated = true;
      input.advance();
    }
  }

  const result = classify(word, abbreviated, where.statement);
  if (result.cls === 'identifier') { input.acceptToken(Identifier); return; }
  if (result.cls === 'function') { input.acceptToken(Builtin); return; }
  if (result.cls === 'word') { input.acceptToken(Keyword); return; }

  if (result.name === 'REM') {
    while (input.next !== EOF && input.next !== NEWLINE && input.next !== RETURN) input.advance();
    input.acceptToken(Remark);
    return;
  }
  // Block tokens are produced only for the first statement of a line whose
  // block really continues on later lines; everything else is a plain
  // statement keyword, which keeps one-line forms from unbalancing the tree.
  if (where.first) {
    const rest = restOfLine(input, 0).toUpperCase();
    const singleLine = rest.includes(':');
    const opener = openers[result.name];
    if (opener !== undefined && !singleLine) {
      if (result.name !== 'IF') { input.acceptToken(opener); return; }
      const then = /\bTH(?:EN|E\.|\.)/.exec(rest);
      if (!then || rest.slice(then.index + then[0].length).trim() === '') { input.acceptToken(opener); return; }
    }
    const closer = closers[result.name];
    if (closer !== undefined) { input.acceptToken(closer); return; }
  }
  input.acceptToken(Statement);
}, { contextual: false });

function classifyHasKeyword(word: string): boolean {
  const upper = word.toUpperCase();
  return statements.has(upper) || functions.has(upper) || wordTable.has(upper);
}
