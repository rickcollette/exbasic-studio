/*
 * exBASIC - BASIC compiler for the Atari 8-bit computers, based on FastBasic
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

// exbprof.c: Runs a compiled exBASIC XEX in the mini65 simulator and reports
//           where the CPU time goes:
//            - cycles per runtime routine (using the ld65 label file),
//            - executed bytecode instructions and instruction pairs.
//
// Usage: exbprof [-q] [-c cycles] program.xex program.lbl
//
//   -q         don't show the program output.
//   -c cycles  stop after the given number of CPU cycles (default 2e9), useful
//              for programs that never end, like games.
//   -d path    folder used as the "D:" disk, default is the current folder.

#include "atari.h"
#include "sim65.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define MAX_LABELS 8192

struct label
{
    unsigned addr;
    char name[128];
};

static struct label labels[MAX_LABELS];
static int num_labels;
static char tok_name[256][128];
static uint64_t tok_count[256];
static uint64_t pair_count[256][256];
static unsigned interp_addr;
static int last_tok = -1;
static int quiet;
static uint64_t max_cycles = 2000000000ULL;
static const char *dos_root = ".";

static int get_char(void)
{
    return -1;
}

static void put_char(int c)
{
    if (!quiet)
        putchar(c == 155 ? '\n' : c);
}

// Called on each execution of the "JMP (JUMPTAB)" instruction of the
// interpreter dispatch loop.  The low byte of the JMP operand is the token.
static int on_dispatch(sim65 s, struct sim65_reg *regs, unsigned addr, int data)
{
    int t = sim65_get_byte(s, interp_addr + 16);
    tok_count[t]++;
    if (last_tok >= 0)
        pair_count[last_tok][t]++;
    last_tok = t;
    return 0;
}

static int cmp_label(const void *a, const void *b)
{
    const struct label *la = a, *lb = b;
    return (int)la->addr - (int)lb->addr;
}

static int is_routine(const char *name)
{
    // "bytecode_start" marks the machine code compiled from loops, that is
    // inside the bytecode.
    return !strncmp(name, "EXE_", 4) || !strncmp(name, "nat_", 4) ||
           !strcmp(name, "interpreter") || !strcmp(name, "pushAX") ||
           !strcmp(name, "move_loop") || !strcmp(name, "divmod_sign_adjust") ||
           !strcmp(name, "neg_AX") || !strcmp(name, "get_op_var") ||
           !strcmp(name, "alloc_array") || !strcmp(name, "bytecode_start");
}

static void read_labels(const char *fname)
{
    FILE *f = fopen(fname, "r");
    if (!f)
    {
        perror(fname);
        exit(1);
    }
    char line[256], name[128];
    unsigned addr;
    while (fgets(line, sizeof(line), f))
    {
        if (sscanf(line, "al %x .%127s", &addr, name) != 2)
            continue;
        if (!strncmp(name, "TOK_", 4) && addr < 256)
            snprintf(tok_name[addr], sizeof(tok_name[addr]), "%s", name + 4);
        if (!strcmp(name, "interpreter"))
            interp_addr = addr;
        if (!is_routine(name) || num_labels >= MAX_LABELS)
            continue;
        // Skip duplicated labels (the label file can list a symbol twice)
        int dup = 0;
        for (int i = 0; i < num_labels; i++)
            if (labels[i].addr == addr && !strcmp(labels[i].name, name))
                dup = 1;
        if (dup)
            continue;
        labels[num_labels].addr = addr;
        snprintf(labels[num_labels].name, sizeof(labels[num_labels].name), "%s", name);
        num_labels++;
    }
    fclose(f);
    qsort(labels, num_labels, sizeof(labels[0]), cmp_label);
    if (!interp_addr)
    {
        fprintf(stderr, "%s: label 'interpreter' not found\n", fname);
        exit(1);
    }
}

int main(int argc, char **argv)
{
    int arg = 1, atari_fp = 0, zero_margin = 0;
    while (arg < argc && argv[arg][0] == '-')
    {
        if (!strcmp(argv[arg], "-q"))
            quiet = 1;
        else if (!strcmp(argv[arg], "-c") && arg + 1 < argc)
            max_cycles = strtoull(argv[++arg], 0, 0);
        else if (!strcmp(argv[arg], "-d") && arg + 1 < argc)
            dos_root = argv[++arg];
        else if (!strcmp(argv[arg], "-a"))
            atari_fp = 1;
        else if (!strcmp(argv[arg], "-m"))
            zero_margin = 1;
        else
            break;
        arg++;
    }
    if (argc - arg != 2)
    {
        fprintf(stderr, "Usage: %s [-q] [-a] [-m] [-c cycles] [-d path] program.xex program.lbl\n",
                argv[0]);
        return 1;
    }
    const char *xex = argv[arg], *lbl = argv[arg + 1];
    read_labels(lbl);

    sim65 s = sim65_new();
    emu_options opts = {
        .get_char = get_char, .put_char = put_char, .flags = atari_opt_cycletime
    };
    // Use the original Atari math package instead of the faster replacement
    if (atari_fp)
        opts.flags |= atari_opt_atari_mathpack;
    atari_init(s, &opts);
    atari_dos_add_cmdline(s, xex);
    atari_dos_set_root(s, dos_root);
    // Left margin 0, as the test harness
    if (zero_margin)
    {
        static unsigned char val = 0;
        sim65_add_data_ram(s, 0x52, &val, 1); // LMARGN
        sim65_add_data_ram(s, 0x55, &val, 1); // COLCRS
    }
    sim65_set_profiling(s, 1);
    sim65_add_callback(s, interp_addr + 15, on_dispatch, sim65_cb_exec);
    sim65_set_cycle_limit(s, max_cycles);
    enum sim65_error e = atari_xex_load(s, xex, 1);
    if (e == sim65_err_cycle_limit)
        printf("\n(stopped at cycle limit)\n");
    else if (e)
    {
        fprintf(stderr, "%s: simulator returned %s at address %04x.\n", xex,
                sim65_error_str(s, e), sim65_error_addr(s));
        return 1;
    }

    // Attribute cycles of each address to the routine label before it.
    struct sim65_profile p = sim65_get_profile_info(s);
    static uint64_t per_label[MAX_LABELS];
    uint64_t total = 0, other = 0;
    int k = -1;
    for (unsigned a = 0; a < p.max; a++)
    {
        while (k + 1 < num_labels && labels[k + 1].addr <= a)
            k++;
        uint64_t c = p.cycle_count[a] + p.extra_cycles[a];
        if (!c)
            continue;
        total += c;
        if (a >= 0xC000 || k < 0)
            other += c;
        else
            per_label[k] += c;
    }

    uint64_t ntok = 0;
    for (int t = 0; t < 256; t++)
        ntok += tok_count[t];

    printf("\n%s: %llu cycles (%.2f s at 1.79MHz, without DMA)\n", xex,
           (unsigned long long)total, total / 1789773.0);
    printf("%llu instructions dispatched, %.1f cycles per instruction\n\n",
           (unsigned long long)ntok, ntok ? (double)total / ntok : 0.0);

    printf("Cycles per runtime routine:\n");
    for (int n = 0; n < 16; n++)
    {
        int best = -1;
        for (int i = 0; i < num_labels; i++)
            if (per_label[i] && (best < 0 || per_label[i] > per_label[best]))
                best = i;
        if (best < 0)
            break;
        printf("  %6.2f%%  %s\n", 100.0 * per_label[best] / total,
               strcmp(labels[best].name, "bytecode_start") ? labels[best].name
                                                           : "(machine code of loops)");
        per_label[best] = 0;
    }
    printf("  %6.2f%%  (OS and other code)\n", 100.0 * other / total);

    printf("\nMost executed instructions:\n");
    for (int n = 0; n < 12; n++)
    {
        int best = -1;
        for (int t = 0; t < 256; t++)
            if (tok_count[t] && (best < 0 || tok_count[t] > tok_count[best]))
                best = t;
        if (best < 0)
            break;
        printf("  %6.2f%%  %s\n", 100.0 * tok_count[best] / ntok, tok_name[best]);
        tok_count[best] = 0;
    }

    printf("\nMost executed instruction pairs:\n");
    for (int n = 0; n < 12; n++)
    {
        int ba = -1, bb = -1;
        for (int a = 0; a < 256; a++)
            for (int b = 0; b < 256; b++)
                if (pair_count[a][b] && (ba < 0 || pair_count[a][b] > pair_count[ba][bb]))
                {
                    ba = a;
                    bb = b;
                }
        if (ba < 0)
            break;
        printf("  %6.2f%%  %s > %s\n", 100.0 * pair_count[ba][bb] / ntok, tok_name[ba],
               tok_name[bb]);
        pair_count[ba][bb] = 0;
    }
    return 0;
}
