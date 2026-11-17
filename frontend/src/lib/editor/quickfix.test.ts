// Quick fixes are derived from the bundled compiler's message text; each
// case below uses a message exactly as the compiler prints it.
import { describe, expect, it } from 'vitest';
import { Text } from '@codemirror/state';
import { quickFixes } from './quickfix';

function fix(source: string, message: string, line: number, column = 1): { name: string; text: string } | null {
  const doc = Text.of(source.split('\n'));
  const [first] = quickFixes(doc, message, line, column);
  if (!first) return null;
  return { name: first.name, text: doc.replace(first.from, first.to, Text.of(first.insert.split('\n'))).toString() };
}

describe('quick fixes', () => {
  it('closes the previous function when a FUNC starts inside another', () => {
    const result = fix('FUNC a()\n  PRINT 1\n\nFUNC b()\n  PRINT 2\nENDFUNC\n', "FUNC can't be inside another FUNC", 4);
    expect(result?.name).toBe('Add ENDFUNC above');
    expect(result?.text).toBe('FUNC a()\n  PRINT 1\nENDFUNC\n\nFUNC b()\n  PRINT 2\nENDFUNC\n');
  });

  it('adds the missing parentheses to a FUNC', () => {
    expect(fix('func oneplusone\n', "expected '(' after FUNC name", 1)?.text).toBe('func oneplusone()\n');
  });

  it('closes an unclosed function at the end of its body', () => {
    expect(fix('FUNC a()\n  PRINT 1\n', 'unclosed FUNC', 1)?.text).toBe('FUNC a()\n  PRINT 1\nENDFUNC\n');
  });

  it('closes an unclosed loop after its indented body, keeping the indent', () => {
    const result = fix('  FOR I = 1 TO 3\n    PRINT I\n  PRINT "DONE"\n', 'unclosed FOR loop', 1);
    expect(result?.name).toBe('Add NEXT');
    expect(result?.text).toBe('  FOR I = 1 TO 3\n    PRINT I\n  NEXT\n  PRINT "DONE"\n');
  });

  it('closes IF, WHILE and DO blocks with their own statements', () => {
    expect(fix('IF A = 1\n  PRINT 1\n', 'unclosed IF', 1)?.text).toBe('IF A = 1\n  PRINT 1\nENDIF\n');
    expect(fix('IF A\n  PRINT 1\nELSE\n  PRINT 2\n', 'unclosed ELSE', 3)?.text).toBe('IF A\n  PRINT 1\nELSE\n  PRINT 2\nENDIF\n');
    expect(fix('WHILE A\n  A = A - 1\n', 'unclosed WHILE loop', 1)?.text).toBe('WHILE A\n  A = A - 1\nWEND\n');
    expect(fix('DO\n  PRINT 1\n', 'unclosed DO loop', 1)?.text).toBe('DO\n  PRINT 1\nLOOP\n');
  });

  it('closes a loop left open before a closing statement', () => {
    const result = fix('FUNC a()\n  FOR I = 1 TO 2\n    PRINT I\nENDFUNC\n', 'ENDFUNC without FUNC (a FOR loop is still open)', 4);
    expect(result?.name).toBe('Add NEXT above');
    expect(result?.text).toBe('FUNC a()\n  FOR I = 1 TO 2\n    PRINT I\n  NEXT\nENDFUNC\n');
  });

  it('removes a closing statement that closes nothing', () => {
    const result = fix('PRINT 1\nENDIF\nPRINT 2\n', 'ENDIF without IF', 2);
    expect(result?.name).toBe('Remove this line');
    expect(result?.text).toBe('PRINT 1\nPRINT 2\n');
  });

  it('closes a parenthesis and a string', () => {
    expect(fix('PRINT (1+2\n', "missing ')' for this '('", 1)?.text).toBe('PRINT (1+2)\n');
    expect(fix('PRINT "HI\n', "unterminated string, missing '\"'", 1)?.text).toBe('PRINT "HI"\n');
  });

  it('creates a stub for an undefined function', () => {
    const result = fix('X = TWICE(2)\n', "parse error, expected: FUNC 'TWICE' is not defined", 1);
    expect(result?.name).toBe('Create FUNC TWICE');
    expect(result?.text).toBe('X = TWICE(2)\n\nFUNC TWICE()\n  \nENDFUNC\n');
  });

  it('offers nothing for a message it does not know', () => {
    expect(fix('PRINT +\n', 'parse error, expected: expression', 1)).toBeNull();
  });
});
