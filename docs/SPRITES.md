# Sprite Studio

The Sprite Studio draws player/missile sprites and turns them into the
files an exBASIC program loads: a PMG frame bank, an index of frame offsets,
an exBASIC include with the same data as `DATA` statements, and a PNG atlas.
It also merges several sheets or banks into one.

Open it from the **Sprite Studio** view in the activity bar (the paint can),
from the *Sprites* menu, or by opening a sprite file from the Explorer.

## What opens where

| File | Opens in | Saving writes |
|------|----------|---------------|
| `*.sprites.json` (a sheet) | Sprite Studio | the sheet, and its frames file when it has one |
| `*.pmg` | Sprite Studio, through the index JSON beside it | the bank and that index |
| `*.png` | Sprite Studio, with the Import panel open | a new sheet (you choose its path) |
| any other `.json` | text editor | — |

A plain frames file (any JSON object mapping frame names to lists of row
strings, such as `hero_pixels.json`) opens in the text editor. Use
*Sprites: Open in Sprite Studio* to draw on it instead. If a sheet in the same
folder names that file as its `source`, or names a bank as its output, the
Studio opens that sheet, so the palette and outputs come along.

*Sprites: Open Sheet as JSON Text* (the `{}` button in the Studio toolbar)
goes the other way. Both editors notice when the other saves.

## Sheets

A sheet records everything needed to generate a bank:

```json
{
  "format": "exbasic-sprites",
  "version": 1,
  "width": 8,
  "height": 24,
  "planes": 2,
  "palette": [
    { "symbol": ".", "name": "transparent", "color": "#0a0f1a", "planes": [] },
    { "symbol": "H", "name": "hat",   "color": "#c1802c", "planes": [0] },
    { "symbol": "S", "name": "shirt", "color": "#237eae", "planes": [1] },
    { "symbol": "F", "name": "skin",  "color": "#eeac70", "planes": [0, 1] }
  ],
  "source": "hero_pixels.json",
  "outputs": {
    "bank": "HERO.PMG",
    "index": "hero_index.json",
    "indexStyle": "planes",
    "include": "HERO.BAS",
    "dataName": "Hero",
    "atlas": "../previews/hero_sheet.png",
    "atlasScale": 4,
    "atlasColumns": 8
  }
}
```

- **Frames** are rows of one character per pixel. `.` is always transparent.
  Without `source`, the frames are stored in the sheet under `"frames"`.
  With `source`, they stay in that plain frames file, so scripts that already
  read it keep working.
- **Planes** are hardware players drawn on top of each other. Each colour
  lists the planes it sets. A pixel in two planes shows GTIA's overlap colour,
  so its preview colour is only a representation; check it in the emulator.
  Two colours with the same planes look identical on the Atari, and the
  Problems list says so.
- **Width** can exceed 8 pixels. Each row then takes `ceil(width / 8)` bytes,
  leftmost pixel in bit 7, and your program places the players side by side.
- **Output paths** are relative to the sheet's folder and must stay inside
  the project.

## Generated files

**Bank.** Frames one after another. Each frame holds its planes one after
another (player 0 first), and each plane holds `height` rows. A one-plane
8x16 frame is 16 bytes; a two-plane 8x24 frame is 48. Load it with:

```basic
DATA HeroData() BYTE FILE "assets/HERO.PMG"
```

**Index.** Two layouts:

- `planes`: `offset`, `player0_offset`, `player1_offset`, …,
  `bytes_per_plane`, `frame_length`, `height`, `width`.
- `simple`: `offset`, `length`, `width`, `height`.

**Include.** The bank as `DATA <name>() BYTE = …` with continuation lines,
and `DATA <name>Frames() WORD = …` with the byte offset of every frame.
`INCLUDE` it instead of loading the bank with `BYTE FILE`.

**Atlas.** Every frame in one PNG with a transparent background, at the
chosen scale and number of columns (0 puts them all in one row).

**Generate** (toolbar, or *Sprites: Generate PMG Bank and Outputs*) saves the
sheet first, then writes every output that is set. It refuses while the
Problems list has errors.

## Drawing

| Key | Action |
|-----|--------|
| B / E / G | Pencil / eraser / fill |
| L / R / Shift+R | Line / rectangle / filled rectangle |
| I | Pick a colour from the frame |
| 0–9 | Choose palette colour 0–9 |
| [ and ] | Previous / next frame |
| Shift+arrows | Shift the frame (wraps around) |
| + / - | Zoom |
| Ctrl+D | Duplicate the frame |
| Ctrl+Z / Ctrl+Y | Undo / redo (a whole pen stroke is one step) |
| Ctrl+S | Save |

The left button draws and the right button erases. The toolbar also flips and
clears frames, shows a pixel grid (with byte boundaries on wide sprites), an
onion skin of the previous frame, and a *Player n bits* view that shows
exactly what one plane holds. The preview animates every frame whose name
shares the current frame's stem (`walk_0`…`walk_3`), or all frames.

## Importing and merging

The **Import** panel brings frames in from:

- **a PNG**, cut into cells of a chosen size. Each pixel takes the nearest
  palette colour, and pixels under 50% alpha stay transparent. The palette
  can come from the image: each colour then gets its own plane combination
  (three colours use P0, P1 and P0+P1).
- **a PMG bank**, through the index beside it, a chosen index, or equal
  frames of a size you give.
- **other sprite files**, whose frames are appended.

**Merge** in the Sprite Studio view creates a new sheet from several
sheets, frame files and banks, in the order ticked. Frames grow to the
largest width and height (padded below or above), the plane count is the
largest of the inputs, palettes are combined (a symbol that clashes is
renamed and its pixels follow), and duplicate frame names get the file name
as a prefix. Generate the merged sheet to get one combined bank and index.
