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

// native.cc: compiles the integer code inside loops to 6502 machine code.
//
// The bytecode instructions inside loops that have a simple 6502 translation
// are replaced by inline machine code, placed in the bytecode stream after a
// TOK_NATIVE instruction (that jumps to the address that follows it).  The
// machine code keeps the interpreter conventions: the top of the stack is in
// the A (low) and X (high) registers, and the rest of the stack is in the
// interpreter stack.  This means that the machine code can return to the
// interpreter at any instruction, just by setting the code pointer:
//
//      ldy #<label / sty cptr / ldy #>label / sty cptr+1 / jmp next_instruction
//
// Instructions without a translation (like PRINT or floating point) stay as
// bytecode, the machine code returns to the interpreter to run them and the
// interpreter continues with the machine code after them with TOK_NATIVE.
//
// When the machine code continues into a label that is also the target of a
// jump from bytecode, the label is placed after a "BIT zp" opcode:
//      .byte $24 / label: / .byte TOK_NATIVE / label_n:
// so the machine code skips the TOK_NATIVE byte and the bytecode executes it.

#include "native.h"
#include <algorithm>
#include <cstdio>
#include <cstdlib>
#include <map>
#include <set>
#include <string>

namespace
{
// One instruction or label of the bytecode
struct item
{
    size_t pos;   // position of the token or label in the code
    size_t len;   // number of codew
    bool label;   // true if it is a label
    bool instr;   // true if it is an instruction (token + operands)
    std::string tok;
    bool hot = false;
    bool native = false;
};

class native_gen
{
    std::vector<codew> &code;
    std::vector<item> items;
    std::map<std::string, size_t> label_item; // label name -> item index
    std::map<std::string, std::vector<size_t>> refs; // label -> referencing items
    std::vector<codew> out;
    int lnum = 0;
    int nlabel = 0;
    bool used = false;

    // Statistics of the generated code: count and bytes per instruction
    std::map<std::string, std::pair<int, int>> stats;

    // Approximate size of one line of machine code
    static int asm_size(std::string l)
    {
        auto p = l.find_first_not_of("\t :");
        if(p == l.npos || l.find(".byte") != l.npos)
            return l.find(".byte") != l.npos ? 1 : 0;
        l = l.substr(p);
        if(l.size() < 3 || l.back() == ':')
            return 0;
        auto tab = l.find('\t');
        if(tab == l.npos)
            return 1;
        auto opnd = l.substr(tab + 1);
        auto m = l.substr(0, 3);
        if(m[0] == 'b' && m != "bit")
            return 2;
        if(opnd[0] == '#' || opnd[0] == '(' || opnd.compare(0, 3, "tmp") == 0 ||
           opnd.compare(0, 7, "nat_stk") == 0 || opnd.compare(0, 4, "sptr") == 0 ||
           opnd.compare(0, 5, "saddr") == 0 || opnd.compare(0, 4, "cptr") == 0)
            return 2;
        return 3;
    }

  public:
    void print_stats()
    {
        std::vector<std::pair<int, std::string>> v;
        int total = 0;
        for(auto &x : stats)
        {
            v.push_back({x.second.second, x.first});
            total += x.second.second;
        }
        std::sort(v.rbegin(), v.rend());
        fprintf(stderr, "native code: %d bytes\n", total);
        for(size_t k = 0; k < v.size() && k < 25; k++)
            fprintf(stderr, "%6d %5d  %s\n", v[k].first, stats[v[k].second].first,
                    v[k].second.c_str());
    }
    std::set<std::string> helpers; // runtime subroutines used

  private:

    static const std::set<std::string> &supported_tokens()
    {
        static const std::set<std::string> s = {
            "TOK_0",           "TOK_1",           "TOK_BYTE",       "TOK_NUM",
            "TOK_PUSH_0",      "TOK_PUSH_1",      "TOK_PUSH_BYTE",  "TOK_PUSH_NUM",
            "TOK_PUSH",        "TOK_POP",         "TOK_VAR_LOAD",   "TOK_PUSH_VAR_LOAD",
            "TOK_VAR_ADDR",    "TOK_VAR_STORE",   "TOK_VAR_STORE_0", "TOK_VAR_SADDR",
            "TOK_SADDR",       "TOK_ADD_VAR",     "TOK_VAR_ADD_VAR", "TOK_PUSH_VAR_ADD_VAR",
            "TOK_ADD",         "TOK_SUB",         "TOK_NEG",        "TOK_ABS",
            "TOK_BIT_AND",     "TOK_BIT_OR",      "TOK_BIT_EXOR",   "TOK_USHL",
            "TOK_SHL8",        "TOK_EQ",          "TOK_NEQ",        "TOK_LT",
            "TOK_GT",          "TOK_EQ_BYTE",     "TOK_COMP_0",     "TOK_L_NOT",
            "TOK_L_AND",       "TOK_L_OR",        "TOK_PEEK",       "TOK_DPEEK",
            "TOK_POKE",        "TOK_DPOKE",       "TOK_POKE_AX_BYTE", "TOK_BYTE_PEEK",
            "TOK_BYTE_POKE",   "TOK_NUM_POKE",    "TOK_INC",        "TOK_DEC",
            "TOK_INCVAR",      "TOK_DECVAR",      "TOK_JUMP",       "TOK_CJUMP",
            "TOK_CNJUMP",      "TOK_SC_AND",      "TOK_SC_OR",      "TOK_FOR",         "TOK_FOR_NEXT",   "TOK_FOR_NEXT_JUMP",
            "TOK_FOR_EXIT",    "TOK_MUL",         "TOK_DIV",        "TOK_MOD",
            "TOK_DIVC",        "TOK_MODC",        "TOK_DIV_POW2",   "TOK_CALL",
            "TOK_RET",         "TOK_CRET",        "TOK_CNRET"};
        return s;
    }

    static bool is_jump(const std::string &t)
    {
        return t == "TOK_JUMP" || t == "TOK_CJUMP" || t == "TOK_CNJUMP" ||
               t == "TOK_FOR_NEXT_JUMP" || t == "TOK_SC_AND" || t == "TOK_SC_OR";
    }

    codew &op(size_t i, size_t n) { return code[items[i].pos + 1 + n]; }

    std::string jump_target(size_t i)
    {
        auto &c = op(i, 0);
        return c.is_sword() ? c.get_str() : std::string();
    }

    // Operand as an assembly expression
    std::string opexpr(size_t i, size_t n)
    {
        auto &c = op(i, n);
        if(c.is_byte() || c.is_word())
            return std::to_string(c.get_val() & (c.is_byte() ? 0xFF : 0xFFFF));
        return c.get_str();
    }

    std::string var(size_t i, size_t n) { return var_symbol(op(i, n).get_varname()); }

    // Checks that the instruction operands have the expected types
    bool valid_operands(size_t i)
    {
        auto &it = items[i];
        auto &t = it.tok;
        size_t nops = it.len - 1;
        auto is_v = [&](size_t n) { return op(i, n).is_varn(); };
        auto is_b = [&](size_t n) { return op(i, n).is_byte() || op(i, n).is_sbyte(); };
        auto is_w = [&](size_t n) { return op(i, n).is_word() || op(i, n).is_sword(); };
        if(t == "TOK_BYTE" || t == "TOK_PUSH_BYTE" || t == "TOK_EQ_BYTE" || t == "TOK_DIV_POW2" ||
           t == "TOK_POKE_AX_BYTE" || t == "TOK_BYTE_PEEK" || t == "TOK_BYTE_POKE")
            return nops == 1 && is_b(0);
        if(t == "TOK_NUM" || t == "TOK_PUSH_NUM" || t == "TOK_NUM_POKE")
            return nops == 1 && is_w(0);
        if(t == "TOK_VAR_LOAD" || t == "TOK_PUSH_VAR_LOAD" || t == "TOK_VAR_ADDR" ||
           t == "TOK_VAR_STORE" || t == "TOK_VAR_STORE_0" || t == "TOK_VAR_SADDR" ||
           t == "TOK_ADD_VAR" || t == "TOK_INCVAR" || t == "TOK_DECVAR")
            return nops == 1 && is_v(0);
        if(t == "TOK_VAR_ADD_VAR" || t == "TOK_PUSH_VAR_ADD_VAR")
            return nops == 2 && is_v(0) && is_v(1);
        if(is_jump(t) || t == "TOK_CALL")
            return nops == 1 && op(i, 0).is_sword() && label_item.count(op(i, 0).get_str());
        return nops == 0;
    }

    // Splits the code into instructions and labels
    void scan()
    {
        for(size_t p = 0; p < code.size();)
        {
            item it;
            it.pos = p;
            it.label = code[p].is_label();
            it.instr = code[p].is_tok();
            size_t q = p + 1;
            if(it.instr)
            {
                it.tok = code[p].get_tok();
                while(q < code.size() && !code[q].is_tok() && !code[q].is_label())
                    q++;
            }
            else if(!it.label)
            {
                // Data, up to the next token or label
                while(q < code.size() && !code[q].is_tok() && !code[q].is_label())
                    q++;
            }
            it.len = q - p;
            if(it.label)
                label_item[code[p].get_str()] = items.size();
            items.push_back(it);
            p = q;
        }
        // Label references
        for(size_t i = 0; i < items.size(); i++)
        {
            if(items[i].label)
                continue;
            for(size_t k = 0; k < items[i].len; k++)
            {
                auto &c = code[items[i].pos + k];
                if(c.is_sword() || c.is_sbyte())
                {
                    auto s = c.get_str();
                    if(label_item.count(s))
                        refs[s].push_back(i);
                }
            }
        }
    }

    // Marks the instructions inside loops (from a label to a jump back to it)
    // Selection of the code to compile: an estimate of how many times each
    // instruction runs, from the loops around it (and around the calls to
    // its FUNC).  Only the instructions that run at least 1/hot_ratio times
    // as often as the hottest code are compiled, as the rest (setup code,
    // the outer levels of loops) adds size without much gain.
    struct loop_info
    {
        size_t first, last; // items of the loop
        double trips;       // estimated iterations
    };
    std::vector<loop_info> loops;
    std::vector<double> weight; // estimated executions of each item
    static constexpr double guess_trips = 16;     // a loop of unknown length
    static constexpr double endless_trips = 1000; // DO / LOOP without exit

    // Constant value loaded by item i (BYTE, NUM, 0, 1 and the PUSH forms)
    bool const_value(size_t i, int &v)
    {
        if(i >= items.size() || !items[i].instr)
            return false;
        auto t = items[i].tok;
        if(t.compare(0, 9, "TOK_PUSH_") == 0)
            t = "TOK_" + t.substr(9);
        if(t == "TOK_0" || t == "TOK_1")
        {
            v = t == "TOK_1";
            return true;
        }
        if(t == "TOK_BYTE" && items[i].len == 2 && op(i, 0).is_byte())
        {
            v = op(i, 0).get_val() & 0xFF;
            return true;
        }
        if(t == "TOK_NUM" && items[i].len == 2 && op(i, 0).is_word())
        {
            v = int16_t(op(i, 0).get_val());
            return true;
        }
        return false;
    }

    // Constant expression ending at item i: a constant, or a constant and
    // NEG.  Sets v and the first item of the expression.
    bool const_expr(size_t i, int &v, size_t &first)
    {
        if(i < items.size() && items[i].instr && items[i].tok == "TOK_NEG" && i > 0 &&
           const_value(i - 1, v))
        {
            v = -v;
            first = i - 1;
            return true;
        }
        first = i;
        return const_value(i, v);
    }

    // Iterations of the FOR loop that starts at item l, from its statement:
    //   VAR_SADDR v, PUSH_<start>, DPOKE, <limit>, PUSH_<step>, FOR, CNJUMP
    // A limit that is a variable is returned in "limit_var" (the trips are
    // then estimated from the loop that changes that variable).
    double for_trips(size_t l, std::string &limit_var, int &lstart, int &lstep)
    {
        limit_var.clear();
        size_t i = l;
        while(i > 0 && items[i - 1].label)
            i--;
        if(i < 2 || items[i - 1].tok != "TOK_CNJUMP" || items[i - 2].tok != "TOK_FOR")
            return guess_trips;
        size_t f = i - 2, s, li, si;
        int start, limit, step;
        if(f < 5 || !const_expr(f - 1, step, s) || step == 0)
            return guess_trips;
        bool climit = const_expr(s - 1, limit, li);
        if(!climit)
        {
            li = s - 1;
            if(items[li].tok != "TOK_VAR_LOAD" || !valid_operands(li))
                return guess_trips;
        }
        if(li < 3 || items[li - 1].tok != "TOK_DPOKE" || !const_expr(li - 2, start, si) ||
           si < 1 || items[si - 1].tok != "TOK_VAR_SADDR")
            return guess_trips;
        lstart = start;
        lstep = step;
        if(!climit)
        {
            limit_var = op(li, 0).get_varname();
            return guess_trips;
        }
        double n = (double(limit) - start) / step + 1;
        return n < 1 ? 1 : n;
    }

    // The variable of the FOR loop that starts at item l, or empty
    std::string for_var(size_t l)
    {
        size_t i = l;
        while(i > 0 && items[i - 1].label)
            i--;
        if(i < 2 || items[i - 1].tok != "TOK_CNJUMP" || items[i - 2].tok != "TOK_FOR")
            return std::string();
        for(size_t k = i - 2; k > 0; k--)
        {
            if(!items[k - 1].instr)
                return std::string();
            if(items[k - 1].tok == "TOK_VAR_SADDR")
                return valid_operands(k - 1) ? op(k - 1, 0).get_varname() : std::string();
            if(items[k - 1].tok == "TOK_FOR" || is_jump(items[k - 1].tok))
                return std::string();
        }
        return std::string();
    }

    void find_loops()
    {
        // Loops with a variable limit: index in "loops" -> limit variable
        std::map<size_t, std::string> var_limit;
        for(size_t i = 0; i < items.size(); i++)
        {
            if(!items[i].instr || !is_jump(items[i].tok) || items[i].len != 2)
                continue;
            auto &c = op(i, 0);
            if(!c.is_sword())
                continue;
            auto l = label_item.find(c.get_str());
            if(l == label_item.end() || l->second > i)
                continue;
            loop_info lp{l->second, i, guess_trips};
            auto &t = items[i].tok;
            if(t == "TOK_FOR_NEXT_JUMP" || (t == "TOK_CJUMP" && i > 0 && items[i - 1].tok == "TOK_FOR_NEXT"))
            {
                std::string lv;
                int st = 0, sp = 0;
                lp.trips = for_trips(l->second, lv, st, sp);
                if(!lv.empty())
                    var_limit[loops.size()] = lv;
            }
            else if(t == "TOK_JUMP")
            {
                // An endless loop, unless a jump inside leaves it
                lp.trips = endless_trips;
                for(size_t k = l->second; k < i; k++)
                    if(items[k].instr && is_jump(items[k].tok) && items[k].len == 2 &&
                       op(k, 0).is_sword())
                    {
                        auto e = label_item.find(op(k, 0).get_str());
                        if(e != label_item.end() && (e->second > i || e->second < l->second))
                            lp.trips = guess_trips;
                    }
            }
            loops.push_back(lp);
        }
        // A FOR loop up to (or down to) the variable of an enclosing FOR loop,
        // as "FOR J = 0 TO I" inside "FOR I": half the trips of that loop on
        // average.  The innermost enclosing loop with that variable.
        for(auto &vl : var_limit)
        {
            auto &in = loops[vl.first];
            const loop_info *best = nullptr;
            for(auto &out : loops)
                if(&out != &in && out.first < in.first && out.last > in.last &&
                   (!best || out.first > best->first) && for_var(out.first) == vl.second)
                    best = &out;
            if(best && best->trips != guess_trips)
                in.trips = std::max(1.0, best->trips / 2);
        }
    }

    void find_hot()
    {
        find_loops();
        // Weight of each item inside its own block
        weight.assign(items.size(), 1);
        for(auto &lp : loops)
            for(size_t k = lp.first; k <= lp.last; k++)
                weight[k] *= lp.trips;
        // FUNC bodies start at a "fb_lbl_NAME" label and end at the next one.
        std::map<std::string, std::pair<size_t, size_t>> body;
        std::vector<std::string> owner(items.size());
        std::string cur;
        for(size_t i = 0; i < items.size(); i++)
        {
            if(items[i].label)
            {
                auto l = code[items[i].pos].get_str();
                if(l.compare(0, 7, "fb_lbl_") == 0)
                {
                    if(!cur.empty())
                        body[cur].second = i;
                    cur = l;
                    body[cur] = {i, items.size()};
                }
            }
            owner[i] = cur;
        }
        // A call, or a jump to a FUNC (a call at the end of a FUNC)
        auto callee = [&](size_t i) -> std::string {
            if((items[i].tok != "TOK_CALL" && items[i].tok != "TOK_JUMP") || items[i].len != 2 ||
               !op(i, 0).is_sword())
                return std::string();
            auto l = op(i, 0).get_str();
            return body.count(l) ? l : std::string();
        };
        // Calls between FUNCs, and the FUNCs each one reaches
        std::map<std::string, std::set<std::string>> calls, reach;
        for(size_t i = 0; i < items.size(); i++)
        {
            auto c = callee(i);
            if(!c.empty())
                calls[owner[i]].insert(c);
        }
        for(auto &b : body)
        {
            std::vector<std::string> work(calls[b.first].begin(), calls[b.first].end());
            auto &r = reach[b.first];
            while(!work.empty())
            {
                auto f = work.back();
                work.pop_back();
                if(r.insert(f).second)
                    for(auto &g : calls[f])
                        work.push_back(g);
            }
        }
        // Times each FUNC is called: the weight of its calls, not counting
        // the calls inside a recursion, as a recursive FUNC is counted as
        // called "endless_trips" times for each outside call.
        std::map<std::string, double> mult;
        mult[std::string()] = 1;
        for(int round = 0; round < 32; round++)
        {
            std::map<std::string, double> m;
            m[std::string()] = 1;
            for(size_t i = 0; i < items.size(); i++)
            {
                auto c = callee(i);
                if(c.empty() || (reach[c].count(owner[i])))
                    continue;
                double w = weight[i] * (mult.count(owner[i]) ? mult[owner[i]] : 0);
                if(reach[c].count(c))
                    w *= endless_trips;
                m[c] = std::min(1e12, std::max(m[c], w));
            }
            if(m == mult)
                break;
            mult = m;
        }
        double wmax = 0;
        for(size_t i = 0; i < items.size(); i++)
        {
            weight[i] *= mult.count(owner[i]) ? mult[owner[i]] : 0;
            wmax = std::max(wmax, weight[i]);
        }
        double limit = std::max(min_weight, wmax / hot_ratio);
        if(getenv("EXBASIC_HOT_DEBUG"))
        {
            for(auto &m : mult)
                fprintf(stderr, "func %s called %g times\n", m.first.c_str(), m.second);
            for(auto &lp : loops)
                fprintf(stderr, "loop at line %d: %g trips, weight %g\n",
                        code[items[lp.first].pos].linenum(), lp.trips, weight[lp.last]);
            fprintf(stderr, "max weight %g, compiling from %g\n", wmax, limit);
        }
        for(size_t i = 0; i < items.size(); i++)
            items[i].hot = weight[i] >= limit;
    }

    // Approximate cycles of one bytecode instruction, with the dispatch: the
    // instructions that stay bytecode and take long (floating point, strings,
    // input and output) make the machine code around them not worth its size.
    static double bytecode_cycles(const std::string &t)
    {
        static const std::map<std::string, double> c = {
            {"TOK_FLOAT", 120},       {"TOK_FP_LOAD", 120},     {"TOK_FP_STORE", 120},
            {"TOK_FP_ABS", 80},       {"TOK_FP_NEG", 80},       {"TOK_FP_SGN", 150},
            {"TOK_FP_ADD", 900},      {"TOK_FP_SUB", 900},      {"TOK_FP_CMP", 900},
            {"TOK_FP_MUL", 2500},     {"TOK_FP_DIV", 4000},     {"TOK_FP_INT", 1500},
            {"TOK_INT_FP", 1000},     {"TOK_UINT_FP", 1000},    {"TOK_FP_RND", 1500},
            {"TOK_FP_TIME", 1000},    {"TOK_FP_IPOW", 20000},   {"TOK_FP_SQRT", 30000},
            {"TOK_FP_EXP", 30000},    {"TOK_FP_EXP10", 30000},  {"TOK_FP_LOG", 30000},
            {"TOK_FP_LOG10", 30000},  {"TOK_FP_SIN", 30000},    {"TOK_FP_COS", 30000},
            {"TOK_FP_ATN", 30000},    {"TOK_FP_STR", 4000},     {"TOK_FP_VAL", 4000},
            {"TOK_STRF", 4000},       {"TOK_VAL", 600},         {"TOK_INT_STR", 500},
            {"TOK_UINT_STR", 500},    {"TOK_CAT_STR", 300},     {"TOK_COPY_STR", 300},
            {"TOK_CMP_STR", 200},     {"TOK_STR_IDX", 150},     {"TOK_CHR", 150},
            {"TOK_TSTR_ADD", 300},    {"TOK_TSTR_SET", 200},    {"TOK_ASC", 80},
            {"TOK_MOVE", 500},        {"TOK_NMOVE", 500},       {"TOK_MSET", 500},
            {"TOK_PRINT_STR", 3000},  {"TOK_PRINT_TAB", 1000},  {"TOK_PRINT_RTAB", 1000},
            {"TOK_PUT", 1500},        {"TOK_BYTE_PUT", 1500},   {"TOK_BPUT", 3000},
            {"TOK_BGET", 3000},       {"TOK_GET", 1500},        {"TOK_GETKEY", 3000},
            {"TOK_INPUT_STR", 5000},  {"TOK_XIO", 5000},        {"TOK_CLOSE", 2000},
            {"TOK_NOTE", 2000},       {"TOK_POINT", 2000},      {"TOK_DRAWTO", 20000},
            {"TOK_GRAPHICS", 50000},  {"TOK_PMGRAPHICS", 2000}, {"TOK_PAUSE", 30000},
            {"TOK_DIM", 500},         {"TOK_MUL", 200},         {"TOK_MUL6", 100},
            {"TOK_DIV", 300},         {"TOK_MOD", 300},         {"TOK_DIVC", 300},
            {"TOK_MODC", 300},        {"TOK_USR_CALL", 200}};
        auto it = c.find(t);
        return it == c.end() ? 40 : it->second;
    }

    // Removes the machine code of the loops where it saves little time: the
    // cycles saved (the dispatch of each native instruction, less a switch
    // to and from the interpreter for each run of bytecode) against the
    // cycles of the whole loop as bytecode.  Inner loops first, so an outer
    // loop is judged by what is left of the inner ones.
    static constexpr double native_saving = 28;  // cycles saved per instruction
    static constexpr double switch_cost = 45;    // machine code -> bytecode -> back
    static constexpr double min_gain = 0.10;     // of the loop time

    void prune_loops()
    {
        auto lps = loops;
        std::sort(lps.begin(), lps.end(), [](const loop_info &x, const loop_info &y) {
            return x.last - x.first < y.last - y.first;
        });
        std::vector<bool> kept(items.size(), false);
        for(auto &lp : lps)
        {
            double total = 0, saved = 0;
            bool in_native = false;
            for(size_t k = lp.first; k <= lp.last; k++)
            {
                auto &it = items[k];
                if(!it.instr)
                    continue;
                total += weight[k] * bytecode_cycles(it.tok);
                if(it.native)
                    saved += weight[k] * native_saving;
                else if(in_native)
                    saved -= weight[k] * switch_cost;
                in_native = it.native;
            }
            bool keep = total > 0 && saved >= min_gain * total;
            if(getenv("EXBASIC_HOT_DEBUG"))
                fprintf(stderr, "loop at line %d: saves %.0f of %.0f cycles: %s\n",
                        code[items[lp.first].pos].linenum(), saved, total,
                        keep ? "machine code" : "bytecode");
            for(size_t k = lp.first; k <= lp.last; k++)
            {
                if(keep)
                    kept[k] = true;
                else if(!kept[k])
                    items[k].native = false;
            }
        }
        // If what is left saves little time in the whole program, no machine
        // code at all: its support routines would only add size.
        {
            double total = 0, saved = 0;
            for(size_t k = 0; k < items.size(); k++)
                if(items[k].instr)
                {
                    total += weight[k] * bytecode_cycles(items[k].tok);
                    if(items[k].native)
                        saved += weight[k] * native_saving;
                }
            if(getenv("EXBASIC_HOT_DEBUG"))
                fprintf(stderr, "program: saves %.0f of %.0f cycles\n", saved, total);
            if(saved < min_gain * total)
            {
                for(auto &it : items)
                    it.native = false;
                return;
            }
        }
        // The FOR (or loop setup) before a loop left in bytecode: only worth
        // in machine code if the code around it is
        for(auto &lp : lps)
        {
            if(kept[lp.first])
                continue;
            size_t f = lp.first;
            while(f > 0 && items[f - 1].label)
                f--;
            if(f >= 2 && items[f - 1].tok == "TOK_CNJUMP" && items[f - 2].tok == "TOK_FOR")
            {
                // From the VAR_SADDR of the FOR statement
                size_t s = f - 2;
                while(s > 0 && items[s].tok != "TOK_VAR_SADDR" && items[s].instr)
                    s--;
                bool around = s > 0 && items[s - 1].native;
                if(!around)
                    for(size_t k = s; k < f; k++)
                        items[k].native = false;
            }
        }
    }

    // Shorter and faster forms of common sequences of the generated code
    // (the same result in the registers that are used after them).
    void asm_peephole()
    {
        auto txt = [&](size_t k) -> std::string {
            if(k >= out.size() || !out[k].is_asm())
                return std::string();
            return out[k].to_asm();
        };
        auto set = [&](size_t k, const std::string &l) { out[k] = codew::casm(l, out[k].linenum()); };
        auto op_of = [](const std::string &l) { return l.size() > 4 ? l.substr(1, 3) : l; };
        auto arg_of = [](const std::string &l) {
            auto t = l.find('\t', 1);
            return t == std::string::npos ? std::string() : l.substr(t + 1);
        };
        for(size_t k = 0; k < out.size(); k++)
        {
            // "ldy sptr / jsr pushAX": the entry that loads Y
            if(txt(k) == "\tldy\tsptr" && txt(k + 1) == "\tjsr\tpushAX")
            {
                set(k, "\tjsr\tpushAXsp");
                out.erase(out.begin() + k + 1);
                helpers.insert("pushAXsp");
                continue;
            }
            // "lda V / ldx V+1 / asl / tay / txa / rol / tax / tya": V * 2
            if(op_of(txt(k)) == "lda" && op_of(txt(k + 1)) == "ldx" &&
               arg_of(txt(k + 1)) == arg_of(txt(k)) + "+1" && arg_of(txt(k))[0] != '#' &&
               arg_of(txt(k)).find(',') == std::string::npos && txt(k + 2) == "\tasl" &&
               txt(k + 3) == "\ttay" && txt(k + 4) == "\ttxa" && txt(k + 5) == "\trol" &&
               txt(k + 6) == "\ttax" && txt(k + 7) == "\ttya")
            {
                auto v = arg_of(txt(k));
                set(k + 1, "\tasl");
                set(k + 2, "\ttay");
                set(k + 3, "\tlda\t" + v + "+1");
                out.erase(out.begin() + k + 4);
                continue;
            }
            // "tax / tya / sta W / stx W+1" when A and X are loaded again
            // right after: store the high byte from A and the low from Y
            if(txt(k) == "\ttax" && txt(k + 1) == "\ttya" && op_of(txt(k + 2)) == "sta" &&
               op_of(txt(k + 3)) == "stx" && arg_of(txt(k + 3)) == arg_of(txt(k + 2)) + "+1" &&
               arg_of(txt(k + 2)).find(',') == std::string::npos &&
               arg_of(txt(k + 2)).find('(') == std::string::npos)
            {
                auto n1 = txt(k + 4), n2 = txt(k + 5);
                bool dead = op_of(n1) == "lda" && n1.find(",x") == std::string::npos &&
                            n1.find(", x") == std::string::npos &&
                            (op_of(n2) == "ldx" || n2 == "\ttax");
                if(dead)
                {
                    auto w = arg_of(txt(k + 2));
                    set(k, "\tsta\t" + w + "+1");
                    set(k + 1, "\tsty\t" + w);
                    out.erase(out.begin() + k + 2, out.begin() + k + 4);
                    continue;
                }
            }
        }
    }

    // Next instruction after item i, or items.size()
    size_t next_instr(size_t i)
    {
        for(i++; i < items.size(); i++)
            if(!items[i].label)
                return i;
        return i;
    }

    void select()
    {
        auto &sup = supported_tokens();
        for(size_t i = 0; i < items.size(); i++)
        {
            auto &it = items[i];
            if(!it.instr || !it.hot || !sup.count(it.tok) || !valid_operands(i))
                continue;
            // FOR_NEXT is only supported followed by CJUMP
            if(it.tok == "TOK_FOR_NEXT")
            {
                size_t n = i + 1;
                if(n >= items.size() || items[n].tok != "TOK_CJUMP" || !valid_operands(n) ||
                   !items[n].hot)
                    continue;
            }
            it.native = true;
        }
        // CJUMP after FOR_NEXT goes with it
        for(size_t i = 0; i + 1 < items.size(); i++)
            if(items[i].tok == "TOK_FOR_NEXT" && !items[i].native)
                items[i + 1].native = false;
        // CALL is only worth in machine code if the FUNC is also hot
        for(size_t i = 0; i < items.size(); i++)
            if(items[i].native && items[i].tok == "TOK_CALL")
            {
                auto l = label_item.find(jump_target(i));
                size_t n = l == label_item.end() ? items.size() : next_instr(l->second);
                if(n >= items.size() || !items[n].hot)
                    items[i].native = false;
            }
        if(lean)
            prune_loops();
        // CJUMP after FOR_NEXT goes with it
        for(size_t i = 0; i + 1 < items.size(); i++)
            if(items[i].tok == "TOK_FOR_NEXT" && !items[i].native)
                items[i + 1].native = false;
        // Remove short runs of instructions, the switch to and from the
        // interpreter costs about two instructions.
        for(size_t i = 0; i < items.size();)
        {
            if(!items[i].native)
            {
                i++;
                continue;
            }
            size_t j = i, n = 0;
            bool loop = false;
            while(j < items.size() && (items[j].native || items[j].label))
            {
                if(items[j].native)
                {
                    n++;
                    if(is_jump(items[j].tok))
                        loop = true;
                }
                j++;
            }
            if(n < 3 && !loop)
                for(size_t k = i; k < j; k++)
                    items[k].native = false;
            i = j;
        }
    }

    // A label is "native" if the next instruction is native
    bool label_native(const std::string &l)
    {
        auto it = label_item.find(l);
        if(it == label_item.end())
            return false;
        size_t n = next_instr(it->second);
        return n < items.size() && items[n].native;
    }

    // True if a label is referenced from bytecode (or data)
    bool label_external(const std::string &l)
    {
        auto it = refs.find(l);
        if(it == refs.end())
            return false;
        // References from bytecode, or used as an address (not a jump) by
        // the machine code, need the original label.
        for(auto i : it->second)
            if(!items[i].native || !is_jump(items[i].tok))
                return true;
        return false;
    }

    std::string nlabel_of(const std::string &l) { return l + "__n"; }

    std::string new_label() { return "native_lbl_" + std::to_string(++nlabel); }

    void a(const std::string &s) { out.push_back(codew::casm("\t" + s, lnum)); }

    void exit_to(const std::string &l)
    {
        a("ldy\t#<" + l);
        a("sty\tcptr");
        a("ldy\t#>" + l);
        a("sty\tcptr+1");
        a("jmp\tnext_instruction");
    }

    static std::string inverse_branch(const std::string &b)
    {
        static const std::map<std::string, std::string> inv = {
            {"bpl", "bmi"}, {"bmi", "bpl"}, {"beq", "bne"}, {"bne", "beq"},
            {"bcc", "bcs"}, {"bcs", "bcc"}, {"bvc", "bvs"}, {"bvs", "bvc"}};
        return inv.at(b);
    }

    // Conditional jump: "b" is the branch instruction taken to jump.  For a
    // machine code target already defined (a jump back, as at the end of
    // loops) the assembler uses a short branch if it is in range.
    void branch(const std::string &b, const std::string &target)
    {
        if(label_native(target))
        {
            auto l = nlabel_of(target);
            a(".if .defined(" + l + ") .and ((" + l + ") - * >= -126)");
            a(b + "\t" + l);
            a(".else");
            a(inverse_branch(b) + "\t:+");
            a("jmp\t" + l);
            a(":");
            a(".endif");
        }
        else
        {
            a(inverse_branch(b) + "\t:+");
            exit_to(target);
            a(":");
        }
    }

    void jump_to(const std::string &l)
    {
        if(label_native(l))
            a("jmp\t" + nlabel_of(l));
        else
            exit_to(l);
    }

    // FOR loops: finds the static variable and STEP from the FOR statement
    struct for_info
    {
        std::string var;  // empty if not known
        bool step_known = false;
        int step = 0;
        // Constant start and limit (with a known step and variable)
        bool range_known = false;
        int start = 0, limit = 0;
    };

    // Information of the FOR instruction at item f
    for_info for_stmt(size_t f)
    {
        for_info r;
        // STEP: a constant pushed, or a constant pushed and negated (as
        // "STEP -1": PUSH_1, NEG)
        size_t step_first = f;
        if(f > 0 && items[f - 1].instr)
        {
            auto &p = items[f - 1];
            if(p.tok == "TOK_PUSH_1")
                r.step_known = true, r.step = 1;
            else if(p.tok == "TOK_PUSH_0")
                r.step_known = true, r.step = 0;
            else if(p.tok == "TOK_PUSH_BYTE" && op(f - 1, 0).is_byte())
                r.step_known = true, r.step = op(f - 1, 0).get_val() & 0xFF;
            else if(p.tok == "TOK_PUSH_NUM" && op(f - 1, 0).is_word())
                r.step_known = true, r.step = int16_t(op(f - 1, 0).get_val());
            else if(p.tok == "TOK_NEG" && f > 1 && items[f - 2].instr &&
                    items[f - 2].tok.compare(0, 9, "TOK_PUSH_") == 0)
            {
                int v;
                if(const_value(f - 2, v))
                    r.step_known = true, r.step = -v, step_first = f - 2;
            }
            if(r.step_known && step_first == f)
                step_first = f - 1;
        }
        // Variable: "VAR_SADDR v, PUSH..., <expr>, DPOKE, <expr>, FOR"
        size_t d = f;
        while(d > 0)
        {
            d--;
            auto &it = items[d];
            if(!it.instr)
                return r;
            if(it.tok == "TOK_DPOKE")
                break;
            if(it.tok == "TOK_SADDR" || it.tok == "TOK_VAR_SADDR" || it.tok == "TOK_CALL" ||
               is_jump(it.tok) || it.tok == "TOK_FOR")
                return r;
        }
        if(items[d].tok != "TOK_DPOKE")
            return r;
        size_t k = d;
        while(k > 0)
        {
            k--;
            auto &it = items[k];
            if(!it.instr)
                return r;
            if(it.tok == "TOK_VAR_SADDR")
                break;
            if(it.tok == "TOK_SADDR" || it.tok == "TOK_DPOKE" || it.tok == "TOK_CALL" ||
               is_jump(it.tok) || it.tok == "TOK_FOR")
                return r;
        }
        if(items[k].tok != "TOK_VAR_SADDR" || k + 1 >= d ||
           items[k + 1].tok.compare(0, 8, "TOK_PUSH") != 0)
            return r;
        r.var = var(k, 0);
        // Constant start ("VAR_SADDR v, PUSH_<start>, DPOKE") and limit
        // ("DPOKE, <limit>, PUSH_<step>, FOR")
        int sv, lv;
        size_t lf;
        if(r.step_known && r.step != 0 && k + 2 == d && const_value(k + 1, sv) &&
           step_first == d + 2 && const_expr(d + 1, lv, lf) && lf == d + 1)
        {
            r.range_known = true;
            r.start = sv;
            r.limit = lv;
        }
        else if(r.step_known && r.step != 0 && k + 2 == d && const_value(k + 1, sv) &&
                step_first == d + 3 && const_expr(d + 2, lv, lf) && lf == d + 1)
        {
            r.range_known = true;
            r.start = sv;
            r.limit = lv;
        }
        return r;
    }

    // True if the FOR loop "f", with its body from item "first" to "last",
    // can't overflow its variable and can compare it with the limit as a
    // constant: the start, limit and step are constants, the values stay in
    // range, and nothing in the body changes the variable.
    bool for_simple(const for_info &f, size_t first, size_t last)
    {
        if(!f.range_known || f.var.empty())
            return false;
        long s = f.start, l = f.limit, p = f.step;
        if(p > 0 && (l + p > 32767 || l - s > 32767))
            return false;
        if(p < 0 && (l + p < -32768 || s - l > 32767))
            return false;
        for(size_t k = first; k <= last && k < items.size(); k++)
        {
            auto &it = items[k];
            if(!it.instr)
                continue;
            if(it.tok == "TOK_CALL" || it.tok == "TOK_USR_CALL")
                return false;
            if((it.tok == "TOK_VAR_SADDR" || it.tok == "TOK_VAR_STORE" ||
                it.tok == "TOK_VAR_STORE_0" || it.tok == "TOK_VAR_ADDR" ||
                it.tok == "TOK_INCVAR" || it.tok == "TOK_DECVAR") &&
               valid_operands(k) && var(k, 0) == f.var)
                return false;
        }
        return true;
    }

    // Finds the FOR of the loop that starts at label "l"
    for_info for_of_label(const std::string &l)
    {
        auto it = label_item.find(l);
        if(it == label_item.end())
            return for_info();
        size_t i = it->second;
        while(i > 0 && items[i - 1].label)
            i--;
        if(i < 2 || items[i - 1].tok != "TOK_CNJUMP" || items[i - 2].tok != "TOK_FOR")
            return for_info();
        return for_stmt(i - 2);
    }

    // Instruction at i+n, if the instructions from i to i+n are all native
    // and there are no labels in between, else an empty string.
    const std::string &peek_tok(size_t i, size_t n)
    {
        static const std::string none;
        for(size_t k = 1; k <= n; k++)
            if(i + k >= items.size() || !items[i + k].instr || !items[i + k].native)
                return none;
        return items[i + n].tok;
    }

    // True if the value in AX is not used by the code starting at item j:
    // it is replaced before being read.
    bool ax_dead(size_t j, int depth = 0)
    {
        static const std::set<std::string> writes = {
            "TOK_VAR_LOAD", "TOK_NUM",       "TOK_BYTE",        "TOK_0",
            "TOK_1",        "TOK_VAR_SADDR", "TOK_VAR_ADDR",    "TOK_VAR_ADD_VAR",
            "TOK_CSTRING",  "TOK_BYTE_PEEK", "TOK_VAR_STORE_0", "TOK_END",
            "TOK_TIME",     "TOK_RAND",      "TOK_FOR_NEXT_JUMP", "TOK_FOR_NEXT"};
        for(int steps = 0; steps < 16; steps++, j++)
        {
            if(j >= items.size())
                return true;
            auto &it = items[j];
            if(it.label)
                continue;
            if(!it.instr)
                return false;
            if(writes.count(it.tok))
                return true;
            if(it.tok == "TOK_FOR_EXIT" || it.tok == "TOK_INCVAR" || it.tok == "TOK_DECVAR")
                continue;
            if(it.tok == "TOK_JUMP" && depth < 4)
            {
                auto l = label_item.find(jump_target(j));
                return l != label_item.end() && ax_dead(l->second, depth + 1);
            }
            return false;
        }
        return false;
    }

    // AX is not used after a conditional jump at item j
    bool ax_dead_after_jump(size_t j)
    {
        auto l = label_item.find(jump_target(j));
        return l != label_item.end() && ax_dead(j + 1) && ax_dead(l->second);
    }

    // Values pushed by the machine code are kept in zero page slots
    // ("nat_stk") instead of the interpreter stack, while possible.  They are
    // always the top of the stack, and are copied to the interpreter stack
    // ("flushed") before anything that needs the real stack: bytecode,
    // labels, jumps and the multiplication / division subroutines.
    static const int max_slots = 4;
    int vdepth = 0;
    bool ended_jump = false; // the last instruction generated ends with a jump

    static std::string slot(int k, bool hi)
    {
        return "nat_stk+" + std::to_string(2 * k + (hi ? 1 : 0));
    }

    // Pushes to the interpreter stack.  These pushes are always followed by
    // a slow operation (or bytecode), so a subroutine call is used.
    void real_push()
    {
        a("ldy\tsptr");
        a("jsr\tpushAX");
        helpers.insert("pushAX");
    }

    // Copies the zero page slots to the interpreter stack, preserving AX
    void flush()
    {
        if(!vdepth)
            return;
        // pushAX checks for stack overflow
        a("sta\ttmp1");
        a("stx\ttmp1+1");
        for(int k = 0; k < vdepth; k++)
        {
            a("lda\t" + slot(k, false));
            a("ldx\t" + slot(k, true));
            a("ldy\tsptr");
            a("jsr\tpushAX");
        }
        helpers.insert("pushAX");
        a("lda\ttmp1");
        a("ldx\ttmp1+1");
        vdepth = 0;
    }

    // True if the value pushed by item i can be kept in a zero page slot: it
    // is used by an operation that can read the slot before any label, jump
    // or instruction that needs the interpreter stack.
    bool slot_push(size_t i)
    {
        static const std::set<std::string> pushers = {
            "TOK_PUSH",     "TOK_PUSH_0",        "TOK_PUSH_1",          "TOK_PUSH_BYTE",
            "TOK_PUSH_NUM", "TOK_PUSH_VAR_LOAD", "TOK_PUSH_VAR_ADD_VAR"};
        static const std::set<std::string> users = {
            "TOK_ADD", "TOK_SUB", "TOK_BIT_AND", "TOK_BIT_OR", "TOK_BIT_EXOR", "TOK_EQ",
            "TOK_NEQ", "TOK_LT",  "TOK_GT",      "TOK_L_AND",  "TOK_L_OR",     "TOK_POP"};
        static const std::set<std::string> neutral = {
            "TOK_0",        "TOK_1",        "TOK_BYTE",      "TOK_NUM",       "TOK_VAR_LOAD",
            "TOK_VAR_ADDR", "TOK_VAR_STORE", "TOK_VAR_STORE_0", "TOK_VAR_SADDR", "TOK_SADDR",
            "TOK_ADD_VAR",  "TOK_VAR_ADD_VAR", "TOK_NEG",     "TOK_ABS",       "TOK_USHL",
            "TOK_SHL8",     "TOK_EQ_BYTE",  "TOK_COMP_0",    "TOK_L_NOT",     "TOK_PEEK",
            "TOK_DPEEK",    "TOK_POKE",     "TOK_DPOKE",     "TOK_POKE_AX_BYTE",
            "TOK_BYTE_PEEK", "TOK_BYTE_POKE", "TOK_NUM_POKE", "TOK_INC",       "TOK_DEC",
            "TOK_INCVAR",   "TOK_DECVAR",   "TOK_DIV_POW2"};
        int depth = 1;
        for(size_t k = 1; k < 64; k++)
        {
            auto &t = peek_tok(i, k);
            if(t.empty())
                return false;
            if(pushers.count(t))
                depth++;
            else if(users.count(t))
            {
                if(--depth == 0)
                    return true;
            }
            else if(!neutral.count(t))
                return false;
        }
        return false;
    }

    void push(size_t i)
    {
        if(!slot_push(i))
        {
            flush();
            real_push();
            return;
        }
        if(vdepth == max_slots)
            flush();
        a("sta\t" + slot(vdepth, false));
        a("stx\t" + slot(vdepth, true));
        vdepth++;
    }

    // The right operand of a binary operation: the top of the stack, or a
    // variable or constant loaded by the previous "PUSH_xxx" instruction.
    struct operand
    {
        bool stack = true;
        bool left = false;  // the operand is the left side (from a slot)
        bool cnst = false;  // a known constant
        int val = 0;        // value of the constant
        std::string lo, hi; // assembly operands
    };

    // Operand loaded by instruction i, if it is "PUSH_VAR_LOAD v",
    // "PUSH_BYTE n", "PUSH_NUM n", "PUSH_0" or "PUSH_1".
    bool push_operand(size_t i, operand &o)
    {
        auto &t = items[i].tok;
        o = operand();
        o.stack = false;
        if(t == "TOK_PUSH_VAR_LOAD")
        {
            o.lo = var(i, 0);
            o.hi = var(i, 0) + "+1";
        }
        else if(t == "TOK_PUSH_0" || t == "TOK_PUSH_1")
        {
            o.cnst = true;
            o.val = (t == "TOK_PUSH_1") ? 1 : 0;
            o.lo = "#" + std::to_string(o.val);
            o.hi = "#0";
        }
        else if(t == "TOK_PUSH_BYTE")
        {
            if(op(i, 0).is_byte())
            {
                o.cnst = true;
                o.val = op(i, 0).get_val() & 0xFF;
            }
            o.lo = "#" + opexpr(i, 0);
            o.hi = "#0";
        }
        else if(t == "TOK_PUSH_NUM")
        {
            if(op(i, 0).is_word())
            {
                o.cnst = true;
                o.val = int16_t(op(i, 0).get_val());
            }
            o.lo = "#<(" + opexpr(i, 0) + ")";
            o.hi = "#>(" + opexpr(i, 0) + ")";
        }
        else
            return false;
        return true;
    }

    enum cmp_kind
    {
        C_EQ,
        C_NEQ,
        C_LT,
        C_GT,
        C_NZ
    };

    // The left operand of a binary operation, from the stack: a zero page
    // slot or the interpreter stack.
    operand pop_operand()
    {
        operand o;
        if(vdepth)
        {
            vdepth--;
            o.stack = false;
            o.left = true;
            o.lo = slot(vdepth, false);
            o.hi = slot(vdepth, true);
        }
        return o;
    }

    // Same, for a comparison: LT and GT are swapped with a slot operand, as
    // "slot < AX" is "AX > slot".
    operand pop_cmp(cmp_kind &k)
    {
        auto o = pop_operand();
        if(o.left)
        {
            if(k == C_LT)
                k = C_GT;
            else if(k == C_GT)
                k = C_LT;
        }
        return o;
    }

    // AX = AX <op> operand, for ADD, SUB, BIT_AND, BIT_OR and BIT_EXOR.
    // With the stack operand the order is (SP+) <op> AX.
    void arith(const std::string &t, const operand &o)
    {
        if(t == "TOK_SUB" && o.left)
        {
            // AX = slot - AX
            a("sta\ttmp1");
            a("stx\ttmp1+1");
            a("sec");
            a("lda\t" + o.lo);
            a("sbc\ttmp1");
            a("tay");
            a("lda\t" + o.hi);
            a("sbc\ttmp1+1");
            a("tax");
            a("tya");
            return;
        }
        if((t == "TOK_SUB" || t == "TOK_ADD") && o.stack)
        {
            auto f = t == "TOK_ADD" ? "nat_add_s" : "nat_sub_s";
            a(std::string("jsr\t") + f);
            helpers.insert(f);
            return;
        }
        if(t == "TOK_SUB" && o.stack)
        {
            a("sta\ttmp1");
            a("stx\ttmp1+1");
            a("ldy\tsptr");
            a("sec");
            a("lda\tstack_l, y");
            a("sbc\ttmp1");
            a("sta\ttmp1");
            a("lda\tstack_h, y");
            a("sbc\ttmp1+1");
            a("tax");
            a("lda\ttmp1");
            a("inc\tsptr");
            return;
        }
        std::string ins = t == "TOK_ADD"     ? "adc"
                          : t == "TOK_SUB"   ? "sbc"
                          : t == "TOK_BIT_AND" ? "and"
                          : t == "TOK_BIT_OR"  ? "ora"
                                               : "eor";
        if(o.stack)
        {
            if(ins == "adc")
                a("clc");
            a("ldy\tsptr");
            a(ins + "\tstack_l, y");
            a("sta\ttmp1");
            a("txa");
            a(ins + "\tstack_h, y");
            a("tax");
            a("lda\ttmp1");
            a("inc\tsptr");
            return;
        }
        // Small constants
        if(o.cnst && o.val >= 0 && o.val < 256)
        {
            if(ins == "adc")
            {
                if(o.val == 0)
                    return;
                a("clc");
                a("adc\t" + o.lo);
                a("bcc\t:+");
                a("inx");
                a(":");
                return;
            }
            if(ins == "sbc")
            {
                if(o.val == 0)
                    return;
                a("sec");
                a("sbc\t" + o.lo);
                a("bcs\t:+");
                a("dex");
                a(":");
                return;
            }
            if(ins == "and")
            {
                a("and\t" + o.lo);
                a("ldx\t#0");
                return;
            }
            // ORA / EOR with 0 in the high byte
            a(ins + "\t" + o.lo);
            return;
        }
        if(ins == "adc")
            a("clc");
        if(ins == "sbc")
            a("sec");
        a(ins + "\t" + o.lo);
        a("tay");
        a("txa");
        a(ins + "\t" + o.hi);
        a("tax");
        a("tya");
    }

    // Comparisons: generates code that leaves the result in a flag.  Returns
    // the flag ('Z' or 'N') and the flag value that means "true".
    std::pair<char, bool> cmp_flags(cmp_kind k, const operand &o)
    {
        if(k == C_NZ)
        {
            a("stx\ttmp1");
            a("ora\ttmp1");
            return {'Z', false};
        }
        if(k == C_EQ || k == C_NEQ)
        {
            if(o.stack)
            {
                a("ldy\tsptr");
                a("inc\tsptr");
                a("cmp\tstack_l, y");
                a("bne\t:+");
                a("txa");
                a("cmp\tstack_h, y");
            }
            else
            {
                a("cmp\t" + o.lo);
                a("bne\t:+");
                a("cpx\t" + o.hi);
            }
            a(":");
            return {'Z', k == C_EQ};
        }
        auto sign = [&]() {
            a("bvc\t:+");
            a("eor\t#$80");
            a(":");
        };
        if(k == C_LT)
        {
            if(o.stack)
            {
                // (SP+) < AX
                a("sta\ttmp1");
                a("stx\ttmp1+1");
                a("ldy\tsptr");
                a("inc\tsptr");
                a("lda\tstack_l, y");
                a("cmp\ttmp1");
                a("lda\tstack_h, y");
                a("sbc\ttmp1+1");
            }
            else
            {
                // AX < operand
                a("cmp\t" + o.lo);
                a("txa");
                a("sbc\t" + o.hi);
            }
            sign();
            return {'N', true};
        }
        // C_GT
        if(o.stack)
        {
            // (SP+) > AX  <=>  AX < (SP+)
            a("ldy\tsptr");
            a("inc\tsptr");
            a("cmp\tstack_l, y");
            a("txa");
            a("sbc\tstack_h, y");
            sign();
            return {'N', true};
        }
        if(o.cnst && o.val < 32767)
        {
            // AX > n  <=>  NOT (AX < n+1)
            a("cmp\t#<" + std::to_string((o.val + 1) & 0xFFFF));
            a("txa");
            a("sbc\t#>" + std::to_string((o.val + 1) & 0xFFFF));
            sign();
            return {'N', false};
        }
        // AX > operand  <=>  operand < AX
        a("sta\ttmp1");
        a("stx\ttmp1+1");
        a("lda\t" + o.lo);
        a("cmp\ttmp1");
        a("lda\t" + o.hi);
        a("sbc\ttmp1+1");
        sign();
        return {'N', true};
    }

    // Converts the result flag to 0 / 1 in AX
    void flag_to_bool(std::pair<char, bool> f)
    {
        if(f.first == 'Z')
        {
            a(std::string(f.second ? "bne" : "beq") + "\t:+");
            a("lda\t#1");
            a("bne\t:++");
            a(":\tlda\t#0");
            a(":\tldx\t#0");
        }
        else
        {
            a("asl");
            a("lda\t#0");
            a("rol");
            if(!f.second)
                a("eor\t#1");
            a("ldx\t#0");
        }
    }

    // Jumps to the target if the result flag is "when"
    void flag_jump(std::pair<char, bool> f, bool when, const std::string &target)
    {
        bool set = (f.second == when); // jump if the flag is set
        if(f.first == 'Z')
            branch(set ? "beq" : "bne", target);
        else
            branch(set ? "bmi" : "bpl", target);
    }

    // Comparison at item c (with operand o), followed by the items from
    // c+1: if they are an optional L_NOT and a conditional jump, generates
    // a direct jump.  Returns the number of items after c used.
    size_t compare(size_t c, cmp_kind k, const operand &o)
    {
        size_t n = 1;
        bool inv = false;
        if(peek_tok(c, n) == "TOK_L_NOT")
        {
            inv = true;
            n++;
        }
        auto &j = peek_tok(c, n);
        if((j == "TOK_CJUMP" || j == "TOK_CNJUMP") && ax_dead_after_jump(c + n))
        {
            // CJUMP jumps if false, CNJUMP if true
            bool when = (j == "TOK_CNJUMP") != inv;
            flush();
            flag_jump(cmp_flags(k, o), when, jump_target(c + n));
            return n;
        }
        flag_to_bool(cmp_flags(k, o));
        return 0;
    }

    // Multiplication of AX by a small constant, using shifts and adds
    bool mul_const(int c)
    {
        if(c == 0)
        {
            a("lda\t#0");
            a("tax");
            return true;
        }
        if(c == 1)
            return true;
        if(c == 256)
        {
            a("tax");
            a("lda\t#0");
            return true;
        }
        if(c < 2 || c > 255)
            return false;
        int top = 7;
        while(!(c & (1 << top)))
            top--;
        // Bytes of code: 6 per shift, 9 per add and 4 to save the value.
        // Bigger multiplications call nat_mulb.
        int adds = __builtin_popcount(c) - 1;
        if(top * 6 + adds * 9 + (adds ? 4 : 0) > 20)
            return false;
        if(__builtin_popcount(c) > 1)
        {
            a("sta\ttmp1");
            a("stx\ttmp1+1");
        }
        for(int b = top - 1; b >= 0; b--)
        {
            a("asl");
            a("tay");
            a("txa");
            a("rol");
            a("tax");
            a("tya");
            if(c & (1 << b))
            {
                a("clc");
                a("adc\ttmp1");
                a("tay");
                a("txa");
                a("adc\ttmp1+1");
                a("tax");
                a("tya");
            }
        }
        return true;
    }

    // Multiplication, division and modulo, with the right operand in AX
    void math(const std::string &t)
    {
        if(t == "TOK_MUL")
        {
            a("jsr\tnat_mul");
            helpers.insert("nat_mul");
        }
        else
        {
            // DIVC / MODC: a division and a modulo of the same operands, the
            // second one uses the result of the first.
            std::string f = t == "TOK_DIV" ? "nat_div" : t == "TOK_MOD" ? "nat_mod"
                          : t == "TOK_DIVC" ? "nat_divc" : "nat_modc";
            a("jsr\t" + f);
            helpers.insert(f);
        }
    }

    static bool is_math(const std::string &t)
    {
        return t == "TOK_MUL" || t == "TOK_DIV" || t == "TOK_DIVC" || t == "TOK_MOD" ||
               t == "TOK_MODC";
    }

    static bool is_arith(const std::string &t)
    {
        return t == "TOK_ADD" || t == "TOK_SUB" || t == "TOK_BIT_AND" || t == "TOK_BIT_OR" ||
               t == "TOK_BIT_EXOR";
    }

    static bool is_compare(const std::string &t, cmp_kind &k)
    {
        if(t == "TOK_EQ")
            k = C_EQ;
        else if(t == "TOK_NEQ")
            k = C_NEQ;
        else if(t == "TOK_LT")
            k = C_LT;
        else if(t == "TOK_GT")
            k = C_GT;
        else
            return false;
        return true;
    }

    // Memory access through the address in tmp1
    void mem_tmp1(size_t i)
    {
        auto &t = items[i].tok;
        if(t == "TOK_PEEK")
        {
            a("ldy\t#0");
            a("lda\t(tmp1), y");
            a("ldx\t#0");
        }
        else if(t == "TOK_DPEEK")
        {
            a("ldy\t#1");
            a("lda\t(tmp1), y");
            a("tax");
            a("dey");
            a("lda\t(tmp1), y");
        }
        else if(t == "TOK_POKE_AX_BYTE")
        {
            a("lda\t#" + opexpr(i, 0));
            a("ldy\t#0");
            a("sta\t(tmp1), y");
        }
        else // INC / DEC
        {
            a("ldy\t#0");
            a("lda\t(tmp1), y");
            a("clc");
            a(t == "TOK_INC" ? "adc\t#1" : "adc\t#$FF");
            a("sta\t(tmp1), y");
            a("iny");
            a("lda\t(tmp1), y");
            a(t == "TOK_INC" ? "adc\t#0" : "adc\t#$FF");
            a("sta\t(tmp1), y");
        }
    }

    static bool is_mem(const std::string &t)
    {
        return t == "TOK_PEEK" || t == "TOK_DPEEK" || t == "TOK_POKE_AX_BYTE" ||
               t == "TOK_INC" || t == "TOK_DEC";
    }

    // Stores to a variable with a known address: "VAR_SADDR v" followed by an
    // expression and DPOKE or POKE.  Item index of the store -> variable.
    std::map<size_t, std::string> static_store;

    size_t find_store(size_t i)
    {
        for(size_t k = 1; k < 64; k++)
        {
            auto &t = peek_tok(i, k);
            if(t.empty() || t == "TOK_SADDR" || t == "TOK_VAR_SADDR" || t == "TOK_NUM_POKE" ||
               is_jump(t) || t == "TOK_FOR" || t == "TOK_FOR_NEXT")
                return 0;
            if(t == "TOK_DPOKE" || t == "TOK_POKE")
                return i + k;
        }
        return 0;
    }

    // Compares the variable with the limit of the FOR in the stack, leaves
    // the N flag set if the loop ended.  The stack has: step, limit and the
    // variable address.
    void for_compare(const for_info &f)
    {
        std::string vl, vh;
        a("ldy\tsptr");
        if(f.var.empty())
        {
            a("lda\tstack_l+2, y");
            a("sta\ttmp3");
            a("lda\tstack_h+2, y");
            a("sta\ttmp3+1");
            a("ldy\t#0");
            a("lda\t(tmp3), y");
            a("sta\ttmp2");
            a("iny");
            a("lda\t(tmp3), y");
            a("sta\ttmp2+1");
            a("ldy\tsptr");
            vl = "tmp2";
            vh = "tmp2+1";
        }
        else
        {
            vl = f.var;
            vh = f.var + "+1";
        }
        auto pos = [&]() {
            // Positive step: ended if LIMIT < VAR
            a("lda\tstack_l+1, y");
            a("cmp\t" + vl);
            a("lda\tstack_h+1, y");
            a("sbc\t" + vh);
        };
        auto neg = [&]() {
            // Negative step: ended if VAR < LIMIT
            a("lda\t" + vl);
            a("cmp\tstack_l+1, y");
            a("lda\t" + vh);
            a("sbc\tstack_h+1, y");
        };
        if(f.step_known)
        {
            if(f.step >= 0)
                pos();
            else
                neg();
        }
        else
        {
            auto ln = new_label(), le = new_label();
            a("lda\tstack_h, y");
            a("bmi\t" + ln);
            pos();
            a("jmp\t" + le);
            out.push_back(codew::casm(ln + ":", lnum));
            neg();
            out.push_back(codew::casm(le + ":", lnum));
        }
        a("bvc\t:+");
        a("eor\t#$80");
        a(":");
    }

    // FOR_NEXT_JUMP: adds STEP to the variable and jumps to the loop start
    // if not ended.
    void for_next(const std::string &target, size_t at)
    {
        flush();
        auto f = for_of_label(target);
        auto tl = label_item.find(target);
        if(tl != label_item.end() && for_simple(f, tl->second, at))
        {
            // No overflow possible: add the step and compare with the
            // constant limit (the result of the subtraction fits too)
            if(f.step == 1)
            {
                auto ok = new_label();
                a("inc\t" + f.var);
                a("bne\t" + ok);
                a("inc\t" + f.var + "+1");
                out.push_back(codew::casm(ok + ":", lnum));
            }
            else if(f.step == -1)
            {
                auto ok = new_label();
                a("lda\t" + f.var);
                a("bne\t" + ok);
                a("dec\t" + f.var + "+1");
                out.push_back(codew::casm(ok + ":", lnum));
                a("dec\t" + f.var);
            }
            else
            {
                a("clc");
                a("lda\t" + f.var);
                a("adc\t#<" + std::to_string(f.step & 0xFFFF));
                a("sta\t" + f.var);
                a("lda\t" + f.var + "+1");
                a("adc\t#>" + std::to_string(f.step & 0xFFFF));
                a("sta\t" + f.var + "+1");
            }
            std::string lim = std::to_string(f.limit & 0xFFFF);
            if(f.step > 0)
            {
                // Continue while LIMIT - VAR >= 0
                a("lda\t#<" + lim);
                a("cmp\t" + f.var);
                a("lda\t#>" + lim);
                a("sbc\t" + f.var + "+1");
            }
            else
            {
                // Continue while VAR - LIMIT >= 0
                a("lda\t" + f.var);
                a("cmp\t#<" + lim);
                a("lda\t" + f.var + "+1");
                a("sbc\t#>" + lim);
            }
            branch("bpl", target);
            return;
        }
        // If the variable overflows (from 32767 to -32768 or the reverse)
        // the loop ends, as the next value is out of range
        auto end = new_label();
        if(!f.var.empty() && f.step_known && f.step == 1)
        {
            auto ok = new_label();
            a("inc\t" + f.var);
            a("bne\t" + ok);
            a("inc\t" + f.var + "+1");
            a("lda\t" + f.var + "+1");
            a("cmp\t#$80");
            a("beq\t" + end);
            out.push_back(codew::casm(ok + ":", lnum));
        }
        else if(!f.var.empty() && f.step_known)
        {
            a("clc");
            a("lda\t" + f.var);
            a("adc\t#<" + std::to_string(f.step & 0xFFFF));
            a("sta\t" + f.var);
            a("lda\t" + f.var + "+1");
            a("adc\t#>" + std::to_string(f.step & 0xFFFF));
            a("sta\t" + f.var + "+1");
            a("bvs\t" + end);
        }
        else if(!f.var.empty())
        {
            a("ldy\tsptr");
            a("clc");
            a("lda\t" + f.var);
            a("adc\tstack_l, y");
            a("sta\t" + f.var);
            a("lda\t" + f.var + "+1");
            a("adc\tstack_h, y");
            a("sta\t" + f.var + "+1");
            a("bvs\t" + end);
        }
        else
        {
            a("ldy\tsptr");
            a("lda\tstack_l+2, y");
            a("sta\ttmp3");
            a("lda\tstack_h+2, y");
            a("sta\ttmp3+1");
            a("ldy\t#0");
            a("clc");
            a("lda\t(tmp3), y");
            a("ldx\tsptr");
            a("adc\tstack_l, x");
            a("sta\t(tmp3), y");
            a("iny");
            a("lda\t(tmp3), y");
            a("adc\tstack_h, x");
            a("sta\t(tmp3), y");
            a("bvs\t" + end);
        }
        for_compare(f);
        // N = 1 if ended
        branch("bpl", target);
        out.push_back(codew::casm(end + ":", lnum));
    }

    // Generates the machine code of instruction i, returns the number of
    // instructions used.
    size_t gen(size_t i)
    {
        auto &t = items[i].tok;
        operand o;
        cmp_kind k;
        // "PUSH_xxx" followed by an operation that uses the value
        if(push_operand(i, o))
        {
            auto &n = peek_tok(i, 1);
            if(is_arith(n))
            {
                arith(n, o);
                return 2;
            }
            if(is_compare(n, k))
                return 2 + compare(i + 1, k, o);
            if(n == "TOK_MUL" && o.cnst && mul_const(o.val))
                return 2;
            // Multiplication and division by a byte constant
            if(is_math(n) && o.cnst && o.val > 0 && o.val < 256)
            {
                flush();
                a("ldy\t#" + std::to_string(o.val));
                std::string f = n == "TOK_MUL"    ? "nat_mulb"
                                : n == "TOK_DIV"  ? "nat_divb"
                                : n == "TOK_MOD"  ? "nat_modb"
                                : n == "TOK_DIVC" ? "nat_divcb"
                                                  : "nat_modcb";
                a("jsr\t" + f);
                helpers.insert(f);
                return 2;
            }
        }
        // "c * v": BYTE c, PUSH_VAR_LOAD v, MUL
        if(t == "TOK_BYTE" && op(i, 0).is_byte() && peek_tok(i, 1) == "TOK_PUSH_VAR_LOAD" &&
           peek_tok(i, 2) == "TOK_MUL")
        {
            int c = op(i, 0).get_val() & 0xFF;
            a("lda\t" + var(i + 1, 0));
            a("ldx\t" + var(i + 1, 0) + "+1");
            if(!mul_const(c))
            {
                a("ldy\t#" + std::to_string(c));
                a("jsr\tnat_mulb");
                helpers.insert("nat_mulb");
            }
            return 3;
        }
        if(is_arith(t))
        {
            arith(t, pop_operand());
            return 1;
        }
        if(is_compare(t, k))
        {
            auto po = pop_cmp(k);
            return 1 + compare(i, k, po);
        }
        if(t == "TOK_EQ_BYTE")
        {
            o.stack = false;
            o.lo = "#" + opexpr(i, 0);
            o.hi = "#0";
            return 1 + compare(i, C_EQ, o);
        }
        if(t == "TOK_COMP_0")
            return 1 + compare(i, C_NZ, operand());
        if(is_math(t))
        {
            flush();
            math(t);
            return 1;
        }
        // Address calculation followed by a memory access
        if(is_mem(peek_tok(i, 1)) &&
           (t == "TOK_ADD_VAR" || t == "TOK_VAR_ADD_VAR" || t == "TOK_VAR_LOAD"))
        {
            if(t == "TOK_ADD_VAR")
            {
                a("clc");
                a("adc\t" + var(i, 0));
                a("sta\ttmp1");
                a("txa");
                a("adc\t" + var(i, 0) + "+1");
                a("sta\ttmp1+1");
            }
            else if(t == "TOK_VAR_ADD_VAR")
            {
                a("clc");
                a("lda\t" + var(i, 0));
                a("adc\t" + var(i, 1));
                a("sta\ttmp1");
                a("lda\t" + var(i, 0) + "+1");
                a("adc\t" + var(i, 1) + "+1");
                a("sta\ttmp1+1");
            }
            else
            {
                a("lda\t" + var(i, 0));
                a("sta\ttmp1");
                a("lda\t" + var(i, 0) + "+1");
                a("sta\ttmp1+1");
            }
            mem_tmp1(i + 1);
            return 2;
        }
        if(is_mem(t))
        {
            a("sta\ttmp1");
            a("stx\ttmp1+1");
            mem_tmp1(i);
            return 1;
        }
        if(t == "TOK_PUSH_0" || t == "TOK_PUSH_1" || t == "TOK_PUSH_BYTE" ||
           t == "TOK_PUSH_NUM" || t == "TOK_PUSH_VAR_LOAD" || t == "TOK_PUSH_VAR_ADD_VAR")
        {
            push(i);
            gen_simple(i, "TOK_" + t.substr(9));
            return 1;
        }
        if(t == "TOK_VAR_SADDR")
        {
            size_t s = find_store(i);
            if(s)
            {
                // The store uses the variable address directly
                static_store[s] = var(i, 0);
                a("lda\t#<" + var(i, 0));
                a("ldx\t#>" + var(i, 0));
                return 1;
            }
        }
        if((t == "TOK_DPOKE" || t == "TOK_POKE") && static_store.count(i))
        {
            auto v = static_store[i];
            a("sta\t" + v);
            if(t == "TOK_DPOKE")
                a("stx\t" + v + "+1");
            return 1;
        }
        if(t == "TOK_FOR")
        {
            flush();
            real_push();
            auto fs = for_stmt(i);
            // With a constant start and limit the first check is known: the
            // loop always runs at least once, or never.
            if(fs.range_known && peek_tok(i, 1) == "TOK_CNJUMP" && ax_dead_after_jump(i + 1))
            {
                if(fs.step > 0 ? fs.start > fs.limit : fs.start < fs.limit)
                    jump_to(jump_target(i + 1));
                return 2;
            }
            for_compare(fs);
            // Followed by CNJUMP to the loop exit
            if(peek_tok(i, 1) == "TOK_CNJUMP" && ax_dead_after_jump(i + 1))
            {
                branch("bmi", jump_target(i + 1));
                return 2;
            }
            flag_to_bool({'N', true});
            return 1;
        }
        if(t == "TOK_FOR_NEXT_JUMP")
        {
            for_next(jump_target(i), i);
            return 1;
        }
        if(t == "TOK_FOR_NEXT")
        {
            // Always followed by CJUMP
            for_next(jump_target(i + 1), i);
            return 2;
        }
        if(t == "TOK_DIV_POW2")
        {
            a("ldy\t#" + opexpr(i, 0));
            a("jsr\tnat_divpow2");
            helpers.insert("nat_divpow2");
            return 1;
        }
        gen_simple(i, t);
        return 1;
    }

    // Instructions without combinations
    void gen_simple(size_t i, const std::string &t)
    {
        if(t == "TOK_0")
        {
            a("lda\t#0");
            a("tax");
        }
        else if(t == "TOK_1")
        {
            a("lda\t#1");
            a("ldx\t#0");
        }
        else if(t == "TOK_BYTE")
        {
            a("lda\t#" + opexpr(i, 0));
            a("ldx\t#0");
        }
        else if(t == "TOK_NUM")
        {
            a("lda\t#<(" + opexpr(i, 0) + ")");
            a("ldx\t#>(" + opexpr(i, 0) + ")");
        }
        else if(t == "TOK_PUSH")
            push(i);
        else if(t == "TOK_POP" && vdepth)
        {
            vdepth--;
            a("lda\t" + slot(vdepth, false));
            a("ldx\t" + slot(vdepth, true));
        }
        else if(t == "TOK_POP")
        {
            a("ldy\tsptr");
            a("lda\tstack_l, y");
            a("ldx\tstack_h, y");
            a("inc\tsptr");
        }
        else if(t == "TOK_VAR_LOAD")
        {
            a("lda\t" + var(i, 0));
            a("ldx\t" + var(i, 0) + "+1");
        }
        else if(t == "TOK_VAR_ADDR")
        {
            a("lda\t#<" + var(i, 0));
            a("ldx\t#>" + var(i, 0));
        }
        else if(t == "TOK_VAR_STORE")
        {
            a("sta\t" + var(i, 0));
            a("stx\t" + var(i, 0) + "+1");
        }
        else if(t == "TOK_VAR_STORE_0")
        {
            a("lda\t#0");
            a("tax");
            a("sta\t" + var(i, 0));
            a("sta\t" + var(i, 0) + "+1");
        }
        else if(t == "TOK_VAR_SADDR")
        {
            a("lda\t#<" + var(i, 0));
            a("sta\tsaddr");
            a("ldx\t#>" + var(i, 0));
            a("stx\tsaddr+1");
        }
        else if(t == "TOK_SADDR")
        {
            a("sta\tsaddr");
            a("stx\tsaddr+1");
        }
        else if(t == "TOK_ADD_VAR")
        {
            a("clc");
            a("adc\t" + var(i, 0));
            a("tay");
            a("txa");
            a("adc\t" + var(i, 0) + "+1");
            a("tax");
            a("tya");
        }
        else if(t == "TOK_VAR_ADD_VAR")
        {
            a("clc");
            a("lda\t" + var(i, 0));
            a("adc\t" + var(i, 1));
            a("tay");
            a("lda\t" + var(i, 0) + "+1");
            a("adc\t" + var(i, 1) + "+1");
            a("tax");
            a("tya");
        }
        else if(t == "TOK_NEG" || t == "TOK_ABS")
        {
            if(t == "TOK_ABS")
            {
                a("cpx\t#0");
                a("bpl\t:+");
            }
            a("clc");
            a("eor\t#$FF");
            a("adc\t#1");
            a("tay");
            a("txa");
            a("eor\t#$FF");
            a("adc\t#0");
            a("tax");
            a("tya");
            if(t == "TOK_ABS")
                a(":");
        }
        else if(t == "TOK_USHL")
        {
            a("asl");
            a("tay");
            a("txa");
            a("rol");
            a("tax");
            a("tya");
        }
        else if(t == "TOK_SHL8")
        {
            a("tax");
            a("lda\t#0");
        }
        else if(t == "TOK_L_NOT")
            a("eor\t#1");
        else if((t == "TOK_L_AND" || t == "TOK_L_OR") && vdepth)
        {
            vdepth--;
            a(std::string(t == "TOK_L_AND" ? "and" : "ora") + "\t" + slot(vdepth, false));
        }
        else if(t == "TOK_L_AND" || t == "TOK_L_OR")
        {
            a("ldy\tsptr");
            a(std::string(t == "TOK_L_AND" ? "and" : "ora") + "\tstack_l, y");
            a("inc\tsptr");
        }
        else if(t == "TOK_POKE")
        {
            a("ldy\t#0");
            a("sta\t(saddr), y");
        }
        else if(t == "TOK_DPOKE")
        {
            a("ldy\t#0");
            a("sta\t(saddr), y");
            a("iny");
            a("txa");
            a("sta\t(saddr), y");
        }
        else if(t == "TOK_BYTE_PEEK")
        {
            a("ldy\t#" + opexpr(i, 0));
            a("lda\t0, y");
            a("ldx\t#0");
        }
        else if(t == "TOK_BYTE_POKE")
        {
            a("ldy\t#" + opexpr(i, 0));
            a("sta\t0, y");
            a("tax");
            a("tya");
        }
        else if(t == "TOK_NUM_POKE")
        {
            // The address in SADDR is not used after this instruction
            a("sta\t" + opexpr(i, 0));
            a("tax");
        }
        else if(t == "TOK_INCVAR")
        {
            a("inc\t" + var(i, 0));
            a("bne\t:+");
            a("inc\t" + var(i, 0) + "+1");
            a(":");
        }
        else if(t == "TOK_DECVAR")
        {
            a("ldy\t" + var(i, 0));
            a("bne\t:+");
            a("dec\t" + var(i, 0) + "+1");
            a(":\tdec\t" + var(i, 0));
        }
        else if(t == "TOK_JUMP")
        {
            flush();
            jump_to(jump_target(i));
        }
        else if(t == "TOK_SC_AND" || t == "TOK_SC_OR")
        {
            // Short-circuit: jump keeping the result (0 or 1) in A
            flush();
            a("cmp\t#1");
            branch(t == "TOK_SC_AND" ? "bcc" : "bcs", jump_target(i));
        }
        else if(t == "TOK_CJUMP" || t == "TOK_CNJUMP")
        {
            flush();
            if(t == "TOK_CNJUMP")
                a("eor\t#1");
            a("lsr");
            branch("bcc", jump_target(i));
        }
        else if(t == "TOK_CALL")
        {
            // Pushes the return address as the interpreter does: the
            // bytecode continues there with TOK_NATIVE.
            flush();
            // Check the CPU stack as the CALL instruction (CALL_STACK_MIN in
            // jump.asm)
            a("tsx");
            a("cpx\t#$40");
            a("bcs\t:+");
            a("jmp\terr_stack");
            a(":");
            helpers.insert("err_stack");
            auto r = new_label();
            a("lda\t#<" + r);
            a("pha");
            a("lda\t#>" + r);
            a("pha");
            jump_to(jump_target(i));
            if(i + 1 < items.size() && items[i + 1].instr && !items[i + 1].native)
            {
                // Returns directly to the bytecode that follows
                out.push_back(codew::clabel(r, lnum));
                ended_jump = true;
            }
            else
            {
                out.push_back(codew::casm(r + ":", lnum));
                out.push_back(codew::ctok("TOK_NATIVE", lnum));
            }
        }
        else if(t == "TOK_RET" || t == "TOK_CRET" || t == "TOK_CNRET")
        {
            flush();
            if(t == "TOK_CNRET")
                a("eor\t#1");
            if(t != "TOK_RET")
            {
                a("lsr");
                a("bcs\t:+");
            }
            a("tay");
            a("pla");
            a("sta\tcptr+1");
            a("pla");
            a("sta\tcptr");
            a("tya");
            a("jmp\tnext_instruction");
            if(t != "TOK_RET")
                a(":");
        }
        else if(t == "TOK_FOR_EXIT")
        {
            flush();
            a("inc\tsptr");
            a("inc\tsptr");
            a("inc\tsptr");
        }
        else
            throw std::runtime_error("internal error: no native code for " + t);
    }

    void emit()
    {
        bool prev_native = false; // previous instruction was native code
        bool prev_jump = false;   // previous native instruction was a JUMP
        for(size_t i = 0; i < items.size(); i++)
        {
            auto &it = items[i];
            lnum = code[it.pos].linenum();
            if(it.label || !it.native)
                flush();
            if(it.label)
            {
                // Group of labels before the next instruction
                size_t j = i;
                while(j < items.size() && items[j].label)
                    j++;
                bool nat = j < items.size() && items[j].native;
                if(nat)
                {
                    bool ext = !prev_native;
                    for(size_t k = i; k < j; k++)
                        if(label_external(code[items[k].pos].get_str()))
                            ext = true;
                    if(ext)
                    {
                        if(prev_native && !prev_jump)
                            out.push_back(codew::casm("\t.byte\t$24\t; BIT zp", lnum));
                        for(size_t k = i; k < j; k++)
                            out.push_back(code[items[k].pos]);
                        out.push_back(codew::ctok("TOK_NATIVE", lnum));
                    }
                    for(size_t k = i; k < j; k++)
                        out.push_back(
                            codew::casm(nlabel_of(code[items[k].pos].get_str()) + ":", lnum));
                    // At the start of each loop, stop the program if the
                    // BREAK key was pressed (BRKKEY is 0)
                    bool head = false;
                    for(auto &lp : loops)
                        if(lp.first >= i && lp.first < j)
                            head = true;
                    if(head)
                    {
                        a("ldy\t$11");
                        a("bne\t:+");
                        a("jmp\terr_break");
                        a(":");
                        helpers.insert("err_break");
                    }
                    prev_native = true;
                    prev_jump = false;
                }
                else
                {
                    if(prev_native && !prev_jump)
                        exit_to(code[it.pos].get_str());
                    for(size_t k = i; k < j; k++)
                        out.push_back(code[items[k].pos]);
                    prev_native = false;
                }
                i = j - 1;
                continue;
            }
            if(it.native)
            {
                if(!prev_native)
                    out.push_back(codew::ctok("TOK_NATIVE", lnum));
                used = true;
                size_t before = out.size();
                ended_jump = false;
                size_t n = gen(i);
                if(getenv("EXBASIC_NATIVE_STATS"))
                {
                    std::string key = it.tok;
                    for(size_t k = 1; k < n; k++)
                        key += " " + items[i + k].tok;
                    stats[key].first++;
                    for(size_t k = before; k < out.size(); k++)
                        stats[key].second += asm_size(out[k].to_asm());
                }
                i += n - 1;
                prev_native = true;
                prev_jump = items[i].tok == "TOK_JUMP" || items[i].tok == "TOK_RET" || ended_jump;
            }
            else
            {
                if(prev_native && !prev_jump)
                {
                    auto l = new_label();
                    exit_to(l);
                    out.push_back(codew::clabel(l, lnum));
                }
                for(size_t k = 0; k < it.len; k++)
                    out.push_back(code[it.pos + k]);
                prev_native = false;
            }
        }
        if(prev_native && !prev_jump)
            throw std::runtime_error("internal error: native code at end of program");
    }

  public:
    double hot_ratio = 64;  // compile code that runs 1/hot_ratio as often as the hottest
    double min_weight = 64; // and at least this many times
    bool lean = false;      // remove the machine code that saves little time

    native_gen(std::vector<codew> &code) : code(code)
    {
        if(auto e = getenv("EXBASIC_HOT_RATIO"))
            hot_ratio = atof(e);
        if(auto e = getenv("EXBASIC_HOT_MIN"))
            min_weight = atof(e);
    }

    bool run()
    {
        scan();
        find_hot();
        select();
        emit();
        // "ldx #0 / stx tmp1 / ora tmp1": X is zero, just test A
        for(size_t k = 0; k + 2 < out.size(); k++)
            if(out[k].is_asm() && out[k].to_asm() == "\tldx\t#0" && out[k + 1].is_asm() &&
               out[k + 1].to_asm() == "\tstx\ttmp1" && out[k + 2].is_asm() &&
               out[k + 2].to_asm() == "\tora\ttmp1")
            {
                out[k + 1] = codew::casm("\tora\t#0", out[k + 1].linenum());
                out.erase(out.begin() + k + 2);
            }
        asm_peephole();
        // When the division with the last result kept (for a division and a
        // modulo of the same operands) is linked anyway, use it also for the
        // single divisions, instead of linking both.
        if(helpers.count("nat_divc") || helpers.count("nat_modc") || helpers.count("nat_divcb") ||
           helpers.count("nat_modcb"))
        {
            static const std::map<std::string, std::string> cached = {
                {"nat_div", "nat_divc"}, {"nat_mod", "nat_modc"},
                {"nat_divb", "nat_divcb"}, {"nat_modb", "nat_modcb"}};
            for(auto &c : out)
                if(c.is_asm())
                {
                    auto l = c.to_asm();
                    for(auto &m : cached)
                        if(l == "\tjsr\t" + m.first)
                        {
                            c = codew::casm("\tjsr\t" + m.second, c.linenum());
                            helpers.erase(m.first);
                            helpers.insert(m.second);
                        }
                }
        }
        if(used)
            code.swap(out);
        return used;
    }
};
} // namespace

bool do_native(std::vector<codew> &code, std::set<std::string> &helpers, bool all, bool lean)
{
    native_gen g(code);
    g.lean = lean && !all;
    if(all)
    {
        g.hot_ratio = 1e30;
        g.min_weight = 0;
    }
    bool r = g.run();
    if(getenv("EXBASIC_NATIVE_STATS"))
        g.print_stats();
    helpers = g.helpers;
    return r;
}
