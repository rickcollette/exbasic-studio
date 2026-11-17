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

// metadata.cc: machine-readable build metadata and feature detection.

#include "metadata.h"
#include "os.h"

#include <algorithm>
#include <cstdint>
#include <cstdio>
#include <fstream>
#include <map>
#include <set>
#include <sstream>

// ---- SHA-256 ----------------------------------------------------------------

namespace
{
class sha256
{
    uint32_t h[8];
    uint8_t buf[64];
    size_t len = 0;
    uint64_t total = 0;

    static uint32_t rotr(uint32_t x, int n) { return (x >> n) | (x << (32 - n)); }

    void block(const uint8_t *p)
    {
        static const uint32_t k[64] = {
            0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4,
            0xab1c5ed5, 0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe,
            0x9bdc06a7, 0xc19bf174, 0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f,
            0x4a7484aa, 0x5cb0a9dc, 0x76f988da, 0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
            0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967, 0x27b70a85, 0x2e1b2138, 0x4d2c6dfc,
            0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85, 0xa2bfe8a1, 0xa81a664b,
            0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070, 0x19a4c116,
            0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
            0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7,
            0xc67178f2};
        uint32_t w[64];
        for(int i = 0; i < 16; i++)
            w[i] = (uint32_t(p[4 * i]) << 24) | (uint32_t(p[4 * i + 1]) << 16) |
                   (uint32_t(p[4 * i + 2]) << 8) | uint32_t(p[4 * i + 3]);
        for(int i = 16; i < 64; i++)
        {
            uint32_t s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >> 3);
            uint32_t s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >> 10);
            w[i] = w[i - 16] + s0 + w[i - 7] + s1;
        }
        uint32_t a = h[0], b = h[1], c = h[2], d = h[3], e = h[4], f = h[5], g = h[6],
                 hh = h[7];
        for(int i = 0; i < 64; i++)
        {
            uint32_t s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
            uint32_t ch = (e & f) ^ (~e & g);
            uint32_t t1 = hh + s1 + ch + k[i] + w[i];
            uint32_t s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
            uint32_t mj = (a & b) ^ (a & c) ^ (b & c);
            uint32_t t2 = s0 + mj;
            hh = g;
            g = f;
            f = e;
            e = d + t1;
            d = c;
            c = b;
            b = a;
            a = t1 + t2;
        }
        h[0] += a;
        h[1] += b;
        h[2] += c;
        h[3] += d;
        h[4] += e;
        h[5] += f;
        h[6] += g;
        h[7] += hh;
    }

  public:
    sha256()
    {
        static const uint32_t init[8] = {0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
                                         0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19};
        for(int i = 0; i < 8; i++)
            h[i] = init[i];
    }
    void update(const uint8_t *p, size_t n)
    {
        total += n;
        while(n)
        {
            size_t take = std::min(n, sizeof(buf) - len);
            std::copy(p, p + take, buf + len);
            len += take;
            p += take;
            n -= take;
            if(len == sizeof(buf))
            {
                block(buf);
                len = 0;
            }
        }
    }
    std::string hex()
    {
        uint64_t bits = total * 8;
        uint8_t pad = 0x80;
        update(&pad, 1);
        pad = 0;
        while(len != 56)
            update(&pad, 1);
        uint8_t tail[8];
        for(int i = 0; i < 8; i++)
            tail[i] = uint8_t(bits >> (56 - 8 * i));
        update(tail, 8);
        static const char digits[] = "0123456789abcdef";
        std::string out;
        for(int i = 0; i < 8; i++)
            for(int j = 28; j >= 0; j -= 4)
                out += digits[(h[i] >> j) & 15];
        return out;
    }
};

// Returns the SHA-256 of a file, or an empty string if it can't be read.
std::string hash_file(const std::string &name)
{
    std::ifstream f(name, std::ios::binary);
    if(!f.is_open())
        return std::string();
    sha256 s;
    char chunk[4096];
    while(f)
    {
        f.read(chunk, sizeof(chunk));
        auto n = f.gcount();
        if(n > 0)
            s.update(reinterpret_cast<const uint8_t *>(chunk), size_t(n));
    }
    return s.hex();
}

// ---- JSON -------------------------------------------------------------------

std::string json_string(const std::string &s)
{
    std::string out = "\"";
    for(unsigned char c : s)
    {
        switch(c)
        {
        case '"':
            out += "\\\"";
            break;
        case '\\':
            out += "\\\\";
            break;
        case '\n':
            out += "\\n";
            break;
        case '\r':
            out += "\\r";
            break;
        case '\t':
            out += "\\t";
            break;
        default:
            if(c < 32 || c == 127)
            {
                char esc[8];
                snprintf(esc, sizeof(esc), "\\u%04x", c);
                out += esc;
            }
            else if(c > 127)
            {
                // Messages can quote bytes of an ATASCII source; keep the
                // document valid UTF-8 by writing them as code points.
                char esc[8];
                snprintf(esc, sizeof(esc), "\\u%04x", c);
                out += esc;
            }
            else
                out += char(c);
        }
    }
    return out + "\"";
}

// Path as written in the metadata: relative to the working directory when
// the file is under it, always with forward slashes.
std::string display_path(const std::string &path)
{
    std::string p = path, cwd = os::current_dir();
    std::replace(p.begin(), p.end(), '\\', '/');
    std::replace(cwd.begin(), cwd.end(), '\\', '/');
    while(cwd.size() > 1 && cwd.back() == '/')
        cwd.pop_back();
    auto same = [](char a, char b) {
#ifdef _WIN32
        if(a >= 'A' && a <= 'Z')
            a = a - 'A' + 'a';
        if(b >= 'A' && b <= 'Z')
            b = b - 'A' + 'a';
#endif
        return a == b;
    };
    if(!cwd.empty() && p.size() > cwd.size() + 1 && p[cwd.size()] == '/' &&
       std::equal(cwd.begin(), cwd.end(), p.begin(), same))
        p = p.substr(cwd.size() + 1);
    while(p.compare(0, 2, "./") == 0)
        p = p.substr(2);
    return p;
}

// Reads a linker label file (VICE format: "al 00ABCD .name").
std::map<std::string, int> read_labels(const std::string &name)
{
    std::map<std::string, int> out;
    std::ifstream f(name);
    std::string line;
    while(std::getline(f, line))
    {
        std::istringstream in(line);
        std::string kind, addr, label;
        if(!(in >> kind >> addr >> label) || kind != "al" || label.size() < 2 ||
           label[0] != '.')
            continue;
        try
        {
            size_t used = 0;
            long value = std::stol(addr, &used, 16);
            if(used == addr.size() && value >= 0 && value <= 0xFFFF)
                out[label.substr(1)] = int(value);
        }
        catch(...)
        {
        }
    }
    return out;
}
} // namespace

// ---- Public interface -------------------------------------------------------

std::string metadata::capabilities_json()
{
    return std::string("{\"schemaVersion\":1,\"version\":") + json_string(VERSION) +
           ",\"features\":[\"metadata-json\",\"structured-diagnostics\","
           "\"debug-metadata\"]}";
}

meta_diag metadata::diag_from_text(const std::string &text, const std::string &default_file,
                                   const std::string &code)
{
    meta_diag d{default_file, 1, 1, 0, "error", code, text};
    // Search "file:line: " allowing a drive letter in the file name.
    size_t from = (text.size() > 2 && text[1] == ':') ? 2 : 0;
    for(auto pos = text.find(':', from); pos != text.npos; pos = text.find(':', pos + 1))
    {
        size_t end = pos + 1;
        while(end < text.size() && text[end] >= '0' && text[end] <= '9')
            end++;
        if(end == pos + 1 || end - pos > 8 || end >= text.size() || text[end] != ':')
            continue;
        d.path = text.substr(0, pos);
        d.line = std::stoi(text.substr(pos + 1, end - pos - 1));
        // Optional column
        size_t cend = end + 1;
        while(cend < text.size() && text[cend] >= '0' && text[cend] <= '9')
            cend++;
        if(cend > end + 1 && cend - end <= 8 && cend < text.size() && text[cend] == ':')
        {
            d.column = std::stoi(text.substr(end + 1, cend - end - 1)) + 1;
            end = cend;
        }
        auto msg = text.find_first_not_of(' ', end + 1);
        d.message = msg == text.npos ? std::string() : text.substr(msg);
        break;
    }
    return d;
}

bool metadata::write(const std::string &path, bool success, const meta_info &info,
                     const std::string &target_name, const std::string &binary_format,
                     const std::string &label_file,
                     const std::vector<std::pair<std::string, std::string>> &artifacts)
{
    std::ostringstream o;
    o << "{\n  \"schemaVersion\": " << EXB_METADATA_SCHEMA << ",\n"
      << "  \"compiler\": {\"name\": \"exbasic\", \"version\": " << json_string(VERSION)
      << "},\n"
      << "  \"success\": " << (success ? "true" : "false") << ",\n"
      << "  \"target\": {\"name\": " << json_string(target_name)
      << ", \"binaryFormat\": " << json_string(binary_format) << "},\n";

    // Sources, with the hash of the bytes that were compiled
    std::map<std::string, int> source_index;
    o << "  \"sources\": [";
    bool first = true;
    for(auto &s : info.sources)
    {
        auto hash = hash_file(s);
        if(hash.empty() || source_index.count(s))
            continue;
        source_index.emplace(s, int(source_index.size()));
        o << (first ? "\n" : ",\n") << "    {\"path\": " << json_string(display_path(s))
          << ", \"sha256\": \"" << hash << "\"}";
        first = false;
    }
    o << (first ? "" : "\n  ") << "],\n";

    o << "  \"diagnostics\": [";
    first = true;
    for(auto &d : info.diags)
    {
        int col = d.column < 1 ? 1 : d.column;
        int end = d.end_column > col ? d.end_column : col;
        o << (first ? "\n" : ",\n") << "    {\"path\": " << json_string(display_path(d.path))
          << ", \"severity\": " << json_string(d.severity)
          << ", \"code\": " << json_string(d.code)
          << ", \"message\": " << json_string(d.message)
          << ", \"range\": {\"start\": {\"line\": " << (d.line < 1 ? 1 : d.line)
          << ", \"column\": " << col << "}, \"end\": {\"line\": " << (d.line < 1 ? 1 : d.line)
          << ", \"column\": " << end << "}}}";
        first = false;
    }
    o << (first ? "" : "\n  ") << "],\n";

    o << "  \"artifacts\": [";
    first = true;
    for(auto &a : artifacts)
    {
        std::ifstream t(a.second, std::ios::binary);
        if(!t.is_open())
            continue;
        o << (first ? "\n" : ",\n") << "    {\"kind\": " << json_string(a.first)
          << ", \"path\": " << json_string(display_path(a.second)) << "}";
        first = false;
    }
    o << (first ? "" : "\n  ") << "]";

    // Debug information needs the addresses assigned by the linker
    auto labels = success && !label_file.empty() ? read_labels(label_file)
                                                 : std::map<std::string, int>();
    auto dispatch = labels.find("next_instruction");
    if(success && dispatch != labels.end() && !info.lines.empty())
    {
        // The end of the code of a line is the start of the next one
        std::set<int> starts;
        for(auto &l : info.lines)
        {
            auto it = labels.find(l.label);
            if(it != labels.end())
                starts.insert(it->second);
        }
        // Group the code ranges by source line
        std::map<std::pair<int, int>, std::vector<std::string>> by_line;
        for(auto &l : info.lines)
        {
            auto addr = labels.find(l.label);
            auto src = source_index.find(l.file);
            if(addr == labels.end() || src == source_index.end() || l.line < 1)
                continue;
            auto next = starts.upper_bound(addr->second);
            int end = next == starts.end() ? addr->second + 1 : *next;
            std::ostringstream r;
            r << "{\"start\": " << addr->second << ", \"end\": " << end << ", \"kind\": \""
              << (l.native ? "native" : "bytecode") << "\"}";
            auto &list = by_line[{src->second, l.line}];
            if(std::find(list.begin(), list.end(), r.str()) == list.end())
                list.push_back(r.str());
        }
        o << ",\n  \"debug\": {\n"
          << "    \"interpreter\": {\"dispatch\": " << dispatch->second
          << ", \"pointer\": " << dispatch->second + 1 << "},\n"
          << "    \"lines\": [";
        first = true;
        for(auto &l : by_line)
        {
            o << (first ? "\n" : ",\n") << "      {\"source\": " << l.first.first
              << ", \"line\": " << l.first.second << ", \"ranges\": [";
            for(size_t i = 0; i < l.second.size(); i++)
                o << (i ? ", " : "") << l.second[i];
            o << "]}";
            first = false;
        }
        o << (first ? "" : "\n    ") << "],\n    \"symbols\": [";
        first = true;
        for(auto &s : info.symbols)
        {
            auto addr = labels.find(s.label);
            if(addr == labels.end() || addr->second + s.size > 0x10000)
                continue;
            o << (first ? "\n" : ",\n") << "      {\"name\": " << json_string(s.name)
              << ", \"type\": " << json_string(s.type) << ", \"address\": " << addr->second
              << ", \"size\": " << s.size << ", \"source\": -1, \"line\": 0}";
            first = false;
        }
        o << (first ? "" : "\n    ") << "]\n  }";
    }
    o << "\n}\n";

    // Write to a temporary file and rename, so a reader never sees a
    // partial document.
    auto tmp = path + ".tmp";
    {
        std::ofstream f(tmp, std::ios::binary | std::ios::trunc);
        if(!f.is_open())
            return false;
        f << o.str();
        f.flush();
        if(!f.good())
        {
            f.close();
            os::remove_file(tmp);
            return false;
        }
    }
    if(!os::rename_file(tmp, path))
    {
        os::remove_file(tmp);
        return false;
    }
    return true;
}
