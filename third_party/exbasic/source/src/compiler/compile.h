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

// compile.h: Main compiler class

#pragma once

#include <string>
#include <vector>

namespace syntax
{
class sm_list;
}

class compiler
{
  public:
    std::string segname;
    bool do_debug;
    bool optimize;
    bool extended;
    bool native;
    // Machine code selection: only the code that runs often, or all the
    // code that has a machine code translation.
    bool native_all;
    // Leaves in bytecode the loops where machine code saves little time
    bool native_lean;
    bool show_stats;
    bool show_text;
    unsigned short_text;
    std::vector<std::string> include_dirs;

    compiler();
    int compile_file(std::string input_filename, std::string output_filename,
                     const syntax::sm_list &sl, std::string listing_filename);
};
