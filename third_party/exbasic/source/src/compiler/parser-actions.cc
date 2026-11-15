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

// parser-actions.cc: parser functions called from the parsing tables
#include <cstring>
#include "parser-actions.h"
#include "ifile.h"
#include "parser-exbasic.h"
#include "parser.h"
#include "vartype.h"

#include <algorithm>
#include <cmath>
#include <map>

static unsigned long get_hex(parse &s)
{
    unsigned num = 0;
    auto start = s.pos;
    while(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        if(c >= '0' && c <= '9')
            num = num * 16 + (c - '0');
        else if(c >= 'a' && c <= 'f')
            num = num * 16 + 10 + (c - 'a');
        else if(c >= 'A' && c <= 'F')
            num = num * 16 + 10 + (c - 'A');
        else
            break;
        s.pos++;
        if(num > 0xFFFF)
            return num;
    }
    if(s.pos == start)
        return 65536; // No digits: error
    return num;
}

static unsigned long get_dec(parse &s)
{
    unsigned num = 0;
    auto start = s.pos;
    while(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        if(c >= '0' && c <= '9')
            num = num * 10 + (c - '0');
        else
            break;
        s.pos++;
        if(num > 0xFFFF)
            return num;
    }
    if(s.pos == start)
        return 65536; // No digits: error
    return num;
}

static unsigned long get_number(parse &s)
{
    auto start = s.pos;
    if(s.expect('$'))
    {
        s.debug("(hex)");
        auto hpos = s.pos;
        auto h = get_hex(s);
        if(h > 65535)
        {
            if(s.pos != hpos)
                throw parse_error("hex constant out of range (max $FFFF)", start);
            return 65536;
        }

        s.debug("(got '" + std::to_string(h) + "')");
        std::string sn = "$";
        if(h > 255)
        {
            sn += s.hexd(h >> 12);
            sn += s.hexd(h >> 8);
        }
        sn += s.hexd(h >> 4);
        sn += s.hexd(h);
        s.add_text(sn);
        s.add_s_int(h);
        return h;
    }
    else
    {
        bool sign = s.expect('-');
        int num = get_dec(s);

        if(num > 65535 || num < 0)
            return 65536;

        if(s.expect('.')) // If ends in a DOT, it's a fp number
        {
            s.pos = start;
            return 65536;
        }
        // An exponent also makes it a floating point number: "1E3"
        if(s.pos + 1 < s.str.length() && (s.str[s.pos] == 'E' || s.str[s.pos] == 'e'))
        {
            char n = s.str[s.pos + 1];
            if((n >= '0' && n <= '9') || n == '-' || n == '+')
            {
                s.pos = start;
                return 65536;
            }
        }

        auto sn = std::to_string(sign ? -num : num);
        s.debug("(got '" + sn + "')");
        s.add_text(sn);
        s.add_s_int(sign ? -num : num);
        if(sign)
            return 65536 - num;
        else
            return num;
    }
}

static bool get_asm_word_constant(parse &s)
{
    auto start = s.pos;
    if(s.expect('@'))
    {
        std::string name;
        // Reads ASM constant
        if(s.get_ident(name))
        {
            // A following '(' means a user function call, not an ASM symbol.
            auto ident_end = s.pos;
            s.skipws();
            if(s.pos < s.str.size() && s.str[s.pos] == '(')
            {
                s.pos = start;
                return false;
            }
            s.pos = ident_end;
            s.add_text("@" + name);
            s.add_s_var("@" + name);
            s.emit_word(name);
            s.skipws();
            return true;
        }
    }
    s.pos = start;
    return false;
}

static bool get_asm_byte_constant(parse &s)
{
    auto start = s.pos;
    if(s.expect('@') && s.expect('@'))
    {
        std::string name;
        // Reads ASM constant
        if(s.get_ident(name))
        {
            s.add_text("@@" + name);
            s.add_s_var("@@" + name);
            s.emit_byte(name);
            s.skipws();
            return true;
        }
    }
    s.pos = start;
    return false;
}

static bool SMB_E_NUMBER_WORD(parse &s)
{
    s.debug("E_NUMBER_WORD");
    s.skipws();
    if(get_asm_word_constant(s))
        return true;
    auto num = get_number(s);
    if(num > 65535)
        return false;
    s.emit_word(num);
    s.skipws();
    return true;
}

static bool SMB_E_NUMBER_BYTE(parse &s)
{
    s.debug("E_NUMBER_BYTE");
    s.skipws();
    if(get_asm_byte_constant(s))
        return true;
    auto num = get_number(s);
    if(num > 255)
        return false;
    s.emit_byte(num);
    s.skipws();
    return true;
}

static bool get_const_string(parse &s, std::string &str)
{
    auto start = s.pos ? s.pos - 1 : 0; // The opening quote
    auto end = [&]() {
        if(str.size() > 255)
            throw parse_error("string too long (max 255 characters)", start);
        s.add_text_str(str);
        s.add_s_str(str);
        return true;
    };
    while(!s.eos())
    {
        if(s.expect('"'))
        {
            if(s.expect('"'))
                str += '"';
            else if(s.expect('$'))
            {
                do
                {
                    auto p = s.pos;
                    auto c = get_hex(s);
                    if(c > 255 || s.pos == p)
                        throw parse_error("bad character code in string, use $00 to $FF",
                                          p ? p - 1 : 0);
                    str += char(c);
                } while(s.expect('$'));
                if(!s.expect('"'))
                    return end();
            }
            else
                return end();
        }
        else
        {
            str += char(s.str[s.pos]);
            s.pos++;
        }
    }
    throw parse_error("unterminated string, missing '\"'", start);
}
static bool SMB_E_CONST_STRING(parse &s)
{
    s.debug("E_CONST_STRING");
    std::string str;
    if(get_const_string(s, str))
        return s.emit_str(str);
    return false;
}

static bool SMB_E_REM(parse &s)
{
    s.debug("E_REM");
    // Remove "REM" from the shorted text
    if(s.expand.stext == ".")
        s.expand.stext = "";
    while(!s.eos() && !s.expect('\n') && !s.expect('\x9b'))
    {
        s.expand.text.push_back(s.str[s.pos]);
        s.pos++;
    }
    return true;
}

static bool next_is_word(parse &s, const char *w);

static bool SMB_E_EOL(parse &s)
{
    s.debug("E_EOL");
    s.skipws();
    if(s.expect('\''))
    {
        s.add_text("\'");
        return SMB_E_REM(s);
    }
    return (s.eos() || s.peek(':') || s.eol() || (s.line_ifs > 0 && next_is_word(s, "ELSE")));
}

// The integer expression ends here: it is not followed by an operator that
// the integer expression could not use (as "+ 1.5" or "* X%").  This makes
// the parser read the whole expression as floating point.
static bool next_is_word(parse &s, const char *w)
{
    size_t n = strlen(w);
    if(s.pos + n > s.str.length())
        return false;
    for(size_t i = 0; i < n; i++)
    {
        char c = s.str[s.pos + i];
        if(c >= 'a' && c <= 'z')
            c = c - 'a' + 'A';
        if(c != w[i])
            return false;
    }
    if(s.pos + n < s.str.length())
    {
        char c = s.str[s.pos + n];
        if((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
           c == '_' || c == '$' || c == '%' || c == '.')
            return false;
    }
    return true;
}

static bool SMB_E_EXPR_END(parse &s)
{
    auto p = s.pos;
    s.skipws();
    bool op = false;
    if(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        op = c == '+' || c == '-' || c == '*' || c == '/' || c == '^' || c == '&' ||
             c == '!' || next_is_word(s, "MOD") || next_is_word(s, "EXOR");
    }
    s.pos = p;
    return !op;
}

// FRE in an integer expression.  FRE is unsigned, so when it is ordered
// directly against another value ("FRE() > 1000" or "1000 <= FRE()") the
// integer reading fails and the comparison is done in floating point.  In
// arithmetic ("F - FRE()") and in "=" or "<>" the 16 bit value is right.
static bool SMB_E_FRE_INT(parse &s)
{
    size_t p = s.pos;
    while(p > 0 && s.str[p - 1] == ' ')
        p--;
    char c = p > 0 ? s.str[p - 1] : 0;
    char c2 = p > 1 ? s.str[p - 2] : 0;
    bool ordered = (c == '<') || (c == '>' && c2 != '<') ||
                   (c == '=' && (c2 == '<' || c2 == '>'));
    if(ordered)
        return false;
    s.fre_arith = c == '+' || c == '-' || c == '*' || c == '/' || c == '&' || c == '!';
    return true;
}

static bool SMB_E_NO_CMP(parse &s)
{
    if(s.fre_arith)
        return true;
    auto p = s.pos;
    s.skipws();
    bool ordered = false;
    if(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        char n = s.pos + 1 < s.str.length() ? s.str[s.pos + 1] : 0;
        ordered = (c == '<' && n != '>') || c == '>';
    }
    s.pos = p;
    return !ordered;
}

// Short-circuit AND / OR: jumps to the end of the expression when the left
// side gives the result.
static bool sc_start(parse &s, const char *tok)
{
    auto l = s.new_label();
    s.emit_tok(tok);
    s.emit_word(l);
    s.sc_labels.emplace_back(l, s.code->size());
    return true;
}
static bool SMB_E_SC_AND(parse &s) { return sc_start(s, "TOK_SC_AND"); }
static bool SMB_E_SC_OR(parse &s) { return sc_start(s, "TOK_SC_OR"); }
static bool SMB_E_SC_END(parse &s)
{
    // Discard the starts removed by the parser backtracking
    auto &c = *s.code;
    while(!s.sc_labels.empty())
    {
        auto &b = s.sc_labels.back();
        if(b.second <= c.size() && b.second >= 2 && c[b.second - 1].is_sword(b.first))
            break;
        s.sc_labels.pop_back();
    }
    if(s.sc_labels.empty())
        throw parse_error("internal error: short-circuit end without start", s.pos);
    s.emit_label(s.sc_labels.back().first);
    s.sc_labels.pop_back();
    return true;
}

// A PRINT item ends here: it is not followed by any operator, so it is the
// whole value and not part of a bigger expression or comparison.
static bool SMB_E_ITEM_END(parse &s)
{
    if(!SMB_E_EXPR_END(s))
        return false;
    auto p = s.pos;
    s.skipws();
    bool op = false;
    if(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        op = c == '=' || c == '<' || c == '>' || next_is_word(s, "AND") ||
             next_is_word(s, "OR");
    }
    s.pos = p;
    return !op;
}

// The floating point expression ends here: it is not followed by an
// integer-only operator (MOD, EXOR, & or !).  The parser then reads the
// expression as integer and converts it.
static bool SMB_E_FP_EXPR_END(parse &s)
{
    auto p = s.pos;
    s.skipws();
    bool op = false;
    if(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        op = c == '&' || c == '!' || next_is_word(s, "MOD") || next_is_word(s, "EXOR");
    }
    s.pos = p;
    return !op;
}

// "IF ... THEN statement": the rest of the line is part of the IF, it is
// closed at the end of the line.
// True if only spaces or a comment follow in the line
static bool at_line_end(parse &s)
{
    auto p = s.pos;
    s.skipws();
    bool end = s.eos() || s.eol() || s.str[s.pos] == '\'';
    s.pos = p;
    return end;
}

static bool SMB_E_THEN_LINE(parse &s)
{
    // A statement must follow THEN in the same line
    if(at_line_end(s))
        return false;
    s.line_ifs++;
    return true;
}

// "ELSE" in the line of an "IF ... THEN"
static bool SMB_E_LINE_ELSE(parse &s)
{
    return s.line_ifs > 0;
}

// "THEN" must be followed by a statement
static bool SMB_E_THEN_EMPTY(parse &s)
{
    throw parse_error("expected a statement after THEN", s.pos);
}

// Remembers the variable of the FOR statement being parsed
static bool SMB_E_FOR_VAR(parse &s)
{
    s.for_var.clear();
    auto &c = *s.code;
    if(c.size() >= 2 && c[c.size() - 2].is_tok("TOK_VAR_SADDR") && c.back().is_varn())
        s.for_var = c.back().get_varname();
    return true;
}

// Checks the size of a DIM: the last value, when constant, must be from 0 to
// the limit, and emits DIM_CHK that checks it at runtime and adds 1.
static bool dim_check(parse &s, int max_hi)
{
    int max = max_hi * 256 + 255;
    auto &c = *s.code;
    int n = -1;
    bool cst = false;
    if(!c.empty() && c.back().is_tok("TOK_0"))
        n = 0, cst = true;
    else if(!c.empty() && c.back().is_tok("TOK_1"))
        n = 1, cst = true;
    else if(c.size() > 1 && (c[c.size() - 2].is_tok("TOK_NUM") ||
                             c[c.size() - 2].is_tok("TOK_BYTE")) &&
            (c.back().is_word() || c.back().is_byte()))
    {
        n = c.back().get_val();
        if(c.back().is_word())
            n = int16_t(n);
        cst = true;
    }
    if(cst && (n < 0 || n > max))
        throw parse_error("array size out of range (0 to " + std::to_string(max) + ")",
                          s.pos);
    if(cst)
    {
        // Already checked: the number of elements, size + 1, as a constant
        bool tok = c.back().is_tok("TOK_0") || c.back().is_tok("TOK_1");
        c.pop_back();
        if(!tok)
            c.pop_back();
        if(n + 1 < 256)
        {
            s.emit_tok("TOK_BYTE");
            return s.emit_byte(n + 1);
        }
        s.emit_tok("TOK_NUM");
        return s.emit_word(n + 1);
    }
    s.emit_tok("TOK_DIM_CHK");
    return s.emit_byte(max_hi);
}
// Byte arrays: up to 32768 bytes
static bool SMB_E_DIM_B(parse &s)
{
    // Only a byte array when "BYTE" follows, so that errors for word arrays
    // report the word array limit
    auto p = s.pos;
    s.skipws();
    bool is_byte = s.peek('B');
    s.pos = p;
    if(!is_byte)
        return false;
    return dim_check(s, 0x7F);
}
// Word and string arrays: up to 32768 bytes
static bool SMB_E_DIM_W(parse &s) { return dim_check(s, 0x3F); }
// Floating point arrays: up to 5376 * 6 = 32256 bytes
static bool SMB_E_DIM_F(parse &s) { return dim_check(s, 0x14); }

static bool SMB_E_FOR_SET(parse &s)
{
    if(!s.jumps.empty())
        s.jumps.back().var = s.for_var;
    return true;
}

static bool SMB_E_PUSH_VAR(parse &s)
{
    // nothing to do!
    s.debug("E_PUSH_VAR");
    s.var_stk.push_back(s.remove_last());
    return true;
}

static bool SMB_E_POP_VAR(parse &s)
{
    s.debug("E_POP_VAR");
    if(s.var_stk.empty())
        throw parse_error("variable stack empty", s.pos);
    s.code->push_back(s.var_stk.back());
    s.var_stk.pop_back();
    return true;
}

static bool SMB_E_PUSH_LT(parse &s)
{
    // nothing to do!
    s.debug("E_PUSH_LT");
    auto t = get_looptype(s.remove_last().get_str());
    auto l = s.push_loop(t);
    switch(t)
    {
    case LT_DO_LOOP:
    case LT_REPEAT:
    case LT_WHILE_1:
    case LT_FOR_1:
        s.emit_label(l);
        break;
    case LT_WHILE_2:
    case LT_FOR_2:
    case LT_IF:
        s.emit_word(l);
        break;
    case LT_EXIT:
    case LT_ELSE:
    case LT_ELIF:
    case LT_FUNC:
    case LT_LAST_JUMP:
        break;
    case LT_BLOCK:
        // Optimize by switching codep
        s.remove_last();
        s.push_block(l);
        break;
    }
    return true;
}

static bool SMB_E_POP_LOOP(parse &s)
{
    // nothing to do!
    s.debug("E_POP_LOOP");
    auto l = s.pop_loop(LT_DO_LOOP);
    if(l.empty())
        return s.loop_error("loop start missing");
    s.emit_word(l);
    s.emit_label(l + "_x");
    return true;
}

static bool SMB_E_POP_WHILE(parse &s)
{
    // nothing to do!
    s.debug("E_POP_WHILE");
    auto l1 = s.pop_loop(LT_WHILE_2);
    auto l2 = s.pop_loop(LT_WHILE_1);
    if(l1.empty() || l2.empty())
        return false;
    s.emit_word(l2);
    s.emit_label(l1);
    s.emit_label(l2 + "_x");
    return true;
}

static bool SMB_E_POP_IF(parse &s)
{
    // nothing to do!
    s.debug("E_POP_IF");
    auto l = s.pop_loop(LT_ELSE);
    if(l.empty())
        return false;
    s.emit_label(l);
    while(s.peek_loop(LT_ELIF))
        s.emit_label(s.pop_loop(LT_ELIF));
    return true;
}

bool line_if_else_next(parse &s)
{
    auto p = s.pos;
    s.skipws();
    bool r = s.line_ifs > 0 && next_is_word(s, "ELSE");
    s.pos = p;
    return r;
}

bool close_line_if(parse &s)
{
    if(s.jumps.empty() || (s.jumps.back().type != LT_IF && s.jumps.back().type != LT_ELSE))
        throw parse_error("blocks started after THEN must end in the same line", s.pos);
    return SMB_E_POP_IF(s);
}

static bool SMB_E_ELSEIF(parse &s)
{
    // nothing to do!
    s.debug("E_ELSEIF");
    auto l1 = s.pop_loop(LT_IF);
    if(l1.empty())
        return false;
    auto t = get_looptype(s.remove_last().get_str());
    auto l2 = s.push_loop(t);
    s.emit_word(l2);
    s.emit_label(l1);
    return true;
}

static bool SMB_E_EXIT_LOOP(parse &s)
{
    // nothing to do!
    s.debug("E_EXIT_LOOP");
    auto last = s.jumps.size();
    while(1)
    {
        if(last == 0)
            return s.loop_error("EXIT without loop");
        last--;
        auto type = s.jumps[last].type;
        if(type == LT_ELIF || type == LT_IF || type == LT_ELSE || type == LT_FOR_2 ||
           type == LT_WHILE_2)
            continue;
        else if(type <= LT_EXIT)
            return s.loop_error("invalid EXIT");
        break;
    }
    s.emit_word(s.jumps[last].label + "_x");
    return true;
}

static bool SMB_E_POP_BLOCK(parse &s)
{
    // nothing to do!
    s.debug("E_POP_BLOCK");
    auto l = s.pop_loop(LT_BLOCK);
    if(l.empty())
        return false;
    s.pop_block(l);
    return true;
}

// The name of a variable as written, from its key (FUNC private variables
// have a prefix with the FUNC name)
static std::string var_display_name(const std::string &key)
{
    if(!key.empty() && key[0] == '0')
    {
        auto u = key.find('_');
        auto len = std::stoul(key.substr(1, u - 1));
        return key.substr(u + 1 + len + 1);
    }
    return key;
}

// "NEXT name" where the name is not a variable: report the FOR variable
static bool SMB_E_NEXT_UNKNOWN(parse &s)
{
    auto p = s.pos;
    s.skipws();
    std::string name;
    if(!s.get_ident(name))
    {
        s.pos = p;
        return false;
    }
    if(!s.jumps.empty() && s.jumps.back().type == LT_FOR_1 && !s.jumps.back().var.empty())
        throw parse_error("NEXT " + name + " does not match FOR " +
                              var_display_name(s.jumps.back().var),
                          s.pos);
    throw parse_error("NEXT " + name + " without FOR", s.pos);
}

static bool SMB_E_POP_FOR(parse &s)
{
    s.debug("E_POP_FOR");
    // "NEXT var" must name the variable of its FOR
    if(!s.jumps.empty() && s.jumps.back().type == LT_FOR_1 && !s.jumps.back().var.empty() &&
       !s.code->empty() && s.code->back().is_varn() &&
       s.code->back().get_varname() != s.jumps.back().var)
        throw parse_error("NEXT " + var_display_name(s.code->back().get_varname()) +
                              " does not match FOR " + var_display_name(s.jumps.back().var),
                          s.pos);
    auto l2 = s.pop_loop(LT_FOR_1);
    auto l1 = s.pop_loop(LT_FOR_2);
    if(l1.empty() || l2.empty())
        return false;
    s.remove_last();
    s.emit_word(l2);
    s.emit_label(l1);
    s.emit_label(l2 + "_x");
    return true;
}

static bool SMB_E_POP_REPEAT(parse &s)
{
    // nothing to do!
    s.debug("E_POP_REPEAT");
    auto l = s.pop_loop(LT_REPEAT);
    if(l.empty())
        return false;
    s.emit_word(l);
    s.emit_label(l + "_x");
    return true;
}

static std::string last_var_name;
// Key of a variable: its name, plus the type suffix ('$' or '%') that follows
// it, so "A", "A$" and "A%" are different variables.
static std::string var_key(parse &s, const std::string &name)
{
    if(s.peek('$'))
        return name + "$";
    if(s.peek('%'))
        return name + "%";
    return name;
}

static bool SMB_E_VAR_CREATE(parse &s)
{
    s.debug("E_VAR_CREATE");
    auto &v = s.vars;
    std::string name;
    if(!s.get_ident(name))
        return false;
    auto key = var_key(s, name);
    if(v.find(key) != v.end())
        return false;
    // Inside a FUNC, a local variable with this name already exists
    if(exb_var_key(s, key) != key)
        return false;
    // A FUNC name can't be used as a variable
    if(s.funcs.count(name))
        return s.loop_error("'" + name + "' is a FUNC, it can't be used as a variable");
    auto v_num = v.size();
    v[key] = 0 + 256 * v_num;
    s.emit_varn(key);
    last_var_name = key;
    s.add_text(name);
    s.add_s_var(name);
    return true;
}

static bool SMB_E_VAR_SET_TYPE(parse &s)
{
    s.debug("E_VAR_SET_TYPE");

    s.skipws();
    // Get type
    enum VarType type = get_vartype(s.remove_last().get_str());
    auto &v = s.vars;
    if(s.do_debug)
        std::cout << "\tset var '" << last_var_name << "' to " << int(type) << "\n";
    v[last_var_name] = (v[last_var_name] & ~0xFF) + type;
    // If type is FLOAT, allocate two more invisible variables
    if(type == VT_FLOAT)
    {
        v["-fake-" + std::to_string(v.size())] = 0;
        v["-fake-" + std::to_string(v.size())] = 0;
    }
    // This rule only succeeds on array types (defined with "DIM"), other
    // variable types create the variable and then fail so the parser can retry
    // with the new created variable.
    return var_type_is_array(type);
}

static bool var_check(parse &s, enum VarType type)
{
    auto &v = s.vars;
    std::string name;
    if(!s.get_ident(name))
        return false;
    // Use the local variable if inside a FUNC that declares it
    auto key = exb_var_key(s, var_key(s, name));
    if(v.find(key) == v.end())
        return s.error("variable name but got '" + name + "'");
    if((v[key] & 0xFF) != type)
        return s.error(get_vt_name(type) + " and got '" + name + "'");
    s.add_text(name);
    s.add_s_var(name);
    s.emit_varn(key);
    return true;
}

static bool SMB_E_VAR_WORD(parse &s)
{
    s.debug("E_VAR_WORD");
    return var_check(s, VT_WORD);
}

static bool SMB_E_VAR_SEARCH(parse &s)
{
    enum VarType type = get_vartype(s.remove_last().get_str());
    s.debug("E_VAR_SEARCH: " + get_vt_name(type));
    return var_check(s, type);
}

// Get optional exponent for FP number
static int parse_fp_exp(parse &s)
{
    auto spos = s.save();
    if(s.expect('E'))
    {
        // Expect either a '-' or a '+'
        int exp = 0;
        bool esign = s.expect('-') || (s.expect('+'), false);
        if(s.range('0', '9'))
        {
            exp = s.str[s.pos - 1] - '0';
            if(s.range('0', '9'))
                exp = exp * 10 + s.str[s.pos - 1] - '0';
            return esign ? -exp : exp;
        }
    }
    s.restore(spos);
    return 0;
}

static atari_fp get_fp_number(parse &s)
{
    // Optional sign
    bool sign = s.expect('-');

    // Get all digits:
    bool ok = false;
    double num = 0;
    int dot = -1, norm = 0;
    while(s.pos < s.str.length())
    {
        char c = s.str[s.pos];
        if(c >= '0' && c <= '9')
        {
            num = num * 10 + (c - '0');
            // Normalize numbers too big
            if(num > 1e30)
            {
                num = num / 1000;
                norm += 3;
            }
            ok = true;
        }
        else if(c == '.')
            dot = s.pos + 1;
        else
            break;
        s.pos++;
    }
    if(!ok)
        return atari_fp(HUGE_VAL); // return invalid number

    // Calculate dot exponent
    dot = dot >= 0 ? s.pos - dot : 0;

    // Optional exponent and resulting number
    num = num * std::pow(10, parse_fp_exp(s) - dot + norm);
    num = sign ? -num : num;

    auto fp = atari_fp(num);
    auto sn = fp.to_string();
    s.debug("(got '" + sn + "')");
    s.add_text(sn);
    s.add_s_fp(sn);
    return fp;
}

static bool SMB_E_NUMBER_FP(parse &s)
{
    s.debug("E_NUMBER_FP");
    s.skipws();
    auto num = get_fp_number(s);
    if(!num.valid())
        return false;
    s.emit_fp(num);
    s.skipws();
    return true;
}

static bool SMB_E_LABEL_DEF(parse &s)
{
    auto l = s.push_loop(LT_BLOCK);
    s.remove_last();
    s.push_block(l);

    s.debug("E_LABEL_DEF");
    auto &v = s.labels;
    auto name = s.last_label;
    s.block_names[l] = name;
    if(v[name].is_defined())
        return s.loop_error("new label, got label already defined '" + name + "'");
    s.emit_label(s.label_prefix + name);
    return true;
}

static bool SMB_E_LABEL(parse &s)
{
    s.debug("E_LABEL");
    // Get type
    auto ltype = labelType(s.remove_last().get_str());
    // Get identifier
    std::string name;
    if(!s.get_ident(name))
        return false;
    auto it = s.labels.find(name);
    if(it == s.labels.end())
        return false;
    // Check type
    if(it->second != ltype)
        return false;
    s.add_text(name);
    s.add_s_var(name);
    s.emit_word(s.label_prefix + name);
    return true;
}

// Called in DATA definitions, creates a label if not exists, if already
// exists checks that it is not already defined.
static bool SMB_E_LABEL_CREATE(parse &s)
{
    s.debug("E_LABEL_CREATE");
    std::string name;
    s.skipws();
    auto where = std::make_pair(s.linenum, s.pos);
    if(!s.get_ident(name))
        return false;
    if(s.funcs.count(name))
        return s.loop_error("'" + name + "' is a FUNC");
    // Get type, create if not exists
    auto &v = s.labels[name];
    // Check type: the label can be already defined at this same position if
    // the parser is retrying the statement
    if(!v.is_code())
    {
        auto it = s.label_where.find(name);
        if(it == s.label_where.end() || it->second != where)
            return s.loop_error("new label, got label already defined '" + name + "'");
        v = labelType();
    }
    s.label_where[name] = where;
    // Store variable name
    s.add_text(name);
    s.add_s_var(name);
    s.last_label = name;
    return true;
}

// "DATA = ..." continuing a floating point DATA array that ended with ","
static bool SMB_E_DATA_FP_CONT(parse &s)
{
    s.debug("E_DATA_FP_CONT");
    if(s.last_label.empty() || !s.peek_loop(LT_BLOCK))
        return false;
    auto it = s.labels.find(s.last_label);
    if(it == s.labels.end() || !it->second.is_float_array())
        return false;
    return true;
}

static bool SMB_E_LABEL_SET_TYPE(parse &s)
{
    s.debug("E_LABEL_SET_TYPE");
    s.skipws();
    // Get type
    s.labels[s.last_label].set_type(s.remove_last().get_str());
    return true;
}

static bool SMB_E_DATA_SET_ROM_SEG(parse &s)
{
    s.debug("E_DATA_SET_ROM_SEG");
    s.skipws();
    s.labels[s.last_label].set_segment("CODE");
    return true;
}

static bool SMB_E_DATA_SET_SEGMENT(parse &s)
{
    s.debug("E_DATA_SET_SEGMENT");
    s.skipws();
    // Get segment name
    std::string seg;
    if(s.get_ident(seg))
    {
        s.labels[s.last_label].set_segment(seg);
        return true;
    }
    else
        return s.error("segment name");
}

// Reads a DATA array from a file
static bool SMB_E_DATA_FILE(parse &s)
{
    s.debug("E_DATA_FILE");
    s.skipws();
    // Get file name until the '"'
    std::string fname;
    auto pos = s.pos;
    if(!get_const_string(s, fname))
        return false;

    auto f = open_include_file(s.in_fname, fname);
    if(!f)
        throw parse_error("can't open data file '" + fname + "'", pos);

    // Read the file to a buffer of max 64k
    for(unsigned i = 0; i < 65536; i++)
    {
        int c = f->get();
        if(c < 0 || c > 255)
            break;
        s.emit_byte(c);
    }
    return true;
}

// List of all actions by name:
static std::map<std::string, bool (*)(parse &s)> actions = {
    {"E_CONST_STRING", SMB_E_CONST_STRING},
    {"E_DATA_FILE", SMB_E_DATA_FILE},
    {"E_ELSEIF", SMB_E_ELSEIF},
    {"E_EOL", SMB_E_EOL},
    {"E_EXPR_END", SMB_E_EXPR_END},
    {"E_ITEM_END", SMB_E_ITEM_END},
    {"E_NO_CMP", SMB_E_NO_CMP},
    {"E_SC_AND", SMB_E_SC_AND},
    {"E_SC_OR", SMB_E_SC_OR},
    {"E_SC_END", SMB_E_SC_END},
    {"E_FRE_INT", SMB_E_FRE_INT},
    {"E_FP_EXPR_END", SMB_E_FP_EXPR_END},
    {"E_THEN_LINE", SMB_E_THEN_LINE},
    {"E_LINE_ELSE", SMB_E_LINE_ELSE},
    {"E_THEN_EMPTY", SMB_E_THEN_EMPTY},
    {"E_FOR_VAR", SMB_E_FOR_VAR},
    {"E_FOR_SET", SMB_E_FOR_SET},
    {"E_NEXT_UNKNOWN", SMB_E_NEXT_UNKNOWN},
    {"E_DIM_B", SMB_E_DIM_B},
    {"E_DIM_W", SMB_E_DIM_W},
    {"E_DIM_F", SMB_E_DIM_F},
    {"E_EXIT_LOOP", SMB_E_EXIT_LOOP},
    {"E_LABEL", SMB_E_LABEL},
    {"E_LABEL_CREATE", SMB_E_LABEL_CREATE},
    {"E_LABEL_DEF", SMB_E_LABEL_DEF},
    {"E_LABEL_SET_TYPE", SMB_E_LABEL_SET_TYPE},
    {"E_DATA_FP_CONT", SMB_E_DATA_FP_CONT},
    {"E_DATA_SET_ROM_SEG", SMB_E_DATA_SET_ROM_SEG},
    {"E_DATA_SET_SEGMENT", SMB_E_DATA_SET_SEGMENT},
    {"E_NUMBER_BYTE", SMB_E_NUMBER_BYTE},
    {"E_NUMBER_FP", SMB_E_NUMBER_FP},
    {"E_NUMBER_WORD", SMB_E_NUMBER_WORD},
    {"E_POP_FOR", SMB_E_POP_FOR},
    {"E_POP_IF", SMB_E_POP_IF},
    {"E_POP_LOOP", SMB_E_POP_LOOP},
    {"E_POP_BLOCK", SMB_E_POP_BLOCK},
    {"E_POP_REPEAT", SMB_E_POP_REPEAT},
    {"E_POP_VAR", SMB_E_POP_VAR},
    {"E_POP_WHILE", SMB_E_POP_WHILE},
    {"E_PUSH_LT", SMB_E_PUSH_LT},
    {"E_PUSH_VAR", SMB_E_PUSH_VAR},
    {"E_REM", SMB_E_REM},
    {"E_VAR_CREATE", SMB_E_VAR_CREATE},
    {"E_VAR_SEARCH", SMB_E_VAR_SEARCH},
    {"E_VAR_SET_TYPE", SMB_E_VAR_SET_TYPE},
    {"E_VAR_WORD", SMB_E_VAR_WORD},
    // exBASIC language
    {"E_FUNC_DEF", exb_E_FUNC_DEF},
    {"E_FUNC_END", exb_E_FUNC_END},
    {"E_FUNC_LOCAL", exb_E_FUNC_LOCAL},
    {"E_IN_FUNC", exb_E_IN_FUNC},
    {"E_RET_INT", exb_E_RET_INT},
    {"E_RET_STR", exb_E_RET_STR},
    {"E_RET_FP", exb_E_RET_FP},
    {"E_RET_VI", exb_E_RET_VI},
    {"E_RET_VS", exb_E_RET_VS},
    {"E_RET_VF", exb_E_RET_VF},
    {"E_FUNC_RETURN", exb_E_FUNC_RETURN},
    {"E_FCALL_I", exb_E_FCALL_I},
    {"E_FCALL_S", exb_E_FCALL_S},
    {"E_FCALL_F", exb_E_FCALL_F},
    {"E_FCALL_STMT", exb_E_FCALL_STMT},
    {"E_FCALL_MULTI", exb_E_FCALL_MULTI},
    {"E_ARG_INT", exb_E_ARG_INT},
    {"E_ARG_STR", exb_E_ARG_STR},
    {"E_ARG_FP", exb_E_ARG_FP},
    {"E_ARG_NEXT", exb_E_ARG_NEXT},
    {"E_ARG_BAD", exb_E_ARG_BAD},
    {"E_FCALL_END", exb_E_FCALL_END},
    {"E_FUNC_UNKNOWN", exb_E_FUNC_UNKNOWN},
    {"E_MULTI_TARGETS", exb_E_MULTI_TARGETS},
    {"E_LOOP_EXIT", exb_E_LOOP_EXIT},
    {"E_EMPTY_STRING", exb_E_EMPTY_STRING},
    {"E_COMMON", exb_E_COMMON},
    {"E_RUN_CALL", exb_E_RUN_CALL},
    {"E_CAT_SET", exb_E_CAT_SET},
    {"E_CAT_ADD", exb_E_CAT_ADD},
    {"E_CAT_END", exb_E_CAT_END}};

bool call_parsing_action(const std::string name, parse &s)
{
    auto i = actions.find(name);
    if(i != actions.end())
        return i->second(s);
    else
        return false;
}
