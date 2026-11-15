# mkatr-go licensing and provenance

The SpartaDOS/BW-DOS ATR implementation and bootloader data are a Go port of
[`dmsc/mkatr`](https://github.com/dmsc/mkatr), copyright (C) 2016–2026 Daniel
Serpell. They are distributed under GNU GPL version 2 only; see `LICENSE` in
this directory. `mkatr-go` is intentionally built as a separate executable so
the MIT-licensed exBASIC Studio process does not link the GPL implementation.

EOSFS support implements the public-domain (CC0) “EOSFS: the EOS native file
system, Version 1” specification supplied by the ExtendedOS project. The
EOSFS-specific extent and directory-index code is released under CC0 to the
extent it is separable; the combined `mkatr-go` executable remains GPL-2.0-only
because it also contains the mkatr-derived SpartaDOS implementation.
