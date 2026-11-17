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

// compile.cc: Main compiler

#include <cctype>
#include "compile.h"
#include <algorithm>
#include <cctype>
#include <fstream>
#include <iostream>

#include "codestat.h"
#include "os.h"
#include "parser-exbasic.h"
#include "parser.h"
#include "native.h"
#include "parser-actions.h"
#include "peephole.h"
#include "vartype.h"

// Reads a complete source line, respecting ATASCII and ASCII EOL inly
// outside strings.
static int readLine(std::string &r, std::istream &is)
{
    // Special handling of EOL: We allow Unix / DOS line endings - except inside
    // strings, where those bytes are part of the string.
    // To properly split lines then, we must pre-parse the content, skipping
    // comments and keeping track of the strings.
    bool in_string = false;
    bool in_comment = false;
    bool in_start = true;
    int num_lines = 0;
    while(-1 != is.peek())
    {
        char c = is.get();
        r += c;
        if(in_string)
        {
            // Inside strings, consume any char except for the '"'
            // but increase line-number for easier debugging.
            if(c == '\x0A')
                num_lines++;
            else if(c == '\"')
                in_string = false;
            continue;
        }
        // Check for DOS end of line
        if(c == '\x0D' && '\x0A' == is.peek())
        {
            // Check if next char is 0x0A and replace
            c = is.get();
            r[r.size() - 1] = c;
        }
        // Check for any end of line
        if(c == '\x0A' || c == '\x9B')
            return num_lines + 1;
        // Check we are not entering a string or a comment
        if(in_start)
        {
            if(c == '.' || c == '\'')
                in_comment = true;
            if(c != ' ')
                in_start = false;
        }
        if(!in_comment)
        {
            if(c == '\'')
                in_comment = true;
            else if(c == ':')
                in_start = true;
            else if(c == '\"')
                in_string = true;
        }
    }
    return 0;
}

// Parses one source line

// Looks for common mistakes in a statement that can't be parsed, to give a
// clear message: unbalanced parentheses and integers that are too big.
static void diagnose_line(const std::string &line, size_t start)
{
    int depth = 0;
    size_t open_pos = 0;
    auto upper = [&](size_t i) { return std::toupper((unsigned char)line[i]); };
    auto name_char = [&](size_t i) {
        return i < line.size() && (std::isalnum((unsigned char)line[i]) || line[i] == '_');
    };
    for(size_t i = start; i < line.size(); i++)
    {
        char c = line[i];
        if(c == '"')
        {
            // Skip the string; doubled quotes are part of it
            for(i++; i < line.size(); i++)
                if(line[i] == '"' && !(i + 1 < line.size() && line[i + 1] == '"'))
                    break;
                else if(line[i] == '"')
                    i++;
            continue;
        }
        // A comment ends the scan
        if(c == '\'')
            break;
        if(upper(i) == 'R' && i + 2 < line.size() && upper(i + 1) == 'E' && upper(i + 2) == 'M' &&
           (i == 0 || !name_char(i - 1)) && !name_char(i + 3))
            break;
        if(c == ':' && depth == 0)
            break;
        if(c == '(')
        {
            if(!depth)
                open_pos = i;
            depth++;
        }
        else if(c == ')')
        {
            if(!depth)
                throw parse_error("')' without '('", i);
            depth--;
        }
        else if(std::isdigit((unsigned char)c) && (i == 0 || !name_char(i - 1)) &&
                (i == 0 || line[i - 1] != '.' ) && (i == 0 || line[i - 1] != '$'))
        {
            // An integer constant, not part of a name or a decimal number
            size_t j = i;
            unsigned long n = 0;
            while(j < line.size() && std::isdigit((unsigned char)line[j]))
            {
                if(n <= 1000000)
                    n = n * 10 + (line[j] - '0');
                j++;
            }
            bool fp = j < line.size() && (line[j] == '.' || upper(j) == 'E');
            if(!fp && n > 65535)
                throw parse_error("integer constant out of range (max 65535), write " +
                                      line.substr(i, j - i) + ".0 for a floating point number",
                                  i);
            i = j - 1;
        }
    }
    if(depth > 0)
        throw parse_error("missing ')' for this '('", open_pos);
}

static bool parse_line(std::string line, int ln, parse &s, bool show_text,
                       unsigned short_text, const syntax::sm_list &sl,
                       std::string &short_line, std::ostream &list_file)
{
    s.new_line(line, ln);
    s.line_ifs = 0;
    while(s.pos != line.length())
    {
        // Temporary strings of "+" are reused in each statement
        s.cat_chains.clear();
        s.cat_open.clear();
        s.block_error.clear();
        auto stmt_start = s.pos;
        if(!syntax::parse_start(s, sl) ||
           (s.pos != line.length() && !s.peek(':') && !line_if_else_next(s)))
        {
            if(!s.block_error.empty())
            {
                // Name the statement: "ELIF without IF"
                auto msg = s.block_error;
                size_t b = stmt_start, e;
                while(b < line.size() && line[b] == ' ')
                    b++;
                for(e = b; e < line.size() && std::isalpha((unsigned char)line[e]); e++)
                    ;
                if(e - b >= 2 && (e >= line.size() || line[e] != '.') &&
                   msg.compare(0, 8, "missing ") == 0)
                {
                    std::string kw = line.substr(b, e - b);
                    for(auto &c : kw)
                        c = std::toupper((unsigned char)c);
                    msg = kw + " without " + msg.substr(8);
                }
                throw parse_error(msg, s.block_error_pos);
            }
            diagnose_line(line, stmt_start);
            std::string msg = "parse error";
            if(!s.saved_errors.empty())
            {
                // Get min level
                auto ml =
                    std::min_element(s.saved_errors.begin(), s.saved_errors.end(),
                                     [](auto &a, auto &b) { return a.lvl < b.lvl; });
                msg += ", expected: ";
                bool first = true;
                for(const auto &i : s.saved_errors)
                {
                    if(i.lvl == ml->lvl)
                    {
                        if(!first)
                            msg += ", ";
                        msg += i.msg;
                        first = false;
                    }
                }
            }
            throw parse_error(msg, s.max_pos);
        }
        else
        {
            if(short_text > 0)
            {
                auto txt = s.expand.get_short();
                if(txt.size())
                {
                    if(!short_line.size())
                        short_line = txt;
                    else if(short_line.size() + 1 + txt.size() <= short_text)
                    {
                        short_line += ':';
                        short_line += txt;
                    }
                    else
                    {
                        list_file << short_line << '\x9B';
                        short_line = txt;
                    }
                }
            }
            else if(show_text)
                list_file << s.expand.get() << "\n";
        }

        s.expect(':');
    }
    // Close the "IF ... THEN" of this line
    for(; s.line_ifs > 0; s.line_ifs--)
        if(!close_line_if(s))
            throw parse_error("parse error, missing IF", s.pos);
    return true;
}

static int show_error(std::string msg)
{
    std::cerr << "exbasic: " << msg << "\n";
    return 1;
}

static char printable(char c)
{
    if(c < 32)
        return '.';
    else if(c < 127)
        return c;
    else
        return '.';
}

compiler::compiler()
{
    optimize = true;
    native = true;
    native_all = false;
    native_lean = false;
    extended = true;
    segname = "BYTECODE";
    show_stats = false;
    show_text = false;
    short_text = 0;
    do_debug = false;
    collect_meta = false;
}

// One line of source, after expanding INCLUDE
struct src_line
{
    std::string text;
    std::string file;
    int line;   // line number in the file
    int nlines; // number of physical lines
};

static std::string to_upper(std::string s)
{
    for(auto &c : s)
        if(c >= 'a' && c <= 'z')
            c = c - 'a' + 'A';
    return s;
}

// Checks if the line is an INCLUDE "file" directive, returns the file name
static bool is_include(const std::string &line, std::string &fname, bool &bad)
{
    size_t p = 0;
    bad = false;
    while(p < line.size() && (line[p] == ' ' || line[p] == '\t'))
        p++;
    if(to_upper(line.substr(p, 7)) != "INCLUDE")
        return false;
    p += 7;
    // Must be followed by a space or a quote, so "INCLUDEX=1" is a variable
    if(p < line.size() && line[p] != ' ' && line[p] != '\t' && line[p] != '"')
        return false;
    while(p < line.size() && (line[p] == ' ' || line[p] == '\t'))
        p++;
    bad = true;
    if(p >= line.size() || line[p] != '"')
        return true;
    auto e = line.find('"', p + 1);
    if(e == std::string::npos || e == p + 1)
        return true;
    fname = line.substr(p + 1, e - p - 1);
    p = e + 1;
    while(p < line.size() && (line[p] == ' ' || line[p] == '\t'))
        p++;
    if(p < line.size() && line[p] != '\'' && line[p] != '\n' && line[p] != '\r' &&
       line[p] != '\x9B')
        return true;
    bad = false;
    return true;
}

// Reads a source file expanding INCLUDE directives
static std::string read_source(const std::string &fname, std::vector<src_line> &out,
                               std::vector<std::string> &stack,
                               const std::vector<std::string> &include_dirs,
                               std::vector<std::string> &files)
{
    if(stack.size() > 16)
        return "too many nested INCLUDE files";
    for(auto &f : stack)
        if(f == fname)
            return "recursive INCLUDE of '" + fname + "'";
    std::ifstream ifile(fname, std::ios::binary);
    if(!ifile.is_open())
        return "can't open input file '" + fname + "'";
    stack.push_back(fname);
    if(std::find(files.begin(), files.end(), fname) == files.end())
        files.push_back(fname);
    int ln = 1;
    while(1)
    {
        std::string line;
        int lines = readLine(line, ifile);
        if(!lines && line.empty())
            break;
        std::string inc;
        bool bad;
        if(is_include(line, inc, bad))
        {
            auto where = fname + ":" + std::to_string(ln) + ": ";
            if(bad)
                return where + "invalid INCLUDE, use: INCLUDE \"file\"";
            // Search relative to the current file, then in the include paths
            std::vector<std::string> paths;
            if(!os::path_absolute(inc))
            {
                paths.push_back(os::dir_name(fname));
                paths.insert(paths.end(), include_dirs.begin(), include_dirs.end());
                paths.push_back(os::compiler_path("include"));
            }
            else
                paths.push_back(std::string());
            std::string found;
            for(auto &d : paths)
            {
                auto full = d.empty() ? inc : os::full_path(d, inc);
                std::ifstream t(full, std::ios::binary);
                if(t.is_open())
                {
                    found = full;
                    break;
                }
            }
            if(found.empty())
                return where + "INCLUDE file '" + inc + "' not found";
            auto e = read_source(found, out, stack, include_dirs, files);
            if(e.size())
                return e;
            // Keep the line count of the INCLUDE line
            out.push_back({"\n", fname, ln, lines ? lines : 1});
        }
        else
            out.push_back({line, fname, ln, lines ? lines : 1});
        ln += lines;
    }
    stack.pop_back();
    return std::string();
}

// Splits a line in statements, skipping strings and comments
static std::vector<std::string> split_statements(const std::string &line)
{
    std::vector<std::string> ret(1);
    bool in_str = false;
    for(size_t i = 0; i < line.size(); i++)
    {
        char c = line[i];
        if(in_str)
        {
            if(c == '"')
                in_str = false;
            ret.back() += c;
            continue;
        }
        if(c == '\'' || c == '\n' || c == '\r' || c == '\x9B')
            break;
        if(c == '"')
            in_str = true;
        if(c == ':')
            ret.emplace_back();
        else
            ret.back() += c;
    }
    for(auto &r : ret)
    {
        size_t a = r.find_first_not_of(" \t");
        r = (a == std::string::npos) ? std::string() : r.substr(a);
    }
    return ret;
}

// Returns true if the statement is a RETURN with a value, also after
// "IF ... THEN"
static bool return_with_value(const std::string &st)
{
    auto u = to_upper(st);
    // Search for THEN outside strings, and check the statement after it
    if(u.compare(0, 2, "IF") == 0)
    {
        bool in_str = false;
        for(size_t i = 0; i + 4 <= u.size(); i++)
        {
            if(u[i] == '"')
                in_str = !in_str;
            else if(!in_str && u.compare(i, 4, "THEN") == 0)
            {
                auto rest = u.substr(i + 4);
                auto a = rest.find_first_not_of(" \t");
                if(a != std::string::npos && return_with_value(rest.substr(a)))
                    return true;
            }
        }
    }
    size_t len = 0;
    if(u.compare(0, 6, "RETURN") == 0)
        len = 6;
    else
        for(size_t l = 3; l < 6; l++)
            if(u.size() > l && u.compare(0, l, std::string("RETURN", l)) == 0 && u[l] == '.')
                len = l + 1;
    if(!len)
        return false;
    auto rest = u.substr(len);
    return rest.find_first_not_of(" \t") != std::string::npos;
}

// Returns the position after the FUNC keyword, or 0 if not a FUNC line
static size_t func_header(const std::string &line)
{
    size_t p = line.find_first_not_of(" \t");
    if(p == std::string::npos || to_upper(line.substr(p, 4)) != "FUNC")
        return 0;
    if(p + 4 >= line.size() || (line[p + 4] != ' ' && line[p + 4] != '\t'))
        return 0;
    return p + 4;
}

// Pre-scan: registers all FUNC signatures, so calls can appear before the
// definition.
static std::string prescan_funcs(parse &s, const std::vector<src_line> &src)
{
    for(size_t i = 0; i < src.size(); i++)
    {
        auto pos = func_header(src[i].text);
        if(!pos)
            continue;
        // Search RETURN with value up to ENDFUNC
        bool value = false;
        for(size_t j = i + 1; j < src.size(); j++)
        {
            auto st = split_statements(src[j].text);
            if(to_upper(st[0]).compare(0, 7, "ENDFUNC") == 0)
                break;
            for(auto &x : st)
                if(return_with_value(x))
                    value = true;
        }
        s.new_line(src[i].text, i + 1);
        s.pos = pos;
        auto err = exb_register_func(s, src[i].line, value);
        if(err.size())
            return src[i].file + ":" + std::to_string(src[i].line) + ": " + err;
    }
    return std::string();
}

int compiler::compile_file(std::string iname, std::string output_filename,
                           const syntax::sm_list &sl, std::string listing_filename)
{
    std::ofstream ofile, lstfile;

    // Each compilation collects its own metadata: with several candidate
    // builds, the document describes the last one, that is the one kept.
    meta.clear();
    // Records a message for the metadata document
    auto note = [&](const std::string &text, const std::string &file, const char *code) {
        if(collect_meta)
            meta.diags.push_back(metadata::diag_from_text(text, file, code));
    };

    // Read the full source, expanding INCLUDE files
    std::vector<src_line> src;
    {
        std::vector<std::string> stack;
        auto err = read_source(iname, src, stack, include_dirs, meta.sources);
        if(err.size())
        {
            note(err, iname, "include");
            return show_error(err);
        }
    }

    ofile.open(output_filename);
    if(!ofile.is_open())
        return show_error("can't open output file '" + output_filename + "'");

    if(show_text)
    {
        lstfile.open(listing_filename);
        if(!lstfile.is_open())
            return show_error("can't open listing file '" + listing_filename + "'");
    }

    parse s(do_debug);
    s.set_input_file(iname);

    // Register FUNC definitions
    {
        auto err = prescan_funcs(s, src);
        if(err.size())
        {
            note(err, iname, "function");
            std::cerr << err << "\n";
            return 1;
        }
    }

    int ln = 1;
    std::string list_prog;
    for(auto &sl_line : src)
    {
        try
        {
            if(do_debug)
                std::cout << sl_line.file << ": parsing line " << sl_line.line << "\n";
            parse_line(sl_line.text, ln, s, show_text, short_text, sl, list_prog, lstfile);
            ln += sl_line.nlines;
        }
        catch(parse_error &e)
        {
            // Get start/end of current line, removing last EOL
            size_t min = 0, max = s.str.length();
            if(max && s.str[max - 1] == '\n')
                max--;
            // Adjust error position to be inside the line
            if(e.pos > max)
                e.pos = max;
            // Only show up to 76 characters total
            if(max > 76)
            {
                if(e.pos > 50)
                    min = e.pos - 50;
                if(max - min > 76)
                    max = min + 76;
            }
            if(collect_meta)
            {
                // The range covers the word at the error position
                size_t end = e.pos;
                while(end < max && (isalnum((unsigned char)s.str[end]) || s.str[end] == '_' ||
                                    s.str[end] == '$' || s.str[end] == '%'))
                    end++;
                if(end == e.pos && end < max)
                    end++;
                meta.diags.push_back({sl_line.file, sl_line.line, int(e.pos) + 1, int(end) + 1,
                                      "error", "syntax", e.what()});
            }
            // Show error position, line and marker
            std::cerr << sl_line.file << ":" << sl_line.line << ":" << e.pos << ": "
                      << e.what() << "\n  ";
            for(auto i = min; i < e.pos; i++)
                std::cerr << printable(s.str[i]);
            std::cerr << " ";
            for(auto i = e.pos; i < max; i++)
                std::cerr << printable(s.str[i]);
            std::cerr << "\n  ";
            for(auto i = min; i < e.pos; i++)
                std::cerr << "-";
            std::cerr << "^\n";

            return 1;
        }
    }
    if(do_debug)
    {
        std::cout << "parse end:\n";
        std::cout << "MAX LEVEL: " << s.maxlvl << "\n";
    }

    // Show short line
    if(short_text && list_prog.size())
        lstfile << list_prog << '\x9B';

    // Check unclosed loops, reporting the file and line where they start
    if(s.jumps.size())
    {
        auto j = s.jumps.back();
        for(auto i = s.jumps.rbegin(); i != s.jumps.rend(); ++i)
            if(i->type != LT_EXIT)
            {
                j = *i;
                break;
            }
        std::string where = iname + ":" + std::to_string(ln);
        int start = 1;
        for(auto &sl_line : src)
        {
            if(j.linenum >= start && j.linenum < start + sl_line.nlines)
            {
                where = sl_line.file + ":" + std::to_string(sl_line.line + j.linenum - start);
                break;
            }
            start += sl_line.nlines;
        }
        auto msg = j.type == LT_EXIT ? std::string("EXIT without loop")
                                     : "unclosed " + get_loop_name(j.type);
        note(where + ": " + msg, iname, "structure");
        std::cerr << where << ": " << msg << "\n";
        return 1;
    }


    s.emit_tok("TOK_END");
    // Resolve FUNC recursion support
    {
        auto err = exb_finalize(s, s.full_code());
        if(err.size())
        {
            note(err, iname, "function");
            std::cerr << iname << ": " << err << "\n";
            return 1;
        }
    }
    // Programs that use floating point must initialize it first
    {
        auto &code = s.full_code();
        bool fp = false;
        for(auto &c : code)
            if(c.is_tok())
            {
                auto t = c.get_tok();
                if(t.compare(0, 7, "TOK_FP_") == 0 || t == "TOK_FLOAT" ||
                   t == "TOK_INT_FP" || t == "TOK_MUL6")
                    fp = true;
            }
        if(fp)
        {
            code.insert(code.begin(), codew::ctok("TOK_FP_INIT", 0));
            // The integer to string conversion (INT_STR, UINT_STR) uses the
            // same OS routines as the floating point one: with floating
            // point, convert to floating point and use FP_STR, instead of
            // linking a second copy of them.
            for(size_t i = 0; i < code.size(); i++)
                if(code[i].is_tok() &&
                   (code[i].get_tok() == "TOK_INT_STR" || code[i].get_tok() == "TOK_UINT_STR"))
                {
                    int ln = code[i].linenum();
                    code[i] = codew::ctok(code[i].get_tok() == "TOK_INT_STR" ? "TOK_INT_FP"
                                                                            : "TOK_UINT_FP",
                                          ln);
                    code.insert(code.begin() + i + 1, codew::ctok("TOK_FP_STR", ln));
                    i++;
                }
        }
    }
    // Optimize
    if(optimize)
        do_peephole(s.full_code(), extended);
    // Statistics
    if(show_stats)
        do_opstat(s.full_code());

    // Get global symbols
    std::set<std::string> globals, globals_zp;
    for(auto &c : s.full_code())
    {
        if(c.is_symbol())
        {
            // Lower-case symbols are internal
            auto s = c.get_str();
            if(!s.empty() && s[0] >= 'A' && s[0] <= '_')
            {
                if(c.is_sword())
                    globals.insert(c.get_str());
                else if(c.is_sbyte())
                    globals_zp.insert(c.get_str());
            }
        }
    }
    // Machine code for loops, after getting the symbols used by the code
    std::set<std::string> native_helpers;
    bool native_code = native && optimize && do_native(s.full_code(), native_helpers, native_all, native_lean);

    // Output all global symbols
    ofile << "; Imported symbols\n";
    for(auto &c : globals)
        ofile << "\t.global " << c << "\n";
    for(auto &c : globals_zp)
        ofile << "\t.globalzp " << c << "\n";

    // Export common symbols and include atari defs
    ofile << "\n"
             "; Exported symbols\n"
             "\t.export bytecode_start\n"
             "\n\t.include \"target.inc\"\n\n";

    // Write tokens
    ofile << "; TOKENS:\n";
    for(auto &i : s.used_tokens())
        ofile << "\t.importzp\t" << i << "\n";
    if(native_code)
        ofile << "; Native code\n"
                 "\t.importzp\tsptr, saddr, cptr, next_instruction, tmp1, tmp2, tmp3, nat_stk\n"
                 "\t.import\tstack_l, stack_h\n";
    for(auto &h : native_helpers)
        ofile << "\t.import\t" << h << "\n";
    ofile << ";-----------------------------\n"
             "; Macro to get variable ID from name\n"
             ".macro makevar sym\n"
             "\t.byte <((sym - (EXB_VARS & $FF00))/2)\n"
             ".endmacro\n"
             "; Variables\n";
    // Create a map to reorder variables by number:
    auto vlist = std::map<int, std::string>();
    int vars_size = 0;
    for(auto &v : s.vars)
        if(!v.first.empty() && v.first[0] != '-')
        {
            vlist.emplace(v.second, v.first);
            vars_size += get_vt_size(VarType(v.second & 0xFF));
        }
    // The variables are addressed as "page + 2 * number", with numbers up to
    // 255.  They go in the page of the interpreter jump table, after it, if
    // they fit (segment "HEAP"), else in their own page (segment "HEAPA").
    // The jump table has two bytes per instruction used, and a few more for
    // the runtime.
    if(vars_size > 512)
    {
        auto msg = "too many variables, they use " + std::to_string(vars_size) +
                   " bytes and the limit is 512 (integer and string variables use 2 "
                   "bytes, floating point 6); use arrays for big tables";
        note(msg, iname, "limit");
        return show_error(iname + ": " + msg);
    }
    int jumptab_size = 2 * (int(s.used_tokens().size()) + 8);
    ofile << "\t.export EXB_VARS, EXB_VARS_END, EXB_FREE\n"
          << "\t.segment \"" << (jumptab_size + vars_size <= 510 ? "HEAP" : "HEAPA") << "\"\n"
          << "EXB_VARS:\n";
    // And now, output all variables:
    for(auto &v : vlist)
    {
        auto vtype = VarType(v.first & 0xFF);
        ofile << "\t.export " << var_symbol(v.second) << "\n";
        ofile << var_symbol(v.second) << ":\t.res " << get_vt_size(vtype) << "\t; "
              << get_vt_name(vtype) << " variable\n";
        if(collect_meta)
        {
            // An array variable holds the address of its data
            const char *type = vtype == VT_WORD           ? "word"
                               : vtype == VT_STRING       ? "string"
                               : vtype == VT_FLOAT        ? "float"
                               : vtype == VT_ARRAY_WORD   ? "word-array"
                               : vtype == VT_ARRAY_BYTE   ? "byte-array"
                               : vtype == VT_ARRAY_STRING ? "string-array"
                               : vtype == VT_ARRAY_FLOAT  ? "float-array"
                                                          : "unknown";
            // Variables of a FUNC are stored as "NN_FUNC_NAME", NN being the
            // length of the name of the function: show them as FUNC.NAME
            std::string shown = v.second;
            if(shown.size() > 3 && isdigit((unsigned char)shown[0]) &&
               isdigit((unsigned char)shown[1]) && shown[2] == '_')
            {
                size_t flen = size_t(shown[0] - '0') * 10 + size_t(shown[1] - '0');
                if(flen > 0 && shown.size() > 3 + flen + 1 && shown[3 + flen] == '_')
                    shown = shown.substr(3, flen) + "." + shown.substr(4 + flen);
            }
            meta.symbols.push_back({shown, type, var_symbol(v.second), get_vt_size(vtype)});
        }
    }
    ofile << "EXB_VARS_END:\n"
             "\t.assert (EXB_VARS_END - (EXB_VARS & $FF00)) <= 512, error, "
             "\"too many variables\"\n"
             "; Start of the free memory, for arrays and strings\n"
             "\t.segment \"ENDMARK\"\n"
             "EXB_FREE:\n"
             ";-----------------------------\n"
             "; Bytecode\n"
             "\t.segment \""
          << segname
          << "\"\n"
             "bytecode_start:\n";
    ln = -1;
    ;
    // Map with all line labels already emitted, this is needed
    // to avoid duplicate labels on reordered lines.
    std::map<int, int> line_labels;
    // Start line of each source line, to locate the file and line of the
    // line numbers in the code (they count the lines of all the files).
    std::vector<int> src_start;
    {
        int start = 1;
        for(auto &sl_line : src)
        {
            src_start.push_back(start);
            start += sl_line.nlines;
        }
    }
    // Line that waits for its first instruction, to know the kind of code
    int pending_line = -1;
    for(auto c : s.full_code())
    {
        if(c.linenum() != ln)
        {
            ln = c.linenum();
            // With metadata, the labels must reach the linker label file, so
            // they can't be local to the assembly file.
            std::string lbl =
                (collect_meta ? "exb_dbg_line_" : "@exBASIC_LINE_") + std::to_string(ln);
            // If we already emitted this label, adds a numbered sufix
            if(line_labels.find(ln) != line_labels.end())
            {
                line_labels[ln]++;
                lbl = lbl + "_" + std::to_string(line_labels[ln]);
            }
            else
                line_labels[ln] = 0;
            // Adds a label to facilitate debugging of resulting program
            ofile << lbl << ":\t; LINE " << ln << "\n";
            pending_line = -1;
            if(collect_meta && ln > 0 && !src_start.empty())
            {
                auto it = std::upper_bound(src_start.begin(), src_start.end(), ln);
                if(it != src_start.begin())
                {
                    size_t idx = size_t(it - src_start.begin()) - 1;
                    // The code after the last line (the end of the program)
                    // does not belong to a source line.
                    if(ln - src_start[idx] < src[idx].nlines)
                        meta.lines.push_back(
                            {src[idx].file, src[idx].line + ln - src_start[idx], lbl, false});
                    if(ln - src_start[idx] < src[idx].nlines)
                        pending_line = int(meta.lines.size()) - 1;
                }
            }
        }
        if(pending_line >= 0 && !c.is_label())
        {
            meta.lines[pending_line].native = c.is_asm();
            pending_line = -1;
        }
        // Handle labels
        if(c.is_label())
        {
            // Check if the name starts with the label prefix
            auto full_name = c.get_str();
            auto ln = std::string(s.label_prefix).length();
            if(full_name.substr(0, ln) == s.label_prefix)
            {
                // Yes, this is a valid label
                auto name = full_name.substr(ln);
                auto it = s.labels.find(name);
                if(it == s.labels.end())
                    std::cerr << "internal error: unknown label type '" << name << "'\n";
                else
                {
                    auto lbl = it->second;
                    auto seg = lbl.get_segment();
                    if(seg.size())
                        ofile << "\t.segment \"" << seg << "\"\n";
                    else if(lbl.is_code())
                        ofile << "\t.segment \"" << segname << "\"\n";
                    else
                        ofile << "\t.segment \"DATA\"\n";
                }
                ofile << "\t.export\t" << full_name << "\n";
                if(collect_meta && it != s.labels.end())
                    meta.symbols.push_back(
                        {name, it->second.is_code() ? "function" : "data", full_name, 0});
            }
        }
        ofile << c.to_asm() << "\n";
    }
    // COMMON variables table, used by COMMON and RUN
    auto common = exb_common_table(s);
    if(common.size())
        ofile << "\t.segment \"" << segname << "\"\n" << common;

    return 0;
}
