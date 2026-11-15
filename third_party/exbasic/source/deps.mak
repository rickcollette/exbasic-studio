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

# Make dependencies

$(RT_OBJS) $(RT_OBJS_ROM): src/deftok.inc

$(EXBASIC_HOST_OBJ) $(EXBASIC_TARGET_OBJ): version.mk

# The compiler dependencies - auto-generated from c++ files
EXBASIC_HOST_DEPS=$(EXBASIC_HOST_OBJ:.o=.d)
EXBASIC_TARGET_DEPS=$(EXBASIC_TARGET_OBJ:.o=.d)

# Automatic generation of dependency information for C++ files
build/obj/cxx/%.d: src/compiler/%.cc | build/obj/cxx
	@$(CXX) -MM -MP -MF $@ -MT "$(@:.d=.o) $@" $(EXB_CXX) $(HOST_CXXFLAGS) $<
ifneq ($(CROSS),)
build/obj/cxx-tgt/%.d: src/compiler/%.cc | build/obj/cxx-tgt
	@$(CROSS)$(CXX) -MM -MP -MF $@ -MT "$(@:.d=.o) $@" $(EXB_CXX) $(TARGET_CXXFLAGS) $<
endif

ifneq "$(MAKECMDGOALS)" "clean"
    ifneq "$(MAKECMDGOALS)" "distclean"
        -include $(EXBASIC_HOST_DEPS)
        ifneq ($(CROSS),)
            -include $(EXBASIC_TARGET_DEPS)
        endif
    endif
endif
