// Generates the exBASIC language catalog used by the editor.
//
// Sources of truth:
//   * the compiler's active syntax definitions (the .syn files named by the
//     `syntax` line of every target file), which decide what is a keyword
//     and how far it can be abbreviated;
//   * Appendix A of the bundled manual, which supplies the display
//     signature, the one-line summary and the chapter link.
//
// Output (checked in; CI fails when it drifts):
//   src/lib/language/catalog.generated.ts
//   src/lib/language/parser.generated.js   (Lezer tables for exbasic.grammar)
//
// Usage: node scripts/gen-language.mjs [--check]
import { readFileSync, writeFileSync, readdirSync, existsSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildParserFile } from '@lezer/generator';

const here = dirname(fileURLToPath(import.meta.url));
const repo = resolve(here, '..', '..');
const compilerDir = join(repo, 'third_party', 'exbasic', 'source', 'compiler');
const syntaxDir = join(repo, 'third_party', 'exbasic', 'source', 'src', 'syntax');
const appendix = join(repo, 'internal', 'manual', 'chapters', 'A-keyword-reference.md');
const outDir = join(here, '..', 'src', 'lib', 'language');
const check = process.argv.includes('--check');

// ---- .syn parsing ---------------------------------------------------------

/** Names of the syntax files used by at least one compiler target. */
function activeSyntaxFiles() {
  const names = new Set();
  for (const file of readdirSync(compilerDir).filter(name => name.endsWith('.tgt')).sort()) {
    const text = readFileSync(join(compilerDir, file), 'utf8').replace(/\\\r?\n/g, ' ');
    for (const line of text.split(/\r?\n/)) {
      const match = /^\s*syntax\s+(.*)$/.exec(line);
      if (match) for (const name of match[1].trim().split(/\s+/)) names.add(name);
    }
  }
  if (!names.size) throw new Error(`no syntax files are named by the targets in ${compilerDir}`);
  return [...names].sort();
}

/** Splits one grammar line into quoted literals and bare words. */
function tokenize(line) {
  const tokens = [];
  let i = 0;
  while (i < line.length) {
    const ch = line[i];
    if (ch === '#') break;
    if (/\s/.test(ch)) { i++; continue; }
    if (ch === '"') {
      let text = '';
      i++;
      while (i < line.length && line[i] !== '"') {
        if (line[i] === '\\' && i + 1 < line.length) i++;
        text += line[i++];
      }
      i++;
      tokens.push({ literal: text });
      continue;
    }
    if (ch === '{') {
      const end = line.indexOf('}', i);
      i = end < 0 ? line.length : end + 1;
      continue;
    }
    let word = '';
    while (i < line.length && !/[\s"{#]/.test(line[i])) word += line[i++];
    tokens.push({ word });
  }
  return tokens;
}

/** Returns {rule name -> alternatives (token arrays)} merged over all files. */
function parseSyntax(files) {
  const rules = new Map();
  for (const file of files) {
    const text = readFileSync(join(syntaxDir, file), 'utf8').replace(/\\\r?\n/g, ' ');
    let current = null;
    let inBlock = false;
    for (const raw of text.split(/\r?\n/)) {
      const stripped = raw.replace(/^\s+|\s+$/g, '');
      if (!stripped || stripped.startsWith('#')) continue;
      if (inBlock) { if (stripped.startsWith('}')) inBlock = false; continue; }
      if (/^[A-Z_]+\s*\{/.test(raw)) { inBlock = !stripped.includes('}'); current = null; continue; }
      const header = /^([A-Z_0-9]+):<?/.exec(raw);
      if (header) {
        current = header[1];
        if (!rules.has(current)) rules.set(current, []);
        continue;
      }
      if (current && /^\s/.test(raw)) {
        const tokens = tokenize(raw).filter(token => token.word !== 'emit');
        if (tokens.length) rules.get(current).push(tokens);
      }
    }
  }
  return rules;
}

/** "PRInt" -> {name: "PRINT", abbr: "PR."}; "CLS" -> {name: "CLS", abbr: ""}. */
function keywordFromLiteral(literal) {
  const match = /^-?([A-Za-z][A-Za-z0-9_]*\$?)(\(\)?)?$/.exec(literal);
  if (!match) return null;
  const word = match[1];
  const name = word.toUpperCase();
  const prefix = /^[A-Z]+/.exec(word)?.[0] ?? '';
  const letters = word.replace(/\$$/, '');
  const abbr = prefix.length > 0 && prefix.length < letters.length ? prefix + '.' : '';
  return { name: literal.startsWith('-') ? '-' + name : name, abbr };
}

function collectKeywords(rules) {
  const found = new Map();
  const add = (keyword, kind) => {
    const key = keyword.name + '|' + kind;
    const previous = found.get(key);
    // The shortest abbreviation accepted anywhere wins.
    if (!previous || (keyword.abbr && (!previous.abbr || keyword.abbr.length < previous.abbr.length))) {
      found.set(key, { ...keyword, kind });
    }
  };
  for (const [rule, alternatives] of rules) {
    const functionRule = /FUNCTIONS$|^FP_FUNCS$/.test(rule);
    for (const tokens of alternatives) {
      tokens.forEach((token, index) => {
        if (token.literal === undefined) return;
        const keyword = keywordFromLiteral(token.literal);
        if (!keyword) return;
        if (index === 0 && rule === 'STATEMENT') add(keyword, 'statement');
        else if (index === 0 && functionRule) add(keyword, 'function');
        else add(keyword, 'word');
      });
    }
  }
  return [...found.values()];
}

// ---- Appendix A parsing ----------------------------------------------------

function parseAppendix() {
  const rows = [];
  const text = readFileSync(appendix, 'utf8');
  for (const line of text.split(/\r?\n/)) {
    if (!line.startsWith('| `')) continue;
    // Table cells may contain an escaped pipe (the `\|` operator).
    const cells = line.split(/(?<!\\)\|/).slice(1, -1).map(cell => cell.trim().replace(/\\\|/g, '|'));
    if (cells.length < 5) continue;
    const display = /^`([^`]+)`(.*)$/.exec(cells[0]);
    if (!display) continue;
    const signature = display[1];
    const context = display[2].trim();
    const name = signature.replace(/\(.*$/, '').replace(/\s.*$/, '').toUpperCase();
    const abbr = /^`([^`]+)`$/.exec(cells[1])?.[1] ?? '';
    const link = /\[[^\]]*\]\(([^)#]+)(?:#([^)]+))?\)/.exec(cells[4]);
    rows.push({
      name, signature, context, abbr, kind: cells[2].replace(/\*/g, ''),
      summary: cells[3].replace(/`/g, ''),
      chapter: link?.[1] ?? '', anchor: link?.[2] ?? '',
    });
  }
  if (rows.length < 50) throw new Error(`only ${rows.length} keyword rows were read from ${appendix}`);
  return rows;
}

// ---- Catalog ---------------------------------------------------------------

function buildCatalog() {
  const files = activeSyntaxFiles();
  for (const file of files) if (!existsSync(join(syntaxDir, file))) throw new Error(`missing syntax file ${file}`);
  const keywords = collectKeywords(parseSyntax(files));
  const documented = parseAppendix();
  const entries = [];
  const used = new Set();
  const kindOf = { statement: 'statement', function: 'function', word: 'word' };
  for (const keyword of keywords) {
    const matches = documented.filter(row => row.name === keyword.name);
    const row = matches.find(item => item.kind === kindOf[keyword.kind])
      ?? matches.find(item => keyword.kind === 'word' && (item.kind === 'operator' || item.kind === 'word'))
      ?? null;
    if (row) used.add(row);
    entries.push({
      name: keyword.name, abbr: keyword.abbr, kind: row?.kind ?? keyword.kind,
      signature: row?.signature ?? keyword.name, summary: row?.summary ?? '',
      chapter: row?.chapter ?? '', anchor: row?.anchor ?? '', source: 'syntax',
    });
  }
  // Directives and symbols are handled outside the .syn grammar (INCLUDE is
  // expanded before parsing) and come from the manual alone.
  for (const row of documented) {
    if (used.has(row) || !/^[A-Z]/.test(row.name)) continue;
    if (entries.some(entry => entry.name === row.name && entry.kind === row.kind)) continue;
    entries.push({
      name: row.name, abbr: row.abbr === '-' ? '' : row.abbr, kind: row.kind,
      signature: row.signature, summary: row.summary, chapter: row.chapter, anchor: row.anchor, source: 'manual',
    });
  }
  entries.sort((a, b) => a.name.localeCompare(b.name) || a.kind.localeCompare(b.kind));
  return { files, entries };
}

function catalogSource({ files, entries }) {
  const lines = [
    '// GENERATED by scripts/gen-language.mjs from the exBASIC syntax definitions',
    '// and Appendix A of the manual. Do not edit; run `npm run gen:language`.',
    '',
    'export type CatalogKind = \'statement\' | \'function\' | \'operator\' | \'word\' | \'directive\' | \'symbol\';',
    '',
    'export interface CatalogEntry {',
    '  /** Upper-case keyword as the compiler spells it. */',
    '  name: string;',
    '  /** Shortest accepted abbreviation including the dot, or "". */',
    '  abbr: string;',
    '  kind: CatalogKind;',
    '  /** Display form, e.g. "MID$(s, start, n)". */',
    '  signature: string;',
    '  summary: string;',
    '  /** Manual chapter file and heading anchor documenting the keyword. */',
    '  chapter: string;',
    '  anchor: string;',
    '}',
    '',
    `export const syntaxFiles: readonly string[] = ${JSON.stringify(files)};`,
    '',
    'export const catalog: readonly CatalogEntry[] = [',
    ...entries.map(entry => {
      const { source, ...rest } = entry;
      return '  ' + JSON.stringify(rest) + ',';
    }),
    '];',
    '',
  ];
  return lines.join('\n');
}

// ---- Outputs ---------------------------------------------------------------

const outputs = new Map();
outputs.set(join(outDir, 'catalog.generated.ts'), catalogSource(buildCatalog()));
const grammar = readFileSync(join(outDir, 'exbasic.grammar'), 'utf8');
const built = buildParserFile(grammar, { fileName: 'exbasic.grammar', moduleStyle: 'es', warn: message => { throw new Error(message); } });
outputs.set(join(outDir, 'parser.generated.js'),
  '// GENERATED by scripts/gen-language.mjs from exbasic.grammar. Do not edit.\n' + built.parser);
outputs.set(join(outDir, 'parser.terms.generated.js'),
  '// GENERATED by scripts/gen-language.mjs from exbasic.grammar. Do not edit.\n' + built.terms);

let drift = false;
for (const [path, content] of outputs) {
  const normalized = content.replace(/\r\n/g, '\n');
  const existing = existsSync(path) ? readFileSync(path, 'utf8').replace(/\r\n/g, '\n') : null;
  if (existing === normalized) continue;
  if (check) {
    console.error(`out of date: ${path}`);
    drift = true;
  } else {
    writeFileSync(path, normalized);
    console.log(`wrote ${path}`);
  }
}
if (drift) {
  console.error('Generated language files do not match the syntax definitions. Run `npm run gen:language` and commit the result.');
  process.exit(1);
}
