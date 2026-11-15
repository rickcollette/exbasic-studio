#!/usr/bin/env python3
"""exBASIC programs on EOS (the SpartaDOS X compatible DOS of the
ExtendedOS project), from its cartridge image EOS.car, in Altirra.

Each test compiles its programs, puts them on a SpartaDOS disk image (made
with mkatr) with an AUTOEXEC.BAT that clears the screen, runs them and then
runs MARK.COM, and boots EOS with that disk in D1:.  The programs print to
the screen (exBASIC calls the E: handler directly, so DOS redirection does
not apply): MARK.COM is only an RTS at $3F00, where an Altirra breakpoint
writes the memory to a file.  The text screen in it is compared with the
expected lines (EXPECTED below, the output of the same programs on SDX
4.50).

usage: run.py [-j N] [test ...]      (all the tests by default)
needs: wine, xvfb-run, Altirra (ALTIRRA_EXE), EOS.car (EOS_CAR, default the
folder above the exBASIC tree)
"""
import concurrent.futures
import os
import re
import shutil
import signal
import subprocess
import sys
import time
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]
OUT = ROOT / 'build' / 'eostest'
ALTIRRA = Path(os.environ.get('ALTIRRA_EXE', Path.home() /
                              'AtariMutineer/Privateer/tools/altirra/4.40/Altirra64.exe'))
EOS_CAR = Path(os.environ.get('EOS_CAR', ROOT.parent / 'EOS.car'))
EXBASIC = Path(os.environ.get('EXBASIC', ROOT / 'build/compiler/exbasic'))
MKATR = ROOT / 'build/bin/mkatr'
OPTS = {'XLOW': ['-S:0x1000']}        # starts below MEMLO: the load check

# name: (programs, commands, options)
TESTS = {
    'hello': (['XHELLO'], ['XHELLO'], {}),
    'file': (['XFILE'], ['XFILE'], {}),
    'bytes': (['XBYTES'], ['XBYTES'], {}),
    'note': (['XNOTE'], ['XNOTE'], {}),
    'xio': (['XXIO'], ['XXIO'], {}),
    'dir': (['XDIR'], ['XDIR'], {}),
    'sdx': (['XSDX'], ['XSDX'], {}),
    'chain': (['XCHAIN', 'XCHAIN2'], ['XCHAIN'], {}),
    'gfx': (['XGFX'], ['XGFX'], {}),
    'err': (['XERR'], ['XERR'], {}),
    'lock': (['XLOCK'], ['XLOCK'], {}),
    'upd': (['XUPD'], ['XUPD'], {}),
    'aux': (['XAUX'], ['XAUX'], {}),
    'mem': (['XMEM'], ['XMEM'], {}),
    'low': (['XLOW', 'XHELLO'], ['XLOW', 'XHELLO'], {}),
    'runlow': (['XRUNLOW', 'XLOW'], ['XRUNLOW'], {}),
    'mem-128K': (['XMEM'], ['XMEM'], {'memsize': '128K'}),
    'twice': (['XHELLO'], ['XHELLO', 'XHELLO'], {}),
    'file-128K': (['XFILE'], ['XFILE'], {'memsize': '128K'}),
}
HELLO = ['HELLO EXBASIC', 'FRE OK 1', 'FP 3.375', 'INT 42']
FILE = ['OPEN W 1', 'CLOSE 1', 'OPEN A 1', 'OPEN R 1', '[LINE 1]', '[LINE 2]', '[LINE 3]',
        '[APPENDED]', 'EOF 136', 'MISSING 170', 'NO DEVICE 130']
MEM = ['TEXT OK', 'FREE OK 1', 'BIG 1234 4321 1', 'ERROR- 2']
# Expected output of each command, as on SDX 4.50
EXPECTED = {
    'hello': [HELLO],
    'file': [FILE],
    'bytes': [['BPUT 1', 'BGET 1', 'DIFF 0', 'GET 65 66 1', 'EOF 136', 'PART 136 12',
               'UPDATE 1', 'FIRST 1']],
    'note': [['NOTE 1', 'RECORD NUMBER 21', 'RECORD NUMBER 31', 'POINT 1', 'RECORD NUMBER 21',
              'RECORD NUMBER 1', 'POS 331 0 0 0']],
    'xio': [['RENAME 1', 'LOCK 1', 'DEL LOCKED 170', 'WRITE LOCKED 151', 'UNLOCK 1', 'DELETE 1',
             'DEL AGAIN 170', 'DEL WILD 1', 'REN MISSING 170', 'OLD GONE 170']],
    'dir': [['OPEN 1', 'ZZ1     TXT 001', 'N FREE SECTORS', 'END 136']],
    'sdx': [['MKDIR 1', 'WRITE 1', 'CHDIR 1', 'READ INSIDE 1', 'CHDIR ROOT 1', 'RMDIR FULL 167',
             'DEL 1', 'RMDIR 1']],
    'chain': [['FIRST PROGRAM', 'SECOND 1234 2.5 PASSED']],
    'gfx': [['BACK 77', 'STATUS', 'E: 1', 'CLOSED 130']],
    'err': [['BEFORE', 'ERROR- 11']],
    'lock': [['WRITE 151', 'APPEND 1', 'PUT 1', 'CLOSE 1', 'UPDATE 1', 'GET 75 1', 'PUT 1',
              'CLOSE 1', 'READ KBEP 1', 'RENAME 170', 'DELETE 170', 'UNLOCK 1']],
    'upd': [['300 15750 0123', '300 15767 A123', '300 15784 AB23', '300 15818 AB23',
             '310 16377 AB23', '310 16567 XYZ']],
    'aux': [['E: 1', 'AUX1 CLEARED']],
    'mem': [MEM],
    'low': [['ERROR- 19'], HELLO],
    'runlow': [['BEFORE RUN', 'ERROR- 19']],
    'mem-128K': [MEM],
    'twice': [HELLO, HELLO],
    'file-128K': [FILE],
}


def win(path):
    return 'Z:' + str(Path(path).resolve()).replace('/', '\\')


def compile_prog(name, dest):
    src = dest / (name + '.bas')
    shutil.copy(HERE / (name + '.bas'), src)
    com = dest / (name + '.COM')
    p = subprocess.run([str(EXBASIC), *OPTS.get(name, []), src.name, '-o', com.name], cwd=dest,
                       capture_output=True, text=True)
    if p.returncode or not com.exists():
        raise RuntimeError('%s: %s' % (name, p.stdout + p.stderr))
    return com


MARK = 0x3F00
# A program that is only an RTS at MARK, run when it is loaded
MARK_COM = bytes([0xFF, 0xFF, MARK & 255, MARK >> 8, MARK & 255, MARK >> 8, 0x60,
                  0xE0, 0x02, 0xE1, 0x02, MARK & 255, MARK >> 8])


def screen(m):
    """The 24 x 40 text screen at SAVMSC as lines (inverse video dropped)."""
    base = m[0x58] | m[0x59] << 8
    lines = []
    for row in range(24):
        t = ''
        for c in m[base + 40 * row:base + 40 * row + 40]:
            c &= 0x7F
            c = c + 0x20 if c < 0x40 else c - 0x40 if c < 0x60 else c
            t += chr(c) if 32 <= c < 127 else '.'
        lines.append(t.strip())
    return lines


def run(test, worker):
    progs, cmds, opts = TESTS[test]
    work = OUT / test
    shutil.rmtree(work, ignore_errors=True)
    (work / 'disk').mkdir(parents=True)
    for p in progs:
        com = compile_prog(p, work)
        shutil.copy(com, work / 'disk' / com.name)
    (work / 'disk' / 'MARK.COM').write_bytes(MARK_COM)
    batch = ['CLS'] + cmds + ['MARK']
    (work / 'disk' / 'AUTOEXEC.BAT').write_bytes(b''.join(l.encode() + b'\x9b' for l in batch))
    atr = work / 'disk.atr'
    names = sorted(f.name for f in (work / 'disk').iterdir())
    subprocess.run([str(MKATR), '-s', '184320', str(atr)] + names, cwd=work / 'disk',
                   check=True, capture_output=True)
    ini = work / 'altirra.ini'
    root = 'User\\Software\\virtualdub.org\\Altirra'
    ini.write_text('\n'.join(['[%s\\Settings]' % root, '"Pause when inactive" = 0',
                              '[%s\\Profiles\\00000000]' % root, '"Pause when inactive" = 0',
                              '"Kernel: Fast boot enabled" = 1', '"Turbo mode" = 1', '']))
    dump = work / 'memory.bin'
    env = dict(os.environ, WINEPREFIX=str(OUT / ('wine-%d' % worker)), WINEDEBUG='-all')
    cmd = ['xvfb-run', '-a', '-s', '-screen 0 1024x768x24', 'wine', str(ALTIRRA),
           '/portablealt:' + win(ini), '/hardware:800xl', '/memsize:' + opts.get('memsize', '64K'),
           '/nobasic', '/ntsc', '/fastboot', '/bootrw', '/cart', win(EOS_CAR), '/disk', win(atr),
           '/debug', '/debugcmd', 'bp $%04X ".writemem %s 0 L10000; g"' % (MARK, win(dump))]
    p = subprocess.Popen(cmd, env=env, stdout=open(work / 'emulator.log', 'wb'),
                         stderr=subprocess.STDOUT, start_new_session=True)
    m = None
    deadline = time.monotonic() + float(os.environ.get('EOSTEST_TIMEOUT', 120))
    try:
        while time.monotonic() < deadline:
            time.sleep(1)
            if dump.exists() and dump.stat().st_size == 0x10000:
                time.sleep(0.5)
                m = dump.read_bytes()
                break
    finally:
        os.killpg(p.pid, signal.SIGTERM)
        subprocess.run(['wineserver', '-k'], env=env, capture_output=True)
        p.wait()
    if m is None:
        return test, False, 'timeout (no memory dump, see %s)' % work
    got = [l for l in screen(m) if l and not l.endswith(':') and l != 'MARK']
    # The free space depends on the size of the programs on the disk
    got = [re.sub(r'^\d+ FREE SECTORS$', 'N FREE SECTORS', l) for l in got]
    want = [l for out in EXPECTED[test] for l in out]
    if got != want:
        (work / 'screen.txt').write_text('\n'.join(screen(m)) + '\n')
        return test, False, 'got %r, expected %r' % (got, want)
    return test, True, ''


def main():
    argv = sys.argv[1:]
    jobs = 3
    if argv[:1] == ['-j']:
        jobs, argv = int(argv[1]), argv[2:]
    tests = argv or list(TESTS)
    for what, path in (('EOS cartridge', EOS_CAR), ('Altirra', ALTIRRA), ('exBASIC', EXBASIC),
                       ('mkatr', MKATR)):
        if not path.exists():
            sys.exit('%s not found: %s' % (what, path))
    OUT.mkdir(parents=True, exist_ok=True)
    workers = list(range(jobs))
    failed = 0

    def task(t):
        w = workers.pop()
        try:
            return run(t, w)
        finally:
            workers.append(w)

    with concurrent.futures.ThreadPoolExecutor(jobs) as ex:
        for test, ok, msg in ex.map(task, tests):
            print('%-10s %s %s' % (test, 'PASS' if ok else 'FAIL', msg), flush=True)
            failed += not ok
    print('%d run, %d failed' % (len(tests), failed))
    sys.exit(1 if failed else 0)


if __name__ == '__main__':
    main()
