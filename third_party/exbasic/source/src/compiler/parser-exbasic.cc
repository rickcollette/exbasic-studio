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

// parser-exbasic.cc: exBASIC language features.
//
// FUNC
// ----
//   FUNC Name[$|%]( [param[$|%], ...] ) [result[$|%], ...]
//     LOCAL var[$|%], ...
//     RETURN [value, ...]
//   ENDFUNC
//
// Parameters, named results and LOCAL variables are private variables of the
// function (stored with a key that can't be a BASIC name).  If a function can
// be called recursively (found from the call graph after parsing), its
// private variables are saved on entry and restored on exit; other functions
// don't pay that cost.
//
// Calling convention:
//  - integer and string arguments are pushed in the interpreter stack, the
//    floating point arguments in the floating point stack.  The function pops
//    them into the parameters.
//  - results are stored in "result slots", private variables of the function
//    that are not saved/restored; the caller reads them right after the call.
//
// The function signatures are registered by a pre-scan of the source, so
// functions can be called before they are defined.

#include "parser-exbasic.h"
#include "parser.h"
#include <functional>

// Variable keys of FUNC private variables: "0<len>_<func>_<name>".  The key
// starts with a digit, so it can't be a BASIC name, and the length prefix
// makes it unique.
std::string exb_local_key(const std::string &func, const std::string &name)
{
    return "0" + std::to_string(func.size()) + "_" + func + "_" + name;
}

// Key of a typed name: the name with its type suffix
static std::string vkey(const parse::typed_name &t);

// Result slot "n" of a function
static std::string slot_key(const std::string &func, size_t n)
{
    return exb_local_key(func, std::to_string(n) + "R");
}

std::string exb_var_key(parse &s, const std::string &name)
{
    if(!s.cur_func.empty())
    {
        auto k = exb_local_key(s.cur_func, name);
        if(s.vars.count(k))
            return k;
    }
    return name;
}

static std::string type_suffix(VarType t)
{
    if(t == VT_STRING)
        return "$";
    if(t == VT_FLOAT)
        return "%";
    return "";
}

static std::string vkey(const parse::typed_name &t)
{
    return t.name + type_suffix(t.type);
}

static std::string type_name(VarType t)
{
    if(t == VT_STRING)
        return "string";
    if(t == VT_FLOAT)
        return "floating point";
    return "integer";
}

static VarType parse_suffix(parse &s)
{
    if(s.peek('$'))
    {
        s.pos++;
        return VT_STRING;
    }
    if(s.peek('%'))
    {
        s.pos++;
        return VT_FLOAT;
    }
    return VT_WORD;
}

static bool at_statement_end(parse &s)
{
    s.skipws();
    if(s.eos() || s.peek(':') || s.peek('\''))
        return true;
    char c = s.str[s.pos];
    return c == '\n' || c == '\r' || c == '\x9B';
}

// Creates a variable with the given key and type, if not already existing
static void create_var(parse &s, const std::string &key, VarType type)
{
    auto &v = s.vars;
    if(v.count(key))
        return;
    int num = v.size();
    v[key] = (num << 8) + type;
    // Floating point variables use three variable slots
    if(type == VT_FLOAT)
    {
        v["-fake-" + std::to_string(v.size())] = 0;
        v["-fake-" + std::to_string(v.size())] = 0;
    }
}

static VarType var_type(parse &s, const std::string &key)
{
    return VarType(s.vars[key] & 0xFF);
}

// Name of the FUNC being parsed, empty in the main code
static std::string current_routine(parse &s)
{
    if(!s.cur_func.empty())
        return s.cur_func;
    for(auto i = s.block_stack.rbegin(); i != s.block_stack.rend(); ++i)
    {
        auto it = s.block_names.find(*i);
        if(it != s.block_names.end())
            return it->second;
    }
    return std::string();
}

static void note_call(parse &s, const std::string &callee)
{
    s.call_graph[current_routine(s)].insert(callee);
}

// Parses a list of "name[$|%]" separated by commas. Returns false if there
// is no name.
static bool parse_name_list(parse &s, std::vector<parse::typed_name> &list,
                            std::string &text)
{
    while(1)
    {
        std::string name;
        if(!s.get_ident(name))
            return false;
        auto t = parse_suffix(s);
        list.push_back({name, t});
        text += name + type_suffix(t);
        s.skipws();
        if(!s.peek(','))
            return true;
        s.pos++;
        text += ", ";
    }
}

// Parses a FUNC header, after the FUNC keyword
static std::string parse_header(parse &s, parse::func_info &f, std::string &text)
{
    if(!s.get_ident(f.name))
        return "expected FUNC name";
    f.type = parse_suffix(s);
    text = f.name + type_suffix(f.type) + "(";
    s.skipws();
    if(!s.peek('('))
        return "expected '(' after FUNC name";
    s.pos++;
    s.skipws();
    if(s.peek(')'))
        s.pos++;
    else
    {
        if(!parse_name_list(s, f.params, text))
            return "expected parameter name";
        s.skipws();
        if(!s.peek(')'))
            return "expected ')' after parameters";
        s.pos++;
    }
    text += ")";
    if(!at_statement_end(s))
    {
        text += " ";
        if(!parse_name_list(s, f.results, text))
            return "expected result name";
        f.named_results = true;
        if(!at_statement_end(s))
            return "expected end of FUNC definition";
    }
    return std::string();
}

std::string exb_register_func(parse &s, int line, bool returns_value)
{
    parse::func_info f;
    std::string text;
    auto err = parse_header(s, f, text);
    if(err.size())
        return err;
    f.line = line;
    if(s.funcs.count(f.name))
        return "FUNC '" + f.name + "' already defined at line " +
               std::to_string(s.funcs[f.name].line);
    // Check repeated names
    std::set<std::string> names;
    for(auto &p : f.params)
        if(!names.insert(p.name).second)
            return "repeated parameter name '" + p.name + "'";
    for(auto &r : f.results)
        if(!names.insert(r.name).second)
            return "repeated result name '" + r.name + "'";
    if(f.named_results)
    {
        if(f.results[0].type != f.type)
            return "FUNC name '" + f.name + type_suffix(f.type) +
                   "' must have the type of its first result '" + f.results[0].name +
                   type_suffix(f.results[0].type) + "'";
    }
    else if(returns_value)
        f.results.push_back({std::string(), f.type});
    else if(f.type != VT_WORD)
        return "FUNC '" + f.name + type_suffix(f.type) + "' must return a value";
    // Create private variables and result slots
    for(auto &p : f.params)
    {
        auto k = exb_local_key(f.name, vkey(p));
        create_var(s, k, p.type);
        f.locals.push_back(k);
    }
    if(f.named_results)
        for(auto &r : f.results)
        {
            auto k = exb_local_key(f.name, vkey(r));
            create_var(s, k, r.type);
            f.locals.push_back(k);
        }
    for(size_t i = 0; i < f.results.size(); i++)
        create_var(s, slot_key(f.name, i), f.results[i].type);
    s.funcs[f.name] = f;
    return std::string();
}

// Emits code to set a variable to zero / empty string
static void emit_zero(parse &s, const std::string &key, VarType t)
{
    if(t == VT_FLOAT)
    {
        s.emit_tok("TOK_VAR_SADDR");
        s.emit_varn(key);
        s.emit_tok("TOK_FLOAT");
        s.emit_fp(atari_fp(0.0));
        s.emit_tok("TOK_FP_STORE");
    }
    else if(t == VT_STRING)
    {
        s.emit_tok("TOK_VAR_SADDR");
        s.emit_varn(key);
        s.emit_tok("TOK_CSTRING");
        s.emit_str(std::string());
        s.emit_tok("TOK_COPY_STR");
    }
    else
    {
        s.emit_tok("TOK_VAR_STORE_0");
        s.emit_varn(key);
    }
}

// Emits code to copy variable "src" to "dst", both of type t.
static void emit_copy(parse &s, const std::string &dst, const std::string &src, VarType t)
{
    if(t == VT_FLOAT)
    {
        s.emit_tok("TOK_VAR_SADDR");
        s.emit_varn(dst);
        s.emit_tok("TOK_VAR_ADDR");
        s.emit_varn(src);
        s.emit_tok("TOK_FP_LOAD");
        s.emit_tok("TOK_FP_STORE");
    }
    else if(t == VT_STRING)
    {
        s.emit_tok("TOK_VAR_SADDR");
        s.emit_varn(dst);
        s.emit_tok("TOK_VAR_LOAD");
        s.emit_varn(src);
        s.emit_tok("TOK_COPY_STR");
    }
    else
    {
        s.emit_tok("TOK_VAR_LOAD");
        s.emit_varn(src);
        s.emit_tok("TOK_VAR_STORE");
        s.emit_varn(dst);
    }
}

static bool inside_routine(parse &s)
{
    for(auto &j : s.jumps)
        if(j.type == LT_FUNC)
            return true;
    return false;
}

// Keywords must be followed by a space, so "LOCALX = 1" is an assignment
static bool space_after_keyword(parse &s)
{
    return s.pos < s.str.size() && (s.str[s.pos] == ' ' || s.str[s.pos] == '\t');
}

bool exb_E_FUNC_DEF(parse &s)
{
    s.debug("E_FUNC_DEF");
    if(!space_after_keyword(s))
        return false;
    if(inside_routine(s))
        throw parse_error("FUNC can't be inside another FUNC", s.pos);
    parse::func_info hdr;
    std::string text;
    auto err = parse_header(s, hdr, text);
    if(err.size())
        return s.loop_error(err);
    auto it = s.funcs.find(hdr.name);
    if(it == s.funcs.end())
        return s.loop_error("FUNC definition must be in its own line");
    auto &f = it->second;
    if(f.defined)
        return s.loop_error("FUNC '" + f.name + "' already defined");
    auto &lbl = s.labels[f.name];
    if(lbl.is_defined())
        return s.loop_error("FUNC name '" + f.name + "' already used by DATA");
    lbl.define();
    s.add_text(text);

    // Move function code out of the main code
    auto l = s.push_loop(LT_BLOCK);
    s.push_block(l);
    s.block_names[l] = f.name;
    s.emit_label(s.label_prefix + f.name);
    s.cur_func_loop = s.push_loop(LT_FUNC);
    s.cur_func = f.name;
    f.defined = true;

    // Saves the private variables if the function is recursive, resolved
    // after parsing by exb_finalize.
    s.emit_tok("TOK__FRAME_SAVE");
    s.emit_word("fb_frame_" + f.name);

    // Pop parameters, in reverse order
    for(auto p = f.params.rbegin(); p != f.params.rend(); ++p)
    {
        auto k = exb_local_key(f.name, vkey(*p));
        if(p->type == VT_FLOAT)
        {
            s.emit_tok("TOK_VAR_SADDR");
            s.emit_varn(k);
            s.emit_tok("TOK_FP_STORE");
        }
        else if(p->type == VT_STRING)
        {
            s.emit_tok("TOK_VAR_SADDR");
            s.emit_varn(k);
            s.emit_tok("TOK_POP");
            s.emit_tok("TOK_COPY_STR");
        }
        else
        {
            s.emit_tok("TOK_POP");
            s.emit_tok("TOK_VAR_STORE");
            s.emit_varn(k);
        }
    }
    // Named results start as zero
    if(f.named_results)
        for(auto &r : f.results)
            emit_zero(s, exb_local_key(f.name, vkey(r)), r.type);
    return true;
}

bool exb_E_FUNC_LOCAL(parse &s)
{
    s.debug("E_FUNC_LOCAL");
    if(!space_after_keyword(s))
        return false;
    if(s.cur_func.empty())
        throw parse_error("LOCAL can only be used inside a FUNC", s.pos);
    std::vector<parse::typed_name> list;
    std::string text;
    if(!parse_name_list(s, list, text))
        return false;
    auto &f = s.funcs[s.cur_func];
    for(auto &v : list)
    {
        auto k = exb_local_key(f.name, vkey(v));
        if(s.vars.count(k))
            return s.loop_error("'" + v.name + "' is already a local variable");
        create_var(s, k, v.type);
        f.locals.push_back(k);
        emit_zero(s, k, v.type);
    }
    s.add_text(text);
    return true;
}

// Emits a FOR_EXIT for each FOR loop open inside the current FUNC
static void emit_for_exits(parse &s)
{
    for(auto i = s.jumps.rbegin(); i != s.jumps.rend(); ++i)
    {
        if(i->type == LT_FUNC)
            break;
        if(i->type == LT_FOR_1)
            s.emit_tok("TOK_FOR_EXIT");
    }
}

bool exb_E_FUNC_END(parse &s)
{
    s.debug("E_FUNC_END");
    if(s.cur_func.empty())
        return s.loop_error("ENDFUNC without FUNC");
    auto &f = s.funcs[s.cur_func];
    auto l = s.pop_loop(LT_FUNC);
    if(l.empty())
        return false;
    // Exit point: copy named results to the result slots
    s.emit_label(l + "_x");
    if(f.named_results)
        for(size_t i = 0; i < f.results.size(); i++)
            emit_copy(s, slot_key(f.name, i), exb_local_key(f.name, vkey(f.results[i])),
                      f.results[i].type);
    // Return point: restore private variables and return
    s.emit_label(l + "_r");
    s.emit_tok("TOK__FRAME_LOAD");
    s.emit_word("fb_frame_" + f.name);
    s.emit_tok("TOK_RET");
    auto p = s.pop_loop(LT_BLOCK);
    if(p.empty())
        return false;
    s.pop_block(p);
    s.cur_func.clear();
    return true;
}

bool exb_E_IN_FUNC(parse &s)
{
    if(s.cur_func.empty())
        throw parse_error("RETURN outside FUNC", s.pos);
    s.ret_vals.clear();
    return true;
}

// Removes values that were discarded by the parser backtracking
static void heal_ret(parse &s)
{
    while(!s.ret_vals.empty() && s.ret_vals.back().second > s.code->size())
        s.ret_vals.pop_back();
}

static bool ret_is(parse &s, VarType t)
{
    if(s.cur_func.empty())
        return false;
    heal_ret(s);
    auto &f = s.funcs[s.cur_func];
    auto i = s.ret_vals.size();
    return i < f.results.size() && f.results[i].type == t;
}

bool exb_E_RET_INT(parse &s) { return ret_is(s, VT_WORD); }
bool exb_E_RET_STR(parse &s) { return ret_is(s, VT_STRING); }
bool exb_E_RET_FP(parse &s) { return ret_is(s, VT_FLOAT); }

static bool ret_value(parse &s, VarType t)
{
    heal_ret(s);
    if(t != VT_FLOAT)
        s.emit_tok("TOK_PUSH");
    s.ret_vals.push_back({t, s.code->size()});
    return true;
}

bool exb_E_RET_VI(parse &s) { return ret_value(s, VT_WORD); }
bool exb_E_RET_VS(parse &s) { return ret_value(s, VT_STRING); }
bool exb_E_RET_VF(parse &s) { return ret_value(s, VT_FLOAT); }

bool exb_E_FUNC_RETURN(parse &s)
{
    s.debug("E_FUNC_RETURN");
    heal_ret(s);
    auto &f = s.funcs[s.cur_func];
    auto n = s.ret_vals.size();
    if(!at_statement_end(s))
    {
        if(n < f.results.size())
            throw parse_error("RETURN value " + std::to_string(n + 1) + " must be " +
                                  type_name(f.results[n].type),
                              s.pos);
        throw parse_error("FUNC '" + f.name + "' returns " +
                              std::to_string(f.results.size()) + " value(s)",
                          s.pos);
    }
    if(n == 0)
    {
        if(!f.named_results && !f.results.empty())
            throw parse_error("RETURN needs a value in FUNC '" + f.name +
                                  type_suffix(f.type) + "'",
                              s.pos);
        emit_for_exits(s);
        s.emit_tok("TOK_JUMP");
        s.emit_word(s.cur_func_loop + "_x");
        return true;
    }
    if(n != f.results.size())
        throw parse_error("FUNC '" + f.name + "' returns " +
                              std::to_string(f.results.size()) + " value(s)",
                          s.pos);
    // Values are in the stacks, store into the result slots in reverse order
    for(size_t i = n; i-- > 0;)
    {
        auto k = slot_key(f.name, i);
        if(f.results[i].type == VT_FLOAT)
        {
            s.emit_tok("TOK_VAR_SADDR");
            s.emit_varn(k);
            s.emit_tok("TOK_FP_STORE");
        }
        else if(f.results[i].type == VT_STRING)
        {
            // Copy the text: the value can be a private string, which is
            // restored when the FUNC returns
            s.emit_tok("TOK_VAR_SADDR");
            s.emit_varn(k);
            s.emit_tok("TOK_POP");
            s.emit_tok("TOK_COPY_STR");
        }
        else
        {
            s.emit_tok("TOK_POP");
            s.emit_tok("TOK_VAR_STORE");
            s.emit_varn(k);
        }
    }
    emit_for_exits(s);
    s.emit_tok("TOK_JUMP");
    s.emit_word(s.cur_func_loop + "_r");
    return true;
}

// Removes call contexts that were discarded by the parser backtracking: a
// live call always starts before the current position.
static void heal_calls(parse &s)
{
    while(!s.fcalls.empty() && s.fcalls.back().start >= s.pos)
        s.fcalls.pop_back();
}

static size_t live_args(parse &s, parse::fcall_ctx &c)
{
    while(!c.args.empty() && c.args.back() > s.code->size())
        c.args.pop_back();
    return c.args.size();
}

static bool fcall_start(parse &s, parse::fcall_kind kind)
{
    heal_calls(s);
    s.skipws();
    auto start = s.pos;
    std::string name;
    if(!s.get_ident(name))
        return false;
    auto it = s.funcs.find(name);
    if(it == s.funcs.end())
        return false;
    auto &f = it->second;
    VarType t;
    switch(kind)
    {
    case parse::FC_INT:
        if(s.peek('$') || s.peek('%'))
            return false;
        t = VT_WORD;
        break;
    case parse::FC_STRING:
        if(!s.peek('$'))
            return false;
        s.pos++;
        t = VT_STRING;
        break;
    case parse::FC_FLOAT:
        if(!s.peek('%'))
            return false;
        s.pos++;
        t = VT_FLOAT;
        break;
    default:
        t = parse_suffix(s);
        break;
    }
    if(t != f.type)
        return false;
    s.skipws();
    if(!s.peek('('))
        return false;
    if(kind == parse::FC_INT || kind == parse::FC_STRING || kind == parse::FC_FLOAT)
    {
        // A FUNC name can only be a function call, so these are always errors
        if(f.results.empty())
            throw parse_error("FUNC '" + name + "' does not return a value", start);
        if(f.results.size() > 1)
            throw parse_error("FUNC '" + name + "' returns " +
                                  std::to_string(f.results.size()) +
                                  " values, use 'A, B = " + name + "(...)'",
                              start);
        s.emit_tok("TOK_PUSH_SADDR");
    }
    else if(kind == parse::FC_MULTI)
    {
        if(f.results.size() != s.multi_targets.size())
            return s.loop_error("FUNC '" + name + "' returns " +
                                std::to_string(f.results.size()) + " value(s), but there are " +
                                std::to_string(s.multi_targets.size()) + " variables");
    }
    s.fcalls.push_back({name, kind, start, {}});
    s.add_text(name + type_suffix(t));
    return true;
}

bool exb_E_FCALL_I(parse &s) { return fcall_start(s, parse::FC_INT); }
bool exb_E_FCALL_S(parse &s) { return fcall_start(s, parse::FC_STRING); }
bool exb_E_FCALL_F(parse &s) { return fcall_start(s, parse::FC_FLOAT); }
bool exb_E_FCALL_STMT(parse &s) { return fcall_start(s, parse::FC_STATEMENT); }
bool exb_E_FCALL_MULTI(parse &s) { return fcall_start(s, parse::FC_MULTI); }

static bool arg_is(parse &s, VarType t)
{
    heal_calls(s);
    if(s.fcalls.empty())
        return false;
    auto &c = s.fcalls.back();
    auto &f = s.funcs[c.name];
    auto i = live_args(s, c);
    if(i >= f.params.size())
        return s.loop_error("FUNC '" + f.name + type_suffix(f.type) + "' needs " +
                            std::to_string(f.params.size()) + " argument(s)");
    if(f.params[i].type != t)
        return s.error(type_name(f.params[i].type) + " argument for '" +
                       f.params[i].name + type_suffix(f.params[i].type) + "'");
    return true;
}

bool exb_E_ARG_INT(parse &s) { return arg_is(s, VT_WORD); }
bool exb_E_ARG_STR(parse &s) { return arg_is(s, VT_STRING); }
bool exb_E_ARG_FP(parse &s) { return arg_is(s, VT_FLOAT); }

// Called when an argument could not be parsed with the parameter type
bool exb_E_ARG_BAD(parse &s)
{
    heal_calls(s);
    s.skipws();
    if(s.fcalls.empty() || s.peek(')'))
        return false;
    auto &c = s.fcalls.back();
    auto &f = s.funcs[c.name];
    auto i = live_args(s, c);
    if(i >= f.params.size())
        return false;
    throw parse_error("argument " + std::to_string(i + 1) + " of '" + f.name +
                          type_suffix(f.type) + "' must be " + type_name(f.params[i].type),
                      s.pos);
}

bool exb_E_ARG_NEXT(parse &s)
{
    heal_calls(s);
    if(s.fcalls.empty())
        return false;
    auto &c = s.fcalls.back();
    live_args(s, c);
    c.args.push_back(s.code->size());
    return true;
}

bool exb_E_FCALL_END(parse &s)
{
    s.debug("E_FCALL_END");
    heal_calls(s);
    if(s.fcalls.empty())
        return false;
    auto c = s.fcalls.back();
    auto &f = s.funcs[c.name];
    auto n = live_args(s, c);
    if(n != f.params.size())
        throw parse_error("FUNC '" + f.name + type_suffix(f.type) + "' needs " +
                              std::to_string(f.params.size()) + " argument(s), got " +
                              std::to_string(n),
                          s.pos);
    s.emit_tok("TOK_CALL");
    s.emit_word(s.label_prefix + f.name);
    note_call(s, f.name);
    switch(c.kind)
    {
    case parse::FC_INT:
    case parse::FC_STRING:
        s.emit_tok("TOK_VAR_LOAD");
        s.emit_varn(slot_key(f.name, 0));
        s.emit_tok("TOK_POP_SADDR");
        break;
    case parse::FC_FLOAT:
        s.emit_tok("TOK_VAR_ADDR");
        s.emit_varn(slot_key(f.name, 0));
        s.emit_tok("TOK_FP_LOAD");
        s.emit_tok("TOK_POP_SADDR");
        break;
    case parse::FC_STATEMENT:
        break;
    case parse::FC_MULTI:
        for(size_t i = 0; i < s.multi_targets.size(); i++)
        {
            auto &t = s.multi_targets[i];
            if(t.name == "_")
                continue;
            auto rtype = f.results[i].type;
            if(t.type != rtype)
                return s.loop_error("result " + std::to_string(i + 1) + " of '" + f.name +
                                    "' is " + type_name(rtype) + ", but '" + t.name +
                                    type_suffix(t.type) + "' is " + type_name(t.type));
            auto k = exb_var_key(s, vkey(t));
            if(s.funcs.count(k))
                return s.loop_error("'" + k + "' is a FUNC");
            if(!s.vars.count(k))
                create_var(s, k, t.type);
            else if(var_type(s, k) != t.type)
                return s.loop_error("variable '" + t.name + "' is not " + type_name(t.type));
            emit_copy(s, k, slot_key(f.name, i), t.type);
        }
        break;
    }
    s.fcalls.pop_back();
    return true;
}

// Reports calls to functions that are not defined: an identifier followed
// by '(' that is not a function, a variable or an array.
bool exb_E_FUNC_UNKNOWN(parse &s)
{
    std::string name;
    if(!s.get_ident(name))
        return false;
    s.skipws();
    if(!s.peek('(') || s.funcs.count(name) || s.vars.count(exb_var_key(s, name)) ||
       s.labels.count(name))
        return false;
    return s.loop_error("FUNC '" + name + "' is not defined");
}

bool exb_E_MULTI_TARGETS(parse &s)
{
    s.debug("E_MULTI_TARGETS");
    std::vector<parse::typed_name> list;
    std::string text;
    if(!parse_name_list(s, list, text) || list.size() < 2)
        return false;
    s.multi_targets = list;
    s.add_text(text);
    return true;
}

bool exb_E_LOOP_EXIT(parse &s)
{
    if(s.jumps.empty())
        return false;
    return s.emit_word(s.jumps.back().label + "_x");
}

bool exb_E_EMPTY_STRING(parse &s)
{
    s.emit_tok("TOK_CSTRING");
    return s.emit_str(std::string());
}

bool exb_E_COMMON(parse &s)
{
    s.debug("E_COMMON");
    if(!space_after_keyword(s))
        return false;
    if(s.has_common)
        throw parse_error("only one COMMON statement is allowed", s.pos);
    if(inside_routine(s))
        throw parse_error("COMMON can't be inside a FUNC", s.pos);
    std::vector<parse::typed_name> list;
    std::string text;
    if(!parse_name_list(s, list, text))
        return false;
    for(auto &v : list)
    {
        if(s.funcs.count(v.name))
            throw parse_error("'" + v.name + "' is a FUNC, it can't be in COMMON", s.pos);
        if(!s.vars.count(vkey(v)))
            create_var(s, vkey(v), v.type);
        else if(var_type(s, vkey(v)) != v.type)
            throw parse_error("variable '" + v.name + "' is not " + type_name(v.type), s.pos);
    }
    s.common_vars = list;
    s.has_common = true;
    s.add_text(text);
    s.emit_tok("TOK_NUM");
    s.emit_word("DO_COMMON_LOAD");
    s.emit_tok("TOK_USR_ADDR");
    s.emit_tok("TOK_NUM");
    s.emit_word("fb_common_tbl");
    s.emit_tok("TOK_USR_CALL");
    return true;
}

bool exb_E_RUN_CALL(parse &s)
{
    s.uses_run = true;
    s.emit_tok("TOK_NUM");
    s.emit_word("DO_RUN");
    s.emit_tok("TOK_USR_ADDR");
    s.emit_tok("TOK_NUM");
    s.emit_word("fb_common_tbl");
    s.emit_tok("TOK_USR_CALL");
    return true;
}

// Returns the set of routines that can call themselves
static std::set<std::string> recursive_routines(parse &s)
{
    std::set<std::string> ret;
    for(auto &f : s.funcs)
    {
        // Depth first search from the callees of f
        std::set<std::string> seen;
        std::vector<std::string> todo;
        for(auto &c : s.call_graph[f.first])
            todo.push_back(c);
        while(!todo.empty())
        {
            auto n = todo.back();
            todo.pop_back();
            if(n == f.first)
            {
                ret.insert(f.first);
                break;
            }
            if(!seen.insert(n).second)
                continue;
            for(auto &c : s.call_graph[n])
                todo.push_back(c);
        }
    }
    return ret;
}

std::string exb_finalize(parse &s, std::vector<codew> &code)
{
    auto rec = recursive_routines(s);
    const std::string prefix = "fb_frame_";
    for(size_t i = 0; i + 1 < code.size(); i++)
    {
        bool save = code[i].is_tok("TOK__FRAME_SAVE");
        if(!save && !code[i].is_tok("TOK__FRAME_LOAD"))
            continue;
        auto name = code[i + 1].get_str().substr(prefix.size());
        int line = code[i].linenum();
        code.erase(code.begin() + i, code.begin() + i + 2);
        auto &f = s.funcs[name];
        if(!rec.count(name) || f.locals.empty())
        {
            i--;
            continue;
        }
        if(f.locals.size() > 255)
            return "too many local variables in FUNC '" + name + "'";
        std::vector<codew> ins;
        ins.push_back(codew::ctok(save ? "TOK_FRAME_SAVE" : "TOK_FRAME_LOAD", line));
        ins.push_back(codew::cbyte(f.locals.size(), line));
        auto add = [&](const std::string &k) {
            ins.push_back(codew::cvarn(k, s.vars[k] >> 8, line));
            // Size code: 1 for 2 bytes, 3 for 6 bytes (floating point) and
            // $80 for the contents of a string
            auto t = var_type(s, k);
            ins.push_back(codew::cbyte(t == VT_FLOAT ? 3 : t == VT_STRING ? 0x80 : 1, line));
        };
        if(save)
            for(auto &k : f.locals)
                add(k);
        else
            for(auto k = f.locals.rbegin(); k != f.locals.rend(); ++k)
                add(*k);
        code.insert(code.begin() + i, ins.begin(), ins.end());
        i += ins.size() - 1;
    }
    return std::string();
}

std::string exb_common_table(parse &s)
{
    if(!s.has_common && !s.uses_run)
        return std::string();
    // Signature, to check that both programs declare the same COMMON
    unsigned sig = 0x5A17 + s.common_vars.size();
    std::string vars;
    for(auto &v : s.common_vars)
    {
        int t = v.type == VT_FLOAT ? 1 : v.type == VT_STRING ? 2 : 0;
        sig = (sig * 31 + t + 1) & 0xFFFF;
        vars += "\t.byte\t" + std::to_string(t) + "\n\t.word\t" + var_symbol(v.name + type_suffix(v.type)) + "\n";
    }
    return "; COMMON variables table\nfb_common_tbl:\n\t.word\t" + std::to_string(sig) +
           "\n\t.byte\t" + std::to_string(s.common_vars.size()) + "\n" + vars;
}

// String concatenation: "A$ + B$ + C$" compiles to
//      A$ TSTR_SET t  B$ TSTR_ADD t  C$ TSTR_ADD t
// where "t" is a hidden string variable.  Each chain in a statement uses its
// own variable, as the results of two chains can be in use at the same time
// (as in "IF A$ + B$ = C$ + D$"); the variables are reused in the next
// statement.  The main code and each FUNC have their own variables, so a
// call in a chain does not overwrite the result being built.
static std::string cat_key(parse &s, size_t k)
{
    return exb_local_key(s.cur_func, std::to_string(k) + "T");
}

// Removes the chains discarded by parser backtracking
static void heal_cat(parse &s)
{
    auto &c = *s.code;
    while(!s.cat_chains.empty())
    {
        size_t k = s.cat_chains.size() - 1, p = s.cat_chains[k];
        if(p + 1 < c.size() && c[p].is_tok("TOK_TSTR_SET") && c[p + 1].is_varn() &&
           s.vars.count(cat_key(s, k)) &&
           c[p + 1].get_varn() == (s.vars[cat_key(s, k)] >> 8))
            break;
        s.cat_chains.pop_back();
    }
    while(!s.cat_open.empty() && s.cat_open.back() >= s.cat_chains.size())
        s.cat_open.pop_back();
}

bool exb_E_CAT_SET(parse &s)
{
    heal_cat(s);
    size_t k = s.cat_chains.size();
    auto key = cat_key(s, k);
    // In a FUNC, the hidden strings are saved in recursive calls as its
    // local variables
    if(!s.vars.count(key) && !s.cur_func.empty() && s.funcs.count(s.cur_func))
        s.funcs[s.cur_func].locals.push_back(key);
    create_var(s, key, VT_STRING);
    s.cat_chains.push_back(s.code->size());
    s.cat_open.push_back(k);
    s.emit_tok("TOK_TSTR_SET");
    s.emit_varn(key);
    return true;
}

bool exb_E_CAT_ADD(parse &s)
{
    heal_cat(s);
    if(s.cat_open.empty())
        return false;
    s.emit_tok("TOK_TSTR_ADD");
    s.emit_varn(cat_key(s, s.cat_open.back()));
    return true;
}

bool exb_E_CAT_END(parse &s)
{
    heal_cat(s);
    if(!s.cat_open.empty())
        s.cat_open.pop_back();
    return true;
}
