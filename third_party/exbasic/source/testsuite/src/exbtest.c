/*
 * FastBasic - Fast basic interpreter for the Atari 8-bit computers
 * Copyright (C) 2017-2025 Daniel Serpell
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program.  If not, see <http://www.gnu.org/licenses/>
 */

// exbtest.c: Runs tests of the compiler and interpreter.

#define _GNU_SOURCE // for asprintf
#include "atari.h"
#include "sim65.h"
#include "atcio.h"
#include <errno.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#ifdef _WIN32
# include <direct.h>
# define MKDIR(path) _mkdir(path)
# define PATH_SEP "\\"
#else
# define MKDIR(path) mkdir(path, 0777)
# define PATH_SEP "/"
#endif

// Private strndup replacement: avoid conflicting with libc/POSIX strndup,
// which newer MinGW headers now declare.
static char *xstrndup(const char *s, size_t n)
{
    size_t len = 0;
    while (len < n && s[len])
        len++;
    char *ret = malloc(len + 1);
    if (!ret)
        return NULL;
    memcpy(ret, s, len);
    ret[len] = 0;
    return ret;
}

// Flags
static int verbose;
static const char *fb_compiler       = "build" PATH_SEP "bin" PATH_SEP "exbasic";
static const char *ca65_path         = "build" PATH_SEP "bin" PATH_SEP "ca65";
static const char *ld65_path         = "build" PATH_SEP "bin" PATH_SEP "ld65";
static const char *fb_lib_path       = "build/compiler";
static const char *output_dir        = "build/tests";

#define CA65_OPTS   "-t atari -g"
#define FB_LIB      "exbasic.lib"
#define FB_CFG_FILE "exbasic.cfg"
#define FB_PATHS    "-target-path:compiler -syntax-path:src/syntax"
#define FB_TARGET   "-t:atari"
#define FB_LIB_ROM  "exbasic-cart.lib"
#define FB_CFG_FILE_ROM "exbasic-cart.cfg"

// Functions to get/put characters to running XEX
static size_t str_out_pos, str_out_len, str_in_pos, str_in_len;
static char *str_out;
static const char *str_in;
static sim65 str_s;
static int str_stop_on_eof;
static int input_eof;
static void str_put_char(int c)
{
    if (c == 0x9b)
        c = '\n';
    else if (c == 0)
        c = '\n';

    if (str_out_pos < str_out_len)
        str_out[str_out_pos++] = c;
}
static int str_get_char(void)
{
    if (str_in_pos < str_in_len)
        return 0xFF & str_in[str_in_pos++];
    // Input exhausted
    input_eof = 1;
    if (str_stop_on_eof)
        sim65_set_cycle_limit(str_s, 1);
    return EOF;
}

// ATASCII→UTF-8 table for visual screen diagnostics.
// Adapted from conv-atascii-utf8.c — indexed by ATASCII byte value.
static const char *const atascii_utf8[256] = {
                    "♥", "├", "🮇", "┘", "┤", "┐", "╱", "╲",
                    "◢", "▗", "◣", "▝", "▘", "🮂", "▂", "▖",
                    "♣", "┌", "─", "┼", "●", "▄", "▎", "┬",
                    "┴", "▌", "└", "␛", "↑", "↓", "←", "→",
                    " ", "!", "\"", "#", "$", "%", "&", "'",
                    "(", ")", "*", "+", ",", "-", ".", "/",
                    "0", "1", "2", "3", "4", "5", "6", "7",
                    "8", "9", ":", ";", "<", "=", ">", "?",
                    "@", "A", "B", "C", "D", "E", "F", "G",
                    "H", "I", "J", "K", "L", "M", "N", "O",
                    "P", "Q", "R", "S", "T", "U", "V", "W",
                    "X", "Y", "Z", "[", "\\", "]", "^", "_",
                    "♦", "a", "b", "c", "d", "e", "f", "g",
                    "h", "i", "j", "k", "l", "m", "n", "o",
                    "p", "q", "r", "s", "t", "u", "v", "w",
                    "x", "y", "z", "♠", "|", "⇱", "◀", "▶",
                    "♥", "┣", "▊", "┛", "┫", "┓", "╱", "╲",
                    "◤", "▛", "◥", "▙", "▟", "▆", "🮅", "▜",
                    "♣", "┏", "━", "╋", "●", "▀", "🮊", "┳",
                    "┻", "▐", "┗", "\n", "⇑", "⇓", "⇐", "⇒",
                    "█", "!", "\"", "#", "$", "%", "&", "'",
                    "(", ")", "*", "+", ",", "-", ".", "/",
                    "𝟬", "𝟭", "𝟮", "𝟯", "𝟰", "𝟱", "𝟲", "𝟳",
                    "𝟴", "𝟵", ":", ";", "<", "=", ">", "?",
                    "@", "𝗔", "𝗕", "𝗖", "𝗗", "𝗘", "𝗙", "𝗚",
                    "𝗛", "𝗜", "𝗝", "𝗞", "𝗟", "𝗠", "𝗡", "𝗢",
                    "𝗣", "𝗤", "𝗥", "𝗦", "𝗧", "𝗨", "𝗩", "𝗪",
                    "𝗫", "𝗬", "𝗭", "[", "\\", "]", "^", "_",
                    "♦", "𝗮", "𝗯", "𝗰", "𝗱", "𝗲", "𝗳", "𝗴",
                    "𝗵", "𝗶", "𝗷", "𝗸", "𝗹", "𝗺", "𝗻", "𝗼",
                    "𝗽", "𝗾", "𝗿", "𝘀", "𝘁", "𝘂", "𝘃", "𝘄",
                    "𝘅", "𝘆", "𝘇", "♠", "|", "⇱", "◀", "▶"
};

// Runs atari XEX file capturing the output
static int run_atari_prog(const char *progname, char *output, size_t *output_len,
                          const char *input, size_t input_len, uint64_t max_cycles,
                          int is_rom, const char *cmdline, int fix_margins,
                          const char *dos_root, int *out_row, int *out_col)
{
    // Init input/output
    str_in = input;
    str_in_pos = 0;
    str_in_len = input_len;
    str_out = output;
    str_out_pos = 0;
    str_out_len = *output_len - 1;
    memset(str_out, 0, *output_len);
    input_eof = 0;
    // Init emulator
    sim65 s = sim65_new();
    if (!s)
        return -1;
    // NOTE: never enable full instruction tracing here, a single run can
    //       log billions of lines and fill the disk.
    // sim65_set_debug(s, sim65_debug_trace);
    if (getenv("EXBTEST_DEBUG"))
        // Log CIO and device activity, useful to debug file I/O tests
        sim65_set_debug(s, sim65_debug_messages);
    emu_options opts = {
        .get_char = str_get_char, .put_char = str_put_char, .flags = atari_opt_cycletime
    };
    atari_init(s, &opts);
    // Add command line
    atari_dos_add_cmdline(s, progname);
    if (cmdline)
        atari_dos_add_cmdline(s, cmdline);
    // Set DOS path
    atari_dos_set_root(s, dos_root ? dos_root : output_dir);
    // Use 0 as left-margin value, to normalize results.
    if (fix_margins)
    {
        unsigned char val = 0;
        sim65_add_data_ram(s, 0x52, &val, 1); // LMARGN
        sim65_add_data_ram(s, 0x55, &val, 1); // COLCRS
    }

    sim65_set_cycle_limit(s, max_cycles);
    str_s = s;
    enum sim65_error e;
    if( is_rom )
        e = atari_rom_load(s, 0xA000, progname);
    else
        e = atari_xex_load(s, progname, 1);
    str_s = NULL;
    // Cycle limit hit after input exhausted is a normal exit for programs
    // that read input until the end
    if (e == sim65_err_cycle_limit && str_stop_on_eof && input_eof)
        e = sim65_err_none;
    if (e == sim65_err_user)
    {
        fprintf(stderr, "%s: error reading XEX/ROM file\n", progname);
        free(s);
        return -1;
    }
    else if (e)
    {
        // Prints error message
        fprintf(stderr, "%s: simulator returned %s at address %04x.\n",
                progname, sim65_error_str(s, e), sim65_error_addr(s));
        free(s);
        return -1;
    }
    // Return the current value of the interpreter stack, used to check for stack errors
    // TODO: this should depend on actual compiled value, we currently expect the
    //       value to be constant.
    int x = 0x40 != (sim65_get_byte(s, 0x8E) & 0xFF);
    // A runtime error ends the program where it happens, with values still
    // on the stack: no check after "ERROR- n" (the last line of the output)
    {
        size_t p = str_out_pos;
        if (p && str_out[p - 1] == '\n')
            p--;
        while (p && str_out[p - 1] >= '0' && str_out[p - 1] <= '9')
            p--;
        if (p >= 7 && !memcmp(str_out + p - 7, "ERROR- ", 7))
            x = 0;
    }
    // Read the OS cursor variables before freeing the simulator.
    if (out_row)
        *out_row = sim65_get_byte(s, ROWCRS);
    if (out_col)
        *out_col = sim65_get_byte(s, COLCRS) |
                   (sim65_get_byte(s, COLCRS + 1) << 8);
    // Update output length
    *output_len = str_out_pos;
    free(s);
    if (x)
        fprintf(stderr, "%s: simulator stack invalid at exit: %d.\n",
                progname, x);
    return x;
}

// Checks that the captured output matches the expected output
static int check_output(const char *fname, const char *expected_out, const char *out)
{
    if (!strcmp(expected_out, out))
        return 0;
    size_t l = 0;
    for (l = 0; out[l] == expected_out[l] && out[l]; l++);

    size_t l1 = l > 20 ? l - 20 : 0;
    size_t l2 = l + 20;
    fprintf(stderr, "%s: output does not match:\n", fname);
    fprintf(stderr, "expected: ");
    for(const char *x = expected_out + l1; x<(expected_out+l2) && *x; x++)
        putc(*x > 31 && *x < 127 ? *x : '.', stderr);
    fprintf(stderr, "\ngot:      ");
    for(const char *x = out + l1; x<(out+l2) && *x; x++)
        putc(*x > 31 && *x < 127 ? *x : '.', stderr);
    fprintf(stderr, "\n          ");
    for(size_t x = l1; x<l; x++)
        putc(' ', stderr);
    fprintf(stderr, "^\n");
    return -1;
}

// Run's an XEX file testing if the output matches the expected output
int run_test_xex(const char *fname, const char *input, const char *expected_out,
                 uint64_t max_cycles, int rom)
{
    size_t len = strlen(expected_out) + 128;
    char *out = calloc(len + 1, 1);

    int e = run_atari_prog(fname, out, &len, input, strlen(input), max_cycles, rom, 0, 1, NULL, NULL, NULL);
    if (!e)
    {
        // Check
        e = check_output(fname, expected_out, out);
        // Keep the actual output to compare, in "<program>.got"
        if (e)
        {
            char *got;
            if (asprintf(&got, "%s.got", fname) >= 0)
            {
                FILE *f = fopen(got, "wb");
                if (f)
                {
                    fwrite(out, 1, len, f);
                    fclose(f);
                }
                free(got);
            }
        }
    }
    else if (e > 0)
        fprintf(stderr,"%s: unexpected stack value.\n", fname);
    else
        fprintf(stderr, "%s: error on execution.\n", fname);
    free(out);
    return e;
}

// A manifest lists the paths created by one test.  make clean reads these
// manifests and removes only the listed files; directories are removed with
// rmdir, so user files left in a test directory are preserved.
static FILE *manifest_open(const char *tag_name)
{
    char *path;
    if (asprintf(&path, "%s/.exbtest-manifest-%s", output_dir, tag_name) < 0)
        return NULL;
    FILE *f = fopen(path, "wb");
    free(path);
    return f;
}

static void manifest_add(FILE *f, char kind, const char *path)
{
    fprintf(f, "%c %s\n", kind, path);
}

static void manifest_add_file(FILE *f, const char *dir, const char *name)
{
    char *path;
    if (asprintf(&path, "%s/%s", dir, name) >= 0)
    {
        manifest_add(f, 'F', path);
        free(path);
    }
}

// Debug dumps use fixed names derived from the test tag.
static void manifest_add_debug(FILE *f, const char *tag_name)
{
    char *path;
    if (asprintf(&path, "%s/%s.screen", output_dir, tag_name) >= 0)
    {
        manifest_add(f, 'F', path);
        free(path);
    }
    if (asprintf(&path, "%s/%s.output", output_dir, tag_name) >= 0)
    {
        manifest_add(f, 'F', path);
        free(path);
    }
}

struct file_check
{
    char *name;
    char *data;
    size_t len;
};

// Checks Check:/Exists: files in the given DOS root.
static int check_test_files(const char *fname, const char *dir,
                            const struct file_check *checks, int nchecks,
                            char **exists, int nexists)
{
    for (int i = 0; i < nchecks; i++)
    {
        char *path;
        if (asprintf(&path, "%s/%s", dir, checks[i].name) < 0)
            exit(1);
        FILE *fc = fopen(path, "rb");
        free(path);
        char *got = 0;
        size_t rlen = 0;
        if (fc)
        {
            fseek(fc, 0, SEEK_END);
            long sz = ftell(fc);
            fseek(fc, 0, SEEK_SET);
            got = calloc(sz + 1, 1);
            rlen = fread(got, 1, sz, fc);
            fclose(fc);
        }
        if (!got)
        {
            fprintf(stderr, "%s: can't open checked file '%s'.\n",
                    fname, checks[i].name);
            return -1;
        }
        else if (rlen != checks[i].len || memcmp(got, checks[i].data, rlen))
        {
            size_t d = 0;
            while (d < rlen && d < checks[i].len &&
                   got[d] == checks[i].data[d])
                d++;
            fprintf(stderr,
                    "%s: file '%s' does not match: %lu bytes vs %lu "
                    "expected, first difference at byte %lu.\n",
                    fname, checks[i].name, (unsigned long)rlen,
                    (unsigned long)checks[i].len, (unsigned long)d);
            free(got);
            return -1;
        }
        free(got);
    }

    for (int i = 0; i < nexists; i++)
    {
        char *path;
        if (asprintf(&path, "%s/%s", dir, exists[i]) < 0)
            exit(1);
        FILE *fx = fopen(path, "rb");
        free(path);
        if (!fx)
        {
            fprintf(stderr, "%s: file '%s' not created.\n", fname, exists[i]);
            return -1;
        }
        fclose(fx);
    }
    return 0;
}

static int run_prog(const char *prog, char *out, size_t *len)
{
    // Redirect standard error
    char *cmd = 0;
    if (asprintf(&cmd, "%s 2>&1", prog) < 0)
    {
        fprintf(stderr, "%s: memory error\n", prog);
        return -1;
    }

    FILE *f = popen(cmd, "r");
    if (!f)
    {
        fprintf(stderr, "%s: can't execute compiler.\n", prog);
        return -1;
    }
    free(cmd);

    int clen = 0;
    int c;
    while (clen < *len && (EOF != (c = fgetc(f))))
        out[clen++] = c;
    *len = clen;
    return pclose(f) ? 1 : 0;
}

// Returns the companion assembly file name for a BASIC source file, or NULL
// on memory errors.
static char *companion_asm_fname(const char *basname)
{
    const char *base = strrchr(basname, '/');
#ifdef _WIN32
    const char *base_win = strrchr(basname, '\\');
    if (!base || (base_win && base_win > base))
        base = base_win;
#endif
    const char *dot = strrchr(base ? base : basname, '.');
    size_t stem = dot ? (size_t)(dot - basname) : strlen(basname);
    char *ret = malloc(stem + 5);
    if (!ret)
        return NULL;
    memcpy(ret, basname, stem);
    strcpy(ret + stem, ".asm");
    return ret;
}

// Appends an optional companion assembly file to the compiler output.  This
// lets cross-only tests supply small 6502 helpers or page-aligned bytecode.
// Returns 1 if a companion was appended, 0 if there is none, -1 on error.
static int append_companion_asm(const char *basname, const char *asmname)
{
    char *compname = companion_asm_fname(basname);
    if (!compname)
        return -1;
    if (!strcmp(compname, asmname))
    {
        free(compname);
        return 0;
    }

    FILE *in = fopen(compname, "rb");
    if (!in)
    {
        free(compname);
        return 0;
    }

    FILE *out = fopen(asmname, "ab");
    if (!out)
    {
        fclose(in);
        free(compname);
        return -1;
    }

    int c;
    fputs("\n; ---- companion assembly ----\n", out);
    while ((c = fgetc(in)) != EOF)
        fputc(c, out);
    int failed = ferror(in) || ferror(out);
    fclose(in);
    if (fclose(out))
        failed = 1;
    free(compname);
    return failed ? -1 : 1;
}

// Expected compiler error message, for compile-error tests
static const char *expected_error;
// Extra compiler options of the current test
static const char *compiler_options = "";

static int compile_cross(const char *basname, const char *asmname,
                         const char *objname, const char *outname,
                         int comp_ok, int error_pos_line, int error_pos_column,
                         int compile_rom)
{
    const char *fb_target = FB_TARGET;
    const char *libs = compile_rom ? FB_LIB_ROM : FB_LIB;
    const char *cfg = compile_rom ? FB_CFG_FILE_ROM : FB_CFG_FILE;
    char *cmd = 0;
    char *out = calloc(8192, 1);
    int e = -1;

    // Erase all files
    unlink(asmname);
    unlink(objname);
    unlink(outname);

    if (asprintf(&cmd, "%s " FB_PATHS " %s %s -c -o %s %s", fb_compiler, fb_target,
                 compiler_options, asmname, basname) < 0)
    {
        fprintf(stderr, "%s: memory error.\n", basname);
        goto xit;
    }

    // Calls compiler
    size_t len = 8191;
    e = run_prog(cmd, out, &len);
    if (e < 0)
        goto xit;
    else if (e && comp_ok)
    {
        fprintf(stderr, "%s: compile error: '%s'\n", basname, out);
        goto xit;
    }
    else if (e && !comp_ok)
    {
        // Extract error line and column, format is:
        //   FILE_NAME:LINE:COLUMN: MESSAGE
        int line = 0, column = 0;
        // Ignore errors, assume 0 as line and column
        sscanf(out, "%*[^:]:%d:%d:", &line, &column);
        if (error_pos_line != line || error_pos_column != column)
        {
            fprintf(stderr, "%s: bad error line or column, actual %d:%d, expected %d:%d\n",
                    basname, line, column, error_pos_line, error_pos_column);
            goto xit;
        }
        if (expected_error && *expected_error && !strstr(out, expected_error))
        {
            fprintf(stderr, "%s: bad error message, expected '%s', got: '%s'\n",
                    basname, expected_error, out);
            e = -1;
            goto xit;
        }
        e = 0;
    }
    else if (!e)
    {
        e = -1;
        if (append_companion_asm(basname, asmname) < 0)
        {
            fprintf(stderr, "%s: error appending companion assembly.\n", basname);
            goto xit;
        }

        // Assemble to object file
        free(cmd);
        if (asprintf(&cmd, "%s %s -o %s -I %s/asminc %s", ca65_path,
                     CA65_OPTS, objname,
                     fb_lib_path, asmname) < 0)
        {
            fprintf(stderr, "%s: memory error.\n", asmname);
            goto xit;
        }
        len = 8191;
        e = run_prog(cmd, out, &len);
        if (e && comp_ok)
        {
            fprintf(stderr, "%s: assembly error: '%s'\n", asmname, out);
            goto xit;
        }

        // Now, link to XEX / ROM
        free(cmd);
        if (asprintf(&cmd, "%s -C %s/%s -o %s %s %s/%s", ld65_path, fb_lib_path,
                     cfg, outname, objname, fb_lib_path, libs) < 0)
        {
            fprintf(stderr, "%s: memory error.\n", asmname);
            goto xit;
        }

        len = 8191;
        e = run_prog(cmd, out, &len);
        if (e && comp_ok)
        {
            fprintf(stderr, "%s: link error: '%s'\n", objname, out);
            goto xit;
        }
        else if (!e && !comp_ok)
        {
            // Compilation succeeded (expected fail!)
            fprintf(stderr, "%s: compiled without error, unexpected\n", basname);
            e = -1;
            goto xit;
        }
        else if (e)
            e = 0;
    }

xit:
    free(cmd);
    free(out);
    return e;
}

static char *build_fname(const char *base_name, const char *ext)
{
    char *ret;
    if (asprintf(&ret, "%s/%s.%s", output_dir, base_name, ext) < 0)
        ret = NULL;
    if( !ret )
    {
        fprintf(stderr,"memory error");
        exit(1);
    }
    return ret;
}


static char *source_fname(const char *base_name, const char *ext)
{
    char *ret;
    if (asprintf(&ret, "%s.%s", base_name, ext) < 0)
        ret = NULL;
    if( !ret )
    {
        fprintf(stderr,"memory error");
        exit(1);
    }
    return ret;
}


// Returns the lower-case file name, without directory and extension, of a
// PROGRAM entry; the compiled program is the D: file "<STEM>.XEX".
static char *program_stem(const char *prog)
{
    const char *base = strrchr(prog, '/');
    base = base ? base + 1 : prog;
    const char *dot = strrchr(base, '.');
    char *stem = xstrndup(base, dot ? (size_t)(dot - base) : strlen(base));
    for (char *p = stem; *p; p++)
        if (*p >= 'A' && *p <= 'Z')
            *p += 'a' - 'A';
    return stem;
}

// Compiles the PROGRAM entries of a test into the D: disk folder, so the
// test program can load them with RUN.
static int compile_programs(const char *fname, const char *srcdir,
                            char **programs, int nprograms)
{
    for (int i = 0; i < nprograms; i++)
    {
        char *src, *stem = program_stem(programs[i]);
        if (asprintf(&src, "%s/%s", srcdir, programs[i]) < 0)
            exit(1);
        char *asmf = build_fname(stem, "asm");
        char *objf = build_fname(stem, "o");
        char *xexf = build_fname(stem, "xex");
        if (verbose)
            fprintf(stderr, "%s: compile program %s\n", fname, src);
        int e = compile_cross(src, asmf, objf, xexf, 1, 0, 0, 0);
        free(src);
        free(stem);
        free(asmf);
        free(objf);
        free(xexf);
        if (e)
            return -1;
    }
    return 0;
}

enum tests {
    test_run = 1,
    test_cross = 8,
    test_compile_error = 32
};

// Expands control-key, named-key, and UTF-8 notation in an input buffer.
// ^X (0x41-0x5A) -> Ctrl-A..Z ($01-$1A)
// <NAME> -> single byte from the Atari special-character table
// \xHH -> hex byte
// UTF-8 multi-byte sequences -> ATASCII byte from atascii_utf8 table
// All other bytes pass through literally.
// Returns a malloc'ed buffer; caller must free the original.
static char *expand_notation(char *input, size_t *len_out)
{
    static const struct { const char *name; int code; } keys[] = {
        { "ESC", 0x1B }, { "UP", 0x1C }, { "DOWN", 0x1D },
        { "LEFT", 0x1E }, { "RIGHT", 0x1F }, { "RETURN", 0x9B },
        { "CLS", 0x7D },
        { "BACKSP", 0x7E }, { "TAB", 0x7F }, { "DELLN", 0x9C },
        { "INSLN", 0x9D }, { "DELTAB", 0x9E }, { "SETTAB", 0x9F },
        { "BELL", 0xFD }, { "DEL", 0xFE }, { "INS", 0xFF },
        { NULL, 0 }
    };
    size_t in_len = strlen(input);
    char *out = malloc(in_len + 1);
    size_t oi = 0;
    for (size_t i = 0; i < in_len; )
    {
        unsigned char c = (unsigned char)input[i];
        if (c == '^' && i + 1 < in_len)
        {
            unsigned char next = (unsigned char)input[i + 1];
            if (next >= 'A' && next <= 'Z')
            {
                out[oi++] = next - 'A' + 1;
                i += 2;
                continue;
            }
            if (next >= 'a' && next <= 'z')
            {
                out[oi++] = next - 'a' + 1;
                i += 2;
                continue;
            }
        }
        if (c == '<')
        {
            size_t j = i + 1;
            while (j < in_len && input[j] != '>')
                j++;
            if (j < in_len)
            {
                size_t nlen = j - i - 1;
                for (int k = 0; keys[k].name; k++)
                {
                    if (strlen(keys[k].name) == nlen &&
                        !strncasecmp(input + i + 1, keys[k].name, nlen))
                    {
                        out[oi++] = (unsigned char)keys[k].code;
                        i = j + 1;
                        goto next;
                    }
                }
            }
        }
        if (c == '\\' && i + 3 < in_len && input[i + 1] == 'x')
        {
            int hi = -1, lo = -1;
            unsigned char h = (unsigned char)input[i + 2];
            unsigned char l = (unsigned char)input[i + 3];
            if      (h >= '0' && h <= '9') hi = h - '0';
            else if (h >= 'A' && h <= 'F') hi = h - 'A' + 10;
            else if (h >= 'a' && h <= 'f') hi = h - 'a' + 10;
            if      (l >= '0' && l <= '9') lo = l - '0';
            else if (l >= 'A' && l <= 'F') lo = l - 'A' + 10;
            else if (l >= 'a' && l <= 'f') lo = l - 'a' + 10;
            if (hi >= 0 && lo >= 0)
            {
                out[oi++] = (unsigned char)(hi * 16 + lo);
                i += 4;
                continue;
            }
        }
        // UTF-8: scan atascii_utf8 table for a match at current position.
        if (c >= 0xC0)
        {
            for (int k = 0; k < 256; k++)
            {
                size_t ulen = strlen(atascii_utf8[k]);
                if (i + ulen <= in_len &&
                    !memcmp(input + i, atascii_utf8[k], ulen))
                {
                    out[oi++] = (unsigned char)k;
                    i += ulen;
                    goto next;
                }
            }
        }
        out[oi++] = c;
        i++;
    next:;
    }
    out[oi] = 0;
    *len_out = oi;
    free(input);
    return out;
}

// Reads a section of lines, terminated by a lone ".", converting newlines
// to ATASCII EOL. Returns a malloc'ed buffer and its size, or NULL if the
// section is unterminated.
static char *read_section(FILE *f, int *line, size_t *len_out)
{
    size_t cap = 4096, size = 0;
    char *buf = calloc(cap, 1), lbuf[256];
    while (fgets(lbuf, sizeof(lbuf)-1, f))
    {
        (*line)++;
        if (!strcmp(lbuf, ".") || !strcmp(lbuf, ".\n"))
        {
            buf[size] = 0;
            *len_out = size;
            return buf;
        }
        size_t l = strlen(lbuf);
        if (l && lbuf[l-1] == '\n')
            lbuf[l-1] = 0x9B;
        while (size + l + 1 > cap)
        {
            cap *= 2;
            char *nb = realloc(buf, cap);
            if (!nb)
            {
                free(buf);
                return NULL;
            }
            buf = nb;
        }
        memcpy(buf + size, lbuf, l);
        size += l;
    }
    free(buf);
    return NULL;
}

// Runs one test from the test-file
int exbtest(const char *fname)
{
    FILE *f = fopen(fname, "rb");
    if (!f)
    {
        fprintf(stderr, "%s: error, can't open file.\n", fname);
        return -1;
    }

    /* File format:
     *   NAME: The name of the test
     *   TEST: The test to do
     *   ERROR: The expected error from compiler (optional)
     *   MAX-CYCLES: The maximum number of cycles to wait for program termination.
     *               (if not given, use 20_000_000.
     *   INPUT:
     *   Optional input, up to a line with only a '.'.
     *   OUTPUT:
     *   The expected output, up to the end of the file.
     */
    char *name = 0, *expected_out = 0, *input_buf = 0, *error_data = 0, *options_data = 0;
    // Expected content of files written during the run
    struct file_check checks[16];
    int nchecks = 0;
    // Names of files that must exist after the run
    char *exists[16];
    int nexists = 0;
    // Extra programs compiled to the D: disk before the run (for RUN)
    char *programs[16];
    int nprograms = 0;
    int error_pos_line = 0, error_pos_column = 0;
    int test = 0, n = 0, line = 0;
    uint64_t max_cycles = 20000000;
    char lbuf[256];
    while ( 0 != fgets(lbuf, sizeof(lbuf)-1, f) )
    {
        line ++;
        if ( !*lbuf || *lbuf == '\n' )
            continue;
        // Header comments
        if (lbuf[0] == '#')
            continue;

        char key[48], buf[256];
        n = sscanf(lbuf, "%15[^:]: %255[^\n\r]", key, buf);
        if (n < 1)
        {
            fprintf(stderr, "%s:%d: error, can't parse 'key: value'\n", fname, line);
            return -1;
        }
        else if (!strcasecmp(key, "name"))
            name = strdup(buf);
        else if (!strcasecmp(key, "test"))
        {
            // Split into words by ',' '-' or spaces
            for( char *p = buf, *tok; (tok = strtok(p, " \t-,")) ; p = NULL)
            {
                if (!strcasecmp(tok, "run"))
                    test = test_cross | test_run;
                else if (!strcasecmp(tok, "compile"))
                    test = test_cross;
                else if (!strcasecmp(tok, "error"))
                    test |= test_compile_error;
                else
                {
                    fprintf(stderr, "%s:%d: error, unknown test '%s'\n", fname, line, buf);
                    return -1;
                }
            }
        }
        else if (!strcasecmp(key, "error"))
            error_data = strdup(buf);
        else if (!strcasecmp(key, "error-pos"))
        {
            if( 2 != sscanf(buf, "%i : %i", &error_pos_line, &error_pos_column) )
            {
                fprintf(stderr, "%s:%d: error, invalid value for error-pos '%s'\n",
                        fname, line, buf);
                return -1;
            }
        }
        else if (!strcasecmp(key, "max-cycles"))
        {
            char *ep = 0;
            errno = 0;
            max_cycles = strtoul(buf, &ep, 0);
            if( errno == ERANGE || (*ep != 0 && *ep != ' ' && *ep != '\t') )
            {
                fprintf(stderr, "%s:%d: error, invalid value for max-cycles '%s'\n",
                        fname, line, buf);
                return -1;
            }
        }
        else if (!strcasecmp(key, "check"))
        {
            // Compare a file written during the session against this content
            if (n < 2 || !*buf)
            {
                fprintf(stderr, "%s:%d: error, missing file name in CHECK\n", fname, line);
                return -1;
            }
            if (nchecks >= 16)
            {
                fprintf(stderr, "%s:%d: error, too many CHECK sections.\n", fname, line);
                return -1;
            }
            checks[nchecks].name = strdup(buf);
            checks[nchecks].data = read_section(f, &line, &checks[nchecks].len);
            if (!checks[nchecks].data)
            {
                fprintf(stderr, "%s:%d: unfinished CHECK section.\n", fname, line);
                return -1;
            }
            nchecks++;
        }
        else if (!strcasecmp(key, "exists"))
        {
            if (n < 2 || !*buf)
            {
                fprintf(stderr, "%s:%d: error, missing file name in EXISTS\n", fname, line);
                return -1;
            }
            if (nexists >= 16)
            {
                fprintf(stderr, "%s:%d: error, too many EXISTS entries.\n", fname, line);
                return -1;
            }
            exists[nexists++] = strdup(buf);
        }
        else if (!strcasecmp(key, "options"))
            options_data = strdup(buf);
        else if (!strcasecmp(key, "program"))
        {
            if (n < 2 || !*buf)
            {
                fprintf(stderr, "%s:%d: error, missing file name in PROGRAM\n", fname, line);
                return -1;
            }
            if (nprograms >= 16)
            {
                fprintf(stderr, "%s:%d: error, too many PROGRAM entries.\n", fname, line);
                return -1;
            }
            programs[nprograms++] = strdup(buf);
        }
        else if (!strcasecmp(key, "input"))
        {
            size_t input_size = 0;
            if (n > 1 && *buf)
            {
                fprintf(stderr, "%s:%d: error, extra characters in INPUT '%s'\n", fname, line, buf);
                return -1;
            }
            if (input_buf)
            {
                fprintf(stderr, "%s:%d: more than one INPUT section.\n", fname, line);
                free(input_buf);
                return -1;
            }
            // Get the rest of the file until the end
            input_buf = calloc(8192, 1);
            // Read lines
            while ( 0 != fgets(lbuf, sizeof(lbuf)-1, f) )
            {
                line++;
                if ( !strcmp(lbuf, ".") || !strcmp(lbuf, ".\n") )
                    break;
                size_t l = strlen(lbuf);
                if (l + input_size > 8191)
                {
                    fprintf(stderr, "%s:%d: error, extra characters in output '%s'\n", fname, line, buf);
                    free(input_buf);
                    return -1;
                }
                if (l && (lbuf[l-1] == '\n'))
                        lbuf[l-1] = 0x9B;
                memcpy(input_buf + input_size, lbuf, l);
                input_size += l;
            }
            input_buf[input_size] = 0;
            if (feof(f))
            {
                fprintf(stderr, "%s:%d: unfinished INPUT section.\n", fname, line);
                free(input_buf);
                return -1;
            }
        }
        else if (!strcasecmp(key, "output"))
        {
            if (n > 1 && *buf)
            {
                fprintf(stderr, "%s:%d: error, extra characters in OUTPUT '%s'\n", fname, line, buf);
                free(input_buf);
                return -1;
            }
            // Get the rest of the file until the end
            expected_out = calloc(65536, 1);
            size_t len = fread(expected_out, 1, 65535, f);
            if (len >= 65535)
            {
                fprintf(stderr, "%s: error, output text too long.\n", fname);
                free(input_buf);
                return -1;
            }
            else if (len < 1)
            {
                fprintf(stderr, "%s: error, no output.\n", fname);
                free(input_buf);
                return -1;
            }
            expected_out[len] = 0;
            break;
        }
        else
        {
            fprintf(stderr, "%s:%d: error, unknown key '%s'\n", fname, line, key);
            return -1;
        }
    }
    fclose(f);

    // Check test specs
    if (!test)
    {
        fprintf(stderr, "%s: error, no test to run\n", fname);
        return -1;
    }
    if (!name)
        name = "unnamed";
    if (!expected_out)
        expected_out = strdup("");
    if (!input_buf)
    {
        input_buf = strdup("");
    }
    // Expand ^X, <NAME>, \xHH notation after the whole file is parsed, so
    // header keys may appear in any order.
    {
        size_t expanded_len;
        input_buf = expand_notation(input_buf, &expanded_len);
    }
    if (error_data && !(test & test_compile_error))
    {
        fprintf(stderr, "%s: error data but no compile error expected\n", fname);
        return -1;
    }
    if ((test & test_compile_error) && !error_data)
        error_data = strdup("");
    expected_error = error_data;
    compiler_options = options_data ? options_data : "";

    // Get file names from test file
    const char *ext = strrchr(fname, '.');
    if (!ext)
        ext = fname + strlen(fname);

    // base_name: file name without extension
    char *base_name = xstrndup(fname, ext - fname);

    // tag_name: file name without directory and extension
    char *tag_name = strrchr(base_name, '/');
    if( !tag_name )
        tag_name = base_name;
    else
        tag_name = tag_name + 1;

    // srcdir: directory containing the test files
    char *srcdir = (tag_name != base_name) ?
        xstrndup(fname, tag_name - base_name - 1) : strdup(".");

    // basname: input basic file name
    char *basname = source_fname(base_name, "bas");
    // xexname: XEX file name (compiled from the cross-compiler)
    char *xexname = build_fname(tag_name, "xex");
    // romname: ROM file name
    char *romname = build_fname(tag_name, "rom");
    // asmname: Assembler file name
    char *asmname = build_fname(tag_name, "asm");
    // objname: Object file name
    char *objname = build_fname(tag_name, "o");

    // Ok, do tests
    int test_ok = 0;
    char *cmd_out = calloc(65536, 1);

    // Write the cleanup manifest from the declarations in the .chk file.
    {
        FILE *manifest = manifest_open(tag_name);
        if (manifest)
        {
            for (int i = 0; i < nchecks; i++)
                manifest_add_file(manifest, output_dir, checks[i].name);
            for (int i = 0; i < nexists; i++)
                manifest_add_file(manifest, output_dir, exists[i]);
            for (int i = 0; i < nprograms; i++)
            {
                char *stem = program_stem(programs[i]);
                const char *exts[] = { "xex", "asm", "o" };
                for (int k = 0; k < 3; k++)
                {
                    char *pf = build_fname(stem, exts[k]);
                    manifest_add(manifest, 'F', pf);
                    free(pf);
                }
                free(stem);
            }
            manifest_add_debug(manifest, tag_name);
            fclose(manifest);
        }
    }

    do
    {
        // Programs loaded with RUN by the test program
        if (compile_programs(fname, srcdir, programs, nprograms))
            break;

        // XEX version:
        if (verbose)
            fprintf(stderr, "%s: compile\n", fname);
        if (compile_cross(basname, asmname, objname, xexname,
                          !(test & test_compile_error),
                          error_pos_line, error_pos_column, 0))
            break;

        if (test & test_run)
        {
            if (verbose)
                fprintf(stderr, "%s: run\n", fname);
            if (run_test_xex(xexname, input_buf, expected_out, max_cycles, 0))
                break;
            if (check_test_files(fname, output_dir, checks, nchecks, exists, nexists))
                break;
        }

        // CARTRIDGE version:
        if (verbose)
            fprintf(stderr, "%s: compile cartridge\n", fname);
        if (compile_cross(basname, asmname, objname, romname,
                          !(test & test_compile_error),
                          error_pos_line, error_pos_column, 1))
            break;

        if (test & test_run)
        {
            if (verbose)
                fprintf(stderr, "%s: run cartridge\n", fname);
            if (run_test_xex(romname, input_buf, expected_out, max_cycles, 1))
                break;
            if (check_test_files(fname, output_dir, checks, nchecks, exists, nexists))
                break;
        }

        // All the code compiled to machine code (unless the test sets its
        // own options), the results must be the same.
        if ((test & test_run) && !options_data)
        {
            if (verbose)
                fprintf(stderr, "%s: compile all machine code\n", fname);
            compiler_options = "-native:all";
            int e = compile_cross(basname, asmname, objname, xexname, 1,
                                  error_pos_line, error_pos_column, 0);
            compiler_options = "";
            if (e)
                break;
            if (verbose)
                fprintf(stderr, "%s: run all machine code\n", fname);
            if (run_test_xex(xexname, input_buf, expected_out, max_cycles, 0))
                break;
            if (check_test_files(fname, output_dir, checks, nchecks, exists, nexists))
                break;
        }

        test_ok = 1;
    }
    while(0);

    printf("TEST '%s': %s\n", name,
           test_ok ? "\e[42m passed \e[0m" : "\e[41m not passed \e[0m");
    free(cmd_out);
    free(error_data);
    free(options_data);
    compiler_options = "";
    free(input_buf);
    free(expected_out);
    free(name);
    free(basname);
    free(romname);
    free(xexname);
    free(asmname);
    free(objname);
    free(base_name);
    free(srcdir);
    for (int i = 0; i < nchecks; i++)
    {
        free(checks[i].name);
        free(checks[i].data);
    }
    for (int i = 0; i < nexists; i++)
        free(exists[i]);
    for (int i = 0; i < nprograms; i++)
        free(programs[i]);
    return !test_ok;
}


int main(int argc, char **argv)
{
    int opt;
    while ((opt = getopt(argc, argv, "hvi:f:l:a:k:")) != -1)
    {
        switch (opt)
        {
            case 'h': // help
                fprintf(stderr, "Usage: %s [options] <test1.chk> ...\n"
                        "Options:\n"
                        " -h: Show this help\n"
                        " -v: Verbose execution\n"
                        " -f <fp-compiler>: Sets path of cross-compiler [%s]\n"
                        " -l <lib-path>: Sets path for the libraries and includes [%s]\n"
                        " -a <ca65-path>: Sets path for the CA65 assembler [%s]\n"
                        " -k <ld65-path>: Sets path for the LD65 linker [%s]\n",
                        argv[0], fb_compiler,
                        fb_lib_path, ca65_path, ld65_path);
                return 0;
            case 'v': // verbose
                verbose = 1;
                break;
            case 'a': // CA65 assembler path
                ca65_path = optarg;
                break;
            case 'k': // LD65 linker path
                ld65_path = optarg;
                break;
            case 'f': // cross-compiler path
                fb_compiler = optarg;
                break;
            case 'l': // cross libraries path
                fb_lib_path = optarg;
                break;
            default:
                return EXIT_FAILURE;
        }
    }

    int pass = 0, fail = 0;
    for(int i=optind; i<argc; i++)
        if (!exbtest(argv[i]))
            pass ++;
        else
            fail ++;

    if( pass + fail > 1 )
    {
        printf("SUMMARY: ");
        if (fail)
            printf("%d tests passed, %d tests failed.\n", pass, fail);
        else
            printf("all %d tests passed.\n", pass);
    }
    return fail ? EXIT_FAILURE : 0;
}

