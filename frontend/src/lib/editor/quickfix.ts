// One-click fixes for compiler messages whose remedy is mechanical. They
// appear as buttons in the problem's tooltip. Each fix is derived from the
// message text the bundled compiler prints, so an unknown or reworded
// message simply offers nothing.
import type { Text } from '@codemirror/state';

export interface QuickFix {
  /** Button label. */
  name: string;
  from: number;
  /** End of the replaced range; equal to `from` for an insertion. */
  to: number;
  insert: string;
  /** Where the cursor goes afterwards. */
  cursor: number;
}

/** The statement that closes each block, keyed by the compiler's block name. */
const closers: Record<string, string> = {
  'FUNC': 'ENDFUNC',
  'FOR loop': 'NEXT',
  'WHILE loop': 'WEND',
  'DO loop': 'LOOP',
  'REPEAT loop': 'UNTIL ',
  'IF': 'ENDIF',
  'ELSE': 'ENDIF',
  'ELIF': 'ENDIF',
};

const indentOf = (text: string) => /^[ \t]*/.exec(text)![0];
const isBlank = (text: string) => text.trim() === '';

/** Inserts `statement` on its own line after the last non-blank line before `lineNumber`. */
function insertBefore(doc: Text, lineNumber: number, statement: string, indent: string, name: string): QuickFix {
  let previous = lineNumber - 1;
  while (previous >= 1 && isBlank(doc.line(previous).text)) previous--;
  if (previous < 1) {
    const insert = `${indent}${statement}\n`;
    return { name, from: 0, to: 0, insert, cursor: insert.length - 1 };
  }
  const at = doc.line(previous).to;
  const insert = `\n${indent}${statement}`;
  return { name, from: at, to: at, insert, cursor: at + insert.length };
}

/**
 * Closes the block opened on `openLine`: the closing statement goes after
 * the block's last line, where the block is taken to end before the next
 * line indented no deeper than its opening line (or before the next FUNC).
 */
function closeBlock(doc: Text, openLine: number, block: string): QuickFix | null {
  const closer = closers[block];
  if (!closer) return null;
  const indent = indentOf(doc.line(openLine).text);
  let end = doc.lines + 1;
  for (let number = openLine + 1; number <= doc.lines; number++) {
    const text = doc.line(number).text;
    if (isBlank(text)) continue;
    const dedented = indentOf(text).length <= indent.length;
    if (/^\s*FUNC\b/i.test(text) || (dedented && indentOf(text).length < indent.length)) { end = number; break; }
    // A line at the opener's own level ends an indented block. When nothing
    // in the block is indented, only the next FUNC or the end of file does.
    if (dedented && blockIsIndented(doc, openLine, number, indent)) { end = number; break; }
  }
  return insertBefore(doc, end, closer, indent, `Add ${closer.trim()}`);
}

function blockIsIndented(doc: Text, openLine: number, before: number, indent: string): boolean {
  for (let number = openLine + 1; number < before; number++) {
    const text = doc.line(number).text;
    if (!isBlank(text) && indentOf(text).length > indent.length) return true;
  }
  return false;
}

/** Fixes for one compiler message reported at `lineNumber`, `column` (1-based). */
export function quickFixes(doc: Text, message: string, lineNumber: number, column: number): QuickFix[] {
  if (lineNumber < 1 || lineNumber > doc.lines) return [];
  const line = doc.line(lineNumber);
  const fixes: QuickFix[] = [];
  let match: RegExpExecArray | null;

  if (/expected '\(' after FUNC name/i.test(message)) {
    const head = /^\s*FUNC\s+[A-Za-z_][A-Za-z0-9_]*[$%]?/i.exec(line.text);
    if (head) {
      const at = line.from + head[0].length;
      fixes.push({ name: 'Add ()', from: at, to: at, insert: '()', cursor: at + 1 });
    }
  } else if (/FUNC can't be inside another FUNC/i.test(message)) {
    // The function above was never closed.
    fixes.push(insertBefore(doc, lineNumber, 'ENDFUNC', '', 'Add ENDFUNC above'));
  } else if ((match = /\((?:an?) (.+?) is still open\)/i.exec(message))) {
    // "ENDFUNC without FUNC (a FOR loop is still open)"
    const closer = closers[match[1]];
    if (closer) fixes.push(insertBefore(doc, lineNumber, closer, indentOf(line.text) + '  ', `Add ${closer.trim()} above`));
  } else if ((match = /^unclosed (.+)$/i.exec(message.trim()))) {
    const fix = closeBlock(doc, lineNumber, match[1]);
    if (fix) fixes.push(fix);
  } else if (/ without (FUNC|FOR|IF|WHILE|REPEAT|DO)\b/i.test(message)) {
    // A closing statement with nothing to close.
    const to = lineNumber < doc.lines ? doc.line(lineNumber + 1).from : line.to;
    const from = lineNumber < doc.lines ? line.from : (lineNumber > 1 ? doc.line(lineNumber - 1).to : line.from);
    fixes.push({ name: 'Remove this line', from, to, insert: '', cursor: from });
  } else if (/missing '\)' for this '\('/.test(message)) {
    const at = line.from + line.text.trimEnd().length;
    fixes.push({ name: 'Add )', from: at, to: at, insert: ')', cursor: at + 1 });
  } else if (/unterminated string/i.test(message)) {
    const at = line.from + line.text.trimEnd().length;
    fixes.push({ name: 'Add "', from: at, to: at, insert: '"', cursor: at + 1 });
  } else if ((match = /FUNC '([A-Za-z_][A-Za-z0-9_]*[$%]?)' is not defined/i.exec(message))) {
    // Offer a stub at the end of the file, with the cursor in its body.
    const tail = doc.sliceString(Math.max(0, doc.length - 2));
    const lead = doc.length === 0 ? '' : tail.endsWith('\n\n') ? '' : tail.endsWith('\n') ? '\n' : '\n\n';
    const head = `${lead}FUNC ${match[1]}()\n  `;
    fixes.push({ name: `Create FUNC ${match[1]}`, from: doc.length, to: doc.length, insert: `${head}\nENDFUNC\n`, cursor: doc.length + head.length });
  }
  void column;
  return fixes;
}
