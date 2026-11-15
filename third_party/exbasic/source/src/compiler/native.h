/*
 * exBASIC - BASIC compiler for the Atari 8-bit computers
 * Based on FastBasic, Copyright (C) 2017-2025 Daniel Serpell
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

// native.h: compiles the integer code inside loops to 6502 machine code

#pragma once

#include "codew.h"
#include <set>
#include <string>
#include <vector>

// Replaces the bytecode of loops with machine code where possible. Returns
// true if any machine code was generated, and the runtime subroutines used.
// With "all", all the code with a machine code translation is compiled, else
// only the code that runs often, estimated from the loops.  With "lean",
// the loops where the machine code would save little time (most of their
// time goes to floating point, strings or input and output) stay bytecode.
bool do_native(std::vector<codew> &code, std::set<std::string> &helpers, bool all,
               bool lean = false);
