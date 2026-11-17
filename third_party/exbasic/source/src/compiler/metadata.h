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

// metadata.h: machine-readable build metadata (--metadata-json) and feature
// detection (--capabilities=json) for tools that drive the compiler.

#pragma once

#include <string>
#include <utility>
#include <vector>

// Version of the metadata document written by --metadata-json.
#define EXB_METADATA_SCHEMA 1

// One compiler message with its position (1-based line and column; an
// end_column of 0 means "unknown").
struct meta_diag
{
    std::string path;
    int line;
    int column;
    int end_column;
    std::string severity;
    std::string code;
    std::string message;
};

// One executable source line and the assembly label placed at its code.
struct meta_line
{
    std::string file;
    int line;
    std::string label;
    bool native; // 6502 code instead of bytecode
};

// A program symbol and the assembly label that carries its address.
struct meta_symbol
{
    std::string name;
    std::string type;
    std::string label;
    int size;
};

// Everything collected while compiling one program.
struct meta_info
{
    std::vector<std::string> sources;
    std::vector<meta_diag> diags;
    std::vector<meta_line> lines;
    std::vector<meta_symbol> symbols;
    void clear()
    {
        sources.clear();
        diags.clear();
        lines.clear();
        symbols.clear();
    }
};

namespace metadata
{
// Returns the document printed by --capabilities=json.
std::string capabilities_json();

// Builds a diagnostic from a message that may start with "file:line: " or
// "file:line:column: ".
meta_diag diag_from_text(const std::string &text, const std::string &default_file,
                         const std::string &code);

// Writes the metadata document atomically (temporary file, then rename).
// "label_file" is the linker label file of a successful build, or empty.
// Returns false if the file can't be written.
bool write(const std::string &path, bool success, const meta_info &info,
           const std::string &target_name, const std::string &binary_format,
           const std::string &label_file,
           const std::vector<std::pair<std::string, std::string>> &artifacts);
} // namespace metadata
