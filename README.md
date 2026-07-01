# go-widgets/toolkit

Pure-Go widget toolkit that renders into an RGBA byte buffer. Zero
JS / DOM / canvas dependency — every widget composes pixels into a
caller-supplied `[]byte`, so the toolkit runs identically in
`GOOS=js GOARCH=wasm` (browser SharedArrayBuffer clients), on any
native target Go ships (native canvas backends, image files), or
in headless tests (screenshot-hash regressions).

Originally lived at `github.com/wasmdesk/toolkit` — moved to
`github.com/go-widgets/toolkit` so non-wasmdesk projects can consume
it without pulling in the wasmdesk story. Its first + reference
consumer is still [wasmdesk/wasmbox](https://github.com/wasmdesk/wasmbox)
(compositor + Calculator + Notepad + Showcase clients).

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

- **Not a CodeMirror replacement.** Complex web-grade editors stay
  embedded via wasmbox's `dom_window` protocol (an iframe overlay).
- **Not GTK.** No CSS engine, no SVG renderer, no full BiDi/IME. A
  light primitive set: Button, Label, TextInput, ListBox, ScrollView,
  HBox, VBox, Splitter, TabBar, MenuBar. Around 5-10k LoC.

## Status

**v0.5 — 35 widgets + 10 stock icons, ~8k LoC, 100% statement
coverage.** Pure Go, no CGO, stdlib only. Builds for
`GOOS=js GOARCH=wasm` and every native target Go ships.

| Family       | Widgets                                                            |
| ------------ | ------------------------------------------------------------------ |
| Base         | `Widget`, `Base`, `Rect`, `Event` (+ IME composition), `Theme`, `RGBA` |
| Text         | bitmap 5x7 font (60+ glyphs), `DrawText`, `TextWidth`, `Label`     |
| Action       | `Button`, `ToggleButton`, `CheckButton`, `RadioButton` + `Group`   |
| Input        | `Entry`, `TextView` + `Selection` + IME preview, `SpinButton`, `Scale` |
| Selection    | `ListBox`, `TreeView`, `DropDown`                                  |
| Layout       | `HBox`, `VBox`, `Grid`, `Frame`, `Stack`, `Paned`, `Expander`      |
| Tabs         | `Notebook`                                                         |
| Scroll       | `ScrollView`                                                       |
| Feedback     | `ProgressBar`, `LevelBar`, `Spinner`, `Image`, `Tooltip`, `Notification` |
| Navigation   | `Menu` + `MenuItem.Shortcut`, `MenuBar` + `Alt+letter`, `Dialog`, `MessageDialog` |
| Chrome       | `Toolbar`, `Statusbar`, 10 stock `DrawIcon*` helpers               |
| Composite    | `FileChooser`, `ColorChooser`, `Calendar`                          |
| Theming      | `LoadGTKTheme(css)` (GTK3 + libadwaita @define-color → Theme)      |

### Earlier releases

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

### Next (v0.6 sketch)

- `FontChooser` — deferred from v0.5 until the toolkit gains a
  real "font family" concept (currently one bitmap font only).
- `MenuItem.Shortcut` accelerator dispatch — right now the string
  is a purely visual hint; a `MenuBar.HandleShortcut(code)` helper
  would route the key event to the matching item's Action.
- Drag-and-drop `Event` kinds + a `DragSource` / `DropTarget`
  interface pair (TreeView + ListBox re-ordering as the first
  driver).
- Right-click context menu helper (spawn a Menu at a
  worker-relative coord, dismiss on outside click).

## Architecture

```
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
