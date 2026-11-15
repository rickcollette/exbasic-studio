import { StreamLanguage } from "@codemirror/language";

const keywords = new Set(`FUNC ENDFUNC LOCAL COMMON INCLUDE RUN PROC ENDPROC RETURN IF THEN ELSE ELIF ENDIF END IF
FOR NEXT TO STEP WHILE WEND REPEAT UNTIL DO LOOP EXIT CONTINUE DIM DATA READ RESTORE GOTO GOSUB ON
PRINT INPUT GET PUT OPEN CLOSE NOTE POINT XIO BPUT BGET GRAPHICS COLOR PLOT DRAWTO POSITION
SETCOLOR SOUND POKE DPOKE PEEK DPEEK USR ADR LEN STR CHR VAL INT ABS SGN RND SIN COS ATN SQR
NOPEN NCLOSE NSTATUS NGET NPUT PMGRAPHICS PMMOVE PMCOLOR DLI VBI REM STOP END NEW TRAP ERROR
? LIST TRACE FAST PROC DEF LET AND OR NOT MOD DIV SHL SHR`.split(/\s+/));

export const exbasicLanguage = StreamLanguage.define({
  startState: () => ({ inString: false }),
  token(stream, state) {
    if (state.inString) {
      while (!stream.eol()) {
        const ch = stream.next();
        if (ch === '"') {
          if (stream.peek() === '"') stream.next();
          else { state.inString = false; break; }
        }
      }
      return "string";
    }
    if (stream.eatSpace()) return null;
    if (stream.peek() === "'") { stream.skipToEnd(); return "comment"; }
    if (stream.peek() === '"') { stream.next(); state.inString = true; return "string"; }
    if (stream.match(/^\$[0-9a-fA-F]+/) || stream.match(/^%[01]+/) || stream.match(/^\d+(?:\.\d+)?/)) return "number";
    if (stream.match(/^[A-Za-z_][\w$]*/)) {
      const word = stream.current().toUpperCase();
      if (word === "REM") {stream.skipToEnd();return "comment";}
      return keywords.has(word) ? "keyword" : "variable";
    }
    stream.next(); return null;
  }
});
