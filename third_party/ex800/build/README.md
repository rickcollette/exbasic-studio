# Cross-platform builds

`build/tools/build-release.sh` produces a versioned binary archive for one
native target. It builds both the SDL2 `ex800` application and the
`studio-helper` executable that embeds the retained `libatari800` API.

Supported targets:

- `linux-x86_64`
- `windows-x86_64` (run from an MSYS2 UCRT64 shell)
- `macos-x86_64`
- `macos-arm64`

Examples:

```sh
build/tools/build-release.sh linux-x86_64
build/tools/build-release.sh macos-arm64
```

Set `BUILD_ROOT` or `DIST_ROOT` to place intermediate files or final archives
outside the defaults (`build/out` and `dist`). The build is out-of-tree and
does not modify the source directory.

## Dependencies

Linux (Debian/Ubuntu):

```sh
sudo apt-get install build-essential libsdl2-dev zlib1g-dev libpng-dev
```

macOS (Homebrew):

```sh
brew install sdl2 libpng dylibbundler
```

An Apple Silicon Mac with Rosetta 2 can also produce the Intel archive. Build
SDL2 and libpng for x86_64 under a separate prefix, then point the release
script at it:

```sh
MACOS_CROSS_PREFIX="$HOME/.local/ex800-x86_64" \
  build/tools/build-release.sh macos-x86_64
```

The prefix must contain `bin/sdl2-config`, the SDL2 and libpng headers, and
x86_64 libraries. `dylibbundler` must also be available on `PATH`.

Windows packages are installed by the GitHub Actions workflow. For a local
MSYS2 UCRT64 build, install `make`, `zip`, GCC, SDL2, zlib and libpng for the
UCRT64 environment.

The workflow at `.github/workflows/build.yml` builds all four archives on
native GitHub-hosted runners. macOS artifacts are separate native arm64 and
x86_64 app bundles; they are not a combined universal binary. Release builds
are ad-hoc signed only. Distribution outside CI may additionally require an
Apple Developer ID signature and notarization.
