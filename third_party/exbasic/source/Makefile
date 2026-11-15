#
#  exBASIC - BASIC compiler for the Atari 8-bit computers
#  Based on FastBasic, Copyright (C) 2017-2025 Daniel Serpell
#
#  This program is free software; you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, either version 2 of the License, or
#  (at your option) any later version.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License along
#  with this program.  If not, see <http://www.gnu.org/licenses/>
#

# Main rule:
target: all

# Set "CROSS" to the compiler prefix to build the compiler for another
# system (ie, Windows from Linux); the host tools are always built too.
CROSS=
CXX=g++
CC=gcc

# Runs SED without locale
SED=LC_ALL=C sed

# Optimization flags, added to C and C++ compiler flags
OPTFLAGS=-O2

# General options for C++ compiler
CXXFLAGS=-Wall -std=c++14

# General flags for 6502 assembly files
CA65_FLAGS=-g -tatari -I cc65/asminc -I src

# Flags added to the ROM-able (cartridge) library
CA65_ROM=-DNO_SMCODE

# Flags for the LD65 linker
LD65_FLAGS=-Ccompiler/exbasic.cfg

# Flags added to C++ sources of the compiler
EXB_CXX=-Isrc/compiler

# Flags to compile the samples
EXB_FLAGS=-target-path:compiler -syntax-path:src/syntax -t:atari

# Flags added to the compilation of CC65 tools (CA65, LD65 and AR65):
CC65_CFLAGS=-Icc65/common -DBUILD_ID="exbasic-$(VERSION)"

# Flags for local tools
HOST_OPTFLAGS=$(OPTFLAGS)
HOST_CXXFLAGS=$(CXXFLAGS) -DVERSION=\"$(VERSION)\" $(HOST_OPTFLAGS)
HOST_CFLAGS=-Wall $(HOST_OPTFLAGS)

# Flags for cross-compilation, could differ from host tools:
TARGET_OPTFLAGS=$(OPTFLAGS)
TARGET_CXXFLAGS=$(CXXFLAGS) -DVERSION=\"$(VERSION)\" $(TARGET_OPTFLAGS)
TARGET_CFLAGS=-Wall $(TARGET_OPTFLAGS)

# By default use quiet build
Q=@
ECHO=@printf "\e[0;32m%s\e[0m\n"

# Detect target Windows OS and set target file extensions:
ifeq ($(strip $(shell echo '_WIN32' | $(CROSS)$(CXX) -E - | grep  "_WIN32")),_WIN32)
    # Linux / OS-X
    TGT_EXT=
else
    # Windows:
    TGT_EXT=.exe
endif

# Detect host Windows OS and set host file extensions:
ifeq ($(strip $(shell echo '_WIN32' | $(CXX) -E - | grep  "_WIN32")),_WIN32)
    # Linux / OS-X
    HOST_EXT=
else
    # Windows:
    HOST_EXT=.exe
endif

# Get version string
include version.mk

# Do not delete intermediate files
.SECONDARY:

ATR=build/exbasic.atr
ZIPFILE=build/exbasic.zip

# Host tools (build machine), and target tools (the distributed compiler)
EXBASIC_HOST=build/bin/exbasic$(HOST_EXT)
CA65_HOST=build/bin/ca65$(HOST_EXT)
LD65_HOST=build/bin/ld65$(HOST_EXT)
AR65_HOST=build/bin/ar65$(HOST_EXT)
MKATR_HOST=build/bin/mkatr$(HOST_EXT)
LSATR_HOST=build/bin/lsatr$(HOST_EXT)

EXBASIC_TARGET=build/compiler/exbasic$(TGT_EXT)
CA65_TARGET=build/compiler/ca65$(TGT_EXT)
LD65_TARGET=build/compiler/ld65$(TGT_EXT)
AR65_TARGET=build/compiler/ar65$(TGT_EXT)
MKATR_TARGET=build/compiler/mkatr$(TGT_EXT)
LSATR_TARGET=build/compiler/lsatr$(TGT_EXT)

# Runtime libraries, for executables and for cartridges
LIB=build/compiler/exbasic.lib
LIB_ROM=build/compiler/exbasic-cart.lib

# Sample programs
SAMPLE_BAS=\
    ahlbench.bas \
    carrera3d.bas \
    dli.bas \
    draw.bas \
    fedora.bas \
    iospeed.bas \
    joyas.bas \
    mastodon.bas \
    nc.bas \
    pi.bas \
    pmtest.bas \
    sieve.bas \

# Files inside the ATR disk image
FILES=\
    build/disk/readme \
    build/disk/startup.bat \
    $(SAMPLE_BAS:%=build/disk/%) \
    $(SAMPLE_BAS:%.bas=build/disk/%.com) \

# BW-DOS files to copy inside the ATR
DOSDIR=disk/dos/
DOS=\
    xbw14d.dos\
    copy.com\
    efast.com\
    pause.com\

# Folders with assembly files
AS_FOLDERS=\
    src\
    src/interp\
    src/interp/a800\
    src/interp/atari\
    src/interp/atarifp\

# Runtime start-up code
RT_AS_SRC=\
    src/standalone.asm\
    src/cartridge.asm\

# Base interpreter
# NOTE: clearmem should be above other files because it defines
#       TOK_END that should be the first token.
BASE_AS_SRC=\
    src/interpreter.asm\
    src/interp/clearmem.asm\
    src/interp/absneg.asm\
    src/interp/addsub.asm\
    src/interp/asc.asm\
    src/interp/bitand.asm\
    src/interp/bitexor.asm\
    src/interp/bitor.asm\
    src/interp/chr.asm\
    src/interp/cmpstr.asm\
    src/interp/comp0.asm\
    src/interp/const.asm\
    src/interp/const_poke.asm\
    src/interp/copystr.asm\
    src/interp/dec.asm\
    src/interp/div.asm\
    src/interp/dpeek.asm\
    src/interp/dpoke.asm\
    src/interp/errors.asm\
    src/interp/dimchk.asm\
    src/interp/shortcirc.asm\
    src/interp/eqbyte.asm\
    src/interp/for.asm\
    src/interp/for_exit.asm\
    src/interp/inc.asm\
    src/interp/jump.asm\
    src/interp/land.asm\
    src/interp/lnot.asm\
    src/interp/lor.asm\
    src/interp/move.asm\
    src/interp/mset.asm\
    src/interp/mul.asm\
    src/interp/native.asm\
    src/interp/natdiv.asm\
    src/interp/natdivb.asm\
    src/interp/natdivs.asm\
    src/interp/natdivsb.asm\
    src/interp/natdivp2.asm\
    src/interp/natmul.asm\
    src/interp/natmulb.asm\
    src/interp/natstk.asm\
    src/interp/negax.asm\
    src/interp/nmove.asm\
    src/interp/peek.asm\
    src/interp/peekb.asm\
    src/interp/poke.asm\
    src/interp/pop.asm\
    src/interp/print_str.asm\
    src/interp/push.asm\
    src/interp/put.asm\
    src/interp/putbyte.asm\
    src/interp/return.asm\
    src/interp/saddr.asm\
    src/interp/saddrstk.asm\
    src/interp/sgn.asm\
    src/interp/shl8.asm\
    src/interp/strfunc.asm\
    src/interp/strindex.asm\
    src/interp/strtmp.asm\
    src/interp/ushl.asm\
    src/interp/usr.asm\
    src/interp/varadd.asm\
    src/interp/varaddr.asm\
    src/interp/varstore.asm\

# Common Atari specific code
ATARI_AS_SRC=\
    src/interp/atari/color.asm\
    src/interp/atari/pause.asm\
    src/interp/atari/pmgraphics.asm\
    src/interp/atari/position.asm\
    src/interp/atari/print_tab.asm\
    src/interp/atari/rand.asm\
    src/interp/atari/soundoff.asm\
    src/interp/atari/time.asm\

# Extended instructions, generated by the optimizer and the FUNC support
EXT_AS_SRC=\
    src/interp/xdivmod.asm\
    src/interp/xdivpow2.asm\
    src/interp/xfornext.asm\
    src/interp/xframe.asm\
    src/interp/xpokeaxb.asm\
    src/interp/xpvaradd.asm\
    src/interp/xvaradd.asm\
    src/interp/a800/xrun.asm\
    src/interp/a800/ldchk.asm\

# The runtime, the linker only includes the parts used by each program
RUNTIME_AS_SRC=\
    $(RT_AS_SRC)\
    $(BASE_AS_SRC)\
    $(ATARI_AS_SRC)\
    $(EXT_AS_SRC)\
    src/interp/a800/bgetput.asm\
    src/interp/a800/getkey.asm\
    src/interp/a800/cio.asm\
    src/interp/a800/close.asm\
    src/interp/a800/drawto.asm\
    src/interp/a800/get.asm\
    src/interp/a800/graphics.asm\
    src/interp/a800/input.asm\
    src/interp/a800/iochn.asm\
    src/interp/a800/putchar.asm\
    src/interp/a800/str.asm\
    src/interp/a800/fpconv.asm\
    src/interp/a800/streol.asm\
    src/interp/a800/val.asm\
    src/interp/a800/notepoint.asm\
    src/interp/a800/xio.asm\
    src/interp/a800/xiocmd.asm\
    src/interp/a800/fujinet.asm\
    src/interp/atarifp/fp_coef.asm\
    src/interp/atarifp/fp_abs.asm\
    src/interp/atarifp/fp_atn.asm\
    src/interp/atarifp/fp_cmp.asm\
    src/interp/atarifp/fp_const.asm\
    src/interp/atarifp/fp_div.asm\
    src/interp/atarifp/fp_evalpoly.asm\
    src/interp/atarifp/fp_exp.asm\
    src/interp/atarifp/fp_exp10.asm\
    src/interp/atarifp/fp_int.asm\
    src/interp/atarifp/fp_intfp.asm\
    src/interp/atarifp/fp_ipow.asm\
    src/interp/atarifp/fp_load.asm\
    src/interp/atarifp/fp_log.asm\
    src/interp/atarifp/fp_log10.asm\
    src/interp/atarifp/fp_mul.asm\
    src/interp/atarifp/fp_pop.asm\
    src/interp/atarifp/fp_push.asm\
    src/interp/atarifp/fp_rnd.asm\
    src/interp/atarifp/fp_set1.asm\
    src/interp/atarifp/fp_sgn.asm\
    src/interp/atarifp/fp_sincos.asm\
    src/interp/atarifp/fp_sqrt.asm\
    src/interp/atarifp/fp_store.asm\
    src/interp/atarifp/fp_str.asm\
    src/interp/atarifp/fp_sub.asm\
    src/interp/atarifp/fp_val.asm\
    src/interp/atarifp/fp_time.asm\
    src/interp/atarifp/fpmain.asm\
    src/interp/atarifp/mul6.asm\

# Object files of each runtime library
RT_OBJS=$(RUNTIME_AS_SRC:src/%.asm=build/obj/rt/%.o)
RT_OBJS_ROM=$(RUNTIME_AS_SRC:src/%.asm=build/obj/rom/%.o)
SAMP_OBJS=$(SAMPLE_BAS:%.bas=build/obj/samples/%.o)

# Pages of the programmer's manual
MANUAL_PAGES=$(sort $(wildcard doc/manual/*.md))

# Files of the compiler distribution
COMPILER_COMMON=\
	 $(LIB)\
	 $(LIB_ROM)\
	 build/compiler/exbasic.cfg\
	 build/compiler/exbasic-cart.cfg\
	 build/compiler/USAGE.md\
	 build/compiler/LICENSE\
	 $(MANUAL_PAGES:doc/manual/%=build/compiler/manual/%)\
	 build/compiler/asminc/atari_antic.inc\
	 build/compiler/asminc/atari_gtia.inc\
	 build/compiler/asminc/atari.inc\
	 build/compiler/asminc/atari_pokey.inc\
	 build/compiler/asminc/target.inc\
	 build/compiler/syntax/a800.syn\
	 build/compiler/syntax/basic.syn\
	 build/compiler/syntax/dli.syn\
	 build/compiler/syntax/exbasic.syn\
	 build/compiler/syntax/exbasic-a800.syn\
	 build/compiler/syntax/exbasic-fp.syn\
	 build/compiler/syntax/extended.syn\
	 build/compiler/syntax/fileio.syn\
	 build/compiler/syntax/float.syn\
	 build/compiler/syntax/fujinet.syn\
	 build/compiler/syntax/graphics.syn\
	 build/compiler/syntax/pm.syn\
	 build/compiler/syntax/sio.syn\
	 build/compiler/syntax/sound.syn\
	 build/compiler/atari.tgt\
	 build/compiler/atari-cart.tgt\
	 build/compiler/default.tgt\

# Compiler source files (C++)
COMPILER_SRC=\
	atarifp.cc\
	codestat.cc\
	compile.cc\
	ifile.cc\
	looptype.cc\
	main.cc\
	os.cc\
	parser.cc\
	parser-actions.cc\
	parser-exbasic.cc\
	native.cc\
	peephole.cc\
	target.cc\
	vartype.cc\
	synt-optimize.cc\
	synt-parser.cc\
	synt-preproc.cc\
	synt-pstate.cc\
	synt-sm.cc\
	synt-symlist.cc\
	synt-wlist.cc\

CA65_SRC=\
	cc65/ca65/anonname.c\
	cc65/ca65/asserts.c\
	cc65/ca65/condasm.c\
	cc65/ca65/dbginfo.c\
	cc65/ca65/ea65.c\
	cc65/ca65/easw16.c\
	cc65/ca65/enum.c\
	cc65/ca65/error.c\
	cc65/ca65/expr.c\
	cc65/ca65/feature.c\
	cc65/ca65/filetab.c\
	cc65/ca65/fragment.c\
	cc65/ca65/global.c\
	cc65/ca65/incpath.c\
	cc65/ca65/instr.c\
	cc65/ca65/istack.c\
	cc65/ca65/lineinfo.c\
	cc65/ca65/listing.c\
	cc65/ca65/macro.c\
	cc65/ca65/main.c\
	cc65/ca65/nexttok.c\
	cc65/ca65/objcode.c\
	cc65/ca65/objfile.c\
	cc65/ca65/options.c\
	cc65/ca65/pseudo.c\
	cc65/ca65/repeat.c\
	cc65/ca65/scanner.c\
	cc65/ca65/segdef.c\
	cc65/ca65/segment.c\
	cc65/ca65/sizeof.c\
	cc65/ca65/span.c\
	cc65/ca65/spool.c\
	cc65/ca65/struct.c\
	cc65/ca65/studyexpr.c\
	cc65/ca65/symbol.c\
	cc65/ca65/symentry.c\
	cc65/ca65/symtab.c\
	cc65/ca65/token.c\
	cc65/ca65/toklist.c\
	cc65/ca65/ulabel.c\
	cc65/common/abend.c\
	cc65/common/addrsize.c\
	cc65/common/alignment.c\
	cc65/common/assertion.c\
	cc65/common/bitops.c\
	cc65/common/chartype.c\
	cc65/common/check.c\
	cc65/common/cmdline.c\
	cc65/common/coll.c\
	cc65/common/cpu.c\
	cc65/common/debugflag.c\
	cc65/common/exprdefs.c\
	cc65/common/filestat.c\
	cc65/common/fname.c\
	cc65/common/gentype.c\
	cc65/common/hashfunc.c\
	cc65/common/hashtab.c\
	cc65/common/intstack.c\
	cc65/common/mmodel.c\
	cc65/common/print.c\
	cc65/common/searchpath.c\
	cc65/common/segnames.c\
	cc65/common/shift.c\
	cc65/common/strbuf.c\
	cc65/common/strpool.c\
	cc65/common/strutil.c\
	cc65/common/target.c\
	cc65/common/tgttrans.c\
	cc65/common/version.c\
	cc65/common/xmalloc.c\
	cc65/common/xsprintf.c\

LD65_SRC=\
	cc65/common/abend.c\
	cc65/common/addrsize.c\
	cc65/common/alignment.c\
	cc65/common/assertion.c\
	cc65/common/chartype.c\
	cc65/common/check.c\
	cc65/common/cmdline.c\
	cc65/common/coll.c\
	cc65/common/exprdefs.c\
	cc65/common/fileid.c\
	cc65/common/filetype.c\
	cc65/common/fname.c\
	cc65/common/gentype.c\
	cc65/common/hashfunc.c\
	cc65/common/hashtab.c\
	cc65/common/print.c\
	cc65/common/searchpath.c\
	cc65/common/strbuf.c\
	cc65/common/strpool.c\
	cc65/common/strutil.c\
	cc65/common/target.c\
	cc65/common/version.c\
	cc65/common/xmalloc.c\
	cc65/common/xsprintf.c\
	cc65/ld65/asserts.c\
	cc65/ld65/bin.c\
	cc65/ld65/binfmt.c\
	cc65/ld65/cfgexpr.c\
	cc65/ld65/condes.c\
	cc65/ld65/config.c\
	cc65/ld65/dbgfile.c\
	cc65/ld65/dbgsyms.c\
	cc65/ld65/error.c\
	cc65/ld65/exports.c\
	cc65/ld65/expr.c\
	cc65/ld65/extsyms.c\
	cc65/ld65/fileinfo.c\
	cc65/ld65/fileio.c\
	cc65/ld65/filepath.c\
	cc65/ld65/fragment.c\
	cc65/ld65/global.c\
	cc65/ld65/library.c\
	cc65/ld65/lineinfo.c\
	cc65/ld65/main.c\
	cc65/ld65/mapfile.c\
	cc65/ld65/memarea.c\
	cc65/ld65/o65.c\
	cc65/ld65/objdata.c\
	cc65/ld65/objfile.c\
	cc65/ld65/scanner.c\
	cc65/ld65/scopes.c\
	cc65/ld65/segments.c\
	cc65/ld65/span.c\
	cc65/ld65/spool.c\
	cc65/ld65/tpool.c\
	cc65/ld65/xex.c\

AR65_SRC=\
	cc65/common/abend.c\
	cc65/common/chartype.c\
	cc65/common/check.c\
	cc65/common/cmdline.c\
	cc65/common/coll.c\
	cc65/common/filestat.c\
	cc65/common/filetime.c\
	cc65/common/fname.c\
	cc65/common/hashfunc.c\
	cc65/common/print.c\
	cc65/common/version.c\
	cc65/common/xmalloc.c\
	cc65/common/xsprintf.c\
	cc65/ar65/add.c\
	cc65/ar65/del.c\
	cc65/ar65/error.c\
	cc65/ar65/exports.c\
	cc65/ar65/extract.c\
	cc65/ar65/fileio.c\
	cc65/ar65/global.c\
	cc65/ar65/library.c\
	cc65/ar65/list.c\
	cc65/ar65/main.c\
	cc65/ar65/objdata.c\
	cc65/ar65/objfile.c\

MKATR_SRC=\
	mkatr/src/atr.c\
	mkatr/src/compat.c\
	mkatr/src/crc32.c\
	mkatr/src/darray.c\
	mkatr/src/flist.c\
	mkatr/src/mkatr.c\
	mkatr/src/msg.c\
	mkatr/src/spartafs.c\

LSATR_SRC=\
	mkatr/src/atr.c\
	mkatr/src/compat.c\
	mkatr/src/crc32.c\
	mkatr/src/lsatr.c\
	mkatr/src/lsdos.c\
	mkatr/src/lsextra.c\
	mkatr/src/lshowfen.c\
	mkatr/src/lssfs.c\
	mkatr/src/msg.c\

# Host tools
COMPILER_HOST=\
	 $(CA65_HOST)\
	 $(LD65_HOST)\
	 $(AR65_HOST)\
	 $(MKATR_HOST)\
	 $(LSATR_HOST)\
	 $(EXBASIC_HOST)\

# Target tools
COMPILER_TARGET=\
	 $(CA65_TARGET)\
	 $(LD65_TARGET)\
	 $(AR65_TARGET)\
	 $(MKATR_TARGET)\
	 $(LSATR_TARGET)\
	 $(EXBASIC_TARGET)\

# On Windows, add manifests
COMPILER_MANIFESTS=$(filter %.manifest, $(COMPILER_TARGET:.exe=.exe.manifest))

# All assembly output files
OBJS=$(RT_OBJS) $(RT_OBJS_ROM) $(SAMP_OBJS)
LSTS=$(OBJS:%.o=%.lst)

# Sample programs, with map and label files
XEXS=$(SAMPLE_BAS:%.bas=build/bin/%.xex)
MAPS=$(SAMPLE_BAS:%.bas=build/bin/%.map)
LBLS=$(SAMPLE_BAS:%.bas=build/bin/%.lbl)

# The compiler object files, for HOST and TARGET
EXBASIC_HOST_OBJ=$(COMPILER_SRC:%.cc=build/obj/cxx/%.o)
EXBASIC_TARGET_OBJ=$(COMPILER_SRC:%.cc=build/obj/cxx-tgt/%.o)

# All folders created during compilation:
BUILD_FOLDERS=\
 $(AS_FOLDERS:src%=build/obj/rt%)\
 $(AS_FOLDERS:src%=build/obj/rom%)\
 build/obj/samples\
 build/bin\
 build/compiler/asminc\
 build/compiler/syntax\
 build/compiler/manual\
 build/compiler\
 build/disk\
 build/gen\
 build/obj/cxx\
 build/obj/cxx-tgt\
 build/obj/tests\
 build/obj\
 build/tests\
 build\

include tests.mak
include rules.mak
include deps.mak
