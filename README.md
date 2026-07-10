# go-widgets/toolkit

[![CI](https://github.com/go-widgets/toolkit/actions/workflows/ci.yml/badge.svg)](https://github.com/go-widgets/toolkit/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/go-widgets/toolkit?display_name=tag&sort=semver&color=0d9488)](https://github.com/go-widgets/toolkit/releases)
[![pkg.go.dev](https://img.shields.io/badge/pkg.go.dev-toolkit-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-widgets/toolkit)
![coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)
![go](https://img.shields.io/badge/Go-1.26.4%2B-00ADD8?logo=go&logoColor=white)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](./LICENSE)

Pure-Go widget toolkit that renders into an RGBA byte buffer. Zero
JS / DOM / canvas dependency — every widget composes pixels into a
caller-supplied `[]byte`, so the toolkit runs identically in
`GOOS=js GOARCH=wasm` (browser SharedArrayBuffer clients), on any
native target Go ships (native canvas backends, image files), or
in headless tests (screenshot-hash regressions).

## Goals

- **One toolkit per app**: consumers stop reinventing buttons,
  scrollbars, text fields. They import
  `github.com/go-widgets/toolkit` and compose.
- **Coherent theming**: a single `Theme` value cascades through every
  widget — change a colour at the top, see it everywhere.
  `LoadGTKTheme(css)` parses libadwaita / GTK3 `@define-color`
  declarations into a `Theme`, so any GTK-desktop palette (Adwaita,
  Juno, WhiteSur, Solarized …) drives the widget ink colours.
- **Pure Go + CGO=0**: no C toolchain, no shared libraries. Builds
  for every Go target including `GOOS=js GOARCH=wasm`.
- **A11y-ready scaffolding**: widgets carry a `Role` + `Label` so a
  future a11y bridge can publish them to screen readers without
  rewriting every widget.

## Non-goals

- **Not a CodeMirror replacement.** Complex web-grade editors are
  best embedded via an iframe overlay at the host level.
- **Not GTK.** No CSS engine, no SVG renderer, no full BiDi/IME. A
  broad primitive set — buttons, inputs, containers, feedback,
  overlays, structural rows, semantic banners, data displays — around
  8 kLoC of widget code with double that in tests.

## Status

**v0.9 — GTK 4 / DaisyUI parity pass.** Three additive waves closed
the coverage gap versus libadwaita and DaisyUI 4: v0.7 shipped 9
core widgets (Switch, Badge, Kbd, Alert, Card, Breadcrumbs, Steps,
HeaderBar, Table), v0.8 shipped 12 overlays and structural rows
(Avatar, Skeleton, Rating, Toast, Banner, Popover, ActionRow,
ViewSwitcher, ChatBubble, SearchEntry, Diff, Pagination), v0.9
shipped 8 finishing widgets (SplitButton, IconButton, Stat, Timeline,
DropZone, Chip, FormField, ProgressCircle).

62 widgets + 10 stock icons, ~8k LoC of widget code, 100% statement
coverage.
Pure Go, no CGO, stdlib only. Builds for `GOOS=js GOARCH=wasm` and
every native target Go ships.

| Family       | Widgets                                                            |
| ------------ | ------------------------------------------------------------------ |
| Base         | `Widget`, `Base`, `Rect`, `Event` (+ IME composition), `Theme`, `RGBA` |
| Text         | bitmap 5x7 font (60+ glyphs), `DrawText`, `TextWidth`, `Label`     |
| Action       | `Button`, `ToggleButton`, `CheckButton`, `RadioButton` + `Group`   |
| Input        | `Entry`, `TextView` + `Selection` + IME preview, `SpinButton`, `Scale`, `RangeSlider` |
| Selection    | `ListBox`, `TreeView`, `DropDown`                                  |
| Layout       | `HBox`, `VBox`, `Grid`, `Frame`, `Stack`, `Paned`, `Expander`      |
| Tabs         | `Notebook`                                                         |
| Scroll       | `ScrollView`                                                       |
| Feedback     | `ProgressBar`, `LevelBar`, `Spinner`, `Image`, `Tooltip`, `Notification` |
| Navigation   | `Menu` + `MenuItem.Shortcut`, `MenuBar` + `Alt+letter`, `Dialog`, `MessageDialog` |
| Bars         | `Toolbar`, `Statusbar`, 10 stock `DrawIcon*` helpers               |
| Composite    | `FileChooser`, `ColorChooser`, `Calendar`, `DatePicker`            |
| Charts       | `LineChart`, `BarChart`                                            |
| Theming      | `LoadGTKTheme(css)` (GTK3 + libadwaita @define-color → Theme)      |

### v0.6 breaking change

`Widget.Draw` signature moved from

```go
Draw(surface []byte, surfaceW int, theme *Theme)
```

to

```go
Draw(p painter.Painter, theme *Theme)
```

where [`painter.Painter`](https://github.com/go-widgets/painter) is
a 5-primitive interface (`FillRect`, `StrokeRect`, `PutPixel`,
`Text`, `Size`). Existing callers that had a `[]byte` + width
migrate one line:

```go
// before v0.6:
wg.Draw(surface, w, theme)

// v0.6+:
p := painter.NewPixelPainter(surface, w, h)
wg.Draw(p, theme)
```

The `[]byte` is still writable + owned by the caller — the
`PixelPainter` just wraps it so the primitives translate to writes.
All other widget APIs (`Bounds`, `SetBounds`, `HitTest`, `OnEvent`,
`NewButton`, …) are unchanged.

`toolkit.Rect` + `toolkit.RGBA` became type aliases of
`painter.Rect` + `painter.RGBA`, so cross-package assignments work
transparently.

The 10 `DrawIcon*` helpers also lost their `(surface, surfaceW)`
prefix; new signature is `DrawIconX(p painter.Painter, r Rect, ink RGBA)`.

### Earlier releases

- **v0.9** — 8 widgets: SplitButton (button + attached dropdown arrow),
  IconButton (toolbar-icon variant), Stat (KPI card with trend
  indicator), Timeline (vertical event log), DropZone (dashed file
  drop target), Chip (removable tag), FormField (Label + Child +
  Help/Error), ProgressCircle (approximated circular progress).
- **v0.8** — 12 widgets: Avatar, Skeleton (Text/Avatar/Block kinds),
  Rating, Toast (transient bottom-of-screen), Banner (persistent
  full-width), Popover (Visible container for a Child), ActionRow
  (libadwaita Title/Subtitle/Prefix/Suffix), ViewSwitcher (segmented
  tab picker), ChatBubble (User/Other bubble), SearchEntry (Entry
  with prefix + clear), Diff (Context/Added/Removed colored lines),
  Pagination (prev/numbers/next with disabled ink).
- **v0.7** — 9 widgets: Switch (iOS-style toggle distinct from
  ToggleButton), Badge (auto-sizing pill), Kbd (keyboard-shortcut
  chip), Alert (Info/Success/Warning/Error semantic banner), Card
  (Title/Body/Footer three-zone), Breadcrumbs (chevron path),
  Steps (numbered indicator with connector), HeaderBar (Start/Title/
  Subtitle/End GTK CSD), Table (Columns/Rows/Selected data grid).
- **v0.6** — Painter back-end abstraction. Every widget's `Draw`
  now takes a [`painter.Painter`](https://github.com/go-widgets/painter)
  instead of a fixed `[]byte` + stride pair, so the same widget code
  renders into a pixel buffer (WUI browser canvas, GUI native window,
  image file), a terminal cell grid (TUI), or an SVG stream.
- **v0.5** — Toolbar / Statusbar / FileChooser / ColorChooser /
  Calendar, Selection on TextView, `LoadGTKTheme(css)`, 10 stock
  icon helpers.
- **v0.4** — 34 widgets. Toolbar / Statusbar, FileChooser /
  ColorChooser / Calendar, Selection model on TextView,
  `LoadGTKTheme(css)`.
- **v0.3** — 28 widgets. TextView (multi-line editor), Menu/MenuBar,
  Dialog/MessageDialog, Tooltip, DropDown, TreeView.
- **v0.2** — 22 widgets. Layout containers (HBox/VBox/Grid/Frame),
  scroll (ScrollView/ListBox), input (Entry/Check/Radio/Toggle),
  structural (Stack/Notebook/Paned/Expander), feedback
  (ProgressBar/LevelBar/Scale/SpinButton/Image/Spinner), bitmap font.
- **v0.1** — scaffolding. Widget interface, Theme value, Button +
  Label, primitive event dispatch.

### Next (v1.0 sketch)

Widget coverage is now materially complete versus GTK 4 + DaisyUI 4.
The remaining pre-1.0 work is around the *edges* of the widget
model, not the widget catalogue:

- **Font family plumbing** — the toolkit still ships one 5×7 bitmap
  font. A `Font` interface (`GlyphAdvance`, `GlyphHeight`, `Draw`)
  so a caller can plug a larger bitmap or a hand-rasterised
  TrueType at boot. Unblocks `FontChooser` (long-standing deferral)
  + retina-size Label rendering.
- **First-class drag-and-drop event kinds** — `EventDragStart` /
  `EventDragMove` / `EventDrop`, plus a `DragSource` / `DropTarget`
  interface pair. DropZone (v0.9) currently uses a synthetic-
  `EventChar` seam; formalising the events lets it and future
  siblings share the same host contract.
- **Context menu helper** — one-line `ShowContextMenu(x, y, *Menu,
  *Popover)` that spawns a Menu at worker-relative coords + auto-
  dismisses on outside-click.
- **Overlay layout container** — z-ordered stacking above a
  primary child, so Popover (v0.8) / Toast (v0.8) / Notification /
  Tooltip stack correctly without hosts arranging screen positions.
- **A11y bridge** — `Role` + `Label` fields on widgets already;
  wire an `A11yPublisher` interface hosts can plug (WAI-ARIA
  wrapping on wasm, TTY-cell metadata on tui, ...).

## Architecture

```text
+----------------------+
| Theme                |  Palette + metrics + font ref
+----------------------+
           |
+----------------------+    +-------------------+
| Widget interface     |    | Event             |
|   Draw(rgba, theme)  |<---|   Kind: click/key |
|   HitTest(x, y)      |    |   X,Y / Code      |
|   OnEvent(ev)        |    +-------------------+
+----------------------+
           |
   +-------+-------+-------+-------+
   |               |       |       |
+--+---+        +--+--+  +-+--+  +-+--+
|Button|        |Label|  |HBox|  | ...|
+------+        +-----+  +----+  +----+
```

## License

BSD 3-Clause. See LICENSE.
