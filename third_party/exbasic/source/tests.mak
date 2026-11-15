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

# Rules for running the tests
MINI65=testsuite/mini65
TEST_CFLAGS=-g -O2 -Wall -I$(MINI65)/src/ -I$(MINI65)/ccan/
TEST_LDLIBS=-lm
RUNTEST=build/bin/exbtest$(HOST_EXT)

MINI65_SRC=\
  atari.c\
  ataridos.c\
  atcio.c\
  atedit.c\
  atsio.c\
  dosfname.c\
  hw.c\
  mathpack.c\
  sim65.c\

# All the tests
TESTS := $(sort $(wildcard testsuite/tests/*.chk))

# The tests need to be rerun if any of this files change:
TESTS_DEPS=\
	build/bin/exbasic$(HOST_EXT)\
	build/bin/ca65$(HOST_EXT)\
	build/bin/ld65$(HOST_EXT)\
        $(COMPILER_COMMON)\

TESTS_XEX=$(TESTS:testsuite/%.chk=build/%.xex) $(TESTS:testsuite/%.chk=build/%.com)
TESTS_ROM=$(TESTS:testsuite/%.chk=build/%.rom)
TESTS_ASM=$(TESTS:testsuite/%.chk=build/%.asm)
TESTS_OBJ=$(TESTS:testsuite/%.chk=build/%.o)
TESTS_ATB=$(TESTS:testsuite/%.chk=build/%.atb)
TESTS_LBL=$(TESTS:testsuite/%.chk=build/%.lbl)
TESTS_STAMP=$(TESTS:testsuite/%.chk=build/%.stamp)

# Files created by testsuite/peephole-coverage.sh.  The coverage run uses the
# normal test sources, but writes the outputs into its own directory.
PEEPHOLE_COVERAGE_BAS := $(sort $(wildcard testsuite/tests/*.bas))
PEEPHOLE_COVERAGE_DIR := build/peephole-coverage
PEEPHOLE_COVERAGE_ASM = \
	$(PEEPHOLE_COVERAGE_BAS:testsuite/tests/%.bas=$(PEEPHOLE_COVERAGE_DIR)/%.atari.asm)
PEEPHOLE_COVERAGE_FILES = \
	$(PEEPHOLE_COVERAGE_DIR)/debug.log \
	$(PEEPHOLE_COVERAGE_DIR)/patterns \
	$(PEEPHOLE_COVERAGE_DIR)/matches \
	$(PEEPHOLE_COVERAGE_DIR)/applied \
	$(PEEPHOLE_COVERAGE_DIR)/missing \
	$(PEEPHOLE_COVERAGE_ASM)

RUNTEST_OBJS=build/obj/tests/exbtest.o $(MINI65_SRC:%.c=build/obj/tests/%.o)

# Profiler: runs a program in the simulator and shows where the time goes
EXBPROF=build/bin/exbprof$(HOST_EXT)
EXBPROF_OBJS=build/obj/tests/exbprof.o $(MINI65_SRC:%.c=build/obj/tests/%.o)
# Samples used by "make profile", these run to completion without input
PROFILE_SAMPLES=sieve pi ahlbench
PROFILE_MAX_CYCLES=1000000000

# Profiles the sample programs
.PHONY: profile
profile: $(EXBPROF) $(PROFILE_SAMPLES:%=build/bin/%.xex)
	$(Q)for p in $(PROFILE_SAMPLES); do \
		$(EXBPROF) -q -c $(PROFILE_MAX_CYCLES) build/bin/$$p.xex build/bin/$$p.lbl || exit 1; \
	done

$(EXBPROF): $(EXBPROF_OBJS) | build/bin
	$(ECHO) "Linking $@"
	$(Q)$(CC) $(TEST_CFLAGS) -o $@ $^ $(TEST_LDLIBS)

# Runs the test suite
.PHONY: test
test: $(TESTS_STAMP) $(RUNTEST)

# Checks that the test sources reach every peephole optimizer rule.
.PHONY: peephole-coverage
peephole-coverage: $(EXBASIC_HOST)
	$(Q)$(SHELL) testsuite/peephole-coverage.sh

# exBASIC programs on EOS, from its cartridge image EOS.car (testsuite/eos,
# needs wine, xvfb-run, Altirra and EOS.car, see testsuite/eos/run.py)
.PHONY: eostest
eostest: $(COMPILER_TARGET) $(COMPILER_COMMON) $(MKATR_HOST)
	$(Q)python3 testsuite/eos/run.py -j 3

# Benchmarks against Atari BASIC, Altirra BASIC and FastBasic in Altirra
# (bench/bench.py: needs wine, xvfb-run, Altirra and the ROMs, see there)
.PHONY: bench
bench: $(COMPILER_TARGET) $(COMPILER_COMMON)
	$(Q)python3 bench/bench.py -j 6
	$(Q)python3 bench/bench.py --report

.SECONDEXPANSION:
build/tests/%.stamp: testsuite/tests/%.chk $$(wildcard testsuite/tests/$$*.bas) \
                     $$(wildcard testsuite/tests/$$*.asm) \
                     $(wildcard testsuite/tests/include/*) \
                     $(wildcard testsuite/tests/run/*) \
                     $(RUNTEST) $(TESTS_DEPS) | build/tests
	$(Q)$(RUNTEST) $<
	@touch $@

$(RUNTEST): $(RUNTEST_OBJS) | build/bin
	$(ECHO) "Linking $@"
	$(Q)$(CC) $(TEST_CFLAGS) -o $@ $^ $(TEST_LDLIBS)

build/obj/tests/%.o: $(MINI65)/src/%.c | build/obj/tests $(MINI65)/src
	$(ECHO) "Compiling $<"
	$(Q)$(CC) $(TEST_CFLAGS) -c -o $@ $<

build/obj/tests/%.o: testsuite/src/%.c | build/obj/tests $(MINI65)/src
	$(ECHO) "Compiling $<"
	$(Q)$(CC) $(TEST_CFLAGS) -c -o $@ $<

# Automatic generation of dependency information for C files
build/obj/tests/%.d: testsuite/src/%.c | build/obj/tests $(MINI65)/src
	@$(CC) -MM -MP -MF $@ -MT "$(@:.d=.o) $@" $(TEST_CFLAGS) $<

build/obj/tests/%.d: $(MINI65)/src/%.c | build/obj/tests $(MINI65)/src
	@$(CC) -MM -MP -MF $@ -MT "$(@:.d=.o) $@" $(TEST_CFLAGS) $<

ifneq "$(MAKECMDGOALS)" "clean"
    ifneq "$(MAKECMDGOALS)" "distclean"
        -include $(RUNTEST_OBJS:%.o=%.d) $(EXBPROF_OBJS:%.o=%.d)
    endif
endif
