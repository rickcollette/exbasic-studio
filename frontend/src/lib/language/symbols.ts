// Lightweight, tolerant symbol scanner for exBASIC sources. It powers the
// outline, workspace symbols, completion and go-to-definition. The compiler
// remains the authority for semantic diagnostics.

export type SymbolKind = 'function' | 'parameter' | 'variable' | 'array' | 'data' | 'dli' | 'include';

export interface DocSymbol {
  name: string;
  kind: SymbolKind;
  /** 1-based line and column of the defining name. */
  line: number;
  column: number;
  /** Short description: a FUNC signature, an array declaration, a file. */
  detail: string;
}

const NAME = '[A-Za-z_][A-Za-z0-9_]*[$%]?';

/** Splits a line into statements at ":" outside strings and comments. */
export function splitStatements(line: string): { text: string; column: number }[] {
  const parts: { text: string; column: number }[] = [];
  let start = 0;
  let inString = false;
  for (let i = 0; i <= line.length; i++) {
    const ch = line[i];
    if (ch === '"') inString = !inString;
    const comment = ch === "'" && !inString;
    if (i === line.length || comment || (ch === ':' && !inString)) {
      parts.push({ text: line.slice(start, i), column: start });
      start = i + 1;
      if (comment) break;
    }
  }
  return parts;
}

/** Splits at top-level commas (outside parentheses and strings). */
function splitCommas(text: string): { text: string; offset: number }[] {
  const out: { text: string; offset: number }[] = [];
  let depth = 0, start = 0, inString = false;
  for (let i = 0; i <= text.length; i++) {
    const ch = text[i];
    if (ch === '"') inString = !inString;
    if (!inString) {
      if (ch === '(' || ch === '[') depth++;
      if (ch === ')' || ch === ']') depth = Math.max(0, depth - 1);
    }
    if (i === text.length || (ch === ',' && depth === 0 && !inString)) {
      out.push({ text: text.slice(start, i), offset: start });
      start = i + 1;
    }
  }
  return out;
}

export function scanSymbols(source: string): DocSymbol[] {
  const symbols: DocSymbol[] = [];
  const seen = new Set<string>();
  const add = (symbol: DocSymbol, unique = true) => {
    const key = symbol.kind + ':' + symbol.name.toUpperCase();
    if (unique && seen.has(key)) return;
    // A name already declared as an array, DATA or parameter is not also a
    // plain variable.
    if (symbol.kind === 'variable' && ['array', 'data', 'parameter', 'function'].some(kind => seen.has(kind + ':' + symbol.name.toUpperCase()))) return;
    seen.add(key);
    symbols.push(symbol);
  };
  const lines = source.split(/\r\n|\r|\n/);
  lines.forEach((line, index) => {
    const lineNumber = index + 1;
    for (const statement of splitStatements(line)) {
      const text = statement.text;
      const indent = text.length - text.trimStart().length;
      const base = statement.column + indent + 1;
      const body = text.trimStart();
      let match: RegExpExecArray | null;

      if ((match = new RegExp(`^FUNC\\s+(${NAME})\\s*\\(([^)]*)\\)\\s*(.*)$`, 'i').exec(body))) {
        const nameColumn = base + body.indexOf(match[1], 4);
        const results = match[3].trim();
        add({ name: match[1], kind: 'function', line: lineNumber, column: nameColumn, detail: `FUNC ${match[1]}(${match[2].trim()})${results ? ' ' + results : ''}` }, false);
        const paramsStart = body.indexOf('(', 4) + 1;
        for (const param of splitCommas(match[2])) {
          const name = param.text.trim();
          if (new RegExp(`^${NAME}$`).test(name)) {
            add({ name, kind: 'parameter', line: lineNumber, column: base + paramsStart + param.offset + (param.text.length - param.text.trimStart().length), detail: `parameter of ${match[1]}` }, false);
          }
        }
        continue;
      }
      if ((match = /^(DIM|LOCAL|COMMON)\s+(.*)$/i.exec(body))) {
        const listStart = body.length - match[2].length;
        for (const item of splitCommas(match[2])) {
          const declaration = new RegExp(`^\\s*(${NAME})\\s*(\\(([^)]*)\\))?\\s*(BYTE|WORD)?`, 'i').exec(item.text);
          if (!declaration) continue;
          const column = base + listStart + item.offset + item.text.indexOf(declaration[1]);
          const isArray = declaration[2] !== undefined;
          add({
            name: declaration[1], kind: isArray ? 'array' : 'variable', line: lineNumber, column,
            detail: isArray ? `${match[1].toUpperCase()} ${declaration[1]}(${declaration[3].trim()})${declaration[4] ? ' ' + declaration[4].toUpperCase() : ''}` : `${match[1].toUpperCase()} ${declaration[1]}`,
          });
        }
        continue;
      }
      if ((match = new RegExp(`^DATA\\s+(${NAME})\\s*\\(\\s*\\)\\s*(BYTE|WORD|B\\.|W\\.)?`, 'i').exec(body))) {
        add({ name: match[1], kind: 'data', line: lineNumber, column: base + body.indexOf(match[1], 4), detail: `DATA ${match[1]}()${match[2] ? ' ' + match[2].toUpperCase() : ''}` });
        continue;
      }
      if ((match = new RegExp(`^DLI\\s+SET\\s+(${NAME})\\s*=`, 'i').exec(body))) {
        add({ name: match[1], kind: 'dli', line: lineNumber, column: base + body.indexOf(match[1], 7), detail: `DLI ${match[1]}` });
        continue;
      }
      if ((match = /^INCLUDE\s*"([^"]+)"/i.exec(body))) {
        add({ name: match[1], kind: 'include', line: lineNumber, column: base + body.indexOf('"') + 1, detail: `INCLUDE "${match[1]}"` }, false);
        continue;
      }
      if ((match = new RegExp(`^FOR\\s+(${NAME})\\s*=`, 'i').exec(body))) {
        add({ name: match[1], kind: 'variable', line: lineNumber, column: base + body.indexOf(match[1], 3), detail: 'variable' });
        continue;
      }
      if ((match = new RegExp(`^(${NAME})\\s*=(?!=)`).exec(body))) {
        add({ name: match[1], kind: 'variable', line: lineNumber, column: base, detail: 'variable' });
      }
    }
  });
  return symbols;
}

/** Symbols shown in the document outline (structure, not every variable). */
export function outline(source: string): DocSymbol[] {
  return scanSymbols(source).filter(symbol => symbol.kind === 'function' || symbol.kind === 'data' || symbol.kind === 'array' || symbol.kind === 'dli' || symbol.kind === 'include');
}

/** Finds the definition of a name within one document. */
export function findDefinition(source: string, name: string): DocSymbol | undefined {
  const upper = name.toUpperCase();
  const candidates = scanSymbols(source).filter(symbol => symbol.kind !== 'include' && symbol.name.toUpperCase() === upper);
  const rank: Record<SymbolKind, number> = { function: 0, data: 1, array: 2, dli: 3, parameter: 4, variable: 5, include: 6 };
  return candidates.sort((a, b) => rank[a.kind] - rank[b.kind] || a.line - b.line)[0];
}

/** Returns the word (with its type suffix) at a column of a line, if any. */
export function wordAt(line: string, column: number): { text: string; from: number; to: number } | null {
  const pattern = /[A-Za-z_][A-Za-z0-9_]*[$%]?/g;
  let match: RegExpExecArray | null;
  while ((match = pattern.exec(line))) {
    if (column >= match.index && column <= match.index + match[0].length) {
      return { text: match[0], from: match.index, to: match.index + match[0].length };
    }
  }
  return null;
}

/** If the position is inside the string of an INCLUDE directive, returns it. */
export function includeAt(line: string, column: number): { text: string; from: number; to: number } | null {
  const match = /^(\s*INCLUDE\s*")([^"]*)("?)/i.exec(line);
  if (!match) return null;
  const from = match[1].length;
  const to = from + match[2].length;
  return column >= from && column <= to ? { text: match[2], from, to } : null;
}
