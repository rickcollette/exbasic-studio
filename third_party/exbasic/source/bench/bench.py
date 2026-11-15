#!/usr/bin/env python3
"""Benchmarks: exBASIC against Atari BASIC, Altirra BASIC and FastBasic.

Each program (atari/*.lst for Atari BASIC and Altirra BASIC, fastbasic/*.bas,
exbasic/*.bas) runs in Altirra under wine, in an 800XL with 64 KB, NTSC, the
Atari XL OS rev. 2 (with its floating point package) and no DOS. The program
measures itself with the frame counter (RTCLOK) and writes to H:R.TXT:

    check frames free memtop

check: a value computed by the program, the same for all the BASICs (it shows
that they all did the same work); frames: the time (1/59.92 s); free: the free
memory at the end (FRE); memtop: MEMTOP (compiled programs only; for the
interpreters the 4th value is the memory used, LOMEM to the top of BASIC's
stack: program, variables, arrays and strings).

Atari BASIC and Altirra BASIC ENTER the listing from H:, SAVE it (for the
file size) and RUN it; the compiled programs are booted as executables.

usage: bench.py [-j N] [system ...] [-p prog ...]   then: bench.py --report
  systems: atari altirra fastbasic exbasic (all by default)
  results: build/bench/results.json; --report prints the Markdown tables

Needs wine, xvfb-run, Altirra (ALTIRRA_EXE), the Atari XL OS ROM (BENCH_XLOS)
and the Atari BASIC rev. C ROM (BENCH_BASIC); Altirra BASIC is Altirra's own.
FastBasic: FASTBASIC (the 4.x compiler, with its libraries next to it).
"""
import concurrent.futures
import json
import math
import os
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent
OUT = ROOT / 'build' / 'bench'
HOME = Path.home()
ALTIRRA = Path(os.environ.get('ALTIRRA_EXE', HOME / 'AtariMutineer/Privateer/tools/altirra/4.40/Altirra64.exe'))
XLOS = Path(os.environ.get('BENCH_XLOS', HOME / 'AtariMutineer/MutineerBASIC/build/native-atari800/lprint/rom/ATARIXL.ROM'))
ABASIC = Path(os.environ.get('BENCH_BASIC', HOME / 'AtariMutineer/MutineerBASIC/build/native-atari800/lprint/rom/ATARIBAS.ROM'))
FASTBASIC = Path(os.environ.get('FASTBASIC', HOME / 'fb/fastbasic'))
EXBASIC = Path(os.environ.get('EXBASIC', ROOT / 'build/compiler/exbasic'))

SYSTEMS = ['atari', 'altirra', 'fastbasic', 'exbasic']
NAMES = {'atari': 'Atari BASIC', 'altirra': 'Altirra BASIC', 'fastbasic': 'FastBasic 4.7',
         'exbasic': 'exBASIC 1.0'}
PROGS = [  # name, description
    ('sieve', 'Sieve of Eratosthenes, 8191 flags'),
    ('bubble', 'Bubble sort, 200 numbers'),
    ('gcd', 'GCD of 1600 pairs (MOD, loops)'),
    ('calls', '3000 subroutine calls'),
    ('peekpoke', 'POKE and PEEK 4096 bytes'),
    ('strings', 'Build and scan 100-char strings, 20 times'),
    ('ahl', "Ahl's benchmark (floating point)"),
    ('print', 'PRINT 300 lines to the screen'),
    ('graphics', '160 DRAWTO lines in GRAPHICS 8'),
]
FPS = 1789772.5 / 29868          # NTSC frames per second (59.92)
START = 0x2000                   # load address of the compiled programs
ROMS = [  # Altirra's firmware IDs for these ROMs, as its firmware scan sets them up
    (0xBC7689ABF9A566C4, 'Atari XL/XE OS ver.2', 'kernelxl', 'XLOS rev 2'),
    (0xAB5769BEFE3EE956, 'Atari BASIC rev. C', 'basic', 'BASIC rev C'),
]


def win(path):
    return 'Z:' + str(Path(path).resolve()).replace('/', '\\')


def write_ini(path, atari_basic):
    """Altirra settings: turbo (the host speed only: the emulated time is
    exact), no floating point acceleration, the XL OS and (atari_basic)
    Atari BASIC; without it /basic gives Altirra's own Altirra BASIC."""
    root = 'User\\Software\\virtualdub.org\\Altirra'
    lines = ['[%s\\Settings]' % root, '"Pause when inactive" = 0',
             '[%s\\Profiles\\00000000]' % root, '"Pause when inactive" = 0',
             '"Kernel: Floating-point patch enabled" = 0', '"Kernel: Fast boot enabled" = 1',
             '"Turbo mode" = 1']
    specific = []
    for fid, name, kind, ref in ROMS:
        if kind == 'basic' and not atari_basic:
            continue
        rom = XLOS if kind == 'kernelxl' else ABASIC
        lines += ['[%s\\Firmware\\Available\\%016X]' % (root, fid), '"Name" = "%s"' % name,
                  '"Path" = "%s"' % win(rom).replace('\\', '\\\\'), '"Type" = "%s"' % kind,
                  '"Flags" = 0']
        specific.append('"%s" = "%016x"' % (ref, fid))
    lines += ['[%s\\Firmware\\Specific]' % root] + specific
    Path(path).write_text('\n'.join(lines + ['']), encoding='utf-8')


def compile_prog(system, prog, dest):
    """The executable of prog for a compiler, in dest; its path."""
    src = HERE / system / (prog + '.bas')
    shutil.copy(src, dest / (prog + '.bas'))          # (the .asm and .o go there too)
    xex = dest / (prog + '.xex')
    if system == 'fastbasic':
        target = 'atari-fp' if '%' in src.read_text() else 'atari-int'
        cmd = [str(FASTBASIC), '-t:' + target, prog + '.bas', '-o', xex.name]
    else:
        cmd = [str(EXBASIC), prog + '.bas', '-o', xex.name]
    p = subprocess.run(cmd, cwd=dest, capture_output=True, text=True)
    if p.returncode or not xex.exists():
        raise RuntimeError('%s %s: %s' % (system, prog, p.stdout + p.stderr))
    return xex


def run_one(system, prog, worker):
    """Run prog on system in Altirra (under its own xvfb display and wine
    prefix); the result dict."""
    work = OUT / 'runs' / ('%s-%s' % (system, prog))
    shutil.rmtree(work, ignore_errors=True)
    hdir = work / 'h'
    hdir.mkdir(parents=True)
    ini = work / 'altirra.ini'
    write_ini(ini, system == 'atari')
    args = ['/portablealt:' + win(ini), '/hardware:800xl', '/memsize:64K', '/ntsc',
            '/hdpathrw:' + win(hdir), '/noaccuratedisk']
    if system in ('atari', 'altirra'):
        lst = (HERE / 'atari' / (prog + '.lst')).read_text().splitlines()
        (hdir / 'P.LST').write_bytes(b''.join(x.encode('ascii') + b'\x9b' for x in lst if x))
        args += ['/basic', '/type', 'ENTER "H:P.LST"~SAVE "H:P.BAS"~RUN~']
        size_file = 'p.bas'
    else:
        xex = compile_prog(system, prog, work)
        args += ['/nobasic', '/run', win(xex)]
        size_file = None
    prefix = OUT / ('wine-%d' % worker)
    env = dict(os.environ, WINEPREFIX=str(prefix), WINEDEBUG='-all')
    cmd = ['xvfb-run', '-a', '-s', '-screen 0 1024x768x24', 'sh', '-c',
           'wine "$0" "$@" & pid=$!; while [ ! -s "%s" ]; do sleep 1; done; sleep 2; '
           'import -window root "%s"; kill $pid; wineserver -k' % (hdir / 'r.txt', work / 'screen.png'),
           str(ALTIRRA)] + args
    t = time.monotonic()
    log = open(work / 'emulator.log', 'wb')
    p = subprocess.Popen(cmd, env=env, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
    try:
        p.wait(timeout=float(os.environ.get('BENCH_TIMEOUT', 900)))
    except subprocess.TimeoutExpired:
        os.killpg(p.pid, signal.SIGKILL)
        subprocess.run(['wineserver', '-k'], env=env, capture_output=True)
        p.wait()
    res = {'system': system, 'prog': prog, 'host_seconds': round(time.monotonic() - t, 1)}
    r = hdir / 'r.txt'
    if not r.exists():
        res['error'] = 'no result (see %s)' % work
        return res
    vals = [int(float(x)) for x in r.read_bytes().replace(b'\x9b', b' ').decode('latin-1').split()]
    if len(vals) != 4:
        res['error'] = 'bad result %r' % vals
        return res
    check, frames, free, last = vals
    free %= 65536
    if system in ('atari', 'altirra'):
        used = last
        size = (hdir / size_file).stat().st_size if (hdir / size_file).exists() else None
    else:
        used = (last % 65536) - free - START
        size = xex.stat().st_size
    res.update(check=check, frames=frames % 65536 if system in ('fastbasic', 'exbasic') else frames,
               free=free, used=used, size=size)
    return res


def load():
    f = OUT / 'results.json'
    return json.loads(f.read_text()) if f.exists() else {}


def report():
    data = load()
    progs = [p for p, _ in PROGS if any('%s/%s' % (s, p) in data for s in SYSTEMS)]
    desc = dict(PROGS)

    def get(s, p, k):
        r = data.get('%s/%s' % (s, p))
        return r.get(k) if r and 'error' not in r else None

    head = '| Program | ' + ' | '.join(NAMES[s] for s in SYSTEMS) + ' | exBASIC vs Atari BASIC |'
    sep = '|---|' + '---:|' * (len(SYSTEMS) + 1)
    print('### Time (seconds)\n')
    print(head + '\n' + sep)
    for p in progs:
        cells = []
        for s in SYSTEMS:
            f = get(s, p, 'frames')
            cells.append('%.2f' % (f / FPS) if f is not None else '-')
        a, x = get('atari', p, 'frames'), get('exbasic', p, 'frames')
        ratio = '%.0fx' % (a / x) if a and x else '-'
        if a and x and a / x < 10:
            ratio = '%.1fx' % (a / x)
        print('| %s | %s | %s |' % (desc[p], ' | '.join(cells), ratio))
    print('\n### Speed against Atari BASIC (geometric mean)\n')
    for group, names in (('all programs', progs),
                         ('computation (no screen output)',
                          [p for p in progs if p not in ('print', 'graphics')])):
        cells = []
        for s in SYSTEMS:
            rs = [get('atari', p, 'frames') / get(s, p, 'frames') for p in names
                  if get('atari', p, 'frames') and get(s, p, 'frames')]
            cells.append('%.1fx' % math.exp(sum(map(math.log, rs)) / len(rs)) if rs else '-')
        print('| %s | %s |' % (group, ' | '.join(cells)))
    for key, title in (('used', 'Memory used by the running program (bytes)'),
                       ('free', 'Free memory while running (bytes)'),
                       ('size', 'File size (bytes)')):
        print('\n### %s\n' % title)
        print('| Program | ' + ' | '.join(NAMES[s] for s in SYSTEMS) + ' |\n|---|' + '---:|' * len(SYSTEMS))
        for p in progs:
            print('| %s | %s |' % (desc[p], ' | '.join(
                str(get(s, p, key)) if get(s, p, key) is not None else '-' for s in SYSTEMS)))
    print('\n### Check values\n')
    for p in progs:
        vals = {s: get(s, p, 'check') for s in SYSTEMS}
        print('%-9s %s' % (p, '  '.join('%s=%s' % kv for kv in vals.items())))


def main():
    argv = sys.argv[1:]
    if argv and argv[0] == '--report':
        return report()
    jobs = 4
    if argv[:1] == ['-j']:
        jobs, argv = int(argv[1]), argv[2:]
    progs = [p for p, _ in PROGS]
    if '-p' in argv:
        i = argv.index('-p')
        progs, argv = argv[i + 1:], argv[:i]
    systems = argv or SYSTEMS
    for tool, path in (('Altirra', ALTIRRA), ('XL OS ROM', XLOS), ('Atari BASIC ROM', ABASIC)):
        if not path.exists():
            sys.exit('%s not found: %s' % (tool, path))
    for tool in ('wine', 'xvfb-run', 'import'):
        if not shutil.which(tool):
            sys.exit('missing tool: ' + tool)
    OUT.mkdir(parents=True, exist_ok=True)
    tasks = [(s, p) for p in progs for s in systems]
    free_workers = list(range(jobs))
    data = load()

    def task(sp):
        w = free_workers.pop()
        try:
            return run_one(sp[0], sp[1], w)
        finally:
            free_workers.append(w)

    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for res in ex.map(task, tasks):
            data['%s/%s' % (res['system'], res['prog'])] = res
            (OUT / 'results.json').write_text(json.dumps(data, indent=1, sort_keys=True))
            print(json.dumps(res), flush=True)


if __name__ == '__main__':
    main()
