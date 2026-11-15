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

# Main compilation rules
all: $(ATR) $(COMPILER_COMMON) $(COMPILER_TARGET) $(COMPILER_MANIFESTS)

dist: $(ATR) $(ZIPFILE)

# Make sure that failed targets are deleted
.DELETE_ON_ERROR:

.PHONY: clean
clean:
	$(Q)rm -f $(OBJS) $(LSTS)
	$(Q)rm -f $(FILES)
	$(Q)rm -f $(ATR) $(ZIPFILE)
	$(Q)rm -f $(XEXS) $(MAPS) $(LBLS)
	$(Q)rm -f $(COMPILER_HOST)
	$(Q)rm -f $(EXBASIC_TARGET_OBJ) $(EXBASIC_HOST_OBJ)
	$(Q)rm -f $(EXBASIC_HOST_DEPS) $(EXBASIC_TARGET_DEPS)
	$(Q)rm -f $(SAMPLE_BAS:%.bas=build/gen/%.asm)
	$(Q)rm -f $(TESTS_XEX) $(TESTS_ROM) $(TESTS_ASM) $(TESTS_OBJ)
	$(Q)rm -f $(TESTS_ATB) $(TESTS_LBL) $(TESTS_STAMP)
	$(Q)rm -f $(PEEPHOLE_COVERAGE_FILES)
	$(Q)rmdir $(PEEPHOLE_COVERAGE_DIR) 2>/dev/null || true
	$(Q)rm -rf build/tests
	$(Q)rm -f $(RUNTEST_OBJS) $(RUNTEST) $(RUNTEST_OBJS:.o=.d)
	$(Q)rm -f $(EXBPROF_OBJS) $(EXBPROF) $(EXBPROF_OBJS:.o=.d)

.PHONY: distclean
distclean: clean
	$(Q)-rm -f $(COMPILER_HOST) $(COMPILER_TARGET) $(COMPILER_COMMON) \
	    $(COMPILER_MANIFESTS)
	$(Q)printf "%s\n" $(BUILD_FOLDERS) | sort -r | while read folder; do \
		test -d $$folder && rmdir $$folder || true ; done
	# The build folder only has generated files, remove what is left from
	# older versions of the sources.
	$(Q)rm -rf build

# Build an ATR disk image using "mkatr".
$(ATR): $(DOS:%=$(DOSDIR)/%) $(FILES) | $(MKATR_HOST) build
	$(ECHO) "Creating ATR disk image"
	$(Q)$(MKATR_HOST) $@ $(DOSDIR) -b $^

# Build compiler ZIP file.
$(ZIPFILE): $(COMPILER_COMMON) $(COMPILER_TARGET) $(COMPILER_MANIFESTS) | build
	$(CROSS)strip $(COMPILER_TARGET)
	# Store only the paths relative to the compiler directory
	(cd build/compiler ; zip -9v ../../$@ $(COMPILER_COMMON:build/compiler/%=%) $(COMPILER_TARGET:build/compiler/%=%) $(COMPILER_MANIFESTS:build/compiler/%=%) )

# BAS sources transformed to ATASCII (replace $0A with $9B)
build/disk/%.bas: samples/%.bas | build/disk
	$(Q)LC_ALL=C tr '\n' '\233' < $< > $@

# Transform a text file to ATASCII (replace $0A with $9B)
build/disk/%: % version.mk | build/disk
	$(Q)$(SED) 's/%VERSION%/$(VERSION)/' < $< | LC_ALL=C tr '\n' '\233' > $@

build/disk/%.txt: %.md version.mk | build/disk
	$(Q)$(SED) 's/%VERSION%/$(VERSION)/' < $< | LC_ALL=C awk 'BEGIN{for(n=0;n<127;n++)chg[sprintf("%c",n)]=128+n} {l=length($$0);for(i=1;i<=l;i++){c=substr($$0,i,1);if(c=="`"){x=1-x;if(x)c="\002";else c="\026";}else if(x)c=chg[c];printf "%c",c;}printf "\233";}' > $@

# Copy ".XEX" as ".COM"
build/disk/%.com: build/bin/%.xex | build/disk
	$(Q)cp $< $@

# Host compiler build
build/obj/cxx/%.o: src/compiler/%.cc | build/obj/cxx
	$(ECHO) "Compile $<"
	$(Q)$(CXX) $(HOST_CXXFLAGS) $(EXB_CXX) -c -o $@ $<

$(EXBASIC_HOST): $(EXBASIC_HOST_OBJ) | build/bin
	$(ECHO) "Linking host compiler"
	$(Q)$(CXX) $(HOST_CXXFLAGS) $(EXB_CXX) -o $@ $^

$(CA65_HOST): $(CA65_SRC) | build/bin
	$(ECHO) "Compile CA65"
	$(Q)$(CC) $(HOST_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(LD65_HOST): $(LD65_SRC) | build/bin
	$(ECHO) "Compile LD65"
	$(Q)$(CC) $(HOST_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(AR65_HOST): $(AR65_SRC) | build/bin
	$(ECHO) "Compile AR65"
	$(Q)$(CC) $(HOST_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(MKATR_HOST): $(MKATR_SRC) | build/bin
	$(ECHO) "Compile MKATR"
	$(Q)$(CC) $(HOST_CFLAGS) -o $@ $^

$(LSATR_HOST): $(LSATR_SRC) | build/bin
	$(ECHO) "Compile LSATR"
	$(Q)$(CC) $(HOST_CFLAGS) -o $@ $^

# Target compiler build
ifeq ($(CROSS),)
# No cross-compilation, just copy host tools to target tools:
$(COMPILER_TARGET): build/compiler/%$(TGT_EXT): build/bin/%$(HOST_EXT) | build/compiler
	$(Q)cp -f $< $@
else
# Cross-compilation: compile for target
build/obj/cxx-tgt/%.o: src/compiler/%.cc | build/obj/cxx-tgt
	$(ECHO) "Compile target $<"
	$(Q)$(CROSS)$(CXX) $(TARGET_CXXFLAGS) $(EXB_CXX) -c -o $@ $<

$(EXBASIC_TARGET): $(EXBASIC_TARGET_OBJ) | build/compiler
	$(ECHO) "Linking target compiler"
	$(Q)$(CROSS)$(CXX) $(TARGET_CXXFLAGS) $(EXB_CXX) -o $@ $^

$(CA65_TARGET): $(CA65_SRC) | build/compiler
	$(ECHO) "Compile target CA65"
	$(Q)$(CROSS)$(CC) $(TARGET_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(LD65_TARGET): $(LD65_SRC) | build/compiler
	$(ECHO) "Compile target LD65"
	$(Q)$(CROSS)$(CC) $(TARGET_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(AR65_TARGET): $(AR65_SRC) | build/compiler
	$(ECHO) "Compile target AR65"
	$(Q)$(CROSS)$(CC) $(TARGET_CFLAGS) $(CC65_CFLAGS) -o $@ $^

$(MKATR_TARGET): $(MKATR_SRC) | build/compiler
	$(ECHO) "Compile target MKATR"
	$(Q)$(CROSS)$(CC) $(TARGET_CFLAGS) -o $@ $^

$(LSATR_TARGET): $(LSATR_SRC) | build/compiler
	$(ECHO) "Compile target LSATR"
	$(Q)$(CROSS)$(CC) $(TARGET_CFLAGS) -o $@ $^
endif

# Windows manifests
build/%.manifest: compiler/utf8.manifest
	$(Q)cp -f $< $@

# Sample programs: BASIC to assembly, with the host compiler
build/gen/%.asm: samples/%.bas $(EXBASIC_HOST) | build/gen
	$(ECHO) "Compiling BASIC sample $<"
	$(Q)$(EXBASIC_HOST) $(EXB_FLAGS) -o $@ -c $<

build/obj/samples/%.o: build/gen/%.asm | build/obj/samples $(CA65_HOST)
	$(ECHO) "Assembly $<"
	$(Q)$(CA65_HOST) $(CA65_FLAGS) -l $(@:.o=.lst) -o $@ $<

# Sample programs: link with the runtime library
build/bin/%.xex: build/obj/samples/%.o $(LIB) | build/bin $(LD65_HOST)
	$(ECHO) "Linking $@"
	$(Q)$(LD65_HOST) $(LD65_FLAGS) -Ln $(@:.xex=.lbl) -vm -m $(@:.xex=.map) -o $@ $^

# Runtime object files
build/obj/rt/%.o: src/%.asm | $(AS_FOLDERS:src%=build/obj/rt%) $(CA65_HOST)
	$(ECHO) "Assembly $<"
	$(Q)$(CA65_HOST) $(CA65_FLAGS) -l $(@:.o=.lst) -o $@ $<

build/obj/rom/%.o: src/%.asm | $(AS_FOLDERS:src%=build/obj/rom%) $(CA65_HOST)
	$(ECHO) "Assembly cartridge $<"
	$(Q)$(CA65_HOST) $(CA65_FLAGS) $(CA65_ROM) -l $(@:.o=.lst) -o $@ $<

# Rule to build all folders
$(BUILD_FOLDERS):
	$(Q)mkdir -p $@

# Runtime libraries
$(LIB): $(RT_OBJS) | build/compiler $(AR65_HOST)
	$(ECHO) "Creating library $@"
	$(Q)rm -f $@
	$(Q)$(AR65_HOST) a $@ $^

$(LIB_ROM): $(RT_OBJS_ROM) | build/compiler $(AR65_HOST)
	$(ECHO) "Creating cartridge library $@"
	$(Q)rm -f $@
	$(Q)$(AR65_HOST) a $@ $^

# Copy the manual pages to the compiler folder, changing the version string.
build/compiler/manual/%.md: doc/manual/%.md version.mk | build/compiler/manual
	$(Q)LC_ALL=C sed 's/%VERSION%/$(VERSION)/' $< > $@

# Copy syntax files to compiler folder
build/compiler/syntax/%: src/syntax/% | build/compiler/syntax
	$(Q)cp -f $< $@

# Copy other files to compiler folder
build/compiler/%: compiler/% | build/compiler
	$(Q)cp -f $< $@

# Copy assembly include files from CC65
build/compiler/asminc/%: cc65/asminc/% | build/compiler/asminc
	$(Q)cp -f $< $@
