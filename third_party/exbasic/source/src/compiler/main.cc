/*
 * FastBasic - Fast basic interpreter for the Atari 8-bit computers
 * Copyright (C) 2017-2026 Daniel Serpell
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

// main.cc: Main compiler file

#include "compile.h"
#include "os.h"
#include "target.h"
#include <algorithm>
#include <fstream>
#include <iostream>
#include <tuple>
#include <vector>

static int show_version()
{
    std::cerr << "exBASIC " VERSION " - based on FastBasic by Daniel Serpell (dmsc)\n";
    return 0;
}

static int show_help()
{
    show_version();
    std::cerr << "Usage: exbasic [options] <input.bas> [<file.asm>...]\n"
                 "\n"
                 "Options:\n"
                 " -d\t\tenable parser debug options (only useful to debug parser)\n"
                 " -n\t\tdon't run the optimizer\n"
                 " -I:<path>\tadd path to search INCLUDE files\n"
                 " -no-ext\tdon't use the extended (faster) instructions, gives the\n"
                 "\t\tsmallest code\n"
                 " -no-native\tdon't compile to machine code, gives smaller but slower\n"
                 "\t\tprograms\n"
                 " -native:hot\tcompile to machine code only the code that runs often\n"
                 " -native:all\tcompile all the code possible to machine code, gives the\n"
                 "\t\tfastest programs\n"
                 " -prof\t\tshow token usage statistics\n"
                 " -s:<name>\tplace code into given segment\n"
                 " -t:<target>\tselect compiler target ('atari' or 'atari-cart')\n"
                 " -l\t\twrite a long BASIC listing of the parsed source\n"
                 " -l:<extension>\tspecify the extension of the BASIC listing\n"
                 " -ls:<num>\twrite a shortened/abbreviated BASIC listing with num columns\n"
                 " -c\t\tonly compile to assembler, don't produce binary\n"
                 " -keep\t\tkeep intermediate files on compilation\n"
                 " -g\t\tsave listing and label files after compilation\n"
                 " -C:<name>\tselect linker config file name\n"
                 " -S:<addr>\tselect binary starting address\n"
                 " -X:<opt>\tpass option to the assembler\n"
                 " -DL:<sym=val>\tdefine linker symbol with given value\n"
                 " -o <name>\tselect output file name\n"
                 " -v\t\tshow version and exit\n"
                 " -h\t\tshow this help\n"
                 "\n"
                 "You can pass one basic file and several assembly and object files to be "
                 "linked together;\nadd other basic files with INCLUDE.\n";
    return 0;
}

static int show_error(std::string msg)
{
    std::cerr << "exbasic: " << msg << "\n";
    return 1;
}

static std::string to_lower(std::string in)
{
    auto ret = in;
    for(auto &c: ret)
        c = std::tolower(c);
    return ret;
}

// Parse path list into a vector of strings
static std::vector<std::string> parse_path_list(const std::string &str)
{
    std::vector<std::string> ret;
    size_t pos = 0;
    while(1)
    {
        auto nxt = str.find(':', pos);
        if(nxt == str.npos)
        {
            ret.emplace_back(str.substr(pos));
            return ret;
        }
        ret.emplace_back(str.substr(pos, nxt - pos));
        pos = nxt + 1;
    }
}

int main(int argc, char **argv)
{
    // OS specific initializations
    os::init(argv[0]);

    // Default folders for target and syntax files
    auto syntax_folder = os::get_search_path("syntax");
    auto target_folder = os::get_search_path("");
    std::vector<std::string> args(argv + 1, argv + argc);
    std::string out_name;
    std::string exe_name;
    bool got_outname = false, one_step = false, next_is_output = false;
    int native_mode = 0; // 0 = smallest of hot, lean and all, 1 = hot, 2 = all
    bool keep_temps = false, do_listing = false;
    std::string target_name = "default";
    std::string cfg_file_def;
    std::string listing_ext = ".list";
    compiler comp;
    std::vector<std::string> link_opts;
    std::vector<std::string> asm_opts = {"-g"};
    // BAS files, compile INPUT(BAS) to OUTPUT(ASM)
    std::vector<std::tuple<std::string, std::string>> bas_files;
    // ASM files, assemble INPUT(ASM) to OUTPUT(OBJ) producing a listing
    std::vector<std::tuple<std::string, std::string>> asm_files;
    // OBJ files, link INPUT(OBJ) to output executable
    std::vector<std::string> link_files;
    // Temporary files to remove
    std::vector<std::string> temp_files;

    // Process command line options
    for(auto &arg : args)
    {
        // Process delayed options
        if(next_is_output)
        {
            out_name = arg;
            if(exe_name.empty())
                exe_name = out_name;
            next_is_output = false;
            continue;
        }
        // Get possible file extension:
        auto lext = os::get_extension_lower(arg);
        // Process options
        if(arg == "-d")
            comp.do_debug = true;
        else if(arg == "-n")
            comp.optimize = false;
        else if(arg == "-no-ext")
            comp.extended = false;
        else if(arg == "-no-native")
            comp.native = false;
        else if(arg == "-native:hot")
            native_mode = 1;
        else if(arg == "-native:all")
            native_mode = 2;
        else if(arg.rfind("-I:", 0) == 0 || arg.rfind("-I=", 0) == 0)
            comp.include_dirs.push_back(arg.substr(3));
        else if(arg == "-prof")
            comp.show_stats = true;
        else if(arg == "-v")
            return show_version();
        else if(arg == "-c")
        {
            one_step = true;
        }
        else if(arg == "-l")
            comp.show_text = true;
        else if(arg.rfind("-l:", 0) == 0 || arg.rfind("-l=", 0) == 0)
        {
            std::string ext = arg.substr(3);
            if(!ext.size() || to_lower(ext) == "bas")
                return show_error("invalid BASIC listing extension");
            comp.show_text = true;
            listing_ext = "." + arg.substr(3);
        }
        else if(arg == "-ls")
        {
            comp.show_text = true;
            comp.short_text = 120;
        }
        else if(arg.rfind("-ls:", 0) == 0 || arg.rfind("-ls=", 0) == 0)
        {
            size_t pos = 0;
            int len = -1;
            try {
                len = std::stoi(arg.substr(4), &pos, 0);
            }
            catch(...) { }
            if(pos != arg.size() - 4 || len < 1 || len > 256)
                return show_error("'-ls' option needs line length from 1 to 256");
            comp.show_text = true;
            comp.short_text = len;
        }
        else if(arg == "-h")
            return show_help();
        else if(arg == "-keep")
        {
            keep_temps = true;
            do_listing = true;
        }
        else if(arg == "-g")
            do_listing = true;
        else if(arg.empty())
            return show_error("invalid argument, try -h for help");
        else if(arg.rfind("-o", 0) == 0)
        {
            if(got_outname)
                return show_error("multiple '-o' option for the same file");
            got_outname = true;
            if(arg.size() > 2)
            {
                out_name = arg.substr(2);
                if(exe_name.empty())
                    exe_name = out_name;
            }
            else
                next_is_output = true;
        }
        else if(arg.rfind("-s:", 0) == 0 || arg.rfind("-s=", 0) == 0)
        {
            auto seg = arg.substr(3);
            if(!seg.size() || (seg.find('"') != std::string::npos))
                return show_error("invalid segment name");
            comp.segname = seg;
        }
        else if(arg.rfind("-t:", 0) == 0 || arg.rfind("-t=", 0) == 0)
        {
            auto tgt = arg.substr(3);
            if(!tgt.size() || (tgt.find('"') != std::string::npos))
                return show_error("invalid compiler target name");
            target_name = tgt;
        }
        else if(arg.rfind("-C:", 0) == 0 || arg.rfind("-C=", 0) == 0)
        {
            cfg_file_def = arg.substr(3);
        }
        else if(arg.rfind("-X:", 0) == 0 || arg.rfind("-X=", 0) == 0)
        {
            asm_opts.push_back(arg.substr(3));
        }
        else if(arg.rfind("-S:", 0) == 0 || arg.rfind("-S=", 0) == 0)
        {
            link_opts.push_back("--start-addr");
            link_opts.push_back(arg.substr(3));
        }
        else if(arg.rfind("-DL:", 0) == 0 || arg.rfind("-DL=", 0) == 0)
        {
            link_opts.push_back("--define");
            link_opts.push_back(arg.substr(4));
        }
        else if(arg.rfind("-syntax-path:", 0) == 0 || arg.rfind("-syntax-path=", 0) == 0)
        {
            syntax_folder = parse_path_list(arg.substr(13));
        }
        else if(arg.rfind("-target-path:", 0) == 0 || arg.rfind("-target-path=", 0) == 0)
        {
            target_folder = parse_path_list(arg.substr(13));
        }
        else if(arg[0] == '-')
            return show_error("invalid option '" + arg + "', try -h for help");
        else if(lext == "o" || lext == "obj")
        {
            // An object file, pass to the linker
            link_files.push_back(arg);
        }
        else if(lext == "s" || lext == "asm")
        {
            // An assembly file, pass to the assembler and linker
            std::string obj_name = os::add_extension(arg, ".o");
            if(got_outname)
            {
                obj_name = os::add_extension(out_name, ".o");
                got_outname = false;
            }

            asm_files.emplace_back(arg, obj_name);
            if(!one_step)
                link_files.push_back(obj_name);
        }
        else
        {
            // Other files are assumed to be BASIC sources
            std::string asm_name = os::add_extension(arg, ".asm");
            std::string obj_name = os::add_extension(arg, ".o");
            if(got_outname)
            {
                asm_name = os::add_extension(out_name, ".asm");
                obj_name = os::add_extension(out_name, ".o");
                got_outname = false;
            }

            bas_files.emplace_back(arg, asm_name);
            if(!one_step)
            {
                asm_files.emplace_back(asm_name, obj_name);
                link_files.push_back(obj_name);
            }
        }
    }
    if(!bas_files.size() && !asm_files.size() && !link_files.size())
        return show_error("missing input file name");
    if(next_is_output)
        return show_error("option '-o' must supply a file name");
    // A program is compiled as a whole, so FUNC calls and variables can be
    // checked: other BASIC files are added with INCLUDE.
    if(bas_files.size() > 1 && !one_step)
        return show_error("only one BASIC file can be linked into a program, '" +
                          std::get<0>(bas_files[1]) + "' is the second one; add it with "
                          "INCLUDE \"" + std::get<0>(bas_files[1]) + "\" instead");

    // Read target definition
    target tgt;

    try
    {
        tgt.load(target_folder, syntax_folder, target_name);
    }
    catch(std::exception &e)
    {
        std::cerr << e.what() << "\n";
        return 1;
    }
    std::string lib_name = os::compiler_path(tgt.lib());
    std::string cfg_file =
        cfg_file_def.size() ? cfg_file_def : os::compiler_path(tgt.cfg());
    asm_opts.insert(asm_opts.end(), tgt.ca65_args().begin(), tgt.ca65_args().end());

    // Guess final exe file name
    if(link_files.size() && exe_name.empty())
        exe_name = os::add_extension(link_files[0], tgt.bin_ext());

    // Compiles, assembles and links all files. Returns 0 on success, 1 on
    // compile or assemble errors and 2 on link errors.
    auto build = [&]() -> int {
        for(auto &f : bas_files)
        {
            auto bas_name = std::get<0>(f), asm_name = std::get<1>(f);
            auto listing_name = os::add_extension(bas_name, listing_ext);
            std::cerr << "BAS compile '" << bas_name << "' to '" << asm_name << "'\n";
            if(comp.show_text)
                std::cerr << "    with " << (comp.short_text ? "minimized" : "expanded")
                          << " listing to '" << listing_name << "'\n";
            auto e = comp.compile_file(bas_name, asm_name, tgt.sl(), listing_name);
            if(e)
                return 1;
            if(!one_step)
                temp_files.push_back(asm_name);
        }
        for(auto &f : asm_files)
        {
            auto asm_name = std::get<0>(f), obj_name = std::get<1>(f);
            auto lst_name = os::add_extension(obj_name, ".lst");

            std::cerr << "ASM assemble '" << asm_name << "' to '" << obj_name << "'\n";
            std::vector<std::string> args{
                "ca65", "-I", os::compiler_path("asminc"), "-o", obj_name};
            if(do_listing)
                args.insert(args.end(), {"-l", lst_name});
            for(auto &o : asm_opts)
                args.push_back(o);
            args.push_back(asm_name);
            auto e = os::prog_exec("ca65", args);
            if(e)
            {
                show_error("can't assemble file\n");
                return 1;
            }
            if(!one_step)
                temp_files.push_back(obj_name);
        }
        if(link_files.size())
        {
            //$LD65" -C "$CFGFILE" "$@" -o "$XEX" -Ln "$LBL" "$FB.lib"
            std::cerr << "LINK " << exe_name << "\n";
            std::vector<std::string> args{"ld65", "-C", cfg_file, "-o", exe_name};
            if(do_listing)
                args.insert(args.end(), {"-Ln", os::add_extension(exe_name, ".lbl")});
            for(auto &l : link_opts)
                args.push_back(l);
            for(auto &f : link_files)
                args.push_back(f);
            args.push_back(lib_name);
            auto e = os::prog_exec("ld65", args);
            if(e)
                return 2;
        }
        return 0;
    };

    // By default, build the program with several selections of machine
    // code (all the code possible, the code that runs often, and the code
    // that runs often less the loops where it saves little time), each with
    // and without the extended instructions, and keep the smallest program.
    // On a tie, the first one in this order (the faster one) is kept.
    auto file_size = [](const std::string &name) -> long {
        std::ifstream f(name, std::ios::binary | std::ios::ate);
        return f ? long(f.tellg()) : -1;
    };
    int e;
    if(native_mode == 0 && comp.native && comp.optimize && bas_files.size() && link_files.size())
    {
        struct cand
        {
            bool all, lean, ext;
        };
        std::vector<cand> cands;
        for(bool ext : {true, false})
        {
            if(ext && !comp.extended)
                continue;
            cands.push_back({true, false, ext});
            cands.push_back({false, false, ext});
            cands.push_back({false, true, ext});
        }
        auto set = [&](const cand &c) {
            comp.native_all = c.all;
            comp.native_lean = c.lean;
            comp.extended = c.ext;
        };
        std::vector<long> size(cands.size(), -1);
        int best = -1, last = -1;
        e = 0;
        for(size_t i = 0; i < cands.size(); i++)
        {
            set(cands[i]);
            int r = build();
            last = i;
            // A compile or assembly error is the same in all builds: report
            // it only once.
            if(r == 1)
            {
                e = 1;
                break;
            }
            size[i] = r ? -1 : file_size(exe_name);
            if(size[i] >= 0 && (best < 0 || size[i] < size[best]))
                best = i;
        }
        if(e != 1 && best < 0)
        {
            // No build linked: report the error of the first one
            set(cands[0]);
            e = build();
        }
        else if(e != 1)
        {
            if(best > 0)
            {
                auto &c = cands[best];
                std::cerr << "Using machine code "
                          << (c.all ? "for all the code" : c.lean ? "only where it saves time"
                                                                  : "only for the code that runs often")
                          << (c.ext || !cands[0].ext ? "" : ", without extended instructions")
                          << " (" << size[best] << " bytes instead of " << size[0] << ").\n";
            }
            set(cands[best]);
            e = best == last ? 0 : build();
        }
        comp.native_all = comp.native_lean = false;
    }
    else
    {
        comp.native_all = native_mode == 2;
        e = build();
    }
    // The extended instructions add to the number of instructions in the
    // program, so a very big program could go over the limit of 127.  Retry
    // without them before reporting the link error.
    if(e == 2 && comp.extended && comp.optimize && bas_files.size())
    {
        std::cerr << "LINK failed, retrying without extended instructions.\n";
        comp.extended = false;
        e = build();
    }
    if(e == 2 && comp.native && comp.optimize && bas_files.size())
    {
        std::cerr << "LINK failed, retrying without machine code.\n";
        comp.native = false;
        e = build();
    }
    if(e == 2)
        return show_error("can't link file\n");
    if(e)
        return e;

    // Remove all intermediate files
    if(!keep_temps)
        for(auto &name: temp_files)
            os::remove_file(name);

    return 0;
}
