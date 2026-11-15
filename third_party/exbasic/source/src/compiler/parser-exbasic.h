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

// parser-exbasic.h: exBASIC language features:
//                   FUNC with typed parameters, local variables and multiple
//                   results, COMMON / RUN and STATUS.

#pragma once

#include "codew.h"
#include <map>
#include <string>
#include <vector>

class parse;

// Returns the variable key used for a local variable (parameter, result or
// LOCAL) of a FUNC.  Keys start with a digit, so they can't collide with
// BASIC variable names.
std::string exb_local_key(const std::string &func, const std::string &name);

// Returns the variable key that a name refers to in the current context:
// the local variable if inside a FUNC that declares it, or the global name.
std::string exb_var_key(parse &s, const std::string &name);

// Pre-scan: registers a FUNC from its header line.  The parse object must
// be positioned just after the "FUNC" keyword.  "returns_value" tells if
// the body has a RETURN with a value, used for functions without named
// results.  Returns an error message, or an empty string.
std::string exb_register_func(parse &s, int line, bool returns_value);

// Called after parsing: resolves the recursion save/restore markers in the
// code. Returns an error message, or an empty string.
std::string exb_finalize(parse &s, std::vector<codew> &code);

// Returns the assembly source for the COMMON variables table, or an empty
// string if not needed.
std::string exb_common_table(parse &s);

// Parsing actions
bool exb_E_FUNC_DEF(parse &s);
bool exb_E_FUNC_END(parse &s);
bool exb_E_FUNC_LOCAL(parse &s);
bool exb_E_IN_FUNC(parse &s);
bool exb_E_RET_INT(parse &s);
bool exb_E_RET_STR(parse &s);
bool exb_E_RET_FP(parse &s);
bool exb_E_RET_VI(parse &s);
bool exb_E_RET_VS(parse &s);
bool exb_E_RET_VF(parse &s);
bool exb_E_FUNC_RETURN(parse &s);
bool exb_E_FCALL_I(parse &s);
bool exb_E_FCALL_S(parse &s);
bool exb_E_FCALL_F(parse &s);
bool exb_E_FCALL_STMT(parse &s);
bool exb_E_FCALL_MULTI(parse &s);
bool exb_E_ARG_INT(parse &s);
bool exb_E_ARG_STR(parse &s);
bool exb_E_ARG_FP(parse &s);
bool exb_E_ARG_NEXT(parse &s);
bool exb_E_ARG_BAD(parse &s);
bool exb_E_FCALL_END(parse &s);
bool exb_E_FUNC_UNKNOWN(parse &s);
bool exb_E_MULTI_TARGETS(parse &s);
bool exb_E_LOOP_EXIT(parse &s);
bool exb_E_EMPTY_STRING(parse &s);
bool exb_E_COMMON(parse &s);
bool exb_E_RUN_CALL(parse &s);
bool exb_E_CAT_SET(parse &s);
bool exb_E_CAT_ADD(parse &s);
bool exb_E_CAT_END(parse &s);
